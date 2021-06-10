/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package prxyxchain

import (
	"context"
	"io"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/xuperchain/xuper-front/config"
	util_cert "github.com/xuperchain/xuper-front/util/cert"
	"github.com/xuperchain/xuper-front/xuperp2p"
)

type XchainP2pProxy struct {
	host string
	conn *grpc.ClientConn
}

var xchainProxy *XchainP2pProxy

func GetXchainP2pProxy() *XchainP2pProxy {
	if xchainProxy == nil {
		//初始化
		var conn *grpc.ClientConn
		var err error
		if config.GetCaConfig().CaSwitch {
			creds, err := util_cert.GenCreds()
			if err != nil {
				log.Printf("failed to serve: %v\n", err)
				return nil
			}
			conn, err = grpc.Dial(config.GetXchainServer().Host, grpc.WithTransportCredentials(creds))
			if err != nil {
				log.Printf("failed to serve: %v\n", err)
				return nil
			}
		} else {
			conn, err = grpc.Dial(config.GetXchainServer().Host, grpc.WithInsecure())
			if err != nil {
				log.Printf("failed to serve: %v\n", err)
				return nil
			}
		}

		xchainProxy = &XchainP2pProxy{
			conn: conn,
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
				log.Printf("failed to serve: %v\n", err)
				return nil
			}
			conn, err = grpc.Dial(config.GetXchainServer().Host, grpc.WithTransportCredentials(creds))
			if err != nil {
				log.Printf("failed to serve: %v\n", err)
				return nil
			}
		} else {
			conn, err = grpc.Dial(config.GetXchainServer().Host, grpc.WithInsecure())
			if err != nil {
				log.Printf("failed to serve: %v\n", err)
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

func (cli *XchainP2pProxy) newClient() (xuperp2p.P2PServiceClient, error) {
	client := xuperp2p.NewP2PServiceClient(cli.conn)
	return client, nil
}

// SendMessage send message to a peer
func (cli *XchainP2pProxy) SendMessage(ctx context.Context, msg *xuperp2p.XuperMessage) error {
	client, err := cli.newClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := client.SendP2PMessage(ctx)
	if err != nil {
		return err
	}
	waitc := make(chan struct{})
	go func() {
		for {
			_, err = stream.Recv()
			if err == io.EOF {
				close(waitc)
				return
			}
			if err != nil {
				close(waitc)
				return
			}
		}
	}()
	err = stream.Send(msg)
	if err != nil {
		stream.CloseSend()
		return err
	}
	stream.CloseSend()
	<-waitc
	if err == io.EOF {
		return nil
	}
	log.Debug("proxy SendMessage msg:", msg)
	return err
}

// SendMessageWithResponse send message to a peer with responce
func (cli *XchainP2pProxy) SendMessageWithResponse(ctx context.Context, msg *xuperp2p.XuperMessage) (*xuperp2p.XuperMessage, error) {
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

	res := &xuperp2p.XuperMessage{}
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
	log.Debug("proxy SendMessageWithResponse ret:", res, err)
	return res, err
}
