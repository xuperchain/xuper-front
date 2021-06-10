/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package dao

import (
	"errors"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	"github.com/xuperchain/xuper-front/config"
)

// 兼容旧rovoke表,存在则rename为revoke_node
const isRevokeTableExistSql = `select count(*)  from sqlite_master where type='table' and name = 'revoke';`
const renameTableSql = `ALTER TABLE revoke RENAME TO revoke_node;`
const defaultSQLiteSchema = `
create table if not exists revoke_node (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
	net varchar(100) NOT NULL,
    serial_num varchar(100) NOT NULL,
    create_time int(10) NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_revoke_serial ON revoke_node(serial_num);
`

// mysql revoke关键字冲突,修改表名revoke_node
const defaultMysqlSchema = `
create table if not exists revoke_node(
    id INTEGER PRIMARY KEY AUTO_INCREMENT NOT NULL,
    net varchar(100) NOT NULL,
    serial_num varchar(100) NOT NULL,
    create_time int(10) NOT NULL,
    UNIQUE KEY uidx_revoke_serial(serial_num)
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8 COMMENT='节点撤销表';
`

type CaDb struct {
	db *sqlx.DB
}

var caDb *CaDb

func GetDbInstance() *CaDb {
	if caDb == nil {
		caDb = NewCaDb()
	}
	return caDb
}

func NewCaDb() (connection *CaDb) {
	var err error
	var db *sqlx.DB
	// db, err = sqlx.Connect(config.GetDBConfig().DbType, config.GetDBConfig().DbPath)
	if config.GetDBConfig().DbType == "mysql" {
		connect, connect_error := InitMysqlConnect()
		if connect_error != nil {
			log.Print(connect_error)
		}
		db, err = sqlx.Connect(config.GetDBConfig().DbType, connect)
	} else {
		db, err = sqlx.Connect(config.GetDBConfig().DbType, config.GetDBConfig().DbPath)
	}
	if err != nil {
		log.Print(err)
	}

	return &CaDb{
		db: db,
	}
}

//DB目前仅支持sqlite3, front启动时会校验是否存在sqlite3 db文件, 如果db不存在则自动创建
func InitTables() {
	if config.GetDBConfig().DbType == "mysql" {
		InitMysqlTable()
		return
	}
	if config.GetDBConfig().DbType != "sqlite3" {
		log.Println("not using sqlite, please check tables by self")
		return
	}

	_, err := os.Stat(config.GetDBConfig().DbPath) //os.Stat获取文件信息
	if err != nil {
		// db 文件不存在
		if os.IsNotExist(err) {
			file, err := os.Create(config.GetDBConfig().DbPath)
			if err != nil {
				log.Println(err)
				panic("create db failed")
			}
			file.Close()
		}
	}
	dbConn := NewCaDb()
	// execute a query on the server
	total := 0
	err = dbConn.db.QueryRow(isRevokeTableExistSql).Scan(&total)
	if err != nil {
		log.Println(err)
		panic(" table failed")
	} else {
		if total == 1 {
			_, err = dbConn.db.Exec(renameTableSql)
		}
	}
	_, err = dbConn.db.Exec(defaultSQLiteSchema)
	if err != nil {
		log.Println(err)
		panic("create table failed")
	}
	log.Println("init tables success")
	return
}

// 初始化mysqldb
func InitMysqlTable() {
	dbConn := NewCaDb()
	//设置连接池最大连接数
	dbConn.db.SetMaxOpenConns(100)
	//设置连接池最大空闲连接数
	dbConn.db.SetMaxIdleConns(20)

	_, err := dbConn.db.Exec(defaultMysqlSchema)
	if err != nil {
		log.Println(err)
		panic("create table failed")
	}
	log.Println("init tables success")
	return
}

// 初始化mysql connect配置
func InitMysqlConnect() (string, error) {
	username := config.GetDBConfig().MysqlDbUser
	password := config.GetDBConfig().MysqlDbPwd
	server_ip := config.GetDBConfig().MysqlDbHost
	db_port := config.GetDBConfig().MysqlDbPort
	database := config.GetDBConfig().MysqlDbDatabase
	if username == "" || password == "" || server_ip == "" || db_port == "" || database == "" {
		return "", errors.New("dbConfig of mysql is error")
	}
	conn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&multiStatements=true", username, password, server_ip, db_port, database)
	// log.Println(conn)
	return conn, nil
}
