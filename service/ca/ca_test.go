package service

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"bou.ke/monkey"

	"github.com/xuperchain/xuper-front/config"
)

func TestCA(t *testing.T) {
	os.Chdir("../../../caserver/output/caserver")

	cmd := exec.Command("./bin/ca-server", "addNet", "--Net", "testnet", "--Addr", "efh28n9mWema7Md6BhuNZeN1h3ULxFsHd")
	err := cmd.Start()
	if err != nil {
		t.Error(err)
	}

	process, err := os.StartProcess("./bin/ca-server", []string{""}, &os.ProcAttr{})
	if err != nil {
		t.Error(err)
	}
	defer process.Kill()
	time.Sleep(1 * time.Second)

	os.Chdir("../../../front")
	if err := config.InstallFrontConfig("./conf/front.yaml"); err != nil {
		t.Error(err)
	}

	monkey.Patch(config.GetKeys, func() string {
		return "../../conf/keys/"
	})

	os.Chdir("./service/ca")
	if err := AddNode("test", "testnet", "efh28n9mWema7Md6BhuNZeN1h3ULxFsHd"); err != nil {
		t.Error(err)
	}

	if err := GetAndWriteCert("testnet"); err == nil {
		t.Error("expect error")
	}

	if _, _, err := GetCurrentCert("testnet"); err == nil {
		t.Error("expect error")
	}

	if err := GetRevokeList("testnet"); err != nil {
		t.Error(err)
	}
}
