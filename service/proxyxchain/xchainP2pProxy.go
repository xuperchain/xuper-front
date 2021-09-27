/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package proxyxchain

import (
	"context"
	"io"

	"github.com/xuperchain/xuper-front/config"
	logs "github.com/xuperchain/xuper-front/logs"
	util_cert "github.com/xuperchain/xuper-front/util/cert"
	p2p "github.com/xuperchain/xupercore/protos"
	"google.golang.org/grpc"
)

const MaxMessageSize = 1024 * 1024 * 1024

type XchainP2pProxy struct {
	host string
	conn *grpc.ClientConn

	log logs.Logger
}

var xchainProxy *XchainP2pProxy

func GetXchainP2pProxy() *XchainP2pProxy {
	if xchainProxy == nil {
		//初始化
		var conn *grpc.ClientConn
		var err error
		log, err := logs.NewLogger("XchainP2pProxy")
		if err != nil {
			return nil
		}
		if config.GetCaConfig().CaSwitch {
			creds, err := util_cert.GenCreds()
			if err != nil {
				log.Error("XchainP2pProxy.GetXchainP2pProxy: InitCA GenCreds failed", "err", err)
				return nil
			}
			conn, err = grpc.Dial(config.GetXchainServer().Host, grpc.WithTransportCredentials(creds), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize), grpc.MaxCallSendMsgSize(MaxMessageSize)))
			if err != nil {
				log.Error("XchainP2pProxy.GetXchainP2pProxy: InitCA Dial failed", "err", err)
				return nil
			}
		} else {
			conn, err = grpc.Dial(config.GetXchainServer().Host, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize), grpc.MaxCallSendMsgSize(MaxMessageSize)))
			if err != nil {
				log.Error("XchainP2pProxy.GetXchainP2pProxy: Init Dial failed", "err", err)
				return nil
			}
		}

		xchainProxy = &XchainP2pProxy{
			conn: conn,
			log:  log,
		}
		return xchainProxy
	}

	connState := xchainProxy.conn.GetState().String()
	if connState == "TRANSIENT_FAILURE" || connState == "SHUTDOWN" || connState == "Invalid-State" {
		//初始化
		var conn *grpc.ClientConn
		var err error
		if config.GetCaConfig().CaSwitch {
			creds, err := util_cert.GenCreds()
			if err != nil {
				xchainProxy.log.Error("XchainP2pProxy.GetXchainP2pProxy: CA failed", "err", err)
				return nil
			}
			conn, err = grpc.Dial(config.GetXchainServer().Host, grpc.WithTransportCredentials(creds), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize), grpc.MaxCallSendMsgSize(MaxMessageSize)))
			if err != nil {
				xchainProxy.log.Error("XchainP2pProxy.GetXchainP2pProxy: CA Dail failed", "err", err)
				return nil
			}
		} else {
			conn, err = grpc.Dial(config.GetXchainServer().Host, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize), grpc.MaxCallSendMsgSize(MaxMessageSize)))
			if err != nil {
				xchainProxy.log.Error("XchainP2pProxy.GetXchainP2pProxy: Dial failed", "err", err)
				return nil
			}
		}
		xchainProxy.conn = conn
	}

	return xchainProxy
}

// @todo 复用一个conn 会出问题, 后面再排查下原因吧
func (cli *XchainP2pProxy) Defer() {
	//cli.conn.Close()
}

func (cli *XchainP2pProxy) newClient() (p2p.P2PServiceClient, error) {
	client := p2p.NewP2PServiceClient(cli.conn)
	return client, nil
}

// SendMessage send message to a peer
func (cli *XchainP2pProxy) SendMessage(ctx context.Context, msg *p2p.XuperMessage) error {
	client, err := cli.newClient()
	if err != nil {
		cli.log.Error("XchainP2pProxy.SendMessage: newClient error", "err", err)
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := client.SendP2PMessage(ctx)
	if err != nil {
		cli.log.Error("XchainP2pProxy.SendMessage: SendP2PMessage error", "err", err)
		return err
	}
	defer stream.CloseSend()
	err = stream.Send(msg)
	if err != nil {
		cli.log.Error("XchainP2pProxy.SendMessage: Send error", "err", err)
		return err
	}
	if err == io.EOF {
		return nil
	}
	return err
}

// SendMessageWithResponse send message to a peer with responce
func (cli *XchainP2pProxy) SendMessageWithResponse(ctx context.Context, msg *p2p.XuperMessage) (*p2p.XuperMessage, error) {
	client, err := cli.newClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := client.SendP2PMessage(ctx)
	if err != nil {
		return nil, err
	}

	res := &p2p.XuperMessage{}
	waitc := make(chan struct{})
	go func() {
		for {
			res, err = stream.Recv()
			if err == io.EOF {
				close(waitc)
				return
			}
			if err != nil {
				close(waitc)
				return
			}
			if res != nil {
				close(waitc)
				return
			}
		}
	}()

	err = stream.Send(msg)
	if err != nil {
		stream.CloseSend()
		return nil, err
	}
	stream.CloseSend()
	<-waitc
	return res, err
}
