/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package ca

import (
	"github.com/spf13/cobra"

	"github.com/xuperchain/xuper-front/config"
	serv_ca "github.com/xuperchain/xuper-front/service/ca"
)

func NewGetRevokeListCmd() *cobra.Command {
	var net string
	var keys string

	getRevokeList := &cobra.Command{
		Use:   "getRevokeList",
		Short: "get revokeList from the caserver",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetRevokeList(net)
		},
	}
	getRevokeList.PersistentFlags().StringVar(&net, "Net", config.GetNet(), "the name of the net")
	getRevokeList.PersistentFlags().StringVar(&keys, "Key", config.GetKeys(), "the path of the keys")

	config.SetKeys(keys)

	return getRevokeList
}

func runGetRevokeList(net string) error {
	err := serv_ca.GetRevokeList(net)
	return err
}
