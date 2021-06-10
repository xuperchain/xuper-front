#!/bin/sh
VERSION=2.0.0
binary=xfront

#使用临时容器编译私有化产出，使得工作镜像更精简，并且不会包含源码
docker run -it --rm \
    -v ${PWD}:/workspace \
    -v ~/.ssh:/root/.ssh \
    -w /workspace \
    -e GONOSUMDB=* \
    -e GOPROXY=https://goproxy.cn,direct \
    -e GO111MODULE=on \
    golang:1.13.4 go build -o ./output/xfront cmd/front.go
docker rmi -f $binary:${VERSION}
docker build -t $binary:${VERSION} .
#清理在当前目录下的二进制产出
rm -rf ./output/xfront