package clixchain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/xuperchain/xuper-front/config"
	logs "github.com/xuperchain/xuper-front/logs"
	pb "github.com/xuperchain/xuperchain/service/pb"
	"github.com/xuperchain/xupercore/lib/utils"

	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/keepalive"
)

// MaxRecvMsgSize max message size
const (
	MaxRecvMsgSize = 1024 * 1024 * 1024
	GRPCTIMEOUT    = 20
	StatusSuccess  = 200
	GroupSliceSize = 10

	ParaModule              = "xkernel"
	ParaChainKernelContract = "$parachain"
	ParaMethod              = "getGroup"
	ParaChainEventName      = "EditParaGroups"
)

var (
	ErrPreExec         = errors.New("Request PreExec error")
	ErrPreExecResponse = errors.New("PreExec Response empty")
)

type GroupClient struct {
	pb.XchainClient
	pb.EventServiceClient
	BcName string
	Cache  *groupCache
	once   sync.Once

	log logs.Logger
}

func (cli *GroupClient) Init() error {
	// 初始化时, 访问xchain获取平行链权限列表
	resp, err := cli.KernelPreExec(ParaModule, ParaChainKernelContract, ParaMethod, map[string][]byte{
		"name": []byte(cli.BcName),
	})
	if err != nil {
		return err
	}
	// 初始化cache
	group := Group{}
	err = json.Unmarshal(resp.Body, &group)
	if err != nil {
		return err
	}
	set := make(map[string]bool)
	cache := groupCache{
		close: make(chan int64, 1),
		ch:    make(chan []string, 1),
		value: group.GetAddrs(set),
	}
	cli.Cache = &cache
	cache.Start()

	// 订阅event监听平行链权限变更
	filter, err := cli.NewParaFilter()
	if err != nil {
		return err
	}
	sw, err := cli.Subscribe(filter)
	if err != nil {
		return err
	}
	cli.listenEvent(*sw)
	return nil
}

func (cli *GroupClient) Stop() {
	cli.Cache.close <- 1
}

func (cli *GroupClient) KernelPreExec(moduleName, contractName, methodName string, Args map[string][]byte) (*pb.ContractResponse, error) {
	var preExeReqs []*pb.InvokeRequest
	preExeReqs = append(preExeReqs, &pb.InvokeRequest{
		ModuleName:   moduleName,
		ContractName: contractName,
		MethodName:   methodName,
		Args:         Args,
	})
	preExeRPCReq := &pb.InvokeRPCRequest{
		Bcname: config.GetXchainServer().Master,
		Header: &pb.Header{
			Logid: utils.GenLogId(),
		},
		Requests: preExeReqs,
	}

	initiator, err := readAddress()
	if err != nil {
		return nil, fmt.Errorf("GroupClient::KernelPreExec::Get initiator error: %s", err.Error())
	}
	preExeRPCReq.Initiator = initiator
	preExeRPCReq.AuthRequire = []string{initiator}

	ctx, cancel := context.WithTimeout(context.Background(), GRPCTIMEOUT*time.Second)
	defer cancel()
	resp, err := cli.XchainClient.PreExec(ctx, preExeRPCReq)
	if err != nil {
		return nil, err
	}
	cr := resp.GetResponse().GetResponses()
	if len(cr) == 0 {
		return nil, ErrPreExecResponse
	}
	return cr[0], nil
}

func (cli *GroupClient) Subscribe(filter []byte) (*streamWrapper, error) {
	in := pb.SubscribeRequest{
		Type:   pb.SubscribeType_BLOCK,
		Filter: filter,
	}
	stream, err := cli.EventServiceClient.Subscribe(context.TODO(), &in)
	if err != nil {
		return nil, err
	}
	return &streamWrapper{
		EventService_SubscribeClient: stream,
		newEvent:                     make(chan []string, 1),
		close:                        make(chan int64, 1),
		log:                          cli.log,
	}, nil
}

// create a block filter
func (cli *GroupClient) NewParaFilter() ([]byte, error) {
	blockFilter := pb.BlockFilter{
		Bcname:    config.GetXchainServer().Master, // 均为基于主链进行的监听
		EventName: ParaChainEventName,
	}
	return proto.Marshal(&blockFilter)
}

