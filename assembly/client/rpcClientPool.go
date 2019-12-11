package client

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yamakiller/magicNet/handler"
	"github.com/yamakiller/magicNet/handler/implement/buffer"
	"github.com/yamakiller/magicNet/handler/implement/connector"
	"github.com/yamakiller/magicNet/handler/net"
	"github.com/yamakiller/magicRpc/code"
)

//Options doc
//@Summary
//@Method string address
//@Method int    receive buffer size
//@Method int    receive event chan size
//@Method int    socket connection time out/millsecond
//@Method int    operation time out/millsecond
//@Method int    connection idle max of number
//@Method int    connection max of number
//@Method int    connection idle time out
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

//Option param
type Option func(*Options) error

var (
	defaultOptions = Options{BufferLimit: 8196, OutChanSize: 32, Timeout: 1000 * 1000, SocketTimeout: 1000 * 60, Idle: 1, Active: 1, IdleTimeout: 1000 * 120}
)

// SetAddr Set Socket Handle
func SetAddr(addr string) Option {
	return func(o *Options) error {
		o.Addr = addr
		return nil
	}
}

//SetBufferLimit Set Connection Receive Buffer size
func SetBufferLimit(limit int) Option {
	return func(o *Options) error {
		o.BufferLimit = limit
		return nil
	}
}

//SetTimeout Set Connection operation time out/millsecond
func SetTimeout(tm int64) Option {
	return func(o *Options) error {
		o.Timeout = tm
		return nil
	}
}

//SetSocketTimeout Set Connection connect time out/millsecond
func SetSocketTimeout(tm int64) Option {
	return func(o *Options) error {
		o.SocketTimeout = tm
		return nil
	}
}

//SetOutChanSize Set Connection receive chan size
func SetOutChanSize(n int) Option {
	return func(o *Options) error {
		o.OutChanSize = n
		return nil
	}
}

//SetIdle Set Connection pool idle max of number
func SetIdle(n int) Option {
	return func(o *Options) error {
		o.Idle = n
		return nil
	}
}

//SetActive Set Connection pool max of number
func SetActive(n int) Option {
	return func(o *Options) error {
		o.Active = n
		return nil
	}
}

//SetIdleTimeout Set Connection pool connection idle time out
func SetIdleTimeout(tm int64) Option {
	return func(o *Options) error {
		o.IdleTimeout = tm
		return nil
	}
}

//New doc
//@Summary new RPC connection pool
//@Param  ...Option
func New(options ...Option) (*RPCClientPool, error) {

	c := &RPCClientPool{_opts: defaultOptions}
	for _, opt := range options {
		if err := opt(&c._opts); err != nil {
			return nil, err
		}
	}

	var err error
	var cc *RPCClient
	var newid int64
	for i := 0; i < c._opts.Idle; i++ {
		newid, cc, err = c.netClient()
		if err != nil {
			goto fail
		}

		c._cs = append(c._cs, &rpcHandle{_id: newid, _client: cc, _ref: 1, _status: constClientIdle})
		c._sz++
	}

	c._wait.Add(1)
	go c.guard()

	return c, nil
fail:
	for _, v := range c._cs {
		v._client.Shutdown()
	}
	c._cs = c._cs[len(c._cs):]
	return nil, err
}

type rpcHandle struct {
	_id     int64
	_client *RPCClient
	_ref    int
	_status int //idle/use/del
}

//RPCClientPool doc
type RPCClientPool struct {
	_cs         []*rpcHandle
	_ids        int64
	_opts       Options
	_sz         int
	_isShutdown bool
	_wait       sync.WaitGroup
	_rpcs       map[string]interface{}
	_sync       sync.Mutex
}

//Call doc
//@Summary Call Remote function non-return
//@Param   string  method name
//@Param   interface param
func (slf *RPCClientPool) Call(method string, param interface{}) error {
	h, err := slf.getPool()
	if err != nil {
		return err
	}
	defer slf.putPool(h)

	return h._client.Call(method, param)
}

//CallWait doc
func (slf *RPCClientPool) CallWait(method string, param interface{}) (interface{}, error) {
	h, err := slf.getPool()
	if err != nil {
		return nil, err
	}

	defer slf.putPool(h)

	return h._client.CallWait(method, param)
}

//Shutdown shutdown Client pools
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
		slf._sz--
		slf._sync.Unlock()
		v._client.Shutdown()
	}

	slf._rpcs = nil
	slf._sync.Lock()
}

