package main

// Code From Vansh Jain
// DS Lab 3
// SBU ID - 116713519
import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/csv"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	pb "project1/proto"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var numClients = 3000
var numClusters = 3
var serversPerCluster = 4
var shardSize = numClients / numClusters

var clients = make([]ClientInfo, numClients)
var pendingTxns = make(map[string]int)
var pendingResultTxns = make(map[int64]int)
var clientQueues = make([]map[int]Transaction, numClients)
var queuedTxns = make(map[int32]Transaction)
var queuedTxnsTimer = make(map[int32]*time.Timer)

type ClientInfo struct {
	pb.UnimplementedBankingServiceServer
	client_id int
	cluster   *Cluster
	// cluster_server   []pb.BankingServiceClient
	potential_leader int
	privateKey       *rsa.PrivateKey
}

type Transaction struct {
	timestamp         int64
	sender            int32
	receiver          int32
	amount            float64
	serverStatus      []int64
	contact_servers   []string
	byzantine_servers []int64
}

type Block struct {
	blockId  int
	TxnBlock []Transaction
}

type Cluster struct {
	id        int
	servers   []pb.BankingServiceClient
	serverKey []*rsa.PublicKey
}

func parseCSV(filePath string, num_servers int) ([]Block, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// url := filePath

	// Step 1: Make HTTP GET request
	// file, err := http.Get(url)
	// if err != nil {
	// 	log.Fatalf("Failed to get CSV from URL: %v", err)
	// }
	// defer file.Body.Close()

	// r := csv.NewReader(file.Body)
	r := csv.NewReader(file)
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	// log.Print(records)
	var TransactionOrder []Block
	var BlockTransaction []Transaction
	blockId := 1
	serverStatus_txn := make([]int64, num_servers)
	byzantine_servers := make([]int64, num_servers)
	contact_server := []string{}

	for id, row := range records {
		x := strings.Split(strings.Trim(row[1], "()"), ",")
		amount, err := strconv.ParseFloat(strings.TrimSpace(x[2]), 64)
		if err != nil {
			log.Fatal(err)
		}

		if id == 0 {
			blockId = 1
			for _, i := range strings.Split(strings.Trim(row[2], "[]"), ",") {
				i = strings.TrimSpace(i)
				i, _ := strconv.Atoi(strings.TrimPrefix(i, "S"))
				serverStatus_txn[i-1] = 1
			}
			if len(strings.Trim(row[4], "[]")) > 0 {
				for _, i := range strings.Split(strings.Trim(row[4], "[]"), ",") {
					i = strings.TrimSpace(i)
					i, _ := strconv.Atoi(strings.TrimPrefix(i, "S"))
					byzantine_servers[i-1] = 1
				}
			} else {
				byzantine_servers = make([]int64, num_servers)
			}

			contact_server = strings.Split(strings.Trim(row[3], "[]"), ",")

		}

		if len(row[2]) != 0 && id != 0 {
			blockId += 1
			TransactionOrder = append(TransactionOrder, Block{blockId: blockId - 1, TxnBlock: BlockTransaction})

			BlockTransaction = []Transaction{}
			serverStatus_txn = make([]int64, num_servers)
			for _, i := range strings.Split(strings.Trim(row[2], "[]"), ",") {
				i = strings.TrimSpace(i)
				i, _ := strconv.Atoi(strings.TrimPrefix(i, "S"))
				serverStatus_txn[i-1] = 1
			}
			if len(strings.Trim(row[4], "[]")) > 0 {
				for _, i := range strings.Split(strings.Trim(row[4], "[]"), ",") {
					i = strings.TrimSpace(i)
					i, _ := strconv.Atoi(strings.TrimPrefix(i, "S"))
					byzantine_servers[i-1] = 1
				}
			} else {
				byzantine_servers = make([]int64, num_servers)
			}
			contact_server = strings.Split(strings.Trim(row[3], "[]"), ",")

		}
		copyServerStat := make([]int64, len(serverStatus_txn))
		copy(copyServerStat, serverStatus_txn[:])
		copyByzantineServerStat := make([]int64, len(byzantine_servers))
		copy(copyByzantineServerStat, byzantine_servers[:])
		s, _ := strconv.Atoi(strings.TrimSpace(x[0]))
		r, _ := strconv.Atoi(strings.TrimSpace(x[1]))
		abd := Transaction{sender: int32(s), receiver: int32(r), amount: float64(amount), serverStatus: copyServerStat, contact_servers: contact_server, byzantine_servers: copyByzantineServerStat}
		BlockTransaction = append(BlockTransaction, abd)
	}
	TransactionOrder = append(TransactionOrder, Block{blockId: blockId, TxnBlock: BlockTransaction})
	return TransactionOrder, nil
}

