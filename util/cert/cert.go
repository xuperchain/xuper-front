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

func GenCreds() (credentials.TransportCredentials, error) {
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
			GMSupport:    &tls.GMSupport{},
			ServerName:   cn,
			Certificates: []tls.Certificate{certificate, certificate},
			RootCAs:      certPool,
			ClientCAs:    certPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		}
		creds := gmcredentials.NewTLS(
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
		creds := credentials.NewTLS(
			tlsCOnfig,
		)
		return creds, nil
	}

}

func IsGM() (bool, error) {
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
		return true, nil
	} else {
		return false, nil
	}
}
