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
	"time"

	"github.com/xuperchain/xuper-front/config"
	logs "github.com/xuperchain/xuper-front/logs"
	clixchain "github.com/xuperchain/xuper-front/server/client"
	serv_ca "github.com/xuperchain/xuper-front/service/ca"
	serv_proxy_xchain "github.com/xuperchain/xuper-front/service/prxyxchain"
	util_cert "github.com/xuperchain/xuper-front/util/cert"
	p2p "github.com/xuperchain/xupercore/protos"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
	ErrUnAuthorized  = errors.New("Request UnAuthorized error")
	ErrInvalidPKType = errors.New("unknown type of public key")
	ErrParseEcdsa    = errors.New("parse ecdsa public key error")
	ErrRpcAddInvalid = errors.New("address invalid")
)

type xchainProxyServer struct {
	groups map[string]*clixchain.GroupClient

	log logs.Logger
}

func (proxy *xchainProxyServer) CheckParachainAuth(bcName string, from string) bool {
	if value, ok := proxy.groups[bcName]; ok {
		for _, v := range value.Cache.Get() {
			if v == from {
				return true
			}
		}
		return false
	}
	client, err := clixchain.NewClientServer(bcName)
	if err != nil {
		return false
	}
	err = client.Init()
	if err != nil {
		proxy.log.Error("XchainProxyServer::CheckParachainAuth::Init error", "err", err)
		return false
	}
	proxy.groups[bcName] = client
	for _, v := range client.Cache.Get() {
		if v == from {
			return true
		}
	}
	return false
}

func (proxy *xchainProxyServer) SendP2PMessage(p2pMsgServer p2p.P2PService_SendP2PMessageServer) error {
	in, err := p2pMsgServer.Recv()
	if err == io.EOF {
		proxy.log.Info(in.GetHeader().Logid, err)
		return nil
	}
	if err != nil {
		if in.GetHeader() != nil {
			proxy.log.Info(in.GetHeader().Logid, err)
		}
		return err
	}
	if config.GetXchainServer().Master != "" {
		address := p2pMsgServer.Context().Value("address")
		add, ok := address.(string)
		if !ok {
			return ErrRpcAddInvalid
		}
		if in.GetHeader().GetBcname() != config.GetXchainServer().Master &&
			!proxy.CheckParachainAuth(in.GetHeader().GetBcname(), add) {
			return ErrUnAuthorized
		}
	}
	ret, err := handleReceivedMsg(in)
	if ret != nil {
		p2pMsgServer.Send(ret)
	}
	return err
}

func handleReceivedMsg(msg *p2p.XuperMessage) (*p2p.XuperMessage, error) {
	c := serv_proxy_xchain.GetXchainP2pProxy()
	if c == nil {
		return nil, errors.New("cat get client")
	}
	defer c.Defer()

	// check msg type
	msgType := msg.GetHeader().GetType()
	if msgType != p2p.XuperMessage_POSTTX && msgType != p2p.XuperMessage_SENDBLOCK && msgType !=
		p2p.XuperMessage_BATCHPOSTTX && msgType != p2p.XuperMessage_NEW_BLOCKID {
		// 期望节点处理后有返回的请求
		ret, err := c.SendMessageWithResponse(context.Background(), msg)
		return ret, err
	}

	// 发送给节点, 不期望节点返回
	c.SendMessage(context.Background(), msg)
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
			proxy.log.Error("XchainProxyServer::StartXchainProxyServer::failed to serve", "err", err)
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

	proxy.log.Info("XchainProxyServer::StartXchainProxyServer::server start", "Port", config.GetXchainServer().Port)

	if err := s.Serve(lis); err != nil {
		proxy.log.Error("XchainProxyServer::StartXchainProxyServer::serve failed", "err", err)
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
		if len(hh.Subject.OrganizationalUnit) == 0 {
			return errors.New("cert is not valid, xchain address is empty.")
		}
		address := hh.Subject.OrganizationalUnit[0]
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
