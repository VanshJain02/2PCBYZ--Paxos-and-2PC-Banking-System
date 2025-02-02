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
	"database/sql"
	"encoding/hex"
	"encoding/json"
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
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

const (
	initialBalance  = 10
	timeoutDuration = 1 * time.Second
)

type server struct {
	pb.UnimplementedBankingServiceServer
	name              string
	id                int
	cluster_id        int
	ShardDB           *sql.DB
	numClients        int
	numClusters       int
	serversPerCluster int
	isActive          bool
	servers           []*grpc.ClientConn
	mu                sync.Mutex
	commitedTxns      map[int32]*pb.Transaction
	WAL               map[int32]*pb.Transaction

	leaderID          int
	isByzantineFaulty bool

	view            int
	seq_num         int
	low_watermark   int
	high_watermark  int
	private_key     *rsa.PrivateKey
	public_key      map[int]*rsa.PublicKey
	temp_log        map[string]*pb.Transaction
	timeoutDuration time.Duration
	requestTimers   map[int64]*requestTimer
	ledger          map[string]TxnInfo
	vcMsgs          map[int32]*pb.RepeatedViewChangeMessages
	// vcResult          map[int32]bool
	vcTimer         map[int]*requestTimer
	preprepareMsgs  map[int]*pb.PreprepareMessage
	commitMsgs      map[int][]*pb.CommitMessage
	underViewChange bool
	under2PCRPC     bool

	start         time.Time
	total_txns    float64
	total_latency time.Duration
}

type TxnInfo struct {
	txn    *pb.Transaction
	status string
}

type requestTimer struct {
	requestID int64
	timer     *time.Timer
}

type Cluster struct {
	id      int
	servers []*server
}

type execTxn struct {
	TxID    int             `json:"tx_id"`
	Seq_Num int             `json:"sq_num"`
	Txn     *pb.Transaction `json:"txn"`
	Status  string          `json:"status"`
}

func CreateSQLiteDB(shardName string) *sql.DB {
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s.db", shardName))
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS accounts (
		id TEXT PRIMARY KEY,
		balance REAL,
		locked BOOLEAN DEFAULT 0,
		executed_txns TEXT DEFAULT NULL
	)`)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS WALTABLE (
		id REAL PRIMARY KEY,
		WAL TEXT DEFAULT NULL
	)`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	return db
}

func InitializeShardData(db *sql.DB, startID, endID int) {
	for id := startID; id <= endID; id++ {
		_, err := db.Exec("INSERT OR REPLACE INTO accounts (id, balance, locked, executed_txns) VALUES (?, ?, ?, ?)", fmt.Sprintf("C%d", id), initialBalance, false, nil)
		if err != nil {
			log.Printf("Failed to initialize data for ID %d: %v", id, err)
		}
		// _, err = db.Exec("INSERT OR REPLACE INTO accounts (id, locked) VALUES (?, ?)", fmt.Sprintf("C%d", id), 0)
		// if err != nil {
		// 	log.Printf("Failed to initialize locked for ID %d: %v", id, err)
		// }
	}
	_, _ = db.Exec(`DELETE FROM WALTABLE;`)

}

func (s *server) SyncServerStatus(ctx context.Context, in *pb.ServerStatus) (*pb.Empty, error) {
	s.isActive = in.ServerStatus[s.id-1] == 1
	s.isByzantineFaulty = in.ByzantineServerStatus[s.id-1] == 1

	// s.leaderID = int(in.ContactServerStatus[s.cluster_id])
	fmt.Printf("\n******** [%s] Server Status Updated [%v, %v]*******\n", s.name, s.isActive, s.isByzantineFaulty)

	return &pb.Empty{}, nil
}

func (s *server) PrintPerformance(ctx context.Context, in *pb.Empty) (*pb.PrintPerformanceResponse, error) {
	if s.total_txns > 0 {
		avgLatency := s.total_latency / time.Duration(s.total_txns)
		fmt.Printf("Server %s Performance:\n", s.name)
		fmt.Printf("  Total Transactions: %d\n", s.total_latency)
		fmt.Printf("  Total Latency: %s\n", s.total_latency)
		fmt.Printf("  Average Latency per Transaction: %s\n", avgLatency)
		fmt.Printf("  Throughput (Txns/sec): %.2f\n", s.total_txns/s.total_latency.Seconds())
	} else {
		fmt.Printf("The server has 0 transactions, hence cannot show performance metrics\n")
	}
	return &pb.PrintPerformanceResponse{TotalTransactions: int64(s.total_txns), TotalLatency: float32(s.total_latency)}, nil
}

func (s *server) PrintBalance(ctx context.Context, in *pb.PrintBalanceRequest) (*pb.PrintBalanceResponse, error) {
	// var balance float64
	var balance float64
	err := s.ShardDB.QueryRow("SELECT balance FROM accounts WHERE id = ?", fmt.Sprintf("C%d", in.ClientId)).Scan(&balance)
	if err != nil {
		log.Printf("[%s] failed to check balance: %v", s.name, err)
	}
	log.Printf("[%s]: %v", s.name, balance)
	// fmt.Printf("Server %s balance: %f\n", s.name, s.Clientbalance)
	return &pb.PrintBalanceResponse{Balance: int64(balance)}, nil
}
func (s *server) PrintLog(ctx context.Context, in *pb.Empty) (*pb.PrintLogResponse, error) {
	var currentTxnData sql.NullString
	err := s.ShardDB.QueryRow(`SELECT executed_txns FROM accounts WHERE id = (?)`, fmt.Sprintf("C%d", 1000*s.cluster_id+1)).Scan(&currentTxnData)
	if err != nil {
		log.Print("Error found")

		if err == sql.ErrNoRows {
			log.Print("Account with id not found")
		}
	}

	var transactions []execTxn

	// If there are existing transactions, deserialize them
	if currentTxnData.Valid {
		err = json.Unmarshal([]byte(currentTxnData.String), &transactions)
		if err != nil {
		}
	}
	convertedTxns := []*pb.ExecTxn{}
	for _, i := range transactions {
		convertedTxns = append(convertedTxns, &pb.ExecTxn{TxID: int32(i.TxID), SeqNum: int32(i.Seq_Num), Txn: &pb.Transaction{Timestamp: i.Txn.Timestamp, Sender: i.Txn.Sender, Receiver: i.Txn.Receiver, Amount: i.Txn.Amount}, Status: i.Status})
	}
	// fmt.Printf("Client %s log: %v\n", s.name, s.commitedTxns)
	return &pb.PrintLogResponse{Logs: convertedTxns}, nil
}

