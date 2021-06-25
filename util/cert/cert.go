/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package cert

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	"google.golang.org/grpc/credentials"

	"github.com/xuperchain/xuper-front/config"
)

const CACERT = "cacert.pem"
const CERT = "cert.pem"
const PRIVATEKEY = "private.key"
const NODEHDPRIKEY = "hd_private.key"

func GenCreds() (credentials.TransportCredentials, error) {
	crt, err := ioutil.ReadFile(config.GetConfig().XchainServer.TlsPath + "/" + CACERT)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(crt)
	if !ok {
		return nil, err
	}
	certificate, err := tls.LoadX509KeyPair(config.GetConfig().XchainServer.TlsPath+"/"+CERT,
		config.GetConfig().XchainServer.TlsPath+"/"+PRIVATEKEY)

	//cn := config.GetNet() + ".server.com"
	cn := config.GetNet()

	var tlsCOnfig *tls.Config

	tlsCOnfig = &tls.Config{
		ServerName:   cn,
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
		ClientCAs:    certPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}

	creds := credentials.NewTLS(
		tlsCOnfig,
	)

	return creds, nil
}
