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

	_ "google.golang.org/grpc/encoding/gzip"
)

// MaxRecvMsgSize max message size
const (
	eventStatusClosed = iota
	eventStatusListening

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
	ErrPreExec       = errors.New("Request PreExec error")
	ErrResponseEmpty = errors.New("Response empty")
)

type GroupClient struct {
	pb.XchainClient
	pb.EventServiceClient
	bcName        string
	eventListener *eventListener
	log           logs.Logger

	Cache *groupCache
}

// NewGroupClient GroupClint bind with a xchainClient & eventServiceClient
func NewGroupClient(bcName string, xchainClient pb.XchainClient, eventClient pb.EventServiceClient) (*GroupClient, error) {
	log, err := logs.NewLogger("xchainProxyServer")
	if err != nil {
		return nil, err
	}
	cli := GroupClient{
		XchainClient:       xchainClient,
		EventServiceClient: eventClient,
		bcName:             bcName,
		log:                log,
		eventListener: &eventListener{
			log: log,
		},
	}
	return &cli, nil
}

// Init get groups from xchain-server and listen para-chain event
func (cli *GroupClient) Init() error {
	// 初始化时, 访问xchain获取平行链权限列表
	resp, err := kernelPreExec(cli.XchainClient, ParaModule, ParaChainKernelContract, ParaMethod, map[string][]byte{
		"name": []byte(cli.bcName),
	})
	if err != nil {
		return err
	}
	// 初始化cache
	group := group{}
	err = func() error {
		// 访问xchain错误时，仍监听group字段
		if resp.Status != StatusSuccess {
			cli.log.Warn("GroupClient.Init: get group from xchain", "err", resp.Message)
			return nil
		}
		return json.Unmarshal(resp.Body, &group)
	}()
	if err != nil {
		return err
	}
	cli.log.Info("GroupClient.Init: get group from xchain", "group", group)
	set := make(map[string]bool)
	cache := groupCache{
		close: make(chan int64, 1),
		ch:    make(chan []string, 1),
		value: group.GetAddrs(set),
	}
	cli.Cache = &cache
	cache.start()
	return cli.listenParachainEvent(cli.Cache)
}

// Get fresh groups from cache
func (cli *GroupClient) Get() []string {
	// 先检查当前singleton是否为空，若是需要重新订阅event
	cli.eventListener.mu.RLock()
	defer cli.eventListener.mu.RUnlock()
	if cli.eventListener.singleton == nil {
		cli.log.Info("GroupClient.Get: re-listenParachainEvent.")
		err := cli.listenParachainEvent(cli.Cache)
		if err != nil {
			cli.log.Error("GroupClient.Get: re-listenParachainEvent error.")
		}
	}
	return cli.Cache.get()
}

// listenParachainEvent register xchain event
func (cli *GroupClient) listenParachainEvent(cache *groupCache) (err error) {
	// 订阅event监听平行链权限变更
	filter, err := newParaFilter()
	if err != nil {
		return err
	}
	stream, err := subscribe(cli.EventServiceClient, filter)
	if err != nil {
		return err
	}

	// 单例去抢注一个stream，并loop检查它
	cli.eventListener.mu.Lock()
	defer cli.eventListener.mu.Unlock()
	if cli.eventListener.singleton == nil {
		cli.eventListener.singleton = stream
		cli.eventListener.listenEvent(cache)
	}
	return nil
}

func (cli *GroupClient) Stop() {
	cli.Cache.close <- 1
}

//////////// EventListener ///////////
type eventListener struct {
	singleton pb.EventService_SubscribeClient
	mu        sync.RWMutex

	log logs.Logger
}

func (e *eventListener) reset() {
	e.mu.Lock()
	e.singleton = nil
	e.mu.Unlock()
}

// listenEvent 单独监听订阅stream
func (e *eventListener) listenEvent(cache *groupCache) {
	e.log.Info("EventListener.listenEvent: start listen event.")
	go func() {
		sw := e.singleton
		for {
			select {
			case <-cache.close:
				e.log.Info("EventListener.listenEvent: close.")
				return
			default:
				event, err := sw.Recv()
				if err == io.EOF {
					e.log.Error("EventListener.listenEvent: EventService_SubscribeClient stream meets EOF.")
					e.reset()
					return
				}
				if err != nil {
					e.log.Error("EventListener.listenEvent: Get block event error", "err", err)
					e.reset()
					return
				}
				groups, err := e.getGroups(event)
				// 接收到有效信息
				if err == nil {
					e.log.Info("EventListener.listenEvent: refresh value", "value", groups)
					cache.ch <- groups
					continue
				}
				if err != ErrResponseEmpty {
					e.log.Error("EventListener.listenEvent: getGroups error", "err", err)
				}
			}
		}
	}()
}

func (e *eventListener) getGroups(event *pb.Event) ([]string, error) {
	var block pb.FilteredBlock
	err := proto.Unmarshal(event.Payload, &block)
	if err != nil {
		return nil, err
	}
	if len(block.GetTxs()) == 0 {
		return nil, ErrResponseEmpty
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
			var groupItem group
			err := json.Unmarshal(b.Body, &groupItem)
			if err != nil {
				continue
			}
			groupAddrs = append(groupAddrs, groupItem.GetAddrs(groupAddrsMap)...)
		}
	}
	return groupAddrs, nil
}

//////////// GroupCache //////////
type groupCache struct {
	close chan int64
	ch    chan []string
	value []string
	sync.RWMutex
}

func (c *groupCache) start() {
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

func (c *groupCache) get() []string {
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
type group struct {
	groupID    string   `json:"name,omitempty"`
	admin      []string `json:"admin,omitempty"`
	identities []string `json:"identities,omitempty"`
}

func (g *group) GetAddrs(set map[string]bool) []string {
	var addrs []string
	for _, value := range g.admin {
		if _, ok := set[value]; ok {
			continue
		}
		addrs = append(addrs, value)
		set[value] = true
	}
	for _, value := range g.identities {
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

///////////// XChain /////////////
func kernelPreExec(service pb.XchainClient, moduleName, contractName, methodName string, Args map[string][]byte) (*pb.ContractResponse, error) {
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
		return nil, fmt.Errorf("GroupClient.KernelPreExec: Get initiator error: %s", err.Error())
	}
	preExeRPCReq.Initiator = initiator
	preExeRPCReq.AuthRequire = []string{initiator}

	ctx, cancel := context.WithTimeout(context.Background(), GRPCTIMEOUT*time.Second)
	defer cancel()
	resp, err := service.PreExec(ctx, preExeRPCReq)
	if err != nil {
		return nil, err
	}
	cr := resp.GetResponse().GetResponses()
	if len(cr) == 0 {
		return nil, ErrResponseEmpty
	}
	return cr[0], nil
}

func subscribe(service pb.EventServiceClient, filter []byte) (pb.EventService_SubscribeClient, error) {
	in := pb.SubscribeRequest{
		Type:   pb.SubscribeType_BLOCK,
		Filter: filter,
	}
	stream, err := service.Subscribe(context.TODO(), &in)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

// create a block filter
func newParaFilter() ([]byte, error) {
	blockFilter := pb.BlockFilter{
		Bcname:    config.GetXchainServer().Master, // 均为基于主链进行的监听
		EventName: ParaChainEventName,
	}
	return proto.Marshal(&blockFilter)
}
