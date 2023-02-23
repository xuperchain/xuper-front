# 支持本地编译
ifeq ($(OS),Windows_NT)
  PLATFORM="Windows"
else
  ifeq ($(shell uname),Darwin)
    PLATFORM="MacOS"
  else
    PLATFORM="Linux"
  endif
endif


#初始化项目目录变量
HOMEDIR := $(shell pwd)
OUTDIR  := $(HOMEDIR)/output
OUTDIRNORMAL := $(OUTDIR)/front

# init GO & GOD path
export GOROOT  := $(shell go env GOROOT)
#export GOPATH  := $(HOMEDIR)/../../../
#export PATH    := $(GOPATH)/bin:$(GOROOT)/bin:$(PATH)
export GOPROXY :=


#初始化命令变量
GO      := $(GOROOT)/bin/go
GOMOD   := $(GO) mod
GOBUILD := $(GO) build
GOGET   :=$(GO) get
GOTEST  := $(GO) test
GOPKGS  := $$($(GO) list ./...| grep -vE "vendor")
#执行编译，可使用命令 make 或 make all 执行， 顺序执行prepare -> compile -> test -> package 几个阶段
#all: prepare compile test package
all: prepare compile package
# prepare阶段
prepare: prepare-dep
prepare-dep:
	git config --global http.sslVerify false #设置git， 保证github mirror能够下载
#	protoc -I pb pb/caserver.proto -I pb/googleapis --go_out=plugins=grpc:pb
#	protoc -I pb pb/localca.proto -I pb/googleapis --go_out=plugins=grpc:pb
#	protoc -I pb pb/xchain.proto -I pb/googleapis --go_out=plugins=grpc:pb --grpc-gateway_out=logtostderr=true:pb

set-env:
	$(GO) env -w GOPROXY=https://goproxy.cn,direct
	$(GO) env -w GONOSUMDB=\*
	$(GOGET) -u github.com/go-kit/kit@v0.8.0
#complile阶段，执行编译命令，可单独执行命令: make compile
compile:build
build: set-env
	$(GOMOD) tidy #下载Go依赖
	$(GOBUILD) -o $(HOMEDIR)/front $(HOMEDIR)/cmd/front.go
#test阶段，进行单元测试， 可单独执行命令: make test
test: test-case
test-case: set-env
	$(GOTEST) -v -cover $(GOPKGS)
#与覆盖率平台打通，输出测试结果到文件中
#@$(GOTEST) -v -json -coverprofile=coverage.out $(GOPKGS) > testlog.out
#package阶段，对编译产出进行打包，输出到output目录， 可单独执行命令: make package
package: package-bin
package-bin:
	mkdir -p $(OUTDIRNORMAL)/bin
	mv front $(OUTDIRNORMAL)/bin/
	cp -r conf $(OUTDIRNORMAL)/
	mkdir -p $(OUTDIRNORMAL)/data/keys/
	mv $(OUTDIRNORMAL)/conf/keys/* $(OUTDIRNORMAL)/data/keys/
	chown -R $(USER):$(GROUP) $(OUTDIRNORMAL)
#install阶段，编译产出放到$GOPATH/bin目录， 可单独执行命令: make install
install: install-bin
install-bin:
	#cp $(OUTDIR)/server  $(GOPATH)/bin/
#clean阶段，清除过程中的输出， 可单独执行命令: make clean
clean:
	rm -rf $(OUTDIR)
#	rm -rf $(HOMEDIR)/bin
# avoid filename conflict and speed up build
.PHONY: all prepare compile test package install clean build 
