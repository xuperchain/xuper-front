/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package config

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

var config *Config

type Config struct {
	XchainServer XchainServer `yaml:"xchainServer,omitempty"`
	DbConfig     DbConfig     `yaml:"dbConfig,omitempty"`
	CaConfig     CaConfig     `yaml:"caConfig,omitempty"`
	NetName      string       `yaml:"netName,omitempty"`
	Keys         string       `yaml:"keys,omitempty"`
	Log          Log          `yaml:"log,omitempty"`
}

//SetDefaults set default values
func (c *Config) SetDefaults() {
	c.XchainServer = XchainServer{}
	c.XchainServer.SetDefaults()

	c.DbConfig = DbConfig{}
	c.CaConfig = CaConfig{}
	c.Log = Log{}
	c.Log.SetDefaults()
}

type XchainServer struct {
	Host      string `yaml:"host,omitempty"`
	Port      string `yaml:"port,omitempty"`
	Rpc       string `yaml:"rpc,omitempty"`
	TlsPath   string `yaml:"tlsPath,omitempty"`
	TlsVerify bool   `yaml:"tlsVerify,omitempty"`
	Master    string `yaml:"master,omitempty"`
	Http      string `yaml:"http, omitempty"`
}

//SetDefaults set default values
func (c XchainServer) SetDefaults() {
}

type DbConfig struct {
	DbType          string `yaml:"dbType,omitempty"`
	DbPath          string `yaml:"dbPath,omitempty"`
	MysqlDbUser     string `yaml:"mysqlDbUser,omitempty"`
	MysqlDbPwd      string `yaml:"mysqlDbPwd,omitempty"`
	MysqlDbHost     string `yaml:"mysqlDbHost,omitempty"`
	MysqlDbPort     string `yaml:"mysqlDbPort,omitempty"`
	MysqlDbDatabase string `yaml:"mysqlDbDatabase,omitempty"`
}

//SetDefaults set default values
func (c DbConfig) SetDefaults() {
}

type CaConfig struct {
	CaSwitch bool   `yaml:"caSwitch,omitempty`
	Host     string `yaml:"host,omitempty"`
}

//SetDefaults set default values
func (c CaConfig) SetDefaults() {
}

type Log struct {
	Level     string `yaml:"level,omitempty"`
	Path      string `yaml:"path,omitempty"`
	FrontName string `yaml:"frontName,omitempty"`
}

//SetDefaults set default values
func (c *Log) SetDefaults() {
	c.Level = "debug"
	c.Path = "./logs"
	c.FrontName = "xfront"
}

func InstallFrontConfig(configFile string) error {
	// 从配置文件中加载配置
	config = &Config{}
	config.SetDefaults()

	filePath, fileName := filepath.Split(configFile)
	file := strings.TrimSuffix(fileName, path.Ext(fileName))
	viper.AddConfigPath(filePath)
	viper.SetConfigName(file)

	viper.SetDefault("caConfig.caSwitch", "true")
	viper.SetDefault("caConfig.localCaSwitch", "true")

	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("Config.InstallFrontConfig: Read config file error, %v", err.Error())
	}
	if err := viper.Unmarshal(config); err != nil {
		return fmt.Errorf("Config.InstallFrontConfig: Unmarshal config from file error, %v", err.Error())
	}

	// 监听配置变化, 重启加载
	//viper.WatchConfig()
	//viper.OnConfigChange(func(e fsnotify.Event) {
	//	// 配置发生变化则重新加载
	//	config = &Config{}
	//	viper.Unmarshal(config)
	//	printConfig()
	//})

	return nil
}

func printConfig() *Config {
	return config
}

func GetConfig() *Config {
	return config
}

func GetXchainServer() XchainServer {
	return config.XchainServer
}

func GetCaConfig() CaConfig {
	return config.CaConfig
}

func GetDBConfig() *DbConfig {
	return &config.DbConfig
}

func SetKeys(keys string) {
	config.Keys = keys
}

func SetTlsPath(path string) {
	config.XchainServer.TlsPath = path
}

func GetNet() string {
	return config.NetName
}

func GetKeys() string {
	path := config.Keys
	if strings.LastIndex(path, "/") != len([]rune(path))-1 {
		path = path + "/"
	}
	return path
}

func GetTlsPath() string {
	path := config.XchainServer.TlsPath
	if strings.LastIndex(path, "/") != len([]rune(path))-1 {
		path = path + "/"
	}
	return path
}

func GetLog() Log {
	return config.Log
}
