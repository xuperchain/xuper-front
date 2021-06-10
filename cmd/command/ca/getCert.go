/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package ca

import (
	"github.com/spf13/cobra"

	"github.com/xuperchain/xuper-front/config"
	serv_ca "github.com/xuperchain/xuper-front/service/ca"
)

func NewGetCertCommand() *cobra.Command {
	var keys string
	var net string
	var path string

	getCertCommand := &cobra.Command{
		Use:   "getCert",
		Short: "get the cert from caserver using keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetCert(net)
		},
	}

	getCertCommand.PersistentFlags().StringVar(&keys, "Key", config.GetKeys(), "the path of the keys")
	getCertCommand.PersistentFlags().StringVar(&net, "Net", config.GetNet(), "the name of the net")
	getCertCommand.PersistentFlags().StringVar(&path, "Path", config.GetTlsPath(), "the path of the cert")

	config.SetKeys(keys)
	config.SetTlsPath(path)

	return getCertCommand
}

func runGetCert(net string) error {
	return serv_ca.GetAndWriteCert(net)
}
