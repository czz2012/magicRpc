package client

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
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
	Name           string
	Addr           string
	BufferCap      int
	OutChanSize    int
	SocketTimeout  int64
	Timeout        int64
	Idle           int
	Active         int
	IdleTimeout    int64
	AsyncConnected func(c *RPCClient)
}

//Option param
type Option func(*Options) error

var (
	defaultOptions = Options{Name: "RPC/Client", BufferCap: 8196, OutChanSize: 32, Timeout: 1000 * 1000, SocketTimeout: 1000 * 60, Idle: 2, Active: 2, IdleTimeout: 1000 * 120}
)

// WithName Set RPC client pool name
func WithName(name string) Option {
	return func(o *Options) error {
		o.Name = name
		return nil
	}
}

// WithAddr Set RPC client connection address
func WithAddr(addr string) Option {
	return func(o *Options) error {
		o.Addr = addr
		return nil
	}
}

//WithBufferCap Set Connection Receive Buffer size
func WithBufferCap(cap int) Option {
	return func(o *Options) error {
		o.BufferCap = cap
		return nil
	}
}

//WithTimeout Set Connection operation time out/millsecond
func WithTimeout(tm int64) Option {
	return func(o *Options) error {
		o.Timeout = tm
		return nil
	}
}

//WithSocketTimeout Set Connection connect time out/millsecond
func WithSocketTimeout(tm int64) Option {
	return func(o *Options) error {
		o.SocketTimeout = tm
		return nil
	}
}

//WithOutChanSize Set Connection receive chan size
func WithOutChanSize(n int) Option {
	return func(o *Options) error {
		o.OutChanSize = n
		return nil
	}
}

//WithIdle Set Connection pool idle max of number
func WithIdle(n int) Option {
	return func(o *Options) error {
		o.Idle = n
		return nil
	}
}

//WithActive Set Connection pool max of number
func WithActive(n int) Option {
	return func(o *Options) error {
		o.Active = n
		return nil
	}
}

//WithIdleTimeout Set Connection pool connection idle time out
func WithIdleTimeout(tm int64) Option {
	return func(o *Options) error {
		o.IdleTimeout = tm
		return nil
	}
}

//WithAsyncConnected Set Connected Callback function
func WithAsyncConnected(f func(*RPCClient)) Option {
	return func(o *Options) error {
		o.AsyncConnected = f
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

//GetName doc
//@Summary Return rpc client pool name
//@Return string name
func (slf *RPCClientPool) GetName() string {
	return slf._opts.Name
}

//Call doc
//@Summary Call Remote function non-return
//@Param   string  method name
//@Param   interface param
func (slf *RPCClientPool) Call(method string, param, ret interface{}) error {
	var r proto.Message
	var h *rpcHandle
	var err error
	ick := 0
	startTime := time.Now().UnixNano()
	for {
		h, err = slf.getPool()
		if err != nil {
			if err != code.ErrConnectNoAvailable {
				return err
			}
			ick++
			if ick > 8 {
				ick = 0
				time.Sleep(time.Duration(100) * time.Millisecond)
			}
			currentTime := time.Now().UnixNano()
			if (currentTime-startTime)/int64(time.Millisecond) >
				slf._opts.SocketTimeout {
				return code.ErrTimeOut
			}
			continue
		}
		break
	}

	if ret == nil {
		err = h._client.Call(method, param.(proto.Message))
	} else {
		r, err = h._client.CallReturn(method, param.(proto.Message))
	}

	if err != nil {
		if err == code.ErrConnectClosed {
			return err
		}
		slf.putPool(h)
		return err
	}
	slf.putPool(h)
	if ret != nil {
		reflect.ValueOf(ret).Elem().Set(reflect.ValueOf(r).Elem())
	}

	return nil
}

//Shutdown shutdown Client pools
func (slf *RPCClientPool) Shutdown() {
	slf._isShutdown = true
	slf._wait.Wait()
	slf._sync.Lock()
	for {
		if len(slf._cs) <= 0 {
			break
		}
		v := slf._cs[0]
		slf._cs = slf._cs[1:]
		slf._sz--
		slf._sync.Unlock()
		v._client.Shutdown()
		slf._sync.Lock()
	}

	slf._rpcs = nil
	slf._sync.Unlock()
}

func (slf *RPCClientPool) netClient() (int64, *RPCClient, error) {
	var err error
	newid := atomic.AddInt64(&slf._ids, 1)
	cc := handler.Spawn(fmt.Sprintf("%s/%s/%d", slf._opts.Name, slf._opts.Addr, newid), func() handler.IService {

		rpc := &RPCClient{}

		l, e := connector.Spawn(
			connector.WithSocket(&net.TCPConnection{}),
			connector.WithReceiveDecoder(rpc.rpcDecode),
			connector.WithReceiveBuffer(buffer.NewRing(slf._opts.BufferCap)),
			connector.WithReceiveOutChanSize(slf._opts.OutChanSize),
			connector.WithAsyncClosed(slf.closePool),
			connector.WithUID(slf._ids))

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

	if slf._opts.AsyncConnected != nil {
		slf._opts.AsyncConnected(cc)
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
		slf._sz++
		slf._sync.Unlock()
		newid, c, err := slf.netClient()
		slf._sync.Lock()
		if err != nil {
			slf._sz--
			slf._sync.Unlock()
			return nil, err
		}

		h := &rpcHandle{_id: newid, _client: c, _ref: 2, _status: constClientRun}
		slf._cs = append(slf._cs, h)
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
	if len(slf._cs) > 1 {
		for k := len(slf._cs) - 1; k >= 0; k-- {
			if slf._cs[k]._id == h._id {
				if k > 0 {
					v := slf._cs[k]
					slf.removeClient(k)
					slf._cs = append([]*rpcHandle{v}, slf._cs...)
					slf._sz++
				}
				break
			}
		}

	}

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
			v._client.closeStop()
			break
		}
	}
}

func (slf *RPCClientPool) guard() {
	var rm []*rpcHandle
	var client, v *rpcHandle
	defer slf._wait.Done()
	for !slf._isShutdown {
		slf._sync.Lock()
		startTime := (time.Now().UnixNano() / int64(time.Millisecond))
		if slf._opts.Active > slf._opts.Idle && slf._sz > slf._opts.Idle {
			for k := 0; k < len(slf._cs); {
				v = slf._cs[k]
				if v._status == constClientDel && v._ref <= 1 {
					slf.removeClient(k)
					v._ref = 0
					rm = append(rm, v)
					continue
				}

				if (v._client._idletime - startTime) > slf._opts.IdleTimeout {
					slf._cs[k]._status = constClientDel
				}
				k++
			}
		}
		slf._sync.Unlock()

		for len(rm) > 0 {
			client = rm[0]
			rm = rm[1:]
			fmt.Println("Shutdown RPC Client")
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
