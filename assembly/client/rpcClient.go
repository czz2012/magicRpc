package client

import (
	"github.com/gogo/protobuf/proto"
	"github.com/yamakiller/magicNet/engine/actor"
	"github.com/yamakiller/magicNet/handler/implement/connector"
	"github.com/yamakiller/magicNet/handler/net"
	"github.com/yamakiller/magicRpc/assembly/codec"
	"github.com/yamakiller/magicRpc/code"
)

//Options RPC Client Options
type Options struct {
	TimeOut     int
	BufferLimit int
	OutChanSize int
}

//RPCClient doc
type RPCClient struct {
	connector.NetConnector
	_parent       *RPCClientPool
	_serial       uint32
	_response     chan proto.Message
	_responseStop chan bool
	_responseWait uint32
}

//Initial doc
//@Summary Initial RPC Client service initial
//@Method Initial
func (slf *RPCClient) Initial() {
	slf._response = make(chan proto.Message)
	slf._responseStop = make(chan bool, 1)
	slf.NetConnector.Initial()
	slf.RegisterMethod(&requestEvent{}, slf.onRequest)
	slf.RegisterMethod(&responseEvent{}, slf.onResponse)
}

//Call doc
func (slf *RPCClient) Call(method string, param interface{}) error {

	return nil
}

//CallWait doc
func (slf *RPCClient) CallWait(method string, param interface{}) (interface{}, error) {
	return nil, nil
}

func (slf *RPCClient) onRequest(context actor.Context, sender *actor.PID, message interface{}) {

}

func (slf *RPCClient) onResponse(context actor.Context, sender *actor.PID, message interface{}) {

}

func (slf *RPCClient) rpcDecode(ontext actor.Context, params ...interface{}) error {
	c := params[0].(*connector.NetConnector)

	block, err := codec.Decode(c)
	if err != nil {
		if err == code.ErrIncompleteData {
			return net.ErrAnalysisProceed
		}
		return err
	}
	method := slf._parent.getRPC(block.Method)
	if method == nil {
		return code.ErrMethodUndefined
	}

	return nil
}

func (slf *RPCClient) incSerial() uint32 {
	slf._serial = ((slf._serial + 1) & 0xFFFFFFF)
	return slf._serial
}
