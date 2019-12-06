package assembly

import (
	"sync"

	"github.com/yamakiller/magicNet/handler"
	"github.com/yamakiller/magicNet/handler/implement"
)

//RPCServClientAllocer doc
//@Summary RPC Server Client memory allocator
//@Struct RPCServClientAllocer
//@Inherit implement.IAllocer instance
//@Member int client receive buffer size of max
type RPCServClientAllocer struct {
	_clientReceive int
	_parent        *RPCServer
	_pool          *sync.Pool
}

//Initial doc
//@Summary RPC Server Client Allocer Initial
//@Method Initial
func (slf *RPCServClientAllocer) Initial() {
	slf._pool = &sync.Pool{
		New: func() interface{} {
			c := new(RPCSrvClient)
			c.GetRecvBuffer().Grow(slf._clientReceive)
			return c
		},
	}
}

//New doc
//@Summary RPC Server Client resource new
//@Method New
//@Return (implement.INetClient) a client
func (slf *RPCServClientAllocer) New() implement.INetClient {
	c := handler.Spawn("rpc", func() handler.IService {
		h := slf._pool.Get().(*RPCSrvClient)
		h._parent = slf._parent
		h.GetRecvBuffer().Reset()
		h.Initial()
		return h
	})

	return c.(implement.INetClient)
}

//Delete doc
//@Summary RPC Server Client resource delete
//@Method Delete
//@Param  implement.INetClient need delete RPC Server client
func (slf *RPCServClientAllocer) Delete(p implement.INetClient) {
	p.Shutdown()
	slf._pool.Put(p)
}
