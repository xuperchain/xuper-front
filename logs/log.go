package logs

import (
	"path/filepath"
	"sync"

	log15 "github.com/xuperchain/log15"
	"github.com/xuperchain/xuper-front/config"
	"github.com/xuperchain/xupercore/lib/utils"
)

const (
	DefaultCallDepth = 3
)

var (
	logHandle LogDriver
	once      sync.Once
	lock      sync.RWMutex
)

func InitLog(cfgFile, logDir string) {
	lock.Lock()
	defer lock.Unlock()
	once.Do(func() {
		// 创建日志实例
		xfLog := log15.New()
		lvLevel := LvlFromString(config.GetLog().Level)
		xfLog.SetLevelLimit(LvlFromString(config.GetLog().Level))
		lfmt := log15.LogfmtFormat()
		// 输出日志
		file := filepath.Join(logDir, cfgFile+".log")
		nmHandler := log15.Must.FileHandler(file, lfmt)
		nmfileh := log15.LvlFilterHandler(lvLevel, nmHandler)
		lhd := log15.SyncHandler(log15.MultiHandler(nmfileh))
		xfLog.SetHandler(lhd)
		logHandle = xfLog
	})
}

// LvlFromString returns the appropriate Lvl from a string name.
// Useful for parsing command line args and configuration files.
func LvlFromString(lvlString string) log15.Lvl {
	switch lvlString {
	case "debug", "dbug":
		return log15.LvlDebug
	case "trace", "trce":
		return log15.LvlTrace
	case "info":
		return log15.LvlInfo
	case "warn":
		return log15.LvlWarn
	case "error", "eror":
		return log15.LvlError
	default:
		return log15.LvlInfo
	}
}

// 底层日志库约束接口
type LogDriver interface {
	Error(msg string, ctx ...interface{})
	Warn(msg string, ctx ...interface{})
	Info(msg string, ctx ...interface{})
	Trace(msg string, ctx ...interface{})
}

type Logger interface {
	Error(msg string, ctx ...interface{})
	Warn(msg string, ctx ...interface{})
	Info(msg string, ctx ...interface{})
	Trace(msg string, ctx ...interface{})
}

////////////// Logger Fitter //////////////
// 方便系统对日志输出做自定义扩展
type LogFitter struct {
	log       LogDriver
	Module    string
	minLvl    uint32
	callDepth int
}

// 需要先调用InitLog全局初始化
func NewLogger(Module string) (*LogFitter, error) {
	// 基础日志实例和日志配置采用单例模式
	lock.RLock()
	defer lock.RUnlock()
	if logHandle == nil {
		InitLog(config.GetLog().FrontName, config.GetLog().Path)
	}
	lf := &LogFitter{
		log:       logHandle,
		Module:    Module,
		minLvl:    uint32(LvlFromString(config.GetLog().Level)),
		callDepth: DefaultCallDepth,
	}
	return lf, nil
}

func (t *LogFitter) fmtCommLogger(ctx ...interface{}) []interface{} {
	if len(ctx)%2 != 0 {
		last := ctx[len(ctx)-1]
		ctx = ctx[:len(ctx)-1]
		ctx = append(ctx, "unknow", last)
	}
	// Ensure consistent output sequence
	fileLine, _ := utils.GetFuncCall(t.callDepth)
	comCtx := make([]interface{}, 0)
	// 保持log_id是第一个写入，方便替换
	comCtx = append(comCtx, "s_mod", t.Module)
	comCtx = append(comCtx, "line", fileLine)
	comCtx = append(comCtx, ctx...)
	return comCtx
}
func (t *LogFitter) isInit() bool {
	if t.log == nil {
		return false
	}
	return true
}

func (t *LogFitter) Error(msg string, ctx ...interface{}) {
	if !t.isInit() {
		return
	}
	t.log.Error(msg, t.fmtCommLogger(ctx...)...)
}

func (t *LogFitter) Warn(msg string, ctx ...interface{}) {
	if !t.isInit() {
		return
	}
	t.log.Warn(msg, t.fmtCommLogger(ctx...)...)
}

func (t *LogFitter) Info(msg string, ctx ...interface{}) {
	if !t.isInit() {
		return
	}
	t.log.Info(msg, t.fmtCommLogger(ctx...)...)
}

func (t *LogFitter) Trace(msg string, ctx ...interface{}) {
	if !t.isInit() {
		return
	}
	t.log.Trace(msg, t.fmtCommLogger(ctx...)...)
}
