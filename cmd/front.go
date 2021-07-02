/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

	"github.com/spf13/cobra"

	cmd_ca "github.com/xuperchain/xuper-front/cmd/command/ca"
	"github.com/xuperchain/xuper-front/config"
	"github.com/xuperchain/xuper-front/dao"
	"github.com/xuperchain/xuper-front/logs"
	server_xchain "github.com/xuperchain/xuper-front/server/xchain"
	serv_ca "github.com/xuperchain/xuper-front/service/ca"
)

const defaultConfigFile = "./conf/front.yaml"

func newFrontCommand() (*cobra.Command, error) {
	var configFile string

	frontCmd := &cobra.Command{
		Use:   "front",
		Short: "front",
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 启动front
			sigc := make(chan os.Signal, 1)
			signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigc)
			quit := make(chan int)

			startFront(quit)

			for {
				select {
				case <-sigc:
					pprof.StopCPUProfile()
					return nil
				case <-quit:
					pprof.StopCPUProfile()
					return nil
				}
			}
		},
	}
	//flags := frontCmd.Flags()
	//flags.StringVar(&configFile, "config-file", defaultConfigFile, "CA Server configuration file")

	frontCmd.PersistentFlags().StringVar(&configFile, "config-file", defaultConfigFile, "CA Server configuration file")

	// 从配置文件中加载配置
	config.InstallFrontConfig(configFile)

	dao.InitTables()

	return frontCmd, nil
}

func runFrontServer() error {
	rootCmd, err := newFrontCommand()
	if err != nil {
		return err
	}
	logs.InitLog(config.GetLog().FrontName, config.GetLog().Path)
	serv_ca.StartCaHandler()
	rootCmd.AddCommand(cmd_ca.NewAddNodeCommand())
	rootCmd.AddCommand(cmd_ca.NewGetCertCommand())
	rootCmd.AddCommand(cmd_ca.NewGetRevokeListCmd())

	return rootCmd.Execute()
}

func main() {
	// init config
	onError := func(err error) {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	// run
	if err := runFrontServer(); err != nil {
		onError(err)
	}
}

// startFront 启动front服务, 监听优雅关停信号
func startFront(quit chan int) {
	// 获取配置
	caSwitch := config.GetCaConfig().CaSwitch
	//localCaSwitch := config.GetCaConfig().LocalCaSwitch
	netName := config.GetNet()

	// 1.联盟网络获取证书
	if caSwitch {
		// 1.拉取证书
		if err := serv_ca.GetAndWriteCert(netName); err != nil {
			// 拉取证书失败
		}

		// 2.定时拉取过期证书
		if err := serv_ca.GetRevokeListRegularly(netName); err != nil {
			// 拉取撤销证书失败
		}
	}

	// 2.启动xchain节点代理,内部判断caSwitch
	go server_xchain.StartXchainProxyServer(quit)
}
