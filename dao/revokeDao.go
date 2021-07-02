/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package dao

import (
	"errors"

	"github.com/xuperchain/xuper-front/logs"
)

type Revoke struct {
	Id         int    `db:"id"`
	Net        string `db:"net"`
	SerialNum  string `db:"serial_num"`
	CreateTime int    `db:"create_time"`
}

// front 本地的撤销节点列表
type RevokeDao struct {
	Log logs.Logger
}

// 通过serialNum查询是否存在已撤销的证书
func (revokeDao *RevokeDao) GetBySerialNum(serialNum string) (*Revoke, error) {
	if serialNum == "" {
		return nil, errors.New("serial_num is illegal")
	}

	var revoke Revoke
	caDb := GetDbInstance()
	err := caDb.db.Get(&revoke,
		"SELECT * FROM revoke_node WHERE serial_num=?", serialNum)
	if err != nil {
		revokeDao.Log.Warn("RevokeDao::GetBySerialNum", "err", err)
		return nil, err
	}
	return &revoke, nil
}

func (revokeDao *RevokeDao) Insert(revoke *Revoke) (int64, error) {

	caDb := GetDbInstance()
	result, err := caDb.db.Exec(
		"INSERT INTO revoke_node(`id`, `net`, `serial_num`, `create_time`) VALUES (?,?,?,?)",
		revoke.Id,
		revoke.Net,
		revoke.SerialNum,
		revoke.CreateTime)
	if err != nil {
		revokeDao.Log.Warn("RevokeDao::Insert", "err", err)
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		revokeDao.Log.Warn("RevokeDao::Insert", "err", err)
		return 0, err
	}
	return id, nil
}

// 获取数据库中最后写入的撤销证书serialNum
func (revokeDao *RevokeDao) GetLatestSerialNum(net string) (string, error) {
	var revoke Revoke
	caDb := GetDbInstance()
	err := caDb.db.Get(&revoke, "SELECT * FROM revoke_node WHERE net=? order BY id LIMIT 1", net)
	if err != nil {
		return "", err
	}
	return revoke.SerialNum, nil
}
