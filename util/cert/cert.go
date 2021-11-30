/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package cert

import (
	defaulttls "crypto/tls"
	defaultx509 "crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"strings"

	tls "github.com/tjfoc/gmsm/gmtls"
	"github.com/tjfoc/gmsm/gmtls/gmcredentials"
	"github.com/tjfoc/gmsm/x509"
	"google.golang.org/grpc/credentials"

	"github.com/xuperchain/xuper-front/config"
)

const CACERT = "cacert.pem"
const CERT = "cert.pem"
const PRIVATEKEY = "private.key"
const NODEHDPRIKEY = "hd_private.key"

var creds credentials.TransportCredentials = nil

func GenCreds() (credentials.TransportCredentials, error) {
	if creds != nil {
		return creds, nil
	}
	crt, err := ioutil.ReadFile(config.GetConfig().XchainServer.TlsPath + "/" + CACERT)
	if err != nil {
		return nil, err
	}
	isgm, err := IsGM()
	if err != nil {
		return nil, err
	}
	if isgm {
		certPool := x509.NewCertPool()
		ok := certPool.AppendCertsFromPEM(crt)
		if !ok {
			return nil, err
		}
		certificate, err := tls.LoadX509KeyPair(config.GetConfig().XchainServer.TlsPath+"/"+CERT,
			config.GetConfig().XchainServer.TlsPath+"/"+PRIVATEKEY)
		if err != nil {
			return nil, err
		}
		//cn := config.GetNet() + ".server.com"
		cn := config.GetNet()
		tlsCOnfig := &tls.Config{
			GMSupport:    tls.NewGMSupport(),
			ServerName:   cn,
			Certificates: []tls.Certificate{certificate, certificate},
			RootCAs:      certPool,
			ClientCAs:    certPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		}
		creds = gmcredentials.NewTLS(
			tlsCOnfig,
		)

		return creds, nil
	} else {

		certPool := defaultx509.NewCertPool()
		ok := certPool.AppendCertsFromPEM(crt)
		if !ok {
			return nil, err
		}
		certificate, err := defaulttls.LoadX509KeyPair(config.GetConfig().XchainServer.TlsPath+"/"+CERT,
			config.GetConfig().XchainServer.TlsPath+"/"+PRIVATEKEY)
		if err != nil {
			return nil, err
		}
		//cn := config.GetNet() + ".server.com"
		cn := config.GetNet()
		tlsCOnfig := &defaulttls.Config{
			ServerName:   cn,
			Certificates: []defaulttls.Certificate{certificate},
			RootCAs:      certPool,
			ClientCAs:    certPool,
			ClientAuth:   defaulttls.RequireAndVerifyClientCert,
		}
		creds = credentials.NewTLS(
			tlsCOnfig,
		)
		return creds, nil
	}

}

//证书类型,front只能有一种模式,国密(1)或者非国密(0)
var cryptoType int = -1

func IsGM() (bool, error) {
	if cryptoType == 1 { //国密
		return true, nil
	} else if cryptoType >= 0 { //非国密
		return false, nil
	} else {
		cacert, err := ioutil.ReadFile(config.GetConfig().XchainServer.TlsPath + "/" + CACERT)
		if err != nil {
			return false, err
		}
		pb, _ := pem.Decode(cacert)
		x509cert, err := x509.ParseCertificate(pb.Bytes)
		if err != nil {
			return false, err
		}
		if strings.Contains(strings.ToLower(x509cert.SignatureAlgorithm.String()), "sm") {
			cryptoType = 1
			return true, nil
		} else {
			cryptoType = 0
			return false, nil
		}
	}
}
