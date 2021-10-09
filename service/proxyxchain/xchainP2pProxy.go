/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package proxyxchain

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/xuperchain/xuper-front/config"
	logs "github.com/xuperchain/xuper-front/logs"
	util_cert "github.com/xuperchain/xuper-front/util/cert"
	p2p "github.com/xuperchain/xupercore/protos"
	"google.golang.org/grpc"
)

var TimeoutDuration = 3 * time.Second

type XchainP2pProxy struct {
	conn *grpc.ClientConn
	log  logs.Logger
}

var (
	xchainProxy *XchainP2pProxy
	proxyMtx    sync.Mutex
)

func GetXchainP2pProxy() *XchainP2pProxy {
	proxyMtx.Lock()
	defer proxyMtx.Unlock()
	if xchainProxy == nil {
		//初始化
		var conn *grpc.ClientConn
		var err error
		log, err := logs.NewLogger("XchainP2pProxy")
		if err != nil {
			return nil
		}
		maxMessageSize := config.GetXchainServer().MaxMsgSize
		if config.GetCaConfig().CaSwitch {
			creds, err := util_cert.GenCreds()
			if err != nil {
				log.Error("XchainP2pProxy.GetXchainP2pProxy: InitCA GenCreds failed", "err", err)
				return nil
			}
			conn, err = grpc.Dial(config.GetXchainServer().Host, grpc.WithTransportCredentials(creds), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMessageSize), grpc.MaxCallSendMsgSize(maxMessageSize)))
			if err != nil {
				log.Error("XchainP2pProxy.GetXchainP2pProxy: InitCA Dial failed", "err", err)
				return nil
			}
		} else {
			conn, err = grpc.Dial(config.GetXchainServer().Host, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMessageSize), grpc.MaxCallSendMsgSize(maxMessageSize)))
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

	// xchainProxy.conn需要一直维持一个链接
	connState := xchainProxy.conn.GetState().String()
	if connState == "TRANSIENT_FAILURE" || connState == "SHUTDOWN" || connState == "Invalid-State" {
		//重置
		var conn *grpc.ClientConn
		var err error
		maxMessageSize := config.GetXchainServer().MaxMsgSize
		if config.GetCaConfig().CaSwitch {
			creds, err := util_cert.GenCreds()
			if err != nil {
				xchainProxy.log.Error("XchainP2pProxy.GetXchainP2pProxy: CA failed", "err", err)
				return nil
			}
			conn, err = grpc.Dial(config.GetXchainServer().Host, grpc.WithTransportCredentials(creds), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMessageSize), grpc.MaxCallSendMsgSize(maxMessageSize)))
			if err != nil {
				xchainProxy.log.Error("XchainP2pProxy.GetXchainP2pProxy: CA Dail failed", "err", err)
				return nil
			}
		} else {
			conn, err = grpc.Dial(config.GetXchainServer().Host, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMessageSize), grpc.MaxCallSendMsgSize(maxMessageSize)))
			if err != nil {
				xchainProxy.log.Error("XchainP2pProxy.GetXchainP2pProxy: Dial failed", "err", err)
				return nil
			}
		}
		xchainProxy.Defer()
		xchainProxy.conn = conn
		xchainProxy.log.Info("XchainP2pProxy: connection re-build.")
	}

	return xchainProxy
}

// @todo 复用一个conn 会出问题, 后面再排查下原因吧
func (cli *XchainP2pProxy) Defer() {
	cli.log.Info("XchainP2pProxy: close connection.")
	cli.conn.Close()
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
		cli.log.Error("XchainP2pProxy.SendMessage: Send EOF.")
		return nil
	}
	// wait for server
	stream.Recv()
	return err
}

// SendMessageWithResponse send message to a peer with responce
func (cli *XchainP2pProxy) SendMessageWithResponse(ctx context.Context, msg *p2p.XuperMessage) (*p2p.XuperMessage, error) {
	// front proxy作为一个客户端向它直连的xchain host请求消息，并期待xchain host的返回
	client, err := cli.newClient()
	if err != nil {
		return nil, err
	}
	stream, err := client.SendP2PMessage(ctx)
	if err != nil {
		return nil, err
	}
	defer stream.CloseSend()

	err = stream.Send(msg)
	if err != nil {
		cli.log.Error("SendMessageWithResponse error", "log_id", msg.GetHeader().GetLogid(), "error", err)
		return nil, err
	}

	resp, err := stream.Recv()
	if err != nil {
		cli.log.Error("SendMessageWithResponse Recv error", "log_id", resp.GetHeader().GetLogid(), "error", err.Error(), "from", msg.GetHeader().From,
			"type", msg.Header.Type)
		return nil, err
	}

	return resp, err
}