func (s *server) PCCommit(ctx context.Context, in *pb.CsCommitMsg) (*pb.PCResponse, error) {
	if !s.isActive {
		return nil, nil
	}
	// s.mu.Lock()
	// defer s.mu.Unlock()
	if s.leaderID == s.id {
		log.Printf("[%s] Initiating 2PC Commit Phase: %v", s.name, in.ClientReq.Txn.Timestamp)
		s.under2PCRPC = true

		if _, exist := s.WAL[int32(in.ClientReq.Txn.Timestamp)]; exist {
			s.StartLPBFT(in.ClientReq, 1, "C")
		}
	}
	// if _, exist := s.WAL[in.TxnCorresponding]; exist {
	// 	start := time.Now()
	// 	// name := fmt.Sprintf("C%d", in.TxnCorresponding)
	// 	// log.Printf("----- [%s] COMMITTED %v -----", s.name, in.TxnCorresponding)

	// 	WAL := s.fetchWAL(int64(in.TxnCorresponding))
	// 	log.Printf("----- [%s] COMMITTED (txn: %d->%d : %v)-----", s.name, WAL.Sender, WAL.Receiver, WAL.Amount)

	// 	x := fmt.Sprintf("C%d", int(WAL.Sender))
	// 	y := fmt.Sprintf("C%d", int(WAL.Receiver))
	// 	sender_id := int(WAL.Sender - 1)
	// 	receiver_id := int(WAL.Receiver - 1)
	// 	cluster_id_sender := (sender_id * s.numClusters) / s.numClients
	// 	cluster_id_receiver := (receiver_id * s.numClusters) / s.numClients
	// 	if cluster_id_sender == s.cluster_id {
	// 		s.AppendExecutedTxn(x, execTxn{TxID: int(WAL.Timestamp), Txn: WAL, Status: "C"})

	// 	}
	// 	if cluster_id_receiver == s.cluster_id {
	// 		s.AppendExecutedTxn(y, execTxn{TxID: int(WAL.Timestamp), Txn: WAL, Status: "C"})

	// 	}

	// 	s.commitedTxns[int32(WAL.Timestamp)] = WAL
	// 	s.deleteFromWAL(int64(in.TxnCorresponding))
	// 	delete(s.WAL, in.TxnCorresponding)
	// 	_, err := s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
	// 	if err != nil {
	// 		log.Printf("failed to release locks: %v", err)
	// 	}
	// 	s.total_latency += time.Since(start)
	// }
	return &pb.PCResponse{Ack: true}, nil
}
func (s *server) PCAbort(ctx context.Context, in *pb.CsCommitMsg) (*pb.PCResponse, error) {
	// s.mu.Lock()
	if !s.isActive {
		return nil, nil
	}
	// defer s.mu.Unlock()

	if s.leaderID == s.id {
		log.Printf("[%s] Initiating 2PC Abort Phase: %v", s.name, in.ClientReq.Txn.Timestamp)
		s.under2PCRPC = true
		if _, exist := s.WAL[int32(in.ClientReq.Txn.Timestamp)]; exist {
			s.StartLPBFT(in.ClientReq, 1, "A")
		}
	}
	// start := time.Now()
	// // name := fmt.Sprintf("C%d", in.TxnCorresponding)

	// if _, exist := s.WAL[in.TxnCorresponding]; exist {
	// 	WAL := s.fetchWAL(int64(in.TxnCorresponding))
	// 	log.Printf("----- [%s] TIME TO ABORT %v-----", s.name, in.TxnCorresponding)

	// 	// log.Printf("[%s] (txn %v: %d->%d : %v)", s.name, in.TxnCorresponding, WAL.Sender, WAL.Receiver, WAL.Amount)

	// 	x := fmt.Sprintf("C%d", int(WAL.Sender))
	// 	y := fmt.Sprintf("C%d", int(WAL.Receiver))
	// 	amt := WAL.Amount

	// 	_, err := s.ShardDB.Exec(`
	//     UPDATE accounts
	//     SET balance = CASE
	//         WHEN id = ? THEN balance + ?
	//         WHEN id = ? THEN balance - ?
	//     END
	//     WHERE id IN (?, ?)`,
	// 		x, amt, y, amt, x, y)
	// 	if err != nil {
	// 		s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y) // Release locks
	// 		log.Printf("failed to perform transfer: %v", err)
	// 	}
	// 	_, err = s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
	// 	if err != nil {
	// 		log.Printf("failed to release locks: %v", err)
	// 	} else {
	// 		// log.Printf("Released Locks for %d  %v", err)

	// 	}
	// 	s.deleteFromWAL(int64(in.TxnCorresponding))
	// 	delete(s.WAL, in.TxnCorresponding)
	// }
	// s.total_latency += time.Since(start)
	return &pb.PCResponse{Ack: true}, nil
}