func viewSeqPair(timestamp int64, result bool) string {
	return fmt.Sprintf("%d-%v ", timestamp, result)
}

var mu sync.Mutex

func SignMessage(privateKey *rsa.PrivateKey, message []byte) ([]byte, error) {
	hashed := sha256.Sum256(message)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return nil, fmt.Errorf("could not sign message: %w", err)
	}
	return signature, nil
}

func VerifySignature(publicKey *rsa.PublicKey, message []byte, signature []byte) error {
	hashed := sha256.Sum256(message)
	err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], signature)
	if err != nil {
		return fmt.Errorf("could not verify signature: %w", err)
	}
	return nil
}

func LoadPrivateKey(filePath string) (*rsa.PrivateKey, error) {
	pemData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(pemData)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
	}

	return privateKey, nil
}

func LoadPublicKey(filePath string) (*rsa.PublicKey, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	pemData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(pemData)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing public key")
	}

	publicKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA public key: %w", err)
	}

	publicKey, ok := publicKeyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return publicKey, nil
}
func StopTimerForRequest(client_txn_id int32) {
	if reqTimer, exists := queuedTxnsTimer[client_txn_id]; exists {
		reqTimer.Stop()
		delete(queuedTxnsTimer, client_txn_id)
	}
}

func (c *ClientInfo) TransferMoneyResponse(ctx context.Context, in *pb.Reply) (*pb.Empty, error) {
	mu.Lock()
	defer mu.Unlock()
	// log.Print(in)
	clients[in.ClientId].potential_leader = int(in.ViewLeader)
	if pendingTxns[viewSeqPair(in.Timestamp, in.Result)] < (((serversPerCluster-1)/3)+1) && pendingResultTxns[in.Timestamp] == 1 {
		pendingTxns[viewSeqPair(in.Timestamp, in.Result)] += 1
	}

	if pendingTxns[viewSeqPair(in.Timestamp, in.Result)] == (((serversPerCluster - 1) / 3) + 1) {
		delete(pendingTxns, viewSeqPair(in.Timestamp, in.Result))
		delete(pendingResultTxns, in.Timestamp)
		client_id := in.ClientId
		StopTimerForRequest(int32(in.Timestamp))
		if len(clientQueues[client_id]) > 0 {
			log.Printf("Retrying Txns of Client: %d", in.ClientId)
			temp := []Transaction{}
			for _, txn := range clientQueues[client_id] {
				temp = append(temp, txn)
			}
			clientQueues[client_id] = make(map[int]Transaction)
			executeTransactions(temp, queuedTxns)
		}
		fmt.Printf("Transaction %v: %d->%d: %v \tRESULT: %v\n", in.Timestamp, queuedTxns[int32(in.Timestamp)].sender, queuedTxns[int32(in.Timestamp)].receiver, queuedTxns[int32(in.Timestamp)].amount, in.Result)

		delete(queuedTxns, int32(in.Timestamp))

	}
	return &pb.Empty{}, nil
}

