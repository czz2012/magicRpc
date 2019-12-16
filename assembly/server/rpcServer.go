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
	BufferCap    int
	OutCChanSize int

	AsyncError    listener.AsyncErrorFunc
	AsyncComplete listener.AsyncCompleteFunc
	AsyncClosed   listener.AsyncClosedFunc
	AsyncAccept   func(uint64)
}

//Option is a function on the options for a rpc listen.
type Option func(*Options) error

//WithName setting name option
func WithName(name string) Option {
	return func(o *Options) error {
		o.Name = name
		return nil
	}
}

//WithID setting id option
func WithID(id int) Option {
	return func(o *Options) error {
		o.ServerID = id
		return nil
	}
}

//WithClientCap Set accesser cap option
func WithClientCap(cap int) Option {
	return func(o *Options) error {
		o.Cap = cap
		return nil
	}
}

//WithClientKeepTime Set client keep time millsecond option
func WithClientKeepTime(tm int) Option {
	return func(o *Options) error {
		o.KeepTime = tm
		return nil
	}
}

//WithClientBufferCap Set client buffer limit option
func WithClientBufferCap(cap int) Option {
	return func(o *Options) error {
		o.BufferCap = cap
		return nil
	}
}

//WithClientOutSize Set client recvice call chan size option
func WithClientOutSize(outSize int) Option {
	return func(o *Options) error {
		o.OutCChanSize = outSize
		return nil
	}
}

//WithAsyncError Set Listen fail Async Error callback option
func WithAsyncError(f listener.AsyncErrorFunc) Option {
	return func(o *Options) error {
		o.AsyncError = f
		return nil
	}
}

//WithAsyncAccept Set Listen client accept Async callback
func WithAsyncAccept(f func(uint64)) Option {
	return func(o *Options) error {
		o.AsyncAccept = f
		return nil
	}
}

//WithAsyncComplete Set Listen async success callback option
func WithAsyncComplete(f listener.AsyncCompleteFunc) Option {
	return func(o *Options) error {
		o.AsyncComplete = f
		return nil
	}
}

//WithAsyncClosed Set Listen closed async callback
func WithAsyncClosed(f listener.AsyncClosedFunc) Option {
	return func(o *Options) error {
		o.AsyncClosed = f
		return nil
	}
}

var (
	defaultOption = Options{Name: "rpc server",
		ServerID:     1,
		Cap:          1024,
		BufferCap:    8196,
		KeepTime:     1000 * 60,
		OutCChanSize: 512,
	}
)

//New doc
//@Summary new a rpc server
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
		group := &RPCSrvGroup{_id: opts.ServerID, _bfSize: opts.BufferCap, _cap: opts.Cap}

		h, err := listener.Spawn(
			listener.WithListener(&net.TCPListen{}),
			listener.WithAsyncError(opts.AsyncError),
			listener.WithClientKeepTime(opts.KeepTime),
			listener.WithClientOutChanSize(opts.OutCChanSize),
			listener.WithAsyncComplete(opts.AsyncComplete),
			listener.WithAsyncAccept(rpc.rpcAccept),
			listener.WithAsyncClosed(rpc.rpcClosed),
			listener.WithClientGroups(group),
			listener.WithClientDecoder(rpc.rpcDecode))

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
//@
//@Member map[string]interface{}  RPC Function map table
type RPCServer struct {
	_listen      *listener.NetListener
	_rpcs        map[string]interface{}
	_asyncAccept func(uint64)
	_asyncClosed listener.AsyncClosedFunc
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
		return slf._asyncClosed(id)
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
		if err == code.ErrIncompleteData {
			return net.ErrAnalysisProceed
		}
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
