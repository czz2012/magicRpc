package client

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/yamakiller/magicNet/handler"
	"github.com/yamakiller/magicNet/handler/implement/buffer"
	"github.com/yamakiller/magicNet/handler/implement/connector"
	"github.com/yamakiller/magicNet/handler/net"

	"github.com/yamakiller/magicRpc/assembly/codec"
	"github.com/yamakiller/magicRpc/assembly/method"
	"github.com/yamakiller/magicRpc/code"
)

type Options struct {
	Addr          string
	BufferLimit   int
	OutChanSize   int
	SocketTimeout int64
	Timeout       int64
	Idle          int
	Active        int
	IdleTimeout   int64
}

type Option func(*Options) error

var (
	defaultOptions = Options{BufferLimit: 8196, OutChanSize: 32, Timeout: 1000 * 1000, SocketTimeout: 1000 * 60, Idle: 1, Active: 1, IdleTimeout: 1000 * 120}
)

// Set Socket Handle
func SetAddr(addr string) Option {
	return func(o *Options) error {
		o.Addr = addr
		return nil
	}
}

func SetBufferLimit(limit int) Option {
	return func(o *Options) error {
		o.BufferLimit = limit
		return nil
	}
}

func SetTimeout(tm int64) Option {
	return func(o *Options) error {
		o.Timeout = tm
		return nil
	}
}

func SetSocketTimeout(tm int64) Option {
	return func(o *Options) error {
		o.SocketTimeout = tm
		return nil
	}
}

func SetOutChanSize(n int) Option {
	return func(o *Options) error {
		o.OutChanSize = n
		return nil
	}
}

func SetIdle(n int) Option {
	return func(o *Options) error {
		o.Idle = n
		return nil
	}
}

func SetActive(n int) Option {
	return func(o *Options) error {
		o.Active = n
		return nil
	}
}

func SetIdleTimeout(tm int64) Option {
	return func(o *Options) error {
		o.IdleTimeout = tm
		return nil
	}
}

func New(options ...Option) (*RPCClientPool, error) {

	c := &RPCClientPool{_opts: defaultOptions}
	for _, opt := range options {
		if err := opt(&c._opts); err != nil {
			return nil, err
		}
	}

	var err error
	for i := 0; i < c._opts.Idle; i++ {
		c._ids++
		cc := handler.Spawn(fmt.Sprintf("RPC/Client/%s/%d", c._opts.Addr, i+1), func() handler.IService {

			rpc := &RPCClient{}

			l, e := connector.Spawn(
				connector.SetSocket(&net.TCPConnection{}),
				connector.SetReceiveDecoder(rpc.rpcDecode),
				connector.SetReceiveBuffer(buffer.New(c._opts.BufferLimit)),
				connector.SetReceiveOutChanSize(c._opts.OutChanSize),
				connector.SetAsyncClosed(c.closePool),
				connector.SetUID(c._ids))

			if e != nil {
				err = e
				return nil
			}
			rpc._parent = c
			rpc._timeOut = c._opts.Timeout
			rpc._connTimeout = c._opts.SocketTimeout

			rpc.NetConnector = *l

			rpc.Initial()

			return rpc
		}).(*RPCClient)

		if cc == nil {
			goto fail
		}

		err = cc.Connection(c._opts.Addr)
		if err != nil {
			goto fail
		}

		c._cs = append(c._cs, &rpcHandle{_id: c._ids, _c: cc})
		c._sz++
	}

	c._wait.Add(1)
	go c.guard()

	return c, nil
fail:
	for _, v := range c._cs {
		v._c.Shutdown()
	}
	c._cs = c._cs[len(c._cs):]
	return nil, err
}

type rpcHandle struct {
	_id int64
	_c  *RPCClient
}

//RPCClientPool doc
type RPCClientPool struct {
	_cs         []*rpcHandle
	_cgc        []*rpcHandle
	_ids        int64
	_opts       Options
	_sz         int
	_isShutdown bool
	_wait       sync.WaitGroup
	_rpcs       map[string]*method.RPCMethod
	_sync       sync.Mutex
}

func (slf *RPCClientPool) Call(method string, param interface{}) error {
	h := slf.getPool()
	if h == nil {
		return code.ErrConnectPoolUp
	}
	defer slf.putPool(h)

	return h._c.Call(method, param)
}

func (slf *RPCClientPool) CallWait(method string, param interface{}) (interface{}, error) {
	h := slf.getPool()
	if h == nil {
		return nil, code.ErrConnectPoolUp
	}
	defer slf.putPool(h)

	return h._c.CallWait(method, param)
}

func (slf *RPCClientPool) Shutdown() {
	slf._isShutdown = true
	slf._wait.Wait()

	for {
		slf._sync.Lock()
		if len(slf._cs) <= 0 {
			slf._sync.Unlock()
			break
		}

		v := slf._cs[0]
		slf._cs = slf._cs[1:]
		slf._sync.Unlock()
		v._c.Shutdown()
	}

	for {
		slf._sync.Lock()
		if len(slf._cgc) <= 0 {
			slf._sync.Unlock()
			break
		}

		v := slf._cgc[0]
		slf._cgc = slf._cgc[1:]
		slf._sync.Unlock()
		v._c.Shutdown()
	}

	slf._sync.Lock()
}

//TODO：优化pool管理
func (slf *RPCClientPool) getPool() *rpcHandle {
	slf._sync.Lock()
	defer slf._sync.Unlock()

	if len(slf._cs) == 0 {
		return nil
	}

	c := slf._cs[0]
	slf._cs = slf._cs[1:]

	return c
}

func (slf *RPCClientPool) putPool(h *rpcHandle) {
	h._c._idletime = time.Now().UnixNano()

	slf._sync.Lock()
	defer slf._sync.Unlock()
	if !h._c.IsConnected() {
		slf._cgc = append(slf._cgc, h)
		slf._sz--
		return
	}

	slf._cs = append(slf._cs, h)
}

func (slf *RPCClientPool) closePool(handle int64) {
	if slf._isShutdown {
		return
	}

	slf._sync.Lock()
	defer slf._sync.Unlock()

	for idx, v := range slf._cs {
		if v._id == handle {
			i := idx + 1
			slf._cs = append(slf._cs[:i], slf._cs[i+1:]...)
			slf._cgc = append(slf._cgc, v)
			slf._sz--
			break
		}
	}
}

func (slf *RPCClientPool) guard() {
	defer slf._wait.Done()
	for !slf._isShutdown {
		slf._sync.Lock()
		for len(slf._cgc) > 0 {
			v := slf._cgc[0]
			slf._cgc = slf._cgc[1:]
			slf._sync.Unlock()
			v._c.Shutdown()
			slf._sync.Lock()
		}
		slf._sync.Unlock()
		//关闭空闲连接
		time.Sleep(time.Duration(350) * time.Millisecond)
	}
}

func (slf *RPCClientPool) getRPC(name string) *method.RPCMethod {
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
func (slf *RPCClientPool) RegRPC(oper codec.RPCOper, met interface{}) error {
	if reflect.ValueOf(met).Type().Kind() != reflect.Func {
		return errors.New("need method is function")
	}
	slf._rpcs[reflect.TypeOf(met).Name()] = &method.RPCMethod{Method: met, Oper: oper}
	return nil
}
