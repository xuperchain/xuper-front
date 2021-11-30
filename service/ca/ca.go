/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package ca

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/xuperchain/crypto/client/service/base"
	"github.com/xuperchain/xuper-front/config"
	"github.com/xuperchain/xuper-front/crypto"
	"github.com/xuperchain/xuper-front/dao"
	logs "github.com/xuperchain/xuper-front/logs"
	"github.com/xuperchain/xuper-front/pb"
	util_cert "github.com/xuperchain/xuper-front/util/cert"
	util_file "github.com/xuperchain/xuper-front/util/file"
	"google.golang.org/grpc"
)

var log *logs.LogFitter

func StartCaHandler() {
	log, _ = logs.NewLogger("CaServer")
}

type CurrentCert struct {
	Cert       string
	PrivateKey string
	CaCert     string
}

// 访问ca的签名校验, 检验的data根据接口不同而不同
func sign(data []byte) (*pb.Sign, error) {

	// 判断公钥是否是GM，来解析AK（是否是GM）
	b, err := crypto.KeyIsGM()
	if err != nil {
		log.Warn("get key is gm failed", "err", err)
		return nil, err
	}
	var cryptoClient base.CryptoClient
	if b {
		cryptoClient = crypto.GetGMCryptoClient()
	} else {
		cryptoClient = crypto.GetCryptoClient()
	}
	// 获取账户
	privateKey, err := cryptoClient.GetEcdsaPrivateKeyFromFile(config.GetKeys() + "private.key")
	if err != nil {
		log.Warn("CaServer.sign: can not get `private.key`", "err", err)
		return nil, err
	}
	pubKey, err := cryptoClient.GetEcdsaPublicKeyJsonFormatStr(privateKey)
	if err != nil {
		log.Warn("CaServer.sign: can not get `public.key`", "err", err)
		return nil, err
	}

	address, err := cryptoClient.GetAddressFromPublicKey(&privateKey.PublicKey)

	// 对数据进行加密
	nonce := strconv.Itoa(int(time.Now().Unix()))
	sign, err := cryptoClient.SignECDSA(privateKey, []byte(string(data)+nonce))
	if err != nil {
		log.Warn("CaServer.sign: sign failed", "err", err)
		return nil, err
	}
	return &pb.Sign{
		Address:   address,
		PublicKey: pubKey,
		Sign:      sign,
		Nonce:     nonce,
	}, nil

}

// 请求ca增加节点
func AddNode(address, net, adminAddress string) error {
	request := &pb.EnrollNodeRequest{
		Net:          net,
		AdminAddress: adminAddress,
		Address:      address,
	}

	conn, err := grpc.Dial(config.GetCaConfig().Host, grpc.WithInsecure())
	if err != nil {
		log.Warn("CaServer.AddNode: create conn to ca failed")
		return err
	}
	client := pb.NewCaserverClient(conn)
	ctx := context.Background()

	sign, err := sign([]byte(string(request.Address + request.Net)))
	if err != nil {
		log.Warn("CaServer.AddNode: sign error", "err", err)
		return err
	}
	request.Sign = sign

	_, err = client.NodeEnroll(ctx, request)
	if err != nil {
		log.Warn("CaServer.AddNode: add node to ca failed", "err", err)
		return err
	}
	return nil
}

// 请求ca 获取节点的证书, 并写入文件
func GetAndWriteCert(net string) error {
	log.Info("CaServer.GetAndWriteCert: get node's cert")
	path := config.GetTlsPath()

	// 判断文件是否存在, 取一个文件
	if ok := util_file.Exist(path + util_cert.CACERT); ok {
		log.Warn("CaServer.GetAndWriteCert: cert is existed")
		return nil
	}

	if ok, _ := util_file.PathExists(config.GetTlsPath()); !ok {
		os.MkdirAll(config.GetTlsPath(), os.ModePerm)
	}

	// 先拉取下证书
	cert, nodeHdPriKey, err := GetCurrentCert(net)
	if err != nil {
		return err
	}
	// 存储节点一级子私钥
	if nodeHdPriKey != "" {
		err = util_file.WriteFileUsingFilename(path+util_cert.NODEHDPRIKEY, []byte(nodeHdPriKey))
	}
	log.Error("CaServer.GetAndWriteCert: get create ca conn failed", "err", nodeHdPriKey)
	// 写文件
	err = util_file.WriteFileUsingFilename(path+util_cert.CACERT, []byte(cert.CaCert))
	err = util_file.WriteFileUsingFilename(path+util_cert.CERT, []byte(cert.Cert))
	err = util_file.WriteFileUsingFilename(path+util_cert.PRIVATEKEY, []byte(cert.PrivateKey))
	return err
}

