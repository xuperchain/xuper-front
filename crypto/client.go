/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package crypto

import (
	"encoding/json"
	"io/ioutil"
	"strings"
	"sync"

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

//key的加密类型,front只能有一种模式,国密(1)或者非国密(0)
var keyType int = -1

// 全局变量修改加锁
var mu sync.Mutex

func KeyIsGM() (bool, error) {
	mu.Lock()
	defer mu.Unlock()
	if keyType == 1 { //国密
		return true, nil
	} else if keyType >= 0 { //非国密
		return false, nil
	} else {
		keyPEMBlock, err := ioutil.ReadFile(config.GetKeys() + "public.key")
		if err != nil {
			return false, err
		}
		ecdsaPublicKey := &account.ECDSAPublicKey{}
		err = json.Unmarshal(keyPEMBlock, ecdsaPublicKey)
		if err != nil {
			return false, err
		}
		if strings.Contains(strings.ToLower(ecdsaPublicKey.Curvname), "sm") {
			keyType = 1
			return true, nil
		} else {
			keyType = 0
			return false, nil
		}
	}
}