func (slf *RPCClientPool) netClient() (int64, *RPCClient, error) {
	var err error
	newid := atomic.AddInt64(&slf._ids, 1)
	cc := handler.Spawn(fmt.Sprintf("RPC/Client/%s/%d", slf._opts.Addr, newid), func() handler.IService {

		rpc := &RPCClient{}

		l, e := connector.Spawn(
			connector.SetSocket(&net.TCPConnection{}),
			connector.SetReceiveDecoder(rpc.rpcDecode),
			connector.SetReceiveBuffer(buffer.New(slf._opts.BufferLimit)),
			connector.SetReceiveOutChanSize(slf._opts.OutChanSize),
			connector.SetAsyncClosed(slf.closePool),
			connector.SetUID(slf._ids))

		if e != nil {
			err = e
			return nil
		}

		rpc._parent = slf
		rpc._timeOut = slf._opts.Timeout
		rpc._connTimeout = slf._opts.SocketTimeout
		rpc._idletime = (time.Now().UnixNano() / int64(time.Millisecond))

		rpc.NetConnector = *l

		rpc.Initial()

		return rpc
	}).(*RPCClient)

	if cc == nil {
		return 0, nil, err
	}

	err = cc.Connection(slf._opts.Addr)
	if err != nil {
		cc.Shutdown()
		return 0, nil, err
	}

	return newid, cc, nil
}

//getPool Return Client
func (slf *RPCClientPool) getPool() (*rpcHandle, error) {
	slf._sync.Lock()

	for idx, v := range slf._cs {
		if v._status != constClientIdle {
			continue
		}

		v._ref++
		v._status = constClientRun

		if len(slf._cs) > 1 {
			slf.removeClient(idx)
			slf._cs = append(slf._cs, v)
			slf._sz++
		}
		slf._sync.Unlock()
		return v, nil
	}

	if slf._sz < slf._opts.Active {
		newid, c, err := slf.netClient()
		if err != nil {
			slf._sync.Unlock()
			return nil, err
		}

		h := &rpcHandle{_id: newid, _client: c, _ref: 2, _status: constClientRun}
		slf._cs = append(slf._cs, h)
		slf._sz++
		slf._sync.Unlock()
		return h, nil
	}
	slf._sync.Unlock()
	return nil, code.ErrConnectNoAvailable
}

func (slf *RPCClientPool) putPool(h *rpcHandle) {
	h._client._idletime = (time.Now().UnixNano() / int64(time.Millisecond))

	slf._sync.Lock()
	defer slf._sync.Unlock()
	if !h._client.IsConnected() || h._status == constClientDel {
		h._status = constClientDel
		h._ref--
		return
	}
	h._ref--
	h._status = constClientIdle
}

func (slf *RPCClientPool) closePool(handle int64) {
	if slf._isShutdown {
		return
	}

	slf._sync.Lock()
	defer slf._sync.Unlock()

	for idx, v := range slf._cs {
		if v._id == handle {
			v._status = constClientDel
			if idx == 0 && len(slf._cs) > 1 {
				slf._cs = slf._cs[1:]
				slf._cs = append(slf._cs, v)
			}
			break
		}
	}
}

func (slf *RPCClientPool) guard() {
	var rm []*rpcHandle
	var client *rpcHandle
	defer slf._wait.Done()
	for !slf._isShutdown {
		slf._sync.Lock()
		startTime := (time.Now().UnixNano() / int64(time.Millisecond))
		if slf._opts.Active > slf._opts.Idle && slf._sz > slf._opts.Idle {
			for k, v := range slf._cs {
				if v._status == constClientDel && v._ref <= 1 {
					slf.removeClient(k)
					v._ref = 0
					rm = append(rm, v)
					continue
				}

				if (v._client._idletime - startTime) > slf._opts.IdleTimeout {
					v._status = constClientDel
				}
			}
		}
		slf._sync.Unlock()

		for len(rm) > 0 {
			client = rm[0]
			rm = rm[1:]
			client._client.Shutdown()
		}

		time.Sleep(time.Duration(500) * time.Millisecond)
	}
}

func (slf *RPCClientPool) removeClient(idx int) {
	i := idx + 1
	if i == 1 {
		slf._cs = slf._cs[i:]
	} else if i == len(slf._cs) {
		slf._cs = slf._cs[:i-1]
	} else {
		slf._cs = append(slf._cs[:i-1], slf._cs[i:]...)
	}
	slf._sz--
}

func (slf *RPCClientPool) getRPC(name string) interface{} {
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
func (slf *RPCClientPool) RegRPC(met interface{}) error {
	if reflect.ValueOf(met).Type().Kind() != reflect.Ptr {
		return errors.New("need object")
	}

	slf._rpcs[reflect.TypeOf(met).Elem().Name()] = met
	return nil
}