func (s *server) CrossShardCommit(ctx context.Context, in *pb.CsCommitMsg) (*pb.Empty, error) {
	if !s.isActive {
		return nil, nil
	}
	// s.mu.Lock()
	// defer s.mu.Unlock()
	// log.Printf("[%s] Received Prepared from participant Leader ", s.name)
	s.commitMsgs[int(in.ClientReq.Txn.Timestamp)] = in.CommitMsg
	if s.leaderID == s.id {
		receiver_id := int(in.ClientReq.Txn.Receiver - 1)
		cluster_id_receiver := (receiver_id * s.numClusters) / s.numClients
		for i := range s.serversPerCluster {
			go func(i int, result bool) {
				conn, err := grpc.NewClient(fmt.Sprintf("localhost:500%d", 51+s.serversPerCluster*cluster_id_receiver+i), grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					log.Fatalf(" did not connect: %v", err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				if result {
					reply, _ := pb.NewBankingServiceClient(conn).PCCommit(ctx, in)
					for reply == nil {
						log.Print("RETRYING COMMIT TO SERVER")
						ctx, cancel := context.WithTimeout(context.Background(), 2000*time.Millisecond)
						defer cancel()
						reply, _ = pb.NewBankingServiceClient(conn).PCCommit(ctx, in)
					}
				} else {
					reply, _ := pb.NewBankingServiceClient(conn).PCAbort(ctx, in)
					for reply == nil {
						log.Print("RETRYING ABORT TO SERVER")
						ctx, cancel := context.WithTimeout(context.Background(), 2000*time.Millisecond)
						defer cancel()
						reply, _ = pb.NewBankingServiceClient(conn).PCAbort(ctx, in)
					}
				}
			}(i, in.ConsensusResult)
		}
		log.Printf("[%s] Initiating 2PC Commit/Abort Phase: %v", s.name, in.ClientReq.Txn.Timestamp)
		if _, exist := s.WAL[int32(in.ClientReq.Txn.Timestamp)]; exist {
			if in.ConsensusResult {
				s.under2PCRPC = true
				s.StartLPBFT(in.ClientReq, 1, "C")

			} else {
				s.under2PCRPC = true
				s.StartLPBFT(in.ClientReq, 1, "A")
			}
		}
	}

	return nil, nil
}

func (s *server) CrossShardPrepare(ctx context.Context, in *pb.CsPrepareMsg) (*pb.Empty, error) {
	if !s.isActive {
		return nil, nil
	}
	s.under2PCRPC = false

	// s.mu.Lock()
	// defer s.mu.Unlock()
	log.Printf("[%s] Received Prepare for coordinator ", s.name)
	if s.leaderID == s.id {
		var lockChk float64
		s.ShardDB.QueryRow("SELECT locked FROM accounts WHERE id = ?", fmt.Sprintf("C%d", in.ClientReq.Txn.Receiver)).Scan(&lockChk)
		if lockChk == 0 {
			s.mu.Lock()
			result := s.StartLPBFT(in.ClientReq, 1, "P")
			s.mu.Unlock()
			log.Print(result)
			sender_id := int(in.ClientReq.Txn.Sender - 1)
			cluster_id_sender := (sender_id * s.numClusters) / s.numClients
			log.Printf("[%s] Sending to participant cluster concensus to Coordinator Cluster: %v", s.name, result)
			for i := range s.serversPerCluster {
				go func(i int) {
					conn, err := grpc.NewClient(fmt.Sprintf("localhost:500%d", 51+s.serversPerCluster*cluster_id_sender+i), grpc.WithTransportCredentials(insecure.NewCredentials()))
					if err != nil {
						log.Fatalf(" did not connect: %v", err)
					}
					ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
					defer cancel()
					pb.NewBankingServiceClient(conn).CrossShardCommit(ctx, &pb.CsCommitMsg{Status: "PREPARED", ClientReq: in.ClientReq, CommitMsg: s.commitMsgs[int(in.ClientReq.Txn.Timestamp)], ConsensusResult: result})
				}(i)
			}
			go func() {

				for s.under2PCRPC == false {
					timer := time.NewTimer(1 * time.Second)
					<-timer.C
					if s.under2PCRPC == false {
						log.Printf("[%s] Retrying To Coordinator Cluster %v", s.name, in.ClientReq.Txn.Timestamp)
						for i := range s.serversPerCluster {
							go func(i int) {
								conn, err := grpc.NewClient(fmt.Sprintf("localhost:500%d", 51+s.serversPerCluster*cluster_id_sender+i), grpc.WithTransportCredentials(insecure.NewCredentials()))
								if err != nil {
									log.Fatalf(" did not connect: %v", err)
								}
								ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
								defer cancel()
								pb.NewBankingServiceClient(conn).CrossShardCommit(ctx, &pb.CsCommitMsg{Status: "PREPARED", ClientReq: in.ClientReq, CommitMsg: s.commitMsgs[int(in.ClientReq.Txn.Timestamp)], ConsensusResult: result})
							}(i)
						}
					} else {
						return
					}
				}
			}()

		}
	}

	return nil, nil
}

func (s *server) TransferMoney(ctx context.Context, in *pb.TransferRequest) (*pb.Empty, error) {
	if !s.isActive {
		for {
			time.Sleep(1 * time.Second)
		}
	}

	sender_id := int(in.Txn.Sender - 1)
	receiver_id := int(in.Txn.Receiver - 1)
	cluster_id_sender := (sender_id * s.numClusters) / s.numClients
	cluster_id_receiver := (receiver_id * s.numClusters) / s.numClients
	if cluster_id_sender == cluster_id_receiver {
		s.mu.Lock()
		// fmt.Print("\nIntra Shard Transaction\n")
		s.start = time.Now()
		s.StartLPBFT(in, 2, "")
		s.total_latency += time.Since(s.start)
		s.mu.Unlock()
		return nil, nil

	} else {
		// fmt.Print("Cross Shard Transaction\n")
		if s.cluster_id == cluster_id_sender {
			s.mu.Lock()
			s.start = time.Now()
			result := s.StartLPBFT(in, 1, "P")
			if result {
				for i := range s.serversPerCluster {
					go func(i int) {
						// log.Print(fmt.Sprintf("localhost:500%d", 51+s.serversPerCluster*cluster_id_receiver+i))
						conn, err := grpc.NewClient(fmt.Sprintf("localhost:500%d", 51+s.serversPerCluster*cluster_id_receiver+i), grpc.WithTransportCredentials(insecure.NewCredentials()))
						if err != nil {
							log.Fatalf(" did not connect: %v", err)
						}
						ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
						defer cancel()
						pb.NewBankingServiceClient(conn).CrossShardPrepare(ctx, &pb.CsPrepareMsg{Status: "PREPARE", ClientReq: in, CommitMsg: s.commitMsgs[int(in.Txn.Timestamp)]})
					}(i)
				}
				go func() {
					timer := time.NewTimer(1 * time.Second)
					<-timer.C
					if _, exist := s.commitMsgs[int(in.Txn.Timestamp)]; !exist {
						log.Printf("[%s] Aborting the Message Phase as no response from Participant cluster: %v", s.name, in.Txn.Timestamp)
						if _, exist := s.WAL[int32(in.Txn.Timestamp)]; exist {
							s.StartLPBFT(in, 1, "A")
						}
					}
				}()
			}
			s.total_latency += time.Since(s.start)
			s.mu.Unlock()

		}
		return nil, nil

	}

}
func hashTransaction(tx *pb.Transaction) string {
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%d:%d:%d:%f", tx.Timestamp, tx.Sender, tx.Receiver, tx.Amount)))
	return hex.EncodeToString(hasher.Sum(nil))
}

func viewSeqPair(view int, seq_num int) string {
	return fmt.Sprintf("%d-%d", view, seq_num)
}

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

func (s *server) StartLPBFT(in *pb.TransferRequest, min_count int, txnType string) bool {
	ch := make(chan bool)
	if s.leaderID == s.id {
		s.seq_num++
		go func(seq_num int, in *pb.TransferRequest, min_count int, txnType string) {
			log.Printf("[%s] STARTING LPBFT %d->%d : %v", s.name, in.Txn.Sender, in.Txn.Receiver, in.Txn.Amount)
			digest := hashTransaction(in.Txn)
			if _, exist := s.ledger[digest]; exist && !(txnType == "C" || txnType == "A") {
				ch <- false
				s.seq_num--
				return
			}

			sender_id := int(in.Txn.Sender - 1)
			receiver_id := int(in.Txn.Receiver - 1)
			cluster_id_sender := (sender_id * s.numClusters) / s.numClients

			x := fmt.Sprintf("C%d", sender_id+1)
			y := fmt.Sprintf("C%d", receiver_id+1)
			if cluster_id_sender == s.cluster_id && !(txnType == "C" || txnType == "A") {
				var balance float64
				err := s.ShardDB.QueryRow("SELECT balance FROM accounts WHERE id = ?", fmt.Sprintf("C%d", sender_id+1)).Scan(&balance)
				if err != nil {
					s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
					log.Printf("[%s] failed to check balance: %v", s.name, err)
				}
				if balance < float64(in.Txn.Amount) {
					s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
					log.Printf("[%s] [ERROR] Insufficient balance for txn: %d", s.name, in.Txn.Timestamp)
					ch <- false
					s.seq_num--
					return
				}
			}
			if !(txnType == "C" || txnType == "A") {
				res, _ := s.ShardDB.Exec("UPDATE accounts SET locked = 1 WHERE id IN (?, ?) AND locked = 0", x, y)
				rowsAffected, _ := res.RowsAffected()
				// log.Printf("[%s] Locks Made: %d", s.name, rowsAffected)
				if rowsAffected < int64(min_count) {
					log.Printf("[%s] [WARNING] Waiting for 100 ms for lock to unlock", s.name)
					time.Sleep(400 * time.Millisecond)
					temp := rowsAffected
					res, _ := s.ShardDB.Exec("UPDATE accounts SET locked = 1 WHERE id IN (?, ?) AND locked = 0", x, y)
					rowsAffected, _ := res.RowsAffected()
					if temp+rowsAffected < int64(min_count) {
						s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
						log.Printf("[%s] [ERROR] Could not acquire sufficient locks for txn: %d", s.name, in.Txn.Timestamp)
						ch <- false
						s.seq_num--
						return
					} else {
						log.Printf("[%s] [SUCCESS] Locks are now unlocked", s.name)

					}
				}
			}

			s.ledger[digest] = TxnInfo{txn: in.Txn, status: "PP"}

			preprepareMsg := pb.PreprepareMessage{Ack: true, View: int32(s.view), SeqNum: int32(seq_num), Digest: digest, Txn: in, RequestType: txnType}

			quorum := 2 * ((s.serversPerCluster - 1) / 3)
			var prepareMsgs []*pb.PrepareMessage
			var wg sync.WaitGroup
			wg.Add((s.serversPerCluster - 1))
			for _, srv := range s.servers {
				if srv == nil {
					s.temp_log[viewSeqPair(int(preprepareMsg.View), int(preprepareMsg.SeqNum))] = preprepareMsg.Txn.Txn
					s.temp_log[viewSeqPair(int(preprepareMsg.View), int(preprepareMsg.SeqNum))].Result = false
					s.preprepareMsgs[seq_num] = &preprepareMsg
					continue
				}
				go func(srv *grpc.ClientConn) {
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second) // Timeout for gRPC call
					defer cancel()
					prepareMsg, _ := pb.NewBankingServiceClient(srv).PrePrepare(ctx, &preprepareMsg)
					defer wg.Done()
					// log.Print(prepareMsg)
					if prepareMsg.Ack {
						if VerifySignature(s.public_key[int(prepareMsg.SenderId)], []byte(fmt.Sprintf("%d|%d|%s|%d", preprepareMsg.View, preprepareMsg.SeqNum, preprepareMsg.Digest, prepareMsg.SenderId)), []byte(prepareMsg.Signature)) == nil {
							prepareMsgs = append(prepareMsgs, prepareMsg)
						} else {
							log.Printf("Couldnt Verify Prepare Signature of Server %s", "S"+strconv.Itoa(int(prepareMsg.SenderId+1)))
						}
					}

				}(srv)
			}
			wg.Wait()

			if s.isByzantineFaulty {
				fmt.Print("Leader Byzantine - Waiting To Trigger Timer\n")
				time.Sleep(1 * time.Second)
				return
			}
			certificate := make(map[int32][]byte)
			var commitMsgs []*pb.CommitMessage
			prepareResponse := pb.PrepareResponse{Ack: true, View: int32(preprepareMsg.View), SeqNum: int32(preprepareMsg.SeqNum), Digest: digest, Certificate: certificate}

			for _, pm := range prepareMsgs {
				certificate[int32(pm.SenderId)] = pm.Signature
			}
			s.total_txns++
			if len(prepareMsgs) < quorum {
				log.Printf("[%s] [ERROR] Prepare Request: Quorum FAILED - %v\n", s.name, seq_num)
				ch <- false
				return
			} else {
				log.Printf("[%s] Prepare Request: Quorum Formed:- %v\n", s.name, seq_num)
				time.Sleep(5 * time.Millisecond)
				for _, pm := range prepareMsgs {
					certificate[int32(pm.SenderId)] = pm.Signature
				}
				prepareResponse.Certificate = certificate
				// if len(prepareMsgs) == len(s.servers) {
				// 	fmt.Print("Skipping Commit Phase(2 Linear Phase) - BONUS 3\n")
				// 	logEntry := s.logMessages[int(sq_num)]
				// 	logEntry.PrepareCertificate = prepareResponse.Certificate
				// 	s.logMessages[int(s.pendingPreprepare[int(sq_num)].SeqNum)] = logEntry
				// 	goto sendClientReply
				// }

			}
			var wg1 sync.WaitGroup

			wg1.Add(s.serversPerCluster - 1)
			for _, srv := range s.servers {
				if srv == nil {
					commitMsg := pb.CommitMessage{Ack: true, View: prepareResponse.View, SeqNum: prepareResponse.SeqNum, Digest: prepareResponse.Digest, SenderId: int32(s.id), Signature: []byte{}}
					signature, _ := SignMessage(s.private_key, []byte(fmt.Sprintf("%d|%d|%s|%d", commitMsg.View, commitMsg.SeqNum, commitMsg.Digest, commitMsg.SenderId)))
					commitMsg.Signature = signature
					commitMsgs = append(commitMsgs, &commitMsg)
					continue
				}
				go func(prepareResponse *pb.PrepareResponse, srv *grpc.ClientConn) {
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					defer cancel()
					commitMsg, _ := pb.NewBankingServiceClient(srv).Prepare(ctx, prepareResponse)
					defer wg1.Done()
					if commitMsg.Ack {
						if VerifySignature(s.public_key[int(commitMsg.SenderId)], []byte(fmt.Sprintf("%d|%d|%s|%d", prepareResponse.View, prepareResponse.SeqNum, prepareResponse.Digest, commitMsg.SenderId)), []byte(commitMsg.Signature)) == nil {
							commitMsgs = append(commitMsgs, commitMsg)
							// if len(commitMsgs) == quorum {
							// 	wg.Done()
							// }
						} else {
							log.Printf("[%s] Couldnt Verify Commit Signature of Server %s", s.name, "S"+strconv.Itoa(int(commitMsg.SenderId+1)))
						}
					}
				}(&prepareResponse, srv)
			}
			wg1.Wait()
			time.Sleep(2 * time.Millisecond)

			if len(commitMsgs) < quorum+1 {
				log.Printf("[%s] [ERROR] Commit Request: Quorum FAILED", s.name)
				ch <- false
				return
			} else {
				log.Printf("[%s] Commit Request: Quorum Formed\n", s.name)
			}
			certificate = make(map[int32][]byte)
			for _, pm := range commitMsgs {
				certificate[int32(pm.SenderId)] = pm.Signature
			}
			s.commitMsgs[int(in.Txn.Timestamp)] = commitMsgs
			commitResponse := pb.CommitResponse{Ack: true, View: int32(prepareResponse.View), SeqNum: int32(prepareResponse.SeqNum), Digest: digest, Certificate: certificate}
			for _, srv := range s.servers {
				if srv == nil {
					continue
				}
				go func(srv *grpc.ClientConn) {
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second) // Timeout for gRPC call
					defer cancel()
					pb.NewBankingServiceClient(srv).Commit(ctx, &commitResponse)
				}(srv)
			}

			log.Printf("[%s] Commiting Transactions......   %d->%d : %v", s.name, in.Txn.Sender, in.Txn.Receiver, in.Txn.Amount)
			x = fmt.Sprintf("C%d", in.Txn.Sender)
			y = fmt.Sprintf("C%d", in.Txn.Receiver)
			amt := in.Txn.Amount
			_, err := s.ShardDB.Exec(`
        UPDATE accounts
        SET balance = CASE
            WHEN id = ? THEN balance - ?
            WHEN id = ? THEN balance + ?
        END
        WHERE id IN (?, ?)`,
				x, amt, y, amt, x, y)

			sender_id = int(in.Txn.Sender - 1)
			receiver_id = int(in.Txn.Receiver - 1)
			cluster_id_sender = (sender_id * s.numClusters) / s.numClients
			cluster_id_receiver := (receiver_id * s.numClusters) / s.numClients
			if cluster_id_receiver != cluster_id_sender {
				if !(s.preprepareMsgs[int(commitResponse.SeqNum)].RequestType == "C" || s.preprepareMsgs[int(commitResponse.SeqNum)].RequestType == "A") {
					if cluster_id_sender == s.cluster_id {
						s.WAL[int32(in.Txn.Timestamp)] = in.Txn
						s.AppendExecutedTxn(x, execTxn{TxID: int(in.Txn.Timestamp), Seq_Num: int(commitResponse.SeqNum), Txn: in.Txn, Status: "P"})
						s.insertIntoWAL(in.Txn.Timestamp, in.Txn)

					}
					if cluster_id_receiver == s.cluster_id {
						s.WAL[int32(in.Txn.Timestamp)] = in.Txn
						s.AppendExecutedTxn(y, execTxn{TxID: int(in.Txn.Timestamp), Seq_Num: int(commitResponse.SeqNum), Txn: in.Txn, Status: "P"})
						s.insertIntoWAL(in.Txn.Timestamp, in.Txn)
					}
				} else {
					txn := s.temp_log[viewSeqPair(int(commitResponse.View), int(commitResponse.SeqNum))]
					if s.preprepareMsgs[int(commitResponse.SeqNum)].RequestType == "C" {
						if _, exist := s.WAL[int32(txn.Timestamp)]; exist {
							// name := fmt.Sprintf("C%d", in.TxnCorresponding)
							// log.Printf("----- [%s] COMMITTED %v -----", s.name, in.TxnCorresponding)

							WAL := s.fetchWAL(int64(txn.Timestamp))
							log.Printf("--- [%s] COMMITTED (txn: %d->%d : %v) ---", s.name, WAL.Sender, WAL.Receiver, WAL.Amount)

							x := fmt.Sprintf("C%d", int(WAL.Sender))
							y := fmt.Sprintf("C%d", int(WAL.Receiver))
							sender_id := int(WAL.Sender - 1)
							receiver_id := int(WAL.Receiver - 1)
							cluster_id_sender := (sender_id * s.numClusters) / s.numClients
							cluster_id_receiver := (receiver_id * s.numClusters) / s.numClients
							if cluster_id_sender == s.cluster_id {
								s.AppendExecutedTxn(x, execTxn{TxID: int(WAL.Timestamp), Seq_Num: int(commitResponse.SeqNum), Txn: WAL, Status: "C"})

							}
							if cluster_id_receiver == s.cluster_id {
								s.AppendExecutedTxn(y, execTxn{TxID: int(WAL.Timestamp), Seq_Num: int(commitResponse.SeqNum), Txn: WAL, Status: "C"})
							}
							s.commitedTxns[int32(WAL.Timestamp)] = WAL
							s.deleteFromWAL(int64(txn.Timestamp))
							delete(s.WAL, int32(txn.Timestamp))
							_, err := s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
							if err != nil {
								log.Printf("failed to release locks: %v", err)
							}
							if cluster_id_sender == s.cluster_id {
								reply := pb.Reply{View: int32(s.view), ViewLeader: int32(s.leaderID), Timestamp: txn.Timestamp, ClientId: txn.Sender, ServerId: int32(s.id), Result: true}
								conn, _ := grpc.NewClient(fmt.Sprintf("localhost:%d", 8000+s.cluster_id), grpc.WithTransportCredentials(insecure.NewCredentials()))
								ctx, cancel := context.WithTimeout(context.Background(), time.Second)
								defer cancel()
								pb.NewBankingServiceClient(conn).TransferMoneyResponse(ctx, &reply)
							}
						}
					} else if s.preprepareMsgs[int(commitResponse.SeqNum)].RequestType == "A" {
						if _, exist := s.WAL[int32(txn.Timestamp)]; exist {
							WAL := s.fetchWAL(int64(txn.Timestamp))
							log.Printf("--- [%s] ABORT (txn: %d->%d : %v) ---", s.name, WAL.Sender, WAL.Receiver, WAL.Amount)
							x := fmt.Sprintf("C%d", int(WAL.Sender))
							y := fmt.Sprintf("C%d", int(WAL.Receiver))
							amt := WAL.Amount
							_, err := s.ShardDB.Exec(`
						UPDATE accounts
						SET balance = CASE
							WHEN id = ? THEN balance + ?
							WHEN id = ? THEN balance - ?
						END
						WHERE id IN (?, ?)`,
								x, amt, y, amt, x, y)
							if err != nil {
								s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y) // Release locks
								log.Printf("failed to perform transfer: %v", err)
							}
							_, err = s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
							if err != nil {
								log.Printf("failed to release locks: %v", err)
							}
							s.deleteFromWAL(int64(txn.Timestamp))
							delete(s.WAL, int32(txn.Timestamp))
							if cluster_id_sender == s.cluster_id {
								s.AppendExecutedTxn(x, execTxn{TxID: int(txn.Timestamp), Seq_Num: int(commitResponse.SeqNum), Txn: txn, Status: "A"})
								reply := pb.Reply{View: int32(s.view), ViewLeader: int32(s.leaderID), Timestamp: txn.Timestamp, ClientId: txn.Sender, ServerId: int32(s.id), Result: false}
								conn, _ := grpc.NewClient(fmt.Sprintf("localhost:%d", 8000+s.cluster_id), grpc.WithTransportCredentials(insecure.NewCredentials()))
								ctx, cancel := context.WithTimeout(context.Background(), time.Second)
								defer cancel()
								pb.NewBankingServiceClient(conn).TransferMoneyResponse(ctx, &reply)
							} else {
								s.AppendExecutedTxn(y, execTxn{TxID: int(txn.Timestamp), Seq_Num: int(commitResponse.SeqNum), Txn: txn, Status: "A"})

							}
						}

					}
				}
			} else {
				s.AppendExecutedTxn(x, execTxn{TxID: int(in.Txn.Timestamp), Seq_Num: int(commitResponse.SeqNum), Txn: in.Txn, Status: ""})
				s.commitedTxns[int32(in.Txn.Timestamp)] = in.Txn

				if err != nil {
					s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y) // Release locks
					log.Printf("failed to perform transfer: %v", err)
				}
				_, err = s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
				if err != nil {
					log.Printf("[%s] failed to release locks: %v", s.name, err)
				}

				// fmt.Printf("[%s] Replying To Client: %d    [%v]\n", s.name, s.temp_log[viewSeqPair(int(commitResponse.View), int(commitResponse.SeqNum))].Sender, fmt.Sprintf("localhost:%d", 8000+s.cluster_id))

				reply := pb.Reply{View: commitResponse.View, ViewLeader: int32(s.leaderID), Timestamp: s.temp_log[viewSeqPair(int(commitResponse.View), int(commitResponse.SeqNum))].Timestamp, ClientId: s.temp_log[viewSeqPair(int(commitResponse.View), int(commitResponse.SeqNum))].Sender, ServerId: int32(s.id), Result: s.temp_log[viewSeqPair(int(commitResponse.View), int(commitResponse.SeqNum))].Result}
				conn, _ := grpc.NewClient(fmt.Sprintf("localhost:%d", 8000+s.cluster_id), grpc.WithTransportCredentials(insecure.NewCredentials()))

				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				pb.NewBankingServiceClient(conn).TransferMoneyResponse(ctx, &reply)
			}

			s.temp_log[viewSeqPair(int(commitResponse.View), int(commitResponse.SeqNum))] = nil

			ch <- true
		}(s.seq_num, in, min_count, txnType)

		result := <-ch
		return result
	} else {
		log.Printf("[%s]Forwarding Txn To Leader", s.name)
		// s.StartTimerForRequest(in.Txn.Timestamp, timeoutDuration)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pb.NewBankingServiceClient(s.servers[s.leaderID-s.serversPerCluster*s.cluster_id-1]).TransferMoney(ctx, in)
	}
	result := <-ch
	return result

}

