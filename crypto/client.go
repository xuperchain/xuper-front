/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package crypto

import (
	"github.com/xuperchain/crypto/client/service/xchain"
)

// GetCryptoClient get crypto client
func GetCryptoClient() *xchain.XchainCryptoClient {
	return &xchain.XchainCryptoClient{}
}
