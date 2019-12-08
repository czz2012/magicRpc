package server

import (
	"errors"
	"reflect"

	"github.com/gogo/protobuf/proto"
	"github.com/yamakiller/magicNet/engine/actor"
	"github.com/yamakiller/magicNet/handler"
	"github.com/yamakiller/magicNet/handler/implement/listener"
	"github.com/yamakiller/magicNet/handler/net"
	"github.com/yamakiller/magicRpc/assembly/codec"
	"github.com/yamakiller/magicRpc/code"
)

func rpcDecode(ontext actor.Context, params ...interface{}) error {
	s := params[0].(*RPCServer)
	c := params[1].(net.INetClient)
	block, err := codec.Decode(c)
	if err != nil {
		if err == code.ErrIncompleteData {
			return net.ErrAnalysisProceed
		}
		return err
	}

	method := s.getRPC(block.Method)
	if block.Oper == codec.RPCRequest {
		if method == nil {
			return code.ErrMethodUndefined
		}
	}

	dataType := proto.MessageType(block.DName)
	if dataType == nil {
		return code.ErrParamUndefined
	}

	data := reflect.New(dataType.Elem()).Interface().(proto.Message)
	if err := proto.Unmarshal(block.Data, data); err != nil {
		return err
	}

	if block.Oper == codec.RPCRequest {
		actor.DefaultSchedulerContext.Send((c.(*RPCSrvClient)).GetPID(), &requestEvent{block.Method, method, data, block.Ser})
	} else {
		actor.DefaultSchedulerContext.Send((c.(*RPCSrvClient)).GetPID(), &responseEvent{block.Method, data, block.Ser})
	}

	return net.ErrAnalysisSuccess
}

func rpcAccept(c net.INetClient) error {
	x := make([]byte, 1)
	x[0] = 0xBF
	return c.(*RPCSrvClient).SendTo(x)
}

//Options RPC Server Options
type Options struct {
	Name         string
	ServerID     int
	Cap          int
	KeepTime     int
	BufferLimit  int
	OutCChanSize int

	AsyncError    func(error)
	AsyncComplete func(int32)
}

// Option is a function on the options for a rpc listen.
type Option func(*Options) error

// SetName setting name option
func SetName(name string) Option {
	return func(o *Options) error {
		o.Name = name
		return nil
	}
}

// SetID setting id option
func SetID(id int) Option {
	return func(o *Options) error {
		o.ServerID = id
		return nil
	}
}

//SetClientCap setting accesser cap option
func SetClientCap(cap int) Option {
	return func(o *Options) error {
		o.Cap = cap
		return nil
	}
}

//SetClientKeepTime setting client keep time millsecond option
func SetClientKeepTime(tm int) Option {
	return func(o *Options) error {
		o.KeepTime = tm
		return nil
	}
}

//SetClientBufferLimit setting client buffer limit option
func SetClientBufferLimit(limit int) Option {
	return func(o *Options) error {
		o.BufferLimit = limit
		return nil
	}
}

//SetClientOutSize setting client recvice call chan size option
func SetClientOutSize(outSize int) Option {
	return func(o *Options) error {
		o.OutCChanSize = outSize
		return nil
	}
}

//SetAsyncError setting listen fail Async Error callback option
func SetAsyncError(f func(error)) Option {
	return func(o *Options) error {
		o.AsyncError = f
		return nil
	}
}

//SetAsyncComplete setting listen async success callback option
func SetAsyncComplete(f func(int32)) Option {
	return func(o *Options) error {
		o.AsyncComplete = f
		return nil
	}
}

var (
	defaultOption = Options{Name: "rpc server",
		ServerID:     1,
		Cap:          1024,
		BufferLimit:  8196,
		KeepTime:     1000 * 60,
		OutCChanSize: 512,
	}
)

//New doc
//@Summary new a rpc server
//@Method New
//@Param ...Option
//@Return *RPCServer
//@Return error
func New(options ...Option) (*RPCServer, error) {
	opts := defaultOption
	for _, opt := range options {
		if err := opt(&opts); err != nil {
			return nil, err
		}
	}

	rpc := &RPCServer{_rpcs: make(map[string]interface{})}
	handler.Spawn(opts.Name, func() handler.IService {
		group := &RPCSrvGroup{_id: opts.ServerID, _bfSize: opts.BufferLimit, _cap: opts.Cap}
		group.Initial()

		h, err := listener.Spawn(
			listener.SetListener(&net.TCPListen{}),
			listener.SetAsyncError(opts.AsyncError),
			listener.SetClientKeepTime(opts.KeepTime),
			listener.SetClientOutChanSize(opts.OutCChanSize),
			listener.SetAsyncComplete(opts.AsyncComplete),
			listener.SetAsyncAccept(rpcAccept),
			listener.SetClientGroups(group),
			listener.SetClientDecoder(rpcDecode))

		if err != nil {
			return nil
		}

		rpc._listen = h
		rpc._listen.Initial()

		return rpc._listen
	})

	return rpc, nil
}

//RPCServer doc
//@Summary RPC Server
//@Struct RPCServer
//@
//@Member map[string]interface{}  RPC Function map table
type RPCServer struct {
	_listen *listener.NetListener
	_rpcs   map[string]interface{}
}

/*//Initial doc
//@Summary 初始化RPC服务
//@Method Initial
//@Return error
func (slf *RPCServer) Initial() {
	slf._rpcs = make(map[string]interface{})
	slf._listen.Initial()
}*/

func (slf *RPCServer) getRPC(name string) interface{} {
	f, ok := slf._rpcs[name]
	if !ok {
		return nil
	}
	return f

}

//RegRPC doc
//@Summary Register RPC Accesser function
//@Method RegRPC
//@Param  string function name
//@Param  interface{} function
//@Return error
func (slf *RPCServer) RegRPC(name string, method interface{}) error {
	if reflect.ValueOf(method).Type().Kind() != reflect.Func {
		return errors.New("need method is function")
	}
	slf._rpcs[name] = method
	return nil
}