func (s *server) StartTimerForRequest(requestID int64, timeout time.Duration) {
	if _, exists := s.requestTimers[requestID]; !exists {
		log.Printf("[%s]: Timer started for request %d", s.name, requestID)
		timer := time.NewTimer(timeout)
		reqTimer := requestTimer{requestID: requestID, timer: timer}
		s.requestTimers[requestID] = &reqTimer
	} else {
		log.Printf("[%s]: Timer Already Exists for Id: %d", s.name, requestID)
		s.requestTimers[requestID].timer.Reset(s.timeoutDuration)
	}

	go func() {
		for {
			<-s.requestTimers[requestID].timer.C
			if !s.underViewChange {
				log.Printf("[%s] Timer expired for request %d, initiating view change", s.name, requestID)

				// sender_id:= s.temp_log[viewSeqPair(s.view,s.seq_num)].Sender-1
				// receiver_id := s.temp_log[viewSeqPair(s.view,s.seq_num)].Receiver-1
				// x := fmt.Sprintf("C%d", sender_id+1)
				// y := fmt.Sprintf("C%d", receiver_id+1)
				//  s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)

				// // s.ViewChange(s.view + 1)
				break
			} else {
				s.requestTimers[requestID].timer.Reset(s.timeoutDuration)
			}
		}
	}()
}

