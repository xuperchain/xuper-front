/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package crypto

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/xuperchain/crypto/client/service/base"
	"github.com/xuperchain/crypto/client/service/gm"
	"github.com/xuperchain/crypto/client/service/xchain"
	"github.com/xuperchain/crypto/gm/account"
	"github.com/xuperchain/xuper-front/config"
)

func GetCryptoClient() base.CryptoClient {
	xcc := new(xchain.XchainCryptoClient)
	return base.CryptoClient(xcc)
}

func GetGMCryptoClient() base.CryptoClient {
	xcc := new(gm.GmCryptoClient)
	return base.CryptoClient(xcc)
}

func KeyIsGM() (bool, error) {
	keyPEMBlock, err := ioutil.ReadFile(config.GetKeys() + "public.key")
	if err != nil {
		return false, err
	}
	ecdsaPublicKey := &account.ECDSAPublicKey{}
	err = json.Unmarshal(keyPEMBlock, ecdsaPublicKey)
	if err != nil {
		return false, err
	}
	if strings.Contains(ecdsaPublicKey.Curvname, "SM") {
		return true, nil
	} else {
		return false, nil
	}
}
