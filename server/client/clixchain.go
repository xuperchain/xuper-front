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
	//MaxRecvMsgSize = 1024 * 1024 * 1024
	GRPCTIMEOUT    = 20
	StatusSuccess  = 200
	GroupSliceSize = 10

	ParaModule              = "xkernel"
	ParaChainKernelContract = "$parachain"
	ParaMethod              = "getGroup"
	ParaChainEventName      = "EditParaGroups"

	unAuthorized = 403
)

var (
	ErrPreExec       = errors.New("request preExec error")
	ErrResponseEmpty = errors.New("response empty")
	ErrInvalidGroup  = errors.New("group is invalid")
	ErrUnAuthorized  = errors.New("local node unAuthorized")

	napDuration = time.Second
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
			bcName: bcName,
			close:  make(chan struct{}),
			log:    log,
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
	var group group
	// 当且仅当无权限访问时，监听group字段
	if resp.Status != StatusSuccess && resp.Status == unAuthorized {
		cli.log.Info("GroupClient.Init: get group from xchain when unauthorized", "err", resp.Message)
		cli.Cache = &groupCache{
			value: make([]string, 0),
		}
		return cli.listenParachainEvent(cli.Cache)
	}
	err = json.Unmarshal(resp.Body, &group)
	if err != nil {
		return err
	}
	if group.GroupID != cli.bcName {
		return ErrInvalidGroup
	}
	if len(group.GetAddrs()) == 0 {
		return ErrInvalidGroup
	}
	cli.log.Info("GroupClient.Init: get group from xchain", "group", group, "bcname", cli.bcName)
	cache := groupCache{
		value: group.GetAddrs(),
	}
	cli.Cache = &cache
	return cli.listenParachainEvent(cli.Cache)
}

// Get fresh groups from cache
func (cli *GroupClient) Get() []string {
	// 先检查当前stream是否为空，若是需要重新订阅event
	err := cli.listenParachainEvent(cli.Cache)
	if err != nil {
		cli.log.Error("GroupClient.Get: listenParachainEvent error", "bcname", cli.bcName, "error", err)
	}
	return cli.Cache.get()
}

// listenParachainEvent register xchain event
func (cli *GroupClient) listenParachainEvent(cache *groupCache) (err error) {
	if cli.eventListener.stream == nil {
		cli.eventListener.mutex.Lock()
		defer cli.eventListener.mutex.Unlock()
		if cli.eventListener.stream == nil {
			// 订阅event监听平行链权限变更
			filter, err := newParaFilter()
			if err != nil {
				return err
			}
			stream, err := subscribe(cli.EventServiceClient, filter)
			if err != nil {
				return err
			}
			// 抢注一个stream，并loop检查它
			cli.eventListener.stream = stream
			cli.eventListener.listenEvent(cache)
			return nil
		}
	}
	return nil
}

func (cli *GroupClient) Stop() {
	close(cli.eventListener.close)
}

//////////// EventListener ///////////
type eventListener struct {
	bcName string
	stream pb.EventService_SubscribeClient
	close  chan struct{}
	mutex  sync.Mutex
	log    logs.Logger
}

func (e *eventListener) reset() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.stream.CloseSend()
	e.stream = nil
}

// listenEvent 单独监听订阅stream
func (e *eventListener) listenEvent(cache *groupCache) {
	e.log.Info("EventListener.listenEvent: start listen event.", "bcname", e.bcName)
	go func() {
		e.mutex.Lock()
		defer e.mutex.Unlock()
		sw := e.stream
		for {
			select {
			case <-e.close:
				e.log.Info("EventListener.listenEvent: close.", "bcname", e.bcName)
				return
			default:
				event, err := sw.Recv()
				if err == io.EOF {
					e.log.Error("EventListener.listenEvent: EventService_SubscribeClient stream meets EOF.", "bcname", e.bcName)
					e.reset()
					return
				}
				if err != nil {
					e.log.Error("EventListener.listenEvent: Get block event error", "err", err, "bcname", e.bcName)
					e.reset()
					return
				}
				groups, err := e.getGroups(event)
				// 接收到有效信息
				if groups != nil {
					e.log.Info("EventListener.listenEvent: refresh value", "value", groups, "bcname", e.bcName)
					cache.put(groups)
					time.Sleep(napDuration)
					continue
				}
				if err != ErrResponseEmpty {
					e.log.Error("EventListener.listenEvent: getGroups error", "err", err, "bcname", e.bcName)
					e.reset()
					return
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
	// 和本链相关的事件订阅，统一仅取最后一次更改的值
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
			if groupItem.GroupID != e.bcName {
				continue
			}
			groupAddrs = groupItem.GetAddrs()
		}
	}
	if len(groupAddrs) == 0 {
		return nil, ErrResponseEmpty
	}
	return groupAddrs, nil
}

//////////// GroupCache //////////
type groupCache struct {
	value []string
	sync.RWMutex
}

func (c *groupCache) get() []string {
	c.RLock()
	defer c.RUnlock()
	return c.value
}

func (c *groupCache) put(value []string) {
	c.Lock()
	defer c.Unlock()
	c.value = value
}

//////////// Xupercore Group ////////////
type group struct {
	GroupID    string   `json:"name,omitempty"`
	Admin      []string `json:"admin,omitempty"`
	Identities []string `json:"identities,omitempty"`
}

func (g *group) GetAddrs() []string {
	set := make(map[string]bool)
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