func (s *server) StopTimerForRequest(requestID int64) {
	if reqTimer, exists := s.requestTimers[requestID]; exists {
		reqTimer.timer.Stop()
		delete(s.requestTimers, requestID)
		log.Printf("[%s] Timer stopped for request %d", s.name, requestID)
	}
}

func (s *server) startViewChangeTimer(view int, timeout time.Duration) {

	if _, exists := s.vcTimer[view]; !exists {
		// log.Printf("[%s]:View Change Timer started for view: %d", s.name, view)
		timer := time.NewTimer(timeout)
		reqTimer := requestTimer{requestID: int64(view), timer: timer}
		s.vcTimer[view] = &reqTimer
		go func() {
			for {
				if _, exists := s.vcTimer[int(view)]; exists {
					<-s.vcTimer[view].timer.C
					log.Printf("[%s] View Change Timer expired: retrying view change: %d", s.name, view+1)
					s.ViewChange(view + 1)
					break
				}
			}
		}()
	} else {
		log.Printf("[%s]: Timer Already Exists for Id: %d", s.name, view)
	}

}

func (s *server) stopViewChangeTimer(view int) {
	if reqTimer, exists := s.vcTimer[int(view)]; exists {
		// log.Printf("[%s] View Change Timer stopped for request %d", s.name, view)
		reqTimer.timer.Stop()
		delete(s.vcTimer, view)
	}
}

func (s *server) ViewChangeMulticast(ctx context.Context, in *pb.ViewChangeMessage) (*pb.AckMessage, error) {
	if !s.isActive {
		return &pb.AckMessage{Success: false}, nil
	}
	log.Printf("[%s] Recieved View Change: %d Request from Server: %d\n", s.name, in.View, in.SenderId)
	if _, exist := s.vcMsgs[int32(in.View)]; !exist {
		s.vcMsgs[int32(in.View)] = &pb.RepeatedViewChangeMessages{}
	}
	s.vcMsgs[int32(in.View)].ViewchangeMsgs = append(s.vcMsgs[int32(in.View)].ViewchangeMsgs, in)
	if len(s.vcMsgs[in.View].ViewchangeMsgs) > (s.serversPerCluster-1)/3 && !s.underViewChange {
		if s.view != int(in.View) {
			s.ViewChange(int(in.View))
		}
	}

	if len(s.vcMsgs[in.View].ViewchangeMsgs) == 2*(s.serversPerCluster-1)/3 && s.underViewChange {
		s.startViewChangeTimer(int(in.View), s.timeoutDuration)
		if int(in.View)%s.serversPerCluster+s.serversPerCluster*s.cluster_id+1 == s.id && !s.isByzantineFaulty {
			s.NewView(int(in.View), s.vcMsgs[in.View].ViewchangeMsgs)
		}
	}
	return &pb.AckMessage{Success: true}, nil
}

func (s *server) ViewChange(view int) {
	s.underViewChange = true
	fmt.Printf("[%s] Initiating View Change\n", s.name)
	preprepare := []*pb.PreprepareMessage{}
	for _, i := range s.preprepareMsgs {
		preprepare = append(preprepare, i)
	}
	viewChangeMsg := pb.ViewChangeMessage{
		Ack:              true,
		View:             int32(view),
		SenderId:         int32(s.id),
		Preprepared:      preprepare,
		StableCheckpoint: 0,
	}

	if _, exist := s.vcMsgs[int32(view)]; !exist {
		s.vcMsgs[int32(view)] = &pb.RepeatedViewChangeMessages{}
	}

	signature, _ := SignMessage(s.private_key, []byte(fmt.Sprintf("%d|%d", viewChangeMsg.View, viewChangeMsg.SenderId)))
	viewChangeMsg.Signature = signature
	temp := s.vcMsgs[int32(viewChangeMsg.View)]
	temp.ViewchangeMsgs = append(temp.ViewchangeMsgs, &viewChangeMsg)
	s.vcMsgs[int32(viewChangeMsg.View)] = temp
	for idx, addr := range s.servers {
		if s.cluster_id*s.serversPerCluster+idx+1 == s.id {
			continue
		}
		go func(viewChangeMsg *pb.ViewChangeMessage, addr grpc.ClientConnInterface) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, _ = pb.NewBankingServiceClient(addr).ViewChangeMulticast(ctx, viewChangeMsg)

		}(&viewChangeMsg, addr)
	}
}

