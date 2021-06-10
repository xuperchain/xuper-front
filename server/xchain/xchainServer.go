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

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"

	"github.com/xuperchain/xuper-front/config"
	serv_ca "github.com/xuperchain/xuper-front/service/ca"
	serv_proxy_xchain "github.com/xuperchain/xuper-front/service/prxyxchain"
	util_cert "github.com/xuperchain/xuper-front/util/cert"
	"github.com/xuperchain/xuper-front/xuperp2p"
	xuper_p2p "github.com/xuperchain/xuper-front/xuperp2p"
)

// MaxRecvMsgSize max message size
const MaxRecvMsgSize = 1024 * 1024 * 1024

// MaxConcurrentStreams max concurrent
const MaxConcurrentStreams = 1000

// GRPCTIMEOUT grpc timeout
const GRPCTIMEOUT = 20

type xchainProxyServer struct{}

func (proxy *xchainProxyServer) SendP2PMessage(p2pMsgServer xuperp2p.P2PService_SendP2PMessageServer) error {
	in, err := p2pMsgServer.Recv()
	if err == io.EOF {
		log.Debug(in.GetHeader().Logid, err)
		return nil
	}
	if err != nil {
		log.Debug(in.GetHeader().Logid, err)
		return err
	}
	ret, err := handleReceivedMsg(in)
	if ret != nil {
		p2pMsgServer.Send(ret)
	}
	return err
}

func handleReceivedMsg(msg *xuperp2p.XuperMessage) (*xuperp2p.XuperMessage, error) {
	c := serv_proxy_xchain.GetXchainP2pProxy()
	if c == nil {
		return nil, errors.New("cat get client")
	}
	defer c.Defer()

	// check msg type
	msgType := msg.GetHeader().GetType()
	if msgType != xuper_p2p.XuperMessage_POSTTX && msgType != xuper_p2p.XuperMessage_SENDBLOCK && msgType !=
		xuper_p2p.XuperMessage_BATCHPOSTTX && msgType != xuper_p2p.XuperMessage_NEW_BLOCKID {
		// 期望节点处理后有返回的请求
		ret, err := c.SendMessageWithResponse(context.Background(), msg)
		log.Debug("SendMessageWithResponse,", ret, err)
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
	if err != nil {
		log.Errorf("StartXchainProxyServer failed to listen: %v\n", err)
	}

	var s *grpc.Server
	// 是否使用tls
	if config.GetCaConfig().CaSwitch {
		// 接收xchian过来的tls请求
		creds, err := util_cert.GenCreds()
		if err != nil {
			log.Errorf("StartXchainProxyServer failed to serve: %v\n", err)
		}
		s = grpc.NewServer(grpc.StreamInterceptor(CheckInterceptor()), grpc.Creds(creds), grpc.MaxRecvMsgSize(MaxRecvMsgSize),
			grpc.MaxConcurrentStreams(MaxConcurrentStreams), grpc.ConnectionTimeout(time.Second*time.Duration(GRPCTIMEOUT)))
		xuperp2p.RegisterP2PServiceServer(s, &xchainProxyServer{})
	} else {
		s = grpc.NewServer(grpc.MaxRecvMsgSize(MaxRecvMsgSize),
			grpc.MaxConcurrentStreams(MaxConcurrentStreams), grpc.ConnectionTimeout(time.Second*time.Duration(GRPCTIMEOUT)))
		xuperp2p.RegisterP2PServiceServer(s, &xchainProxyServer{})
	}

	// Register reflection service on gRPC server.
	reflection.Register(s)

	log.Infof("StartXchainProxyServer start server for xchain proxy, %v", config.GetXchainServer().Port)

	if err := s.Serve(lis); err != nil {
		log.Fatalf("StartXchainProxyServer failed, err: %v\n", err)
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
		return handler(srv, ss)
	}
}
