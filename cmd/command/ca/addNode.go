/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package ca

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xuperchain/xuper-front/config"

	serv_ca "github.com/xuperchain/xuper-front/service/ca"
)

func NewAddNodeCommand() *cobra.Command {
	var address string
	var net string
	var keys string
	var adminAddress string

	addNodeCommand := &cobra.Command{
		Use:   "addNode",
		Short: "request ca and add a node for the net using keys",
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddNode(address, net, adminAddress)
		},
	}
	addNodeCommand.PersistentFlags().StringVar(&address, "Addr", "", "Address to add")
	addNodeCommand.PersistentFlags().StringVar(&adminAddress, "Admin", "", "Address for net admin")
	addNodeCommand.PersistentFlags().StringVar(&net, "Net", config.GetNet(), "the name of the net")
	addNodeCommand.PersistentFlags().StringVar(&keys, "Keys", config.GetKeys(), "the path of the keys")

	config.SetKeys(keys)

	return addNodeCommand
}

func runAddNode(address string, net, adminAddress string) error {

	err := serv_ca.AddNode(address, net, adminAddress)
	if err != nil {
		fmt.Println("create node failed,", err)
	} else {
		fmt.Println("add node success")
	}
	return err
}
