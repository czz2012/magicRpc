package server

import (
	"reflect"

	"github.com/gogo/protobuf/proto"
	"github.com/yamakiller/magicNet/engine/actor"
	"github.com/yamakiller/magicNet/handler/implement/client"
	"github.com/yamakiller/magicRpc/assembly/codec"
)

//RPCSrvClient doc
//@Summary RPC server accesser
//@Struct RPCSrvClient
//@
//@Member uint64 is handle/id
type RPCSrvClient struct {
	client.NetSSrvCleint
	_handle uint64
	//_response     chan proto.Message
	//_responseStop chan bool
	//_responseWait bool
}

//Initial doc
//@Summary Initial rpc server accesser
//@Method Initial
func (slf *RPCSrvClient) Initial() {
	//slf._response = make(chan proto.Message)
	//slf._responseStop = make(chan bool, 1)
	slf.NetSSrvCleint.Initial()
	slf.RegisterMethod(&requestEvent{}, slf.onRequest)
	//slf.RegisterMethod(&responseEvent{}, slf.onResponse)
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
	data, err := proto.Marshal(param.(proto.Message))
	if err != nil {
		return err
	}

	data = codec.Encode(1, method, 0, codec.RPCRequest, proto.MessageName(param.(proto.Message)), data)
	if err := slf.SendTo(data); err != nil {
		return err
	}

	return nil
}

/*
//CallWait doc 需要增加超时操作
func (slf *RPCSrvClient) CallWait(method string, param interface{}) (interface{}, error) {
	if slf._responseWait {
		return nil, errors.New("call waitting")
	}
	data, err := proto.Marshal(param.(proto.Message))
	if err != nil {
		return nil, err
	}

	slf._responseWait = true
	data = codec.Encode(1, method, 0, codec.RPCRequest, proto.MessageName(param.(proto.Message)), data)
	if err := slf.SendTo(data); err != nil {
		slf._responseWait = false
		return nil, err
	}

	for {
		select {
		case isStop := <-slf._responseStop:
			if isStop {
				slf._responseWait = false
				return nil, errors.New("client close")
			}
		case result := <-slf._response:
			slf._responseWait = false
			return result, nil
		}
	}
}
*/

func (slf *RPCSrvClient) onRequest(context actor.Context, sender *actor.PID, message interface{}) {
	request := message.(*requestEvent)
	method := reflect.ValueOf(request._method)
	params := make([]reflect.Value, 1)
	params[0] = reflect.ValueOf(request._param)

	rs := method.Call(params)
	if len(rs) > 0 {
		msgPb := rs[0].Interface().(proto.Message)
		data, err := proto.Marshal(msgPb)
		if err != nil {
			slf.LogError("RPC Response error:%s  =>  %d[%+v]", request._method, request._ser, err)
			return
		}

		data = codec.Encode(1, request._methodName, request._ser, codec.RPCResponse, proto.MessageName(msgPb), data)
		if err := slf.SendTo(data); err != nil {
			slf.LogError("RPC Response error:%s  => %d[%+v]", request._method, request._ser, err)
		}
		return
	}
	return
}

/*func (slf *RPCSrvClient) onResponse(context actor.Context, sender *actor.PID, message interface{}) {
	response := message.(*responseEvent)
	if !slf._responseWait {
		slf.LogError("RPC Response error not request wait")
		return
	}

	select {
	case isStop := <-slf._responseStop:
		if isStop {
			slf._responseWait = false
			slf.LogError("client closed")
			return
		}
	case slf._response <- response._return:
	}
}*/