// Singleton
func (cli *GroupClient) listenEvent(s streamWrapper) {
	go cli.once.Do(func() {
		go s.loop()
		for {
			select {
			case done := <-cli.Cache.close:
				s.close <- done
				return
			case value := <-s.newEvent:
				cli.log.Info("GroupClient::listenEvent:refresh value", "value", value)
				cli.Cache.ch <- value
			}
		}
	})
}

// StartClientServer start server for lcv
func NewClientServer(bcName string) (*GroupClient, error) {
	log, err := logs.NewLogger("xchainProxyServer")
	if err != nil {
		return nil, err
	}
	conn, err := grpc.Dial(config.GetXchainServer().Rpc, grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxRecvMsgSize)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
			Timeout:             5 * time.Second,  // wait 1 second for ping ack before considering the connection dead
			PermitWithoutStream: true,             // send pings even without active streams
		}))
	if err != nil {
		log.Error("GroupClient::NewClientServer::create conn to xchain failed", "bcName", bcName)
		return nil, err
	}
	cli := GroupClient{
		XchainClient:       pb.NewXchainClient(conn),
		EventServiceClient: pb.NewEventServiceClient(conn),
		BcName:             bcName,

		log: log,
	}
	return &cli, nil
}

//////////// streamWrapper ///////////
type streamWrapper struct {
	pb.EventService_SubscribeClient
	newEvent chan []string
	close    chan int64

	log logs.Logger
}

func (s *streamWrapper) loop() {
	for {
		select {
		case <-s.close:
			return
		default:
			event, err := s.EventService_SubscribeClient.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				s.log.Error("GroupClient::loop::Get block event error", "err", err)
				return
			}
			s.getEvent(event)
		}
	}
}

func (s *streamWrapper) getEvent(event *pb.Event) {
	var block pb.FilteredBlock
	err := proto.Unmarshal(event.Payload, &block)
	if err != nil {
		s.log.Error("GroupClient::getEvent::get block event error", "err", err)
		return
	}
	if len(block.GetTxs()) == 0 {
		return
	}
	var groupAddrs []string
	groupAddrsMap := make(map[string]bool)
	for _, tx := range block.Txs {
		if tx.Events == nil {
			continue
		}
		for _, b := range tx.Events {
			if b.Name != ParaChainEventName {
				continue
			}
			var groupItem Group
			err := json.Unmarshal(b.Body, &groupItem)
			if err != nil {
				continue
			}
			groupAddrs = append(groupAddrs, groupItem.GetAddrs(groupAddrsMap)...)
		}
	}
	s.newEvent <- groupAddrs
}

//////////// GroupCache //////////
type groupCache struct {
	close chan int64
	ch    chan []string
	value []string
	sync.RWMutex
}

func (c *groupCache) Start() {
	go func() {
		for {
			select {
			case <-c.close:
				return
			case newV := <-c.ch:
				c.Lock()
				c.value = newV
				c.Unlock()
			}
		}
	}()
}

func (c *groupCache) Get() []string {
	select {
	case newV := <-c.ch:
		return newV
	default:
		c.RLock()
		defer c.RUnlock()
		return c.value
	}
}

//////////// Xupercore Group ////////////
type Group struct {
	GroupID    string   `json:"name,omitempty"`
	Admin      []string `json:"admin,omitempty"`
	Identities []string `json:"identities,omitempty"`
}

func (g *Group) GetAddrs(set map[string]bool) []string {
	var addrs []string
	for _, value := range g.Admin {
		if _, ok := set[value]; ok {
			continue
		}
		addrs = append(addrs, value)
		set[value] = true
	}
	for _, value := range g.Identities {
		if _, ok := set[value]; ok {
			continue
		}
		addrs = append(addrs, value)
		set[value] = true
	}
	return addrs
}

////////////// Authority ////////////
func readAddress() (string, error) {
	path := filepath.Join(config.GetKeys(), "address")
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	buf = bytes.TrimSpace(buf)
	return string(buf), nil
}
