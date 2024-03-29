package server

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/yamakiller/magicLibs/util"
	"github.com/yamakiller/magicNet/handler"
	"github.com/yamakiller/magicNet/handler/implement/buffer"
	"github.com/yamakiller/magicNet/handler/net"
)

const (
	constIDMask  = 0x1F
	constIDShift = 5
)

//RPCSrvClientAllocer doc
//@Summary RPC  Visitor Allocator
//@Member _parent *RPCSrvGroup Allocator parent and manager objects
//@Member _pool *sync.Pool Memory pool
type RPCSrvClientAllocer struct {
	_parent *RPCSrvGroup
	_pool   *sync.Pool
	_sn     uint32
}

//Initial doc
//@Summary Initialize the allocator
func (slf *RPCSrvClientAllocer) Initial() {
	slf._pool = &sync.Pool{
		New: func() interface{} {
			c := new(RPCSrvClient)
			c.ReceiveBuffer = buffer.NewRing(slf._parent._bfSize)
			return c
		},
	}
}

//New doc
//@Summary Allocate a new RPC visitor object
//@Return net.INetClient A new RPC visitor object
func (slf *RPCSrvClientAllocer) New() net.INetClient {
	c := handler.Spawn(fmt.Sprintf("rpc/client/%d/%d",
		slf._parent._id,
		atomic.AddUint32(&slf._sn, 1)), func() handler.IService {

		h := slf._pool.Get().(*RPCSrvClient)
		h.ClearBuffer()
		h.Initial()
		return h
	})

	return c.(net.INetClient)
}

//Delete doc
//@Summary Release an RPC visitor object
//@Param  p net.INetClient The object that needs to be released
func (slf *RPCSrvClientAllocer) Delete(p net.INetClient) {
	p.Shutdown()
	slf._pool.Put(p)
}

//RPCSrvGroup doc
type RPCSrvGroup struct {
	_id        int
	_handles   map[uint64]net.INetClient
	_sockets   map[int32]net.INetClient
	_snowflake *util.SnowFlake
	_allocer   *RPCSrvClientAllocer
	_bfSize    int
	_cap       int
	_sz        int
	_sync      sync.Mutex
}

//Initial doc
//@Summary initialization rpc server client manage
func (slf *RPCSrvGroup) Initial() {
	slf._handles = make(map[uint64]net.INetClient)
	slf._sockets = make(map[int32]net.INetClient)
	slf._allocer = &RPCSrvClientAllocer{_parent: slf}
	workerID := int((slf._id >> constIDShift) & constIDMask)
	subWorkerID := (slf._id & constIDMask)
	slf._snowflake = util.NewSnowFlake(int64(workerID), int64(subWorkerID))
	slf._allocer.Initial()
}

//Allocer doc
//@Summary Retruns A distributor
//@Return net.IAllocer
func (slf *RPCSrvGroup) Allocer() net.IAllocer {
	return slf._allocer
}

//Occupy doc
//@Summary  occupy a client resouse
//@Param (implement.INetClient) a client object
//@Return (uint64) a resouse id
//@Return (error) error informat
func (slf *RPCSrvGroup) Occupy(c net.INetClient) (uint64, error) {
	slf._sync.Lock()
	defer slf._sync.Unlock()

	handle, err := slf._snowflake.NextID()
	if err != nil {
		return 0, err
	}

	handleKey := uint64(handle)

	slf._handles[handleKey] = c
	slf._sockets[c.GetSocket()] = c

	c.SetRef(2)
	c.WithID(handleKey)
	slf._sz++

	return handleKey, nil
}

//Grap doc
//@Summary return client and inc add 1
//@Param (uint64) a client (Handle/ID)
//@Return (implement.INetClient) a client
func (slf *RPCSrvGroup) Grap(h uint64) net.INetClient {
	slf._sync.Lock()
	defer slf._sync.Unlock()

	if v, ok := slf._handles[h]; ok {
		v.IncRef()
		return v
	}

	return nil
}

//GrapSocket doc
//@Summary return client and ref add 1
//@Param (int32) a socket id
//@Return (implement.INetClient) a client
func (slf *RPCSrvGroup) GrapSocket(sock int32) net.INetClient {
	slf._sync.Lock()
	defer slf._sync.Unlock()

	if v, ok := slf._sockets[sock]; ok {
		v.IncRef()
		return v
	}

	return nil
}

//Erase doc
//@Summary remove client
//@Param (uint64) a client is (Handle/ID)
func (slf *RPCSrvGroup) Erase(h uint64) {
	slf._sync.Lock()
	defer slf._sync.Unlock()

	c, ok := slf._handles[h]
	if !ok {
		return
	}

	s := c.GetSocket()
	if s > 0 {
		if _, ok = slf._sockets[s]; ok {
			delete(slf._sockets, s)
		}
	}

	delete(slf._handles, h)

	if c.DecRef() <= 0 {
		slf.Allocer().Delete(c)
	}
	slf._sz--
}

//Release doc
//@Summary release client grap
//@Param implement.INetClient a client
func (slf *RPCSrvGroup) Release(c net.INetClient) {
	slf._sync.Lock()
	defer slf._sync.Unlock()

	if c.DecRef() <= 0 {
		slf.Allocer().Delete(c)
	}
}

//Size doc
//@Summary Return Number of connected clients
//@Return int
func (slf *RPCSrvGroup) Size() int {
	slf._sync.Lock()
	defer slf._sync.Unlock()
	return slf._sz
}

//Cap doc
//@Summary Returns Maximum number of client connections
func (slf *RPCSrvGroup) Cap() int {
	return slf._cap
}

//GetHandles doc
//@Summary Returns Clients in all connections Handle/ID
//@Return ([]uint64) all client of (Handle/ID)
func (slf *RPCSrvGroup) GetHandles() []uint64 {
	slf._sync.Lock()
	defer slf._sync.Unlock()

	i := 0
	result := make([]uint64, slf._sz)
	for _, v := range slf._handles {
		result[i] = v.GetID()
		i++
	}

	return result
}
