# Xuper-Front

[![License](https://img.shields.io/github/license/xuperchain/xuperchain?style=flat-square)](/LICENSE)

## Xuper-Front是什么？

基于xuperchain底层技术，提供CA服务，通过证书控制全节点的权限。

xchain和front，构成了一个区块链的全节点；全节点和全节点之间的通信是由xchain作为client和作为server的front建立grpcs的连接，这里的grpcs即是带有tls证书校验的grpc通信, grpcs中使用的证书采用x509协议体系，每个全节点有自己的证书，该证书由caserver颁发，caserver的根证书为根ca证书，产生对应网络的一级CA证书作为中间证书，然后基于该中间证书给每一个全节点颁发证书;全节点内部的front和xchain进程通信是 front作为client连接xchain。

核心功能

1. front在启动时，会向caserver拉取对应的全节点证书
2. front会校验其他全节点连接的证书，判断其他全节点是否有权限访问本节点(1.同一个网络 2.撤销列表中没有)
3. front定期拉取caserver的撤销证书列表 

## 快速使用

### 环境配置

* 操作系统：支持Linux以及Mac OS
* 开发语言：Go 1.13.x及以上
* 版本控制工具：Git

### 构建

克隆Xuper-Front仓库
```
git clone https://github.com/xuperchain/xuper-front
```

编译Front
```
make
```

## 许可证
Xuper-Front使用的许可证是Apache 2.0

## 参与贡献
对Xuper-Front感兴趣的同学可以一起参与贡献，如果你遇到问题或需要新功能，欢迎创建issue，如果你可以解决某个issue, 欢迎发送PR。

## 联系我们
商务合作，请Email：xchain-help@baidu.com, 来源请注明Github。
如果你对XuperChain开源技术及应用感兴趣，欢迎添加“百度超级链·小助手“微信，回复“技术论坛进群”，加入“百度超级链开发者社区”，与百度资深工程师深度交流!微信二维码如下:

![微信二维码](https://github.com/ToWorld/xuperchain-image/blob/master/baidu-image-xuperchain.png)