func StartTimerForRequest(transactionId int32) {
	// log.Printf("Server %d: Txn %d: Timer started", client_txn_id, queuedTxns[client_txn_id].timestamp)
	queuedTxnsTimer[transactionId] = time.NewTimer(2 * time.Second)
	go func(timer *time.Timer) {
		for {
			<-timer.C
			log.Printf("Transaction %d: Timer out: Sending to all Servers", transactionId)

			for _, server := range clients[queuedTxns[transactionId].sender].cluster.servers {
				go func(server pb.BankingServiceClient) {
					block := queuedTxns[transactionId]
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
					defer cancel()
					signatureTxn, _ := SignMessage(clients[queuedTxns[transactionId].sender].privateKey, []byte(fmt.Sprintf("%d|%d|%d|%f", block.timestamp, block.sender, block.receiver, block.amount)))
					server.TransferMoney(ctx, &pb.TransferRequest{
						Txn: &pb.Transaction{Sender: block.sender,
							Timestamp: block.timestamp,
							Receiver:  block.receiver,
							Amount:    float32(block.amount)},
						SignatureTxn: signatureTxn,
					})
				}(server)
			}
		}

	}(queuedTxnsTimer[transactionId])

}

func executeTransactions(transaction_block []Transaction, queuedTxns map[int32]Transaction) {
	for _, block := range transaction_block {
		go func(block Transaction) {
			sender_id := int(block.sender)
			// receiver_id := int(block.receiver)
			block.timestamp = int64(time.Now().Nanosecond())

			ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
			defer cancel()

			// if clients[sender_id].cluster.id == clients[receiver_id].cluster.id {
			fmt.Printf("Executing: %d to %d on %v of amount %.2f...\n", block.sender, block.receiver, block.timestamp, block.amount)
			queuedTxns[int32(block.timestamp)] = block
			pendingResultTxns[block.timestamp] = 1
			signatureTxn, _ := SignMessage(clients[sender_id].privateKey, []byte(fmt.Sprintf("%d|%d|%d|%f", block.timestamp, block.sender, block.receiver, block.amount)))
			// StartTimerForRequest(int32(block.timestamp))
			randServer, _ := strconv.Atoi(string(strings.TrimSpace(block.contact_servers[clients[sender_id].cluster.id-1])[1]))

			// log.Print(randServer)
			// log.Print(clients[sender_id].cluster.id)
			clients[sender_id].cluster.servers[(randServer-1)%serversPerCluster].TransferMoney(ctx, &pb.TransferRequest{
				Txn: &pb.Transaction{Sender: block.sender,
					Timestamp: block.timestamp,
					Receiver:  block.receiver,
					Amount:    float32(block.amount)},
				SignatureTxn: signatureTxn,
			})
			// } else {
			// 	var wg sync.WaitGroup
			// 	// re/sult_sender := false
			// 	// result_receiver := false
			// 	fmt.Print("\nCross Shard Transaction\n")
			// 	log.Printf("Initiating transfer from %d to %d on %v of amount %.2f\n", block.sender, block.receiver, block.timestamp, block.amount)

			// 	wg.Add(1)
			// 	go func(sender_id int, sender *Cluster) {
			// 		defer wg.Done()
			// 		// randServer := (int(block.contactServer[sender_id][1]) - 1) % serversPerCluster
			// 		log.Print(clients[sender_id].potential_leader)
			// 		temp_result, _ := clients[sender_id].cluster.servers[clients[sender_id].potential_leader].TransferMoney(ctx, &pb.TransferRequest{
			// 			Txn: &pb.Transaction{Sender: block.sender,
			// 				Timestamp: block.timestamp,
			// 				Receiver:  block.receiver,
			// 				Amount:    float32(block.amount)},
			// 		})
			// 		// log.Printf("Cluster:%d Server:%d", sender.id, randServer)
			// 		// log.Printf("Cluster:%d Server:%d ---Result: %v", sender.id, randServer, temp_result)
			// 		if temp_result != nil {

			// 		}
			// 	}(sender_id, clients[sender_id].cluster)
			// wg.Add(1)
			// go func(receiver_id int, receiver *Cluster) {
			// 	defer wg.Done()
			// 	// randServer := (int(block.contactServer[cluster_id_receiver][1]) - 1) % serversPerCluster
			// 	// log.Print(randServer)
			// 	temp_result, _ := receiver.servers[clients[receiver_id].potential_leader].TransferMoney(ctx, &pb.TransferRequest{
			// 		Txn: &pb.Transaction{Sender: block.sender,
			// 			Timestamp: block.timestamp,
			// 			Receiver:  block.receiver,
			// 			Amount:    float32(block.amount)},
			// 	})
			// 	// log.Printf("Cluster:%d Server:%d", receiver.id, randServer)
			// 	// log.Printf("Cluster:%d Server:%d ---Result: %v", receiver.id, randServer, temp_result)
			// 	if temp_result != nil {

			// 	}
			// }(receiver_id, clients[receiver_id].cluster)
			// wg.Wait()
			// time.Sleep(5 * time.Millisecond)
			// if result_sender && result_receiver {
			// 	log.Print("SENDING COMMIT TO ALL 6 SERVERS")
			// 	go func(sender *Cluster) {
			// 		for _, srv := range sender.servers {
			// 			go func(server pb.BankingServiceClient) {
			// 				ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
			// 				defer cancel()
			// 				reply, _ := server.PCCommit(ctx, &pb.PCCommitMessage{TxnCorresponding: int32(block.timestamp)})
			// 				for reply == nil {
			// 					log.Print("RETRYING COMMIT TO SERVER")
			// 					ctx, cancel := context.WithTimeout(context.Background(), 2000*time.Millisecond)
			// 					defer cancel()
			// 					reply, _ = server.PCCommit(ctx, &pb.PCCommitMessage{TxnCorresponding: int32(block.timestamp)})
			// 				}
			// 				// log.Print(reply)

			// 			}(srv)
			// 		}
			// 	}(clients[sender_id].cluster)
			// 	go func(sender *Cluster) {
			// 		for _, srv := range sender.servers {
			// 			go func(server pb.BankingServiceClient) {
			// 				ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
			// 				defer cancel()
			// 				reply, _ := server.PCCommit(ctx, &pb.PCCommitMessage{TxnCorresponding: int32(block.timestamp)})
			// 				for reply == nil {
			// 					log.Print("RETRYING COMMIT TO SERVER")
			// 					ctx, cancel = context.WithTimeout(context.Background(), 2000*time.Millisecond)
			// 					defer cancel()
			// 					reply, _ = server.PCCommit(ctx, &pb.PCCommitMessage{TxnCorresponding: int32(block.timestamp)})
			// 					time.Sleep(5 * time.Second)
			// 				}
			// 				// log.Printf("Sender Result: %v", reply)
			// 			}(srv)
			// 		}
			// 	}(clients[receiver_id].cluster)

			// } else {
			// 	log.Print("SENDING ABORT TO ALL 6 SERVERS")

			// 	go func(sender *Cluster) {
			// 		for _, srv := range sender.servers {
			// 			go func(server pb.BankingServiceClient) {
			// 				ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
			// 				defer cancel()
			// 				reply, _ := server.PCAbort(ctx, &pb.PCAbortMessage{TxnCorresponding: int32(block.timestamp)})
			// 				for reply == nil {
			// 					log.Print("RETRYING ABORT TO SERVER")
			// 					ctx, cancel = context.WithTimeout(context.Background(), 2000*time.Millisecond)
			// 					defer cancel()
			// 					reply, _ = server.PCAbort(ctx, &pb.PCAbortMessage{TxnCorresponding: int32(block.timestamp)})
			// 				}
			// 				// log.Printf("Receiver Result: %v", reply)
			// 			}(srv)
			// 		}
			// 	}(clients[sender_id].cluster)
			// 	go func(sender *Cluster) {
			// 		for _, srv := range sender.servers {
			// 			go func(server pb.BankingServiceClient) {
			// 				ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
			// 				defer cancel()
			// 				reply, _ := server.PCAbort(ctx, &pb.PCAbortMessage{TxnCorresponding: int32(block.timestamp)})
			// 				for reply == nil {
			// 					log.Print("RETRYING ABORT TO SERVER")
			// 					ctx, cancel = context.WithTimeout(context.Background(), 2000*time.Millisecond)
			// 					defer cancel()
			// 					reply, _ = server.PCAbort(ctx, &pb.PCAbortMessage{TxnCorresponding: int32(block.timestamp)})

			// 				}
			// 				// log.Print(reply)
			// 			}(srv)
			// 		}
			// 	}(clients[receiver_id].cluster)

			// }

			// }

		}(block)
		sender_id := int(block.sender)
		receiver_id := int(block.receiver)
		cluster_id_sender := ((sender_id - 1) * numClusters) / numClients
		cluster_id_receiver := ((receiver_id - 1) * numClusters) / numClients
		time.Sleep(1 * time.Millisecond)
		if cluster_id_receiver != cluster_id_sender {
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func main() {
	// csvFilePath := "/Users/vansh/Desktop/SBU/DS_GIT/2pc-VanshJain02/Test_Cases_-_Lab3.csv"
	csvFilePath := "Lab4_Testset_1.csv"
	flag.Parse()
	clientTransactions, err := parseCSV(csvFilePath, numClusters*serversPerCluster)
	if err != nil {
		log.Fatalf("Failed to parse CSV: %v", err)
	}
	// GenerateRSAKeyPair(2048, fmt.Sprintf("ClientKey/privateKey.pem"), fmt.Sprintf("ClientKey/publicKey.pem"))

	var wg sync.WaitGroup
	doneChan := make(chan bool, numClients)
	clusters := make([]*Cluster, numClusters)

	for i := 0; i < numClusters; i++ {
		clusters[i] = &Cluster{id: i + 1}
		for j := 0; j < serversPerCluster; j++ {
			conn, err := grpc.NewClient(fmt.Sprintf("localhost:500%d", 50+serversPerCluster*i+j+1), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf(" did not connect: %v", err)
			}
			clusters[i].servers = append(clusters[i].servers, pb.NewBankingServiceClient(conn))
			publicKey, _ := LoadPublicKey(fmt.Sprintf("Key/publicKey_C%d_S%d.pem", i+1, serversPerCluster*i+j+1))
			clusters[i].serverKey = append(clusters[i].serverKey, publicKey)
		}
	}
	for i := 0; i < numClients; i++ {
		clientInfo := ClientInfo{
			client_id: i + 1,
		}
		clientInfo.cluster = clusters[i/shardSize]
		clientInfo.potential_leader = 0
		clientInfo.privateKey, _ = LoadPrivateKey("ClientKey/privateKey.pem")
		clients[i] = clientInfo
	}

	i := 0
	for idx := range numClusters {
		go func(addr string) {
			lis, err := net.Listen("tcp", "localhost:"+addr)
			if err != nil {
				log.Fatalf("failed to listen on port %v: %v", addr, err)
			}
			grpcServer := grpc.NewServer()
			client_obj := &ClientInfo{}
			pb.RegisterBankingServiceServer(grpcServer, client_obj)
			log.Printf("gRPC server listening at %v", lis.Addr())
			if err := grpcServer.Serve(lis); err != nil {
				log.Fatalf("failed to serve: %v", err)
			}
		}(fmt.Sprintf("%d", 8000+idx))
	}
	for i < len(clientTransactions)+1 {

		fmt.Println("\nChoose an option:")
		if i < len(clientTransactions) {
			fmt.Println("1. Proceed to a New Block of Transactions")
		}
		fmt.Println("2. Print Balance")
		fmt.Println("3. Print Datastore")
		fmt.Println("4. Print Performance")
		fmt.Println("5. Exit")

		var option int
		fmt.Scan(&option)

		switch option {
		case 1:

			if i < len(clientTransactions) {
				var wg sync.WaitGroup
				for j := 0; j < numClusters; j++ {
					for k := 0; k < serversPerCluster; k++ {
						wg.Add(1)
						go func() {
							defer wg.Done()
							ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
							defer cancel()
							clients[j*shardSize].cluster.servers[k].SyncServerStatus(ctx, &pb.ServerStatus{ServerStatus: clientTransactions[i].TxnBlock[0].serverStatus, ContactServerStatus: clientTransactions[i].TxnBlock[0].contact_servers, ByzantineServerStatus: clientTransactions[i].TxnBlock[0].byzantine_servers})
						}()
					}
				}
				wg.Wait()
				queuedTxns = make(map[int32]Transaction)
				executeTransactions(clientTransactions[i].TxnBlock, queuedTxns)
			}
			i++
		case 2:
			fmt.Printf("Enter the Client ID: \n")
			var ch int
			fmt.Scan(&ch)
			for idx, i := range clients[ch].cluster.servers {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				balance, _ := i.PrintBalance(ctx, &pb.PrintBalanceRequest{ClientId: int32(ch)})
				// performance, _ := i.server[mrand.Intn(len(i.server))].PrintPerformance(ctx, &pb.Empty{})
				fmt.Printf("Server %s Client C%d Balance: %v\n", "S"+strconv.Itoa(3*int(((ch-1)*numClusters)/numClients)+idx+1), ch, balance.Balance)
			}

		case 3:
			for j := 0; j < numClusters; j++ {
				for k := 0; k < serversPerCluster; k++ {
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					defer cancel()
					log, _ := clients[j*shardSize+1].cluster.servers[k].PrintLog(ctx, &pb.Empty{})
					fmt.Printf("\n***** Log of Server S%d *****\n", serversPerCluster*j+k+1)
					for _, txn := range log.Logs {
						fmt.Printf("%v\n", txn)
					}
					fmt.Print("\n")

					// performance, _ := i.server[mrand.Intn(len(i.server))].PrintPerformance(ctx, &pb.Empty{})

				}
			}

		case 4:

			for j := 0; j < numClusters; j++ {
				for k := 0; k < serversPerCluster; k++ {
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					defer cancel()
					performance, _ := clients[j*shardSize+1].cluster.servers[k].PrintPerformance(ctx, &pb.Empty{})
					// performance, _ := i.server[mrand.Intn(len(i.server))].PrintPerformance(ctx, &pb.Empty{})
					if performance.TotalTransactions > 0 {
						avgLatency := time.Duration(performance.TotalLatency) / time.Duration(performance.TotalTransactions)
						fmt.Printf("Server %s Performance:\n", "S"+strconv.Itoa(3*j+k+1))
						fmt.Printf("  Total Transactions: %d\n", performance.TotalTransactions)
						fmt.Printf("  Total Latency: %s\n", time.Duration(performance.TotalLatency))
						fmt.Printf("  Average Latency per Transaction: %s\n", avgLatency)
						fmt.Printf("  Throughput (Txns/sec): %.2f\n\n", float64(performance.TotalTransactions)/time.Duration(performance.TotalLatency).Seconds())
					} else {
						fmt.Printf("The Server %s has 0 transactions, hence cannot show performance metrics\n\n", "S"+strconv.Itoa(3*j+k+1))
					}
				}
			}
		case 5:
			i++
			return
		default:
			fmt.Println("Invalid option. Please try again.")
		}

	}

	wg.Wait()
	close(doneChan)
}

func GenerateRSAKeyPair(bits int, prifileName string, pubfileName string) {
	keyFolder := filepath.Dir(prifileName)
	err := os.MkdirAll(keyFolder, os.ModePerm)
	if err != nil {
		fmt.Println("Error creating folder:", err)
		return
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		fmt.Println("Error generating RSA key pair:", err)
		return
	}
	outFile, err := os.Create(prifileName)
	if err != nil {
		fmt.Println("Error creating PEM file:", err)
		return
	}
	defer outFile.Close()

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	var pemBlock = &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	pem.Encode(outFile, pemBlock)

	pubkeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		fmt.Println("Error marshalling public key:", err)
		return
	}

	var pubpemBlock = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubkeyBytes,
	}

	pubFile, err := os.Create(pubfileName)
	if err != nil {
		fmt.Println("Error creating PEM file:", err)
		return
	}
	defer pubFile.Close()

	pem.Encode(pubFile, pubpemBlock)
}