func (s *server) NewViewRpc(ctx context.Context, in *pb.NewViewMessage) (*pb.AckMessage, error) {
	if !s.isActive {
		return &pb.AckMessage{Success: false}, nil
	}
	s.leaderID = int(in.SenderId)
	s.view = int(in.View)
	s.seq_num = 0
	s.stopViewChangeTimer(int(in.View))
	fmt.Printf("[%s] New view established. Server %d is the new leader.\n", s.name, s.leaderID)
	// s.vcResult[int32(in.View)] = true
	temp := s.vcMsgs[int32(in.View)]
	temp.NewPreprepareMsgs = in.PrePrepareMessages
	s.vcMsgs[int32(in.View)] = temp
	s.preprepareMsgs = make(map[int]*pb.PreprepareMessage)
	// s.requestTimers=  make(map[int64]*requestTimer)
	for _, i := range s.requestTimers {
		i.timer.Reset(s.timeoutDuration)
	}
	s.underViewChange = false
	return &pb.AckMessage{Success: true}, nil
}

func (s *server) NewView(view int, viewChangeMsgs []*pb.ViewChangeMessage) {
	highest_id := s.id
	preprepare := []*pb.PreprepareMessage{}
	for _, i := range s.preprepareMsgs {
		preprepare = append(preprepare, i)
	}
	highest_val := len(preprepare)
	for id, msg := range viewChangeMsgs {
		if VerifySignature(s.public_key[int(msg.SenderId)], []byte(fmt.Sprintf("%d|%d", msg.View, msg.SenderId)), msg.Signature) != nil {
			fmt.Printf("Invalid ViewChange message from Server %d\n", msg.SenderId)
			return
		}
		if len(msg.Preprepared) > highest_val {
			log.Print(id)
			highest_id = id
			highest_val = len(msg.Preprepared)
		}
	}
	fmt.Printf("[%s][Sign Verified]: Valid ViewChange message from Servers \n", s.name)

	newView := view
	ready_preprepareMsgs := []*pb.PreprepareMessage{}
	if highest_id != s.id {
		for _, i := range viewChangeMsgs[highest_id].Preprepared {
			ready_preprepareMsgs = append(ready_preprepareMsgs, &pb.PreprepareMessage{Ack: i.Ack, View: int32(newView), SeqNum: i.SeqNum, Digest: i.Digest})
		}
	} else {
		for _, i := range preprepare {
			ready_preprepareMsgs = append(ready_preprepareMsgs, &pb.PreprepareMessage{Ack: i.Ack, View: int32(newView), SeqNum: i.SeqNum, Digest: i.Digest})
		}
	}
	s.seq_num = 0
	newViewMsg := pb.NewViewMessage{
		Ack:                true,
		View:               int32(newView),
		ViewChanges:        viewChangeMsgs,
		SenderId:           int32(s.id),
		PrePrepareMessages: ready_preprepareMsgs,
	}
	signature, _ := SignMessage(s.private_key, []byte(fmt.Sprintf("%d|%d", newViewMsg.View, newViewMsg.SenderId)))
	newViewMsg.Signature = signature
	temp := s.vcMsgs[int32(newView)]
	temp.NewPreprepareMsgs = newViewMsg.PrePrepareMessages
	s.vcMsgs[int32(newView)] = temp

	for idx, addr := range s.servers {
		if s.serversPerCluster*s.cluster_id+idx+1 == s.id {
			continue
		}
		go func(newViewMsg *pb.NewViewMessage, addr grpc.ClientConnInterface) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			pb.NewBankingServiceClient(addr).NewViewRpc(ctx, newViewMsg)
		}(&newViewMsg, addr)
	}

	s.view = newView
	s.leaderID = s.id
	s.stopViewChangeTimer(int(view))

	fmt.Printf("[%s] New view established. Server %d is the new leader.\n", s.name, s.id)
	// for _, i := range s.requestTimers {
	// 	i.timer.Reset(s.timeoutDuration)
	// }
	// s.activeRequests = true
	// if len(s.pendingPreprepare) != 0 {
	// 	x := s.pendingPreprepare
	// 	s.pendingPreprepare = make(map[int]pb.PreprepareMessage)
	// 	time.Sleep(2 * time.Millisecond)
	// 	for _, i := range x {
	// 		s.StopTimerForRequest(i.Txn.Txn.Timestamp)
	// 		s.StartLPBFT(*i.Txn)
	// 	}
	// }
}

func (s *server) PrePrepare(ctx context.Context, in *pb.PreprepareMessage) (*pb.PrepareMessage, error) {
	if s.isActive {
		s.seq_num = int(in.SeqNum)
		s.preprepareMsgs[int(in.SeqNum)] = in
		missing_flag := false
		if !(in.RequestType == "C" || in.RequestType == "A") {
			for i := 1; i < int(in.SeqNum); i++ {
				if _, exist := s.preprepareMsgs[int(i)]; !exist {
					log.Printf("[%s] Missing prepreparemsg for Seq Num: %d", s.name, i)
					missing_flag = true
				}
			}
			if missing_flag {
				time.Sleep(300 * time.Millisecond)
				for i := range in.SeqNum {
					if _, exist := s.preprepareMsgs[int(i)]; !exist {
						log.Printf("[%s] MISSING FLAG", s.name)
						return nil, nil
					}
				}
			}
		}
		sender_id := int(in.Txn.Txn.Sender - 1)
		receiver_id := int(in.Txn.Txn.Receiver - 1)
		cluster_id_sender := (sender_id * s.numClusters) / s.numClients
		cluster_id_receiver := (receiver_id * s.numClusters) / s.numClients
		if cluster_id_receiver == cluster_id_sender {
			x := fmt.Sprintf("C%d", sender_id+1)
			y := fmt.Sprintf("C%d", receiver_id+1)
			res, _ := s.ShardDB.Exec("UPDATE accounts SET locked = 1 WHERE id IN (?, ?) AND locked = 0", x, y)
			rowsAffected, _ := res.RowsAffected()
			if rowsAffected < int64(2) {
				s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
				log.Printf("[%s] Could not acquire sufficient locks for txn: %d", s.name, in.Txn.Txn.Timestamp)

				return nil, nil
			}
		} else {
			if !(in.RequestType == "C" || in.RequestType == "A") {
				if s.cluster_id == cluster_id_sender {
					x := fmt.Sprintf("C%d", sender_id+1)
					y := fmt.Sprintf("C%d", receiver_id+1)
					res, _ := s.ShardDB.Exec("UPDATE accounts SET locked = 1 WHERE id IN (?) AND locked = 0", x)
					rowsAffected, _ := res.RowsAffected()
					if rowsAffected < int64(1) {
						s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
						log.Printf("[%s] Could not acquire sufficient locks for txn: %d", s.name, in.Txn.Txn.Timestamp)
						return nil, nil
					}
				}
				if s.cluster_id == cluster_id_receiver {
					x := fmt.Sprintf("C%d", sender_id+1)
					y := fmt.Sprintf("C%d", receiver_id+1)
					res, _ := s.ShardDB.Exec("UPDATE accounts SET locked = 1 WHERE id IN  (?) AND locked = 0", y)
					rowsAffected, _ := res.RowsAffected()
					if rowsAffected < int64(1) {
						s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
						log.Printf("[%s] Could not acquire sufficient locks for txn: %d", s.name, in.Txn.Txn.Timestamp)
						return nil, nil
					}
				}

			}
		}

		if s.isByzantineFaulty {
			// s.StartTimerForRequest(in.Txn.Txn.Timestamp, timeoutDuration)
			// log.Printf("[%s] BYZANTINE", s.name)
			return nil, nil
		} else {
			// _, exists := s.ledger[viewSeqPair(int(in.View), int(in.SeqNum))]
			// exists := false
			if in.Ack && in.SeqNum > int32(s.low_watermark) && in.SeqNum < int32(s.high_watermark) && in.View == int32(s.view) && hashTransaction(in.Txn.Txn) == in.Digest {
				// s.StartTimerForRequest(in.Txn.Txn.Timestamp, timeoutDuration)
				prepareMsg := pb.PrepareMessage{Ack: true, View: in.View, SeqNum: in.SeqNum, Digest: in.Digest, SenderId: int32(s.id)}
				signature, _ := SignMessage(s.private_key, []byte(fmt.Sprintf("%d|%d|%s|%d", prepareMsg.View, prepareMsg.SeqNum, prepareMsg.Digest, prepareMsg.SenderId)))
				prepareMsg.Signature = signature
				s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))] = in.Txn.Txn
				s.ledger[in.Digest] = TxnInfo{txn: in.Txn.Txn, status: "PP"}
				s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Result = false
				// fmt.Printf("[%s] Requesting Prepare Message: %d\n", s.name, in.SeqNum)
				return &prepareMsg, nil
			} else {
				fmt.Printf("[%s] Sending Rejected Prepare Message: %d\n", s.name, in.SeqNum)
				prepareMsg := &pb.PrepareMessage{Ack: false, SeqNum: in.SeqNum, Digest: in.Digest}
				return prepareMsg, nil
			}
		}
	} else {
		fmt.Printf("[%s] Inactive: %d\n", s.name, in.SeqNum)
		return &pb.PrepareMessage{Ack: false}, nil
	}
}

