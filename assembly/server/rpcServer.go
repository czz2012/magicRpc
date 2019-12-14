package server

import (
	"errors"
	"reflect"

	"github.com/yamakiller/magicNet/engine/actor"
	"github.com/yamakiller/magicNet/handler"
	"github.com/yamakiller/magicNet/handler/implement/listener"
	"github.com/yamakiller/magicNet/handler/net"
	"github.com/yamakiller/magicRpc/assembly/common"
	"github.com/yamakiller/magicRpc/code"
)

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
			listener.SetAsyncClosed(rpc.rpcClosed),
			listener.SetClientGroups(group),
			listener.SetClientDecoder(rpc.rpcDecode))

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

//Listen doc
//@Summary RPC Server Listen
//@Param   string Listen address [ip:port]
//@Return  error
func (slf *RPCServer) Listen(addr string) error {
	return slf._listen.Listen(addr)
}

//Shutdown doc
//@Summary RPC Server shutdown
func (slf *RPCServer) Shutdown() {
	if slf._listen != nil {
		slf._listen.Shutdown()
		slf._listen = nil
	}

	slf._rpcs = nil
}

//Call doc
//@Summary RPC Call the function of the specified connection
//@Method Call
//@Param uint64  connection id
//@Param  string remote method
//@Param  interface{} remote method param
func (slf *RPCServer) Call(handle uint64, method string, param interface{}) error {
	c := slf._listen.Grap(handle)
	if c == nil {
		return code.ErrConnectNon
	}

	defer slf._listen.Release(c)
	return c.(*RPCSrvClient).Call(method, param)
}

func (slf *RPCServer) rpcClosed(id uint64) error {
	if slf._asyncClosed != nil {
		slf._asyncClosed(id)
	}
	return nil
}

func (slf *RPCServer) rpcAccept(c net.INetClient) error {
	x := make([]byte, 1)
	x[0] = common.ConstHandShakeCode
	if err := c.(*RPCSrvClient).SendTo(x); err != nil {
		return err
	}

	if slf._asyncAccept != nil {
		slf._asyncAccept(c.GetID())
	}
	return nil
}

func (slf *RPCServer) rpcDecode(context actor.Context, params ...interface{}) error {
	c := params[1].(net.INetClient)
	data, err := common.RPCDecodeServer(slf.getRPC, c)
	if err != nil {
		return err
	}

	actor.DefaultSchedulerContext.Send((c.(*RPCSrvClient)).GetPID(), data)
	return net.ErrAnalysisSuccess
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
//@Param  interface{} function
//@Return error
func (slf *RPCServer) RegRPC(met interface{}) error {
	if reflect.ValueOf(met).Type().Kind() != reflect.Ptr {
		return errors.New("need object")
	}
	slf._rpcs[reflect.TypeOf(met).Elem().Name()] = met
	return nil
}