// 请求ca获取本节点的证书
func GetCurrentCert(net string) (*CurrentCert, string, error) {
	// 拿不到证书 3秒超时
	conn, err := grpc.Dial(config.GetCaConfig().Host, grpc.WithInsecure(),
		grpc.WithTimeout(time.Second*time.Duration(3)))
	if err != nil {
		log.Error("CaServer.GetCurrentCert: create ca conn failed", "err", err)
		return nil, "", err
	}
	defer conn.Close()
	// 判断公钥 public.key 是否是GM，来解析AK（是否是GM）
	b, err := crypto.KeyIsGM()
	if err != nil {
		log.Warn("get key is gm failed", "err", err)
		return nil, "", err
	}
	var cryptoClient base.CryptoClient
	if b {
		cryptoClient = crypto.GetGMCryptoClient()
	} else {
		cryptoClient = crypto.GetCryptoClient()
	}

	// get publicKey
	publicKey, err := cryptoClient.GetEcdsaPublicKeyFromFile(config.GetKeys() + "public.key")
	address, err := cryptoClient.GetAddressFromPublicKey(publicKey)
	if err != nil {
		log.Warn("CaServer.GetCurrentCert: get address failed", "err", err)
	}

	sign, err := sign([]byte(address + net))
	if err != nil {
		log.Error("CaServer.GetCurrentCert: sign error", "err", err)
		return nil, "", err
	}

	c := pb.NewCaserverClient(conn)
	ret, err := c.GetCurrentCert(context.Background(), &pb.CurrentCertRequest{
		Sign:    sign,
		Net:     net,
		Address: address,
	})
	if err != nil {
		log.Error("CaServer.GetCurrentCert: get current cert request failed")
		return nil, "", err
	}
	return &CurrentCert{
		Cert:       ret.Cert,
		PrivateKey: ret.PrivateKey,
		CaCert:     ret.CaCert,
	}, ret.NodeHdPriKey, nil
}

// 获取证书的撤销列表
func GetRevokeList(net string) error {
	// 从数据库获取最新的serial_num
	revokeDao := dao.RevokeDao{
		Log: log,
	}
	serialNum, err := revokeDao.GetLatestSerialNum(net)
	if err != nil {
		log.Warn("CaServer.GetRevokeList: cat get latest serial num")
	}

	request := &pb.RevokeListRequest{
		Net:       net,
		SerialNum: serialNum,
	}

	conn, err := grpc.Dial(config.GetCaConfig().Host, grpc.WithInsecure())
	if err != nil {
		log.Error("CaServer.GetRevokeList: create ca conn failed", "err", err)
		return err
	}
	defer conn.Close()

	sign, err := sign([]byte(serialNum + net))
	if err != nil {
		log.Error("CaServer.GetRevokeList: sign error", "err", err)
		return err
	}

	client := pb.NewCaserverClient(conn)
	ctx := context.Background()
	request.Sign = sign

	ret, err := client.GetRevokeList(ctx, request)
	if err != nil {
		log.Error("CaServer.GetRevokeList: get revoke list request failed")
		return err
	}

	// 保存到数据库
	for _, row := range ret.List {
		_, err = revokeDao.Insert(&dao.Revoke{
			Id:         int(row.Id),
			Net:        net,
			SerialNum:  row.SerialNum,
			CreateTime: int(row.CreateTime),
		})
		if err != nil {
			// @todo batch insert?
			log.Warn("CaServer.GetRevokeList: insert into revoke failed", "err", err, "id", row.Id)
		}
	}
	return nil
}

// 启动定时器拉取撤销证书
func GetRevokeListRegularly(net string) error {
	go func() {
		for {
			// 拉取证书撤销列表
			err := GetRevokeList(net)
			if err != nil {
				log.Error("CaServer.GetRevokeListRegularly: get revoke list", "err", err)
			}
			now := time.Now()
			// 每十分钟执行一次
			next := now.Add(time.Minute * 10)
			next = time.Date(next.Year(), next.Month(), next.Day(), next.Hour(), next.Minute(), next.Second(), 0,
				next.Location())
			t := time.NewTimer(next.Sub(now))
			<-t.C
		}
	}()
	return nil
}

// 证书是否有效,使用serialNum进行判断
func IsValidCert(serialNum string) bool {
	revoleNodeDao := dao.RevokeDao{
		Log: log,
	}
	ret, err := revoleNodeDao.GetBySerialNum(serialNum)
	// 数据库中查不到
	if err != nil || ret == nil {
		return true
	}
	return false
}