func (s *server) Prepare(ctx context.Context, in *pb.PrepareResponse) (*pb.CommitMessage, error) {
	if s.isByzantineFaulty {
		return nil, nil
	}

	if s.isActive {
		s.ledger[in.Digest] = TxnInfo{txn: s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))], status: "P"}
		commitMsg := pb.CommitMessage{Ack: true, View: in.View, SeqNum: in.SeqNum, Digest: in.Digest, SenderId: int32(s.id), Signature: []byte{}}
		signature, _ := SignMessage(s.private_key, []byte(fmt.Sprintf("%d|%d|%s|%d", commitMsg.View, commitMsg.SeqNum, commitMsg.Digest, commitMsg.SenderId)))
		commitMsg.Signature = signature
		return &commitMsg, nil
	}
	return nil, nil
}

func (s *server) Commit(ctx context.Context, in *pb.CommitResponse) (*pb.Empty, error) {
	if s.isByzantineFaulty {
		return nil, nil
	}
	if s.isActive {
		s.ledger[in.Digest] = TxnInfo{txn: s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))], status: "C"}
		sender_id := int(s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Sender - 1)
		receiver_id := int(s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Receiver - 1)
		cluster_id_sender := (sender_id * s.numClusters) / s.numClients
		cluster_id_receiver := (receiver_id * s.numClusters) / s.numClients
		x := fmt.Sprintf("C%d", s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Sender)
		y := fmt.Sprintf("C%d", s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Receiver)
		amt := s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Amount
		log.Printf("[%s] Commiting Transactions......   %d->%d : %v", s.name, s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Sender, s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Receiver, s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Amount)

		_, err := s.ShardDB.Exec(`
        UPDATE accounts
        SET balance = CASE
            WHEN id = ? THEN balance - ?
            WHEN id = ? THEN balance + ?
        END
        WHERE id IN (?, ?)`,
			x, amt, y, amt, x, y)
		if err != nil {
			s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y) // Release locks
			log.Printf("failed to perform transfer: %v", err)
		}
		s.ledger[in.Digest] = TxnInfo{txn: s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))], status: "E"}
		s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Result = true

		if cluster_id_receiver != cluster_id_sender {
			if !(s.preprepareMsgs[int(in.SeqNum)].RequestType == "C" || s.preprepareMsgs[int(in.SeqNum)].RequestType == "A") {
				txn := s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))]
				if cluster_id_sender == s.cluster_id {
					s.AppendExecutedTxn(x, execTxn{TxID: int(txn.Timestamp), Seq_Num: int(in.SeqNum), Txn: txn, Status: "P"})
					s.WAL[int32(txn.Timestamp)] = txn
					s.insertIntoWAL(txn.Timestamp, txn)
				}
				if cluster_id_receiver == s.cluster_id {
					s.AppendExecutedTxn(y, execTxn{TxID: int(txn.Timestamp), Seq_Num: int(in.SeqNum), Txn: txn, Status: "P"})
					s.WAL[int32(txn.Timestamp)] = txn
					s.insertIntoWAL(txn.Timestamp, txn)
				}
			} else {
				txn := s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))]
				if s.preprepareMsgs[int(in.SeqNum)].RequestType == "C" {
					if _, exist := s.WAL[int32(txn.Timestamp)]; exist {
						WAL := s.fetchWAL(int64(txn.Timestamp))
						log.Printf("--- [%s] COMMITTED (txn: %d->%d : %v) ---", s.name, WAL.Sender, WAL.Receiver, WAL.Amount)

						x := fmt.Sprintf("C%d", int(WAL.Sender))
						y := fmt.Sprintf("C%d", int(WAL.Receiver))
						sender_id := int(WAL.Sender - 1)
						receiver_id := int(WAL.Receiver - 1)
						cluster_id_sender := (sender_id * s.numClusters) / s.numClients
						cluster_id_receiver := (receiver_id * s.numClusters) / s.numClients
						if cluster_id_sender == s.cluster_id {
							s.AppendExecutedTxn(x, execTxn{TxID: int(WAL.Timestamp), Seq_Num: int(in.SeqNum), Txn: WAL, Status: "C"})

						}
						if cluster_id_receiver == s.cluster_id {
							s.AppendExecutedTxn(y, execTxn{TxID: int(WAL.Timestamp), Seq_Num: int(in.SeqNum), Txn: WAL, Status: "C"})

						}

						s.commitedTxns[int32(WAL.Timestamp)] = WAL
						s.deleteFromWAL(int64(txn.Timestamp))
						delete(s.WAL, int32(txn.Timestamp))
						_, err := s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
						if err != nil {
							log.Printf("failed to release locks: %v", err)
						}

						if cluster_id_sender == s.cluster_id {
							reply := pb.Reply{View: int32(s.view), ViewLeader: int32(s.leaderID), Timestamp: txn.Timestamp, ClientId: txn.Sender, ServerId: int32(s.id), Result: true}
							conn, _ := grpc.NewClient(fmt.Sprintf("localhost:%d", 8000+s.cluster_id), grpc.WithTransportCredentials(insecure.NewCredentials()))
							ctx, cancel := context.WithTimeout(context.Background(), time.Second)
							defer cancel()
							pb.NewBankingServiceClient(conn).TransferMoneyResponse(ctx, &reply)
						}
					}
				} else if s.preprepareMsgs[int(in.SeqNum)].RequestType == "A" {
					if _, exist := s.WAL[int32(txn.Timestamp)]; exist {
						WAL := s.fetchWAL(int64(txn.Timestamp))
						log.Printf("--- [%s] ABORT (txn: %d->%d : %v) ---", s.name, WAL.Sender, WAL.Receiver, WAL.Amount)

						// log.Printf("[%s] (txn %v: %d->%d : %v)", s.name, in.TxnCorresponding, WAL.Sender, WAL.Receiver, WAL.Amount)

						x := fmt.Sprintf("C%d", int(WAL.Sender))
						y := fmt.Sprintf("C%d", int(WAL.Receiver))
						amt := WAL.Amount

						_, err := s.ShardDB.Exec(`
						UPDATE accounts
						SET balance = CASE
							WHEN id = ? THEN balance + ?
							WHEN id = ? THEN balance - ?
						END
						WHERE id IN (?, ?)`,
							x, amt, y, amt, x, y)
						if err != nil {
							s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y) // Release locks
							log.Printf("failed to perform transfer: %v", err)
						}
						_, err = s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
						if err != nil {
							log.Printf("failed to release locks: %v", err)
						}

						s.deleteFromWAL(int64(txn.Timestamp))
						delete(s.WAL, int32(txn.Timestamp))

						if cluster_id_sender == s.cluster_id {
							s.AppendExecutedTxn(x, execTxn{TxID: int(WAL.Timestamp), Seq_Num: int(in.SeqNum), Txn: WAL, Status: "A"})
							reply := pb.Reply{View: int32(s.view), ViewLeader: int32(s.leaderID), Timestamp: txn.Timestamp, ClientId: txn.Sender, ServerId: int32(s.id), Result: false}
							conn, _ := grpc.NewClient(fmt.Sprintf("localhost:%d", 8000+s.cluster_id), grpc.WithTransportCredentials(insecure.NewCredentials()))
							ctx, cancel := context.WithTimeout(context.Background(), time.Second)
							defer cancel()
							pb.NewBankingServiceClient(conn).TransferMoneyResponse(ctx, &reply)
						} else {
							s.AppendExecutedTxn(y, execTxn{TxID: int(WAL.Timestamp), Seq_Num: int(in.SeqNum), Txn: WAL, Status: "A"})

						}
					}

				}
			}

		} else {
			// fmt.Printf("[%s] Replying To Client: %d    [%v]\n", s.name, s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Sender, fmt.Sprintf("localhost:%d", 8000+s.cluster_id))
			_, err := s.ShardDB.Exec("UPDATE accounts SET locked = 0 WHERE id IN (?, ?)", x, y)
			if err != nil {
				log.Printf("[%s]failed to release locks: %v", s.name, err)
			}
			s.commitedTxns[int32(s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Timestamp)] = s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))]
			s.AppendExecutedTxn(x, execTxn{TxID: int(s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Timestamp), Seq_Num: int(in.SeqNum), Txn: s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))], Status: ""})

			reply := pb.Reply{View: in.View, ViewLeader: int32(s.leaderID), Timestamp: s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Timestamp, ClientId: s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Sender, ServerId: int32(s.id), Result: s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Result}
			conn, _ := grpc.NewClient(fmt.Sprintf("localhost:%d", 8000+s.cluster_id), grpc.WithTransportCredentials(insecure.NewCredentials()))
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			pb.NewBankingServiceClient(conn).TransferMoneyResponse(ctx, &reply)

		}

		// s.StopTimerForRequest(s.temp_log[viewSeqPair(int(in.View), int(in.SeqNum))].Timestamp)
		delete(s.temp_log, viewSeqPair(int(in.View), int(in.SeqNum)))

	}
	return nil, nil

}

