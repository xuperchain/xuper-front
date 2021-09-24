/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package xchain

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/xuperchain/xuper-front/config"
	logs "github.com/xuperchain/xuper-front/logs"
	clixchain "github.com/xuperchain/xuper-front/server/client"
	serv_ca "github.com/xuperchain/xuper-front/service/ca"
	serv_proxy_xchain "github.com/xuperchain/xuper-front/service/prxyxchain"
	util_cert "github.com/xuperchain/xuper-front/util/cert"
	pb "github.com/xuperchain/xuperchain/service/pb"
	p2p "github.com/xuperchain/xupercore/protos"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
)

const (
	// MaxRecvMsgSize max message size
	MaxRecvMsgSize = 1024 * 1024 * 1024
	// MaxConcurrentStreams max concurrent
	MaxConcurrentStreams = 1000
	// GRPCTIMEOUT grpc timeout
	GRPCTIMEOUT = 20
)

var (
	ErrUnAuthorized  = errors.New("request unAuthorized error")
	ErrInvalidPKType = errors.New("unknown type of public key")
	ErrParseEcdsa    = errors.New("parse ecdsa public key error")
	ErrRpcAddInvalid = errors.New("address invalid")

	// SendMsgMap
	sendMsgMap = map[p2p.XuperMessage_MessageType]bool{
		p2p.XuperMessage_POSTTX:                       true,
		p2p.XuperMessage_SENDBLOCK:                    true,
		p2p.XuperMessage_BATCHPOSTTX:                  true,
		p2p.XuperMessage_NEW_BLOCKID:                  true,
		p2p.XuperMessage_CHAINED_BFT_NEW_PROPOSAL_MSG: true,
		p2p.XuperMessage_CHAINED_BFT_VOTE_MSG:         true,
	}
)

type xchainProxyServer struct {
	pb.XchainClient
	pb.EventServiceClient

	groups map[string]*clixchain.GroupClient
	mutex  sync.Mutex

	log logs.Logger
}

func (proxy *xchainProxyServer) GetGroupClient(bcName string) (*clixchain.GroupClient, error) {
	proxy.mutex.Lock()
	defer proxy.mutex.Unlock()
	if value, ok := proxy.groups[bcName]; ok {
		return value, nil
	}
	// 初始化client并注册到groups中, groupclient注册了一条平行链事件订阅流到map中
	client, err := clixchain.NewGroupClient(bcName, proxy.XchainClient, proxy.EventServiceClient)
	if err != nil {
		proxy.log.Error("XchainProxyServer.RegisterClientServer: NewGroupClient error", "err", err)
		return nil, err
	}
	err = client.Init()
	if err != nil {
		proxy.log.Error("XchainProxyServer.RegisterClientServer: Init error", "err", err)
		return nil, err
	}
	proxy.log.Info("XchainProxyServer.CheckParachainAuth: init client success", "groups", client.Get(), "bcname", bcName)
	proxy.groups[bcName] = client
	return client, nil
}

func (proxy *xchainProxyServer) CheckParachainAuth(bcName string, from string) bool {
	client, err := proxy.GetGroupClient(bcName)
	if err != nil {
		return false
	}
	groups := client.Get()
	for _, v := range groups {
		if v == from {
			return true
		}
	}
	return false
}

func (proxy *xchainProxyServer) SendP2PMessage(stream p2p.P2PService_SendP2PMessageServer) error {
	ctx := stream.Context()
	in, err := stream.Recv()
	if err == io.EOF {
		proxy.log.Warn("XchainProxyServer.SendP2PMessage: streamServer meets EOF", "logid", in.GetHeader().Logid)
		return nil
	}
	if err != nil {
		proxy.log.Error("XchainProxyServer.SendP2PMessage: streamServer err", "error", err)
		return err
	}
	if config.GetXchainServer().Master != "" {
		address := ctx.Value("address")
		add, ok := address.(string)
		if !ok {
			return ErrRpcAddInvalid
		}
		// 若为平行链请求，需要进行群组权限检验
		if in.GetHeader().GetBcname() != config.GetXchainServer().Master &&
			!proxy.CheckParachainAuth(in.GetHeader().GetBcname(), add) {
			return ErrUnAuthorized
		}
	}
	ret, err := handleReceivedMsg(in)
	if err != nil {
		proxy.log.Error("XchainProxyServer.SendP2PMessageServer: handleReceivedMsg error", "logid", in.GetHeader().Logid, "type", in.GetHeader().Type,
			"from", in.GetHeader().From, "err", err)
	}
	if ret != nil {
		stream.Send(ret)
	}
	return err
}

