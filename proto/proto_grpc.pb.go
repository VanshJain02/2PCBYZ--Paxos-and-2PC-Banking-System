// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.28.2
// source: proto.proto

package __

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	BankingService_TransferMoney_FullMethodName         = "/proto.BankingService/TransferMoney"
	BankingService_SyncServerStatus_FullMethodName      = "/proto.BankingService/SyncServerStatus"
	BankingService_TransferMoneyResponse_FullMethodName = "/proto.BankingService/TransferMoneyResponse"
	BankingService_PrePrepare_FullMethodName            = "/proto.BankingService/PrePrepare"
	BankingService_Prepare_FullMethodName               = "/proto.BankingService/Prepare"
	BankingService_Commit_FullMethodName                = "/proto.BankingService/Commit"
	BankingService_ViewChangeMulticast_FullMethodName   = "/proto.BankingService/ViewChangeMulticast"
	BankingService_NewViewRpc_FullMethodName            = "/proto.BankingService/NewViewRpc"
	BankingService_CrossShardPrepare_FullMethodName     = "/proto.BankingService/CrossShardPrepare"
	BankingService_CrossShardCommit_FullMethodName      = "/proto.BankingService/CrossShardCommit"
	BankingService_PCCommit_FullMethodName              = "/proto.BankingService/PCCommit"
	BankingService_PCAbort_FullMethodName               = "/proto.BankingService/PCAbort"
	BankingService_PrintBalance_FullMethodName          = "/proto.BankingService/PrintBalance"
	BankingService_PrintLog_FullMethodName              = "/proto.BankingService/PrintLog"
	BankingService_PrintPerformance_FullMethodName      = "/proto.BankingService/PrintPerformance"
)

// BankingServiceClient is the client API for BankingService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type BankingServiceClient interface {
	TransferMoney(ctx context.Context, in *TransferRequest, opts ...grpc.CallOption) (*Empty, error)
	SyncServerStatus(ctx context.Context, in *ServerStatus, opts ...grpc.CallOption) (*Empty, error)
	TransferMoneyResponse(ctx context.Context, in *Reply, opts ...grpc.CallOption) (*Empty, error)
	PrePrepare(ctx context.Context, in *PreprepareMessage, opts ...grpc.CallOption) (*PrepareMessage, error)
	Prepare(ctx context.Context, in *PrepareResponse, opts ...grpc.CallOption) (*CommitMessage, error)
	Commit(ctx context.Context, in *CommitResponse, opts ...grpc.CallOption) (*Empty, error)
	ViewChangeMulticast(ctx context.Context, in *ViewChangeMessage, opts ...grpc.CallOption) (*AckMessage, error)
	NewViewRpc(ctx context.Context, in *NewViewMessage, opts ...grpc.CallOption) (*AckMessage, error)
	CrossShardPrepare(ctx context.Context, in *CsPrepareMsg, opts ...grpc.CallOption) (*Empty, error)
	CrossShardCommit(ctx context.Context, in *CsCommitMsg, opts ...grpc.CallOption) (*Empty, error)
	PCCommit(ctx context.Context, in *CsCommitMsg, opts ...grpc.CallOption) (*PCResponse, error)
	PCAbort(ctx context.Context, in *CsCommitMsg, opts ...grpc.CallOption) (*PCResponse, error)
	PrintBalance(ctx context.Context, in *PrintBalanceRequest, opts ...grpc.CallOption) (*PrintBalanceResponse, error)
	PrintLog(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*PrintLogResponse, error)
	PrintPerformance(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*PrintPerformanceResponse, error)
}

type bankingServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewBankingServiceClient(cc grpc.ClientConnInterface) BankingServiceClient {
	return &bankingServiceClient{cc}
}