func (s *server) insertIntoWAL(txn_timestamp int64, txn *pb.Transaction) {
	// log.Print(txn_timestamp)
	serialized, err := proto.Marshal(txn)
	if err != nil {
		log.Fatalf("Failed to serialize transaction: %v", err)
	}
	_, err = s.ShardDB.Exec(`INSERT OR REPLACE INTO WALTABLE (id, WAL) VALUES (?, ?)`, txn_timestamp, string(serialized))
	if err != nil {
		log.Fatalf("Failed to Update WAL transaction: %v", err)
	}

}

func (s *server) deleteFromWAL(txn_timestamp int64) {
	_, err := s.ShardDB.Exec(`DELETE FROM WALTABLE WHERE id = ?`, txn_timestamp)
	// _, err := s.ShardDB.Exec(`UPDATE WALTABLE SET WAL = ? WHERE id = ?`, nil, txn_timestamp)
	if err != nil {
		log.Fatalf("Failed to Delete WAL transaction: %v", err)
	}

}

func (s *server) fetchWAL(txn_timestamp int64) *pb.Transaction {
	var currentTxnData sql.NullString
	err := s.ShardDB.QueryRow(`SELECT WAL FROM WALTABLE WHERE id = (?)`, txn_timestamp).Scan(&currentTxnData)
	if err != nil {
		log.Printf("[%s] Error found 1 : %v", s.name, err)
		if err == sql.ErrNoRows {
			log.Printf("[%s] WALTABLE with id not found", s.name)
			return nil
		}
	}
	var transactions pb.Transaction
	if currentTxnData.Valid {
		err = proto.Unmarshal([]byte(currentTxnData.String), &transactions)
		if err != nil {
		}
	}
	return &transactions
}

func (s *server) AppendExecutedTxn(sender string, newTransactions execTxn) error {
	var currentTxnData sql.NullString
	err := s.ShardDB.QueryRow(`SELECT executed_txns FROM accounts WHERE id = (?)`, sender).Scan(&currentTxnData)
	// log.Print(sender)
	if err != nil {
		log.Print("Error found")

		if err == sql.ErrNoRows {
			log.Print("Account with id not found")
			return nil
		}
		return err
	}
	var transactions []execTxn
	if currentTxnData.Valid {
		err = json.Unmarshal([]byte(currentTxnData.String), &transactions)
		if err != nil {
			return err
		}
	}
	transactions = append(transactions, newTransactions)
	updatedTxnData, err := json.Marshal(transactions)
	if err != nil {
		return err
	}
	_, err = s.ShardDB.Exec(`UPDATE accounts SET executed_txns = ?`, string(updatedTxnData))
	if err != nil {
		return err
	}
	return nil
}

func main() {
	numClusters := 3
	serversPerCluster := 4
	numClients := 3000
	// totalDataItems := 3000
	shardSize := numClients / numClusters
	flag.Parse()

	clusters := make([]*Cluster, numClusters)
	for i := 0; i < numClusters; i++ {
		clusters[i] = &Cluster{id: i + 1}
		startID := i*shardSize + 1
		endID := (i + 1) * shardSize

		for j := 0; j < serversPerCluster; j++ {
			// db, _ := sql.Open("sqlite3", fmt.Sprintf("shard_C%d_S%d.db", i+1, 3*i+j+1))
			// log.Print(fmt.Sprintf("shard_C%d_S%d", i+1, serversPerCluster*i+j+1))
			db := CreateSQLiteDB(fmt.Sprintf("shard_C%d_S%d", i+1, serversPerCluster*i+j+1))
			InitializeShardData(db, startID, endID)
			server := &server{id: j + 1, ShardDB: db}
			clusters[i].servers = append(clusters[i].servers, server)
			GenerateRSAKeyPair(2048, fmt.Sprintf("Key/privateKey_C%d_S%d.pem", i+1, serversPerCluster*i+j+1), fmt.Sprintf("Key/publicKey_C%d_S%d.pem", i+1, serversPerCluster*i+j+1))
		}
	}
	for i := 0; i < numClusters*serversPerCluster; i++ {
		go func(idx int) {
			port := fmt.Sprintf("localhost:500%d", 50+idx+1)
			lis, err := net.Listen("tcp", port)
			if err != nil {
				log.Fatalf("Failed to listen on port %s: %v", port, err)
			}
			grpcServer := grpc.NewServer()
			db, _ := sql.Open("sqlite3", fmt.Sprintf("shard_C%d_S%d.db", (idx/serversPerCluster)+1, idx+1))
			s := &server{
				id:                idx + 1,
				name:              fmt.Sprintf("S%d", idx+1),
				numClusters:       numClusters,
				numClients:        numClients,
				serversPerCluster: serversPerCluster,
				ShardDB:           db,
				cluster_id:        idx / serversPerCluster,
				under2PCRPC:       false,
				view:              0,
				seq_num:           0,
				low_watermark:     0,
				high_watermark:    100,
			}
			s.leaderID = 1 + s.cluster_id*serversPerCluster
			s.WAL = make(map[int32]*pb.Transaction)
			s.requestTimers = make(map[int64]*requestTimer)
			s.temp_log = make(map[string]*pb.Transaction)
			s.commitedTxns = make(map[int32]*pb.Transaction)
			s.public_key = make(map[int]*rsa.PublicKey)
			s.servers = make([]*grpc.ClientConn, serversPerCluster)
			s.preprepareMsgs = make(map[int]*pb.PreprepareMessage)
			s.commitMsgs = make(map[int][]*pb.CommitMessage)
			s.ledger = make(map[string]TxnInfo)
			s.vcMsgs = make(map[int32]*pb.RepeatedViewChangeMessages)
			// s.vcResult = make(map[int32]bool)
			s.vcTimer = make(map[int]*requestTimer)
			s.underViewChange = false
			s.timeoutDuration = timeoutDuration
			s.private_key, _ = LoadPrivateKey(fmt.Sprintf("Key/privateKey_C%d_S%d.pem", s.cluster_id+1, s.id))
			for id := range serversPerCluster {
				if id+s.cluster_id*serversPerCluster+1 != s.id {
					// log.Printf("[%d] %s",idx+1,(fmt.Sprintf("localhost:500%d", 50+s.cluster_id*serversPerCluster+id+1)))
					// log.Printf("[%s][%d] %d", s.name,idx,(s.cluster_id*serversPerCluster)+id+1)
					conn, _ := grpc.NewClient(fmt.Sprintf("localhost:500%d", 50+s.cluster_id*serversPerCluster+id+1), grpc.WithTransportCredentials(insecure.NewCredentials()))
					s.servers[id] = conn
					s.public_key[id+s.cluster_id*serversPerCluster+1], _ = LoadPublicKey(fmt.Sprintf("Key/publicKey_C%d_S%d.pem", s.cluster_id+1, id+s.cluster_id*serversPerCluster+1))

				} else {
					s.servers[id] = nil
					s.public_key[id+s.cluster_id*serversPerCluster+1], _ = LoadPublicKey(fmt.Sprintf("Key/publicKey_C%d_S%d.pem", s.cluster_id+1, id+s.cluster_id*serversPerCluster+1))
				}
			}
			pb.RegisterBankingServiceServer(grpcServer, s)
			log.Printf("[%s] gRPC server listening at %v", s.name, lis.Addr())
			if err := grpcServer.Serve(lis); err != nil {
				log.Fatalf("Failed to serve: %v", err)
			}
		}(i)
	}

	select {}

}

//CRYPTOGRAPHIC KEYS

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

func LoadPublicKey(filePath string) (*rsa.PublicKey, error) {
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
