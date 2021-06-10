/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package file

import (
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/xuperchain/xuper-front/config"
)

func Exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

/**
 * 生成文件
 */
func WriteFileUsingFilename(filename string, content []byte) error {
	err := ioutil.WriteFile(filename, content, 0666)
	return err
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func GetLogOut() io.Writer {
	path := config.GetLog().Path
	if path == "" {
		return os.Stderr
	}
	if strings.LastIndex(path, "/") != len([]rune(path))-1 {
		path = path + "/"
	}
	file, err := os.Create(path + "caserver.log")
	if err != nil {
		return os.Stderr
	}

	return file
}
