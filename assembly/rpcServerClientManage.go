package assembly

import (
	"sync"

	"github.com/yamakiller/magicLibs/util"
	"github.com/yamakiller/magicNet/handler/implement"
	"github.com/yamakiller/magicRpc/code"
)

const (
	constIDMask  = 0x1F
	constIDShift = 5
)

type RPCServerManage struct {
	implement.NetClientManager
	_id        int
	_handles   map[uint64]implement.INetClient
	_sockets   map[int32]implement.INetClient
	_snowflake *util.SnowFlake
	_max       int
	_sz        int
	_sync      sync.Mutex
}

//Initial doc
//@Summary initialization rpc server client manage
//@Method Initial
func (slf *RPCServerManage) Initial() {
	slf._handles = make(map[uint64]implement.INetClient)
	slf._sockets = make(map[int32]implement.INetClient)
	workerID := int((slf._id >> constIDShift) & constIDMask)
	subWorkerID := (slf._id & constIDMask)
	slf._snowflake = util.NewSnowFlake(int64(workerID), int64(subWorkerID))
}

//Occupy doc
//@Summary  occupy a client resouse
//@Method Occupy
//@Param (implement.INetClient) a client object
//@Return (uint64) a resouse id
//@Return (error) error informat
func (slf *RPCServerManage) Occupy(c implement.INetClient) (uint64, error) {
	slf._sync.Lock()
	defer slf._sync.Unlock()

	if slf._sz >= slf._max {
		return 0, code.ErrConnectFull
	}

	handle, err := slf._snowflake.NextID()
	if err != nil {
		return 0, err
	}

	handleKey := uint64(handle)

	slf._handles[handleKey] = c
	slf._sockets[c.GetSocket()] = c

	c.SetRef(2)
	c.SetID(handleKey)
	slf._sz++

	return handleKey, nil
}

//Grap doc
//@Summary return client and inc add 1
//@Method Grap
//@Param (uint64) a client (Handle/ID)
//@Return (implement.INetClient) a client
func (slf *RPCServerManage) Grap(h uint64) implement.INetClient {
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
//@Method GrapSocket desc
//@Param (int32) a socket id
//@Return (implement.INetClient) a client
func (slf *RPCServerManage) GrapSocket(sock int32) implement.INetClient {
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
//@Method Erase
//@Param (uint64) a client is (Handle/ID)
func (slf *RPCServerManage) Erase(h uint64) {
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
//@Method Release
//@Param implement.INetClient a client
func (slf *RPCServerManage) Release(c implement.INetClient) {
	slf._sync.Lock()
	defer slf._sync.Unlock()

	if c.DecRef() <= 0 {
		slf.Allocer().Delete(c)
	}
}

//Size doc
//@Summary Return client number
//@Method Size
//@Return int
func (slf *RPCServerManage) Size() int {
	slf._sync.Lock()
	defer slf._sync.Unlock()
	return slf._sz
}

//GetHandles doc
//@Summary return client handles
//@Method GetHandles
//@Return ([]uint64) all client of (Handle/ID)
func (slf *RPCServerManage) GetHandles() []uint64 {
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