func (c *bankingServiceClient) TransferMoney(ctx context.Context, in *TransferRequest, opts ...grpc.CallOption) (*Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Empty)
	err := c.cc.Invoke(ctx, BankingService_TransferMoney_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) SyncServerStatus(ctx context.Context, in *ServerStatus, opts ...grpc.CallOption) (*Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Empty)
	err := c.cc.Invoke(ctx, BankingService_SyncServerStatus_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) TransferMoneyResponse(ctx context.Context, in *Reply, opts ...grpc.CallOption) (*Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Empty)
	err := c.cc.Invoke(ctx, BankingService_TransferMoneyResponse_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) PrePrepare(ctx context.Context, in *PreprepareMessage, opts ...grpc.CallOption) (*PrepareMessage, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(PrepareMessage)
	err := c.cc.Invoke(ctx, BankingService_PrePrepare_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) Prepare(ctx context.Context, in *PrepareResponse, opts ...grpc.CallOption) (*CommitMessage, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CommitMessage)
	err := c.cc.Invoke(ctx, BankingService_Prepare_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) Commit(ctx context.Context, in *CommitResponse, opts ...grpc.CallOption) (*Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Empty)
	err := c.cc.Invoke(ctx, BankingService_Commit_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) ViewChangeMulticast(ctx context.Context, in *ViewChangeMessage, opts ...grpc.CallOption) (*AckMessage, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AckMessage)
	err := c.cc.Invoke(ctx, BankingService_ViewChangeMulticast_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) NewViewRpc(ctx context.Context, in *NewViewMessage, opts ...grpc.CallOption) (*AckMessage, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AckMessage)
	err := c.cc.Invoke(ctx, BankingService_NewViewRpc_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) CrossShardPrepare(ctx context.Context, in *CsPrepareMsg, opts ...grpc.CallOption) (*Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Empty)
	err := c.cc.Invoke(ctx, BankingService_CrossShardPrepare_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) CrossShardCommit(ctx context.Context, in *CsCommitMsg, opts ...grpc.CallOption) (*Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Empty)
	err := c.cc.Invoke(ctx, BankingService_CrossShardCommit_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) PCCommit(ctx context.Context, in *CsCommitMsg, opts ...grpc.CallOption) (*PCResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(PCResponse)
	err := c.cc.Invoke(ctx, BankingService_PCCommit_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) PCAbort(ctx context.Context, in *CsCommitMsg, opts ...grpc.CallOption) (*PCResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(PCResponse)
	err := c.cc.Invoke(ctx, BankingService_PCAbort_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) PrintBalance(ctx context.Context, in *PrintBalanceRequest, opts ...grpc.CallOption) (*PrintBalanceResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(PrintBalanceResponse)
	err := c.cc.Invoke(ctx, BankingService_PrintBalance_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) PrintLog(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*PrintLogResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(PrintLogResponse)
	err := c.cc.Invoke(ctx, BankingService_PrintLog_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bankingServiceClient) PrintPerformance(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*PrintPerformanceResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(PrintPerformanceResponse)
	err := c.cc.Invoke(ctx, BankingService_PrintPerformance_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// BankingServiceServer is the server API for BankingService service.
// All implementations must embed UnimplementedBankingServiceServer
// for forward compatibility.
type BankingServiceServer interface {
	TransferMoney(context.Context, *TransferRequest) (*Empty, error)
	SyncServerStatus(context.Context, *ServerStatus) (*Empty, error)
	TransferMoneyResponse(context.Context, *Reply) (*Empty, error)
	PrePrepare(context.Context, *PreprepareMessage) (*PrepareMessage, error)
	Prepare(context.Context, *PrepareResponse) (*CommitMessage, error)
	Commit(context.Context, *CommitResponse) (*Empty, error)
	ViewChangeMulticast(context.Context, *ViewChangeMessage) (*AckMessage, error)
	NewViewRpc(context.Context, *NewViewMessage) (*AckMessage, error)
	CrossShardPrepare(context.Context, *CsPrepareMsg) (*Empty, error)
	CrossShardCommit(context.Context, *CsCommitMsg) (*Empty, error)
	PCCommit(context.Context, *CsCommitMsg) (*PCResponse, error)
	PCAbort(context.Context, *CsCommitMsg) (*PCResponse, error)
	PrintBalance(context.Context, *PrintBalanceRequest) (*PrintBalanceResponse, error)
	PrintLog(context.Context, *Empty) (*PrintLogResponse, error)
	PrintPerformance(context.Context, *Empty) (*PrintPerformanceResponse, error)
	mustEmbedUnimplementedBankingServiceServer()
}

// UnimplementedBankingServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedBankingServiceServer struct{}

func (UnimplementedBankingServiceServer) TransferMoney(context.Context, *TransferRequest) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method TransferMoney not implemented")
}
func (UnimplementedBankingServiceServer) SyncServerStatus(context.Context, *ServerStatus) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SyncServerStatus not implemented")
}
func (UnimplementedBankingServiceServer) TransferMoneyResponse(context.Context, *Reply) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method TransferMoneyResponse not implemented")
}
func (UnimplementedBankingServiceServer) PrePrepare(context.Context, *PreprepareMessage) (*PrepareMessage, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PrePrepare not implemented")
}
func (UnimplementedBankingServiceServer) Prepare(context.Context, *PrepareResponse) (*CommitMessage, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Prepare not implemented")
}
func (UnimplementedBankingServiceServer) Commit(context.Context, *CommitResponse) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Commit not implemented")
}
func (UnimplementedBankingServiceServer) ViewChangeMulticast(context.Context, *ViewChangeMessage) (*AckMessage, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ViewChangeMulticast not implemented")
}
func (UnimplementedBankingServiceServer) NewViewRpc(context.Context, *NewViewMessage) (*AckMessage, error) {
	return nil, status.Errorf(codes.Unimplemented, "method NewViewRpc not implemented")
}
func (UnimplementedBankingServiceServer) CrossShardPrepare(context.Context, *CsPrepareMsg) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CrossShardPrepare not implemented")
}
func (UnimplementedBankingServiceServer) CrossShardCommit(context.Context, *CsCommitMsg) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CrossShardCommit not implemented")
}
func (UnimplementedBankingServiceServer) PCCommit(context.Context, *CsCommitMsg) (*PCResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PCCommit not implemented")
}
func (UnimplementedBankingServiceServer) PCAbort(context.Context, *CsCommitMsg) (*PCResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PCAbort not implemented")
}
func (UnimplementedBankingServiceServer) PrintBalance(context.Context, *PrintBalanceRequest) (*PrintBalanceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PrintBalance not implemented")
}
func (UnimplementedBankingServiceServer) PrintLog(context.Context, *Empty) (*PrintLogResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PrintLog not implemented")
}
func (UnimplementedBankingServiceServer) PrintPerformance(context.Context, *Empty) (*PrintPerformanceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PrintPerformance not implemented")
}
func (UnimplementedBankingServiceServer) mustEmbedUnimplementedBankingServiceServer() {}
func (UnimplementedBankingServiceServer) testEmbeddedByValue()                        {}

// UnsafeBankingServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to BankingServiceServer will
// result in compilation errors.
type UnsafeBankingServiceServer interface {
	mustEmbedUnimplementedBankingServiceServer()
}

func RegisterBankingServiceServer(s grpc.ServiceRegistrar, srv BankingServiceServer) {
	// If the following call pancis, it indicates UnimplementedBankingServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&BankingService_ServiceDesc, srv)
}

func _BankingService_TransferMoney_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TransferRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).TransferMoney(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_TransferMoney_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).TransferMoney(ctx, req.(*TransferRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_SyncServerStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServerStatus)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).SyncServerStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_SyncServerStatus_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).SyncServerStatus(ctx, req.(*ServerStatus))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_TransferMoneyResponse_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Reply)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).TransferMoneyResponse(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_TransferMoneyResponse_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).TransferMoneyResponse(ctx, req.(*Reply))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_PrePrepare_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PreprepareMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).PrePrepare(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_PrePrepare_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).PrePrepare(ctx, req.(*PreprepareMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_Prepare_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PrepareResponse)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).Prepare(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_Prepare_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).Prepare(ctx, req.(*PrepareResponse))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_Commit_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CommitResponse)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).Commit(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_Commit_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).Commit(ctx, req.(*CommitResponse))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_ViewChangeMulticast_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ViewChangeMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).ViewChangeMulticast(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_ViewChangeMulticast_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).ViewChangeMulticast(ctx, req.(*ViewChangeMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_NewViewRpc_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NewViewMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).NewViewRpc(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_NewViewRpc_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).NewViewRpc(ctx, req.(*NewViewMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_CrossShardPrepare_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CsPrepareMsg)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).CrossShardPrepare(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_CrossShardPrepare_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).CrossShardPrepare(ctx, req.(*CsPrepareMsg))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_CrossShardCommit_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CsCommitMsg)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).CrossShardCommit(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_CrossShardCommit_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).CrossShardCommit(ctx, req.(*CsCommitMsg))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_PCCommit_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CsCommitMsg)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).PCCommit(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_PCCommit_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).PCCommit(ctx, req.(*CsCommitMsg))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_PCAbort_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CsCommitMsg)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).PCAbort(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_PCAbort_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).PCAbort(ctx, req.(*CsCommitMsg))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_PrintBalance_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PrintBalanceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).PrintBalance(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_PrintBalance_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).PrintBalance(ctx, req.(*PrintBalanceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_PrintLog_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).PrintLog(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_PrintLog_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).PrintLog(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _BankingService_PrintPerformance_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BankingServiceServer).PrintPerformance(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BankingService_PrintPerformance_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BankingServiceServer).PrintPerformance(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// BankingService_ServiceDesc is the grpc.ServiceDesc for BankingService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var BankingService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "proto.BankingService",
	HandlerType: (*BankingServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "TransferMoney",
			Handler:    _BankingService_TransferMoney_Handler,
		},
		{
			MethodName: "SyncServerStatus",
			Handler:    _BankingService_SyncServerStatus_Handler,
		},
		{
			MethodName: "TransferMoneyResponse",
			Handler:    _BankingService_TransferMoneyResponse_Handler,
		},
		{
			MethodName: "PrePrepare",
			Handler:    _BankingService_PrePrepare_Handler,
		},
		{
			MethodName: "Prepare",
			Handler:    _BankingService_Prepare_Handler,
		},
		{
			MethodName: "Commit",
			Handler:    _BankingService_Commit_Handler,
		},
		{
			MethodName: "ViewChangeMulticast",
			Handler:    _BankingService_ViewChangeMulticast_Handler,
		},
		{
			MethodName: "NewViewRpc",
			Handler:    _BankingService_NewViewRpc_Handler,
		},
		{
			MethodName: "CrossShardPrepare",
			Handler:    _BankingService_CrossShardPrepare_Handler,
		},
		{
			MethodName: "CrossShardCommit",
			Handler:    _BankingService_CrossShardCommit_Handler,
		},
		{
			MethodName: "PCCommit",
			Handler:    _BankingService_PCCommit_Handler,
		},
		{
			MethodName: "PCAbort",
			Handler:    _BankingService_PCAbort_Handler,
		},
		{
			MethodName: "PrintBalance",
			Handler:    _BankingService_PrintBalance_Handler,
		},
		{
			MethodName: "PrintLog",
			Handler:    _BankingService_PrintLog_Handler,
		},
		{
			MethodName: "PrintPerformance",
			Handler:    _BankingService_PrintPerformance_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto.proto",
}