func handleReceivedMsg(msg *p2p.XuperMessage) (*p2p.XuperMessage, error) {
	c := serv_proxy_xchain.GetXchainP2pProxy()
	if c == nil {
		return nil, errors.New("cat get client")
	}

	// check msg type
	msgType := msg.GetHeader().GetType()
	if _, ok := sendMsgMap[msgType]; !ok {
		// 期望节点处理后有返回的请求
		ret, err := c.SendMessageWithResponse(context.Background(), msg)
		return ret, err
	}

	// 发送给节点, 不期望节点返回
	err := c.SendMessage(context.Background(), msg)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// StartXchainProxyServer 开启服务
func StartXchainProxyServer(quit chan int) {
	// start server
	lis, err := net.Listen("tcp", config.GetXchainServer().Port)
	log, err := logs.NewLogger("xchainProxyServer")
	if err != nil {
		return
	}
	proxy := xchainProxyServer{
		log: log,
	}
	if config.GetXchainServer().Master != "" {
		proxy.groups = make(map[string]*clixchain.GroupClient)
	}
	var s *grpc.Server
	// 是否使用tls
	if config.GetCaConfig().CaSwitch {
		// 接收xchian过来的tls请求
		creds, err := util_cert.GenCreds()
		if err != nil {
			proxy.log.Error("XchainProxyServer.StartXchainProxyServer: failed to serve", "err", err)
		}
		s = grpc.NewServer(grpc.StreamInterceptor(CheckInterceptor()), grpc.Creds(creds), grpc.MaxRecvMsgSize(MaxRecvMsgSize),
			grpc.MaxConcurrentStreams(MaxConcurrentStreams), grpc.ConnectionTimeout(time.Second*time.Duration(GRPCTIMEOUT)))
		p2p.RegisterP2PServiceServer(s, &proxy)
	} else {
		s = grpc.NewServer(grpc.MaxRecvMsgSize(MaxRecvMsgSize),
			grpc.MaxConcurrentStreams(MaxConcurrentStreams), grpc.ConnectionTimeout(time.Second*time.Duration(GRPCTIMEOUT)))
		p2p.RegisterP2PServiceServer(s, &proxy)
	}
	// Register reflection service on gRPC server.
	reflection.Register(s)

	// 注册XchainClint和XchainEventClient
	conn, err := grpc.Dial(config.GetXchainServer().Rpc, grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxRecvMsgSize)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
			Timeout:             5 * time.Second,  // wait 5 second for ping ack before considering the connection dead
			PermitWithoutStream: true,             // send pings even without active streams
		}))
	if err != nil {
		proxy.log.Error("XchainProxyServer.Dial: create conn to xchain failed")
		return
	}
	proxy.XchainClient = pb.NewXchainClient(conn)
	proxy.EventServiceClient = pb.NewEventServiceClient(conn)

	proxy.log.Info("XchainProxyServer.StartXchainProxyServer: server start", "Port", config.GetXchainServer().Port)

	if err := s.Serve(lis); err != nil {
		proxy.log.Error("XchainProxyServer.StartXchainProxyServer: serve failed", "err", err)
		quit <- 1
	}
}

// Interceptor
func CheckInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		p, _ := peer.FromContext(ss.Context())
		hh, err := x509.ParseCertificate(p.AuthInfo.(credentials.TLSInfo).State.PeerCertificates[0].Raw)
		if err != nil {
			return errors.New("cert is not valid")
		}
		ok := serv_ca.IsValidCert(hh.SerialNumber.String())
		if ok == false {
			return errors.New("cert is not valid")
		}
		if config.GetXchainServer().Master == "" {
			return handler(srv, ss)
		}
		address := hh.Subject.SerialNumber
		ctx := context.WithValue(ss.Context(), "address", address)
		return handler(srv, newWrappedStream(ss, &ctx))
	}
}

////////////// wrappedStream ///////////////

type wrappedStream struct {
	grpc.ServerStream
	Ctx *context.Context
}

// Context 的作用为覆盖stream的Context方法
func (w *wrappedStream) Context() context.Context {
	return *w.Ctx
}

func newWrappedStream(s grpc.ServerStream, ctx *context.Context) grpc.ServerStream {
	wrapper := wrappedStream{
		ServerStream: s,
		Ctx:          ctx,
	}
	return grpc.ServerStream(&wrapper)
}
