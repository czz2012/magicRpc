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
	AsyncClosed   func(uint64)
	AsyncAccept   func(uint64)
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

//SetClientCap Set accesser cap option
func SetClientCap(cap int) Option {
	return func(o *Options) error {
		o.Cap = cap
		return nil
	}
}

//SetClientKeepTime Set client keep time millsecond option
func SetClientKeepTime(tm int) Option {
	return func(o *Options) error {
		o.KeepTime = tm
		return nil
	}
}

//SetClientBufferLimit Set client buffer limit option
func SetClientBufferLimit(limit int) Option {
	return func(o *Options) error {
		o.BufferLimit = limit
		return nil
	}
}

//SetClientOutSize Set client recvice call chan size option
func SetClientOutSize(outSize int) Option {
	return func(o *Options) error {
		o.OutCChanSize = outSize
		return nil
	}
}

//SetAsyncError Set Listen fail Async Error callback option
func SetAsyncError(f func(error)) Option {
	return func(o *Options) error {
		o.AsyncError = f
		return nil
	}
}

//SetAsyncAccept Set Listen client accept Async callback
func SetAsyncAccept(f func(uint64)) Option {
	return func(o *Options) error {
		o.AsyncAccept = f
		return nil
	}
}

//SetAsyncComplete Set Listen async success callback option
func SetAsyncComplete(f func(int32)) Option {
	return func(o *Options) error {
		o.AsyncComplete = f
		return nil
	}
}

//SetAsyncClosed Set Listen closed async callback
func SetAsyncClosed(f func(uint64)) Option {
	return func(o *Options) error {
		o.AsyncClosed = f
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
	rpc._asyncAccept = opts.AsyncAccept
	rpc._asyncClosed = opts.AsyncClosed
	handler.Spawn(opts.Name, func() handler.IService {
		group := &RPCSrvGroup{_id: opts.ServerID, _bfSize: opts.BufferLimit, _cap: opts.Cap}

		h, err := listener.Spawn(
			listener.SetListener(&net.TCPListen{}),
			listener.SetAsyncError(opts.AsyncError),
			listener.SetClientKeepTime(opts.KeepTime),
			listener.SetClientOutChanSize(opts.OutCChanSize),
			listener.SetAsyncComplete(opts.AsyncComplete),
			listener.SetAsyncAccept(rpc.rpcAccept),
			listener.SetAsyncClose(rpc.rpcClosed),
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
	_listen      *listener.NetListener
	_rpcs        map[string]interface{}
	_asyncAccept func(uint64)
	_asyncClosed func(uint64)
}

func (slf *RPCServer) rpcClosed(id uint64) error {
	if slf._asyncAccept != nil {
		slf._asyncAccept(id)
	}
	return nil
}

func (slf *RPCServer) rpcAccept(c net.INetClient) error {
	x := make([]byte, 1)
	x[0] = 0xBF
	if err := c.(*RPCSrvClient).SendTo(x); err != nil {
		return err
	}

	if slf._asyncAccept != nil {
		slf._asyncAccept(c.GetID())
	}
	return nil
}

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
