package server

import (
	"github.com/yamakiller/magicNet/engine/actor"
	"github.com/yamakiller/magicNet/handler/implement/client"
	"github.com/yamakiller/magicRpc/assembly/common"
)

//RPCSrvClient doc
//@Summary RPC server accesser
//@Struct RPCSrvClient
//@
//@Member uint64 is handle/id
type RPCSrvClient struct {
	client.NetSSrvCleint
	_handle uint64
}

//Initial doc
//@Summary Initial rpc server accesser
//@Method Initial
func (slf *RPCSrvClient) Initial() {
	slf.NetSSrvCleint.Initial()
	slf.RegisterMethod(&common.RequestEvent{}, slf.onRequest)
}

//SetID doc
//@Summary Setting handle/id
//@Method SetID
//@Param uint64  handle/id
func (slf *RPCSrvClient) SetID(id uint64) {
	slf._handle = id
}

//GetID doc
//@Summary Returns handle/id
//@Method GetID
//@Return uint64
func (slf *RPCSrvClient) GetID() uint64 {
	return slf._handle
}

//Call doc
func (slf *RPCSrvClient) Call(method string, param interface{}) error {
	data, err := common.Call(method, param)
	if err != nil {
		return err
	}
	return slf.SendTo(data)
}

func (slf *RPCSrvClient) onRequest(context actor.Context, sender *actor.PID, message interface{}) {
	if err := common.RPCRequestProcess(context, slf.SendTo, message); err != nil {
		slf.LogError("%s", err)
		return
	}
}
