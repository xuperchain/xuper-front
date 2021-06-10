/*
 * Copyright (c) 2019. Baidu Inc. All Rights Reserved.
 */
package config

import (
	"testing"
)

const defaultConfigFile = "../conf/front.yaml"

func TestInstallFrontConfig(t *testing.T) {

	InstallFrontConfig(defaultConfigFile)

}
