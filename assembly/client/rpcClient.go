package client

import (
	"errors"
	"reflect"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/yamakiller/magicNet/engine/actor"
	"github.com/yamakiller/magicNet/handler/implement/connector"
	"github.com/yamakiller/magicNet/handler/net"
	"github.com/yamakiller/magicNet/timer"
	"github.com/yamakiller/magicRpc/assembly/codec"
	"github.com/yamakiller/magicRpc/code"
)

//RPCClient doc
type RPCClient struct {
	connector.NetConnector
	_parent       *RPCClientPool
	_auth         uint64
	_connTimeout  int64
	_timeOut      int64
	_idletime     int64
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

func (slf *RPCClient) Connection(addr string) error {
	startTime := time.Now().UnixNano()
	if err := slf.NetConnector.Connection(addr); err != nil {
		return err
	}

	ick := 0
	for {
		if slf._auth > 0 {
			return nil
		}

		if slf._connTimeout > 0 {
			if (time.Now().UnixNano() - startTime) > slf._connTimeout {
				slf.Shutdown() //?
				return errors.New("RPC Connection time out")
			}
		}

		ick++
		if ick < 6 {
			continue
		}
		ick = 0
		time.Sleep(time.Millisecond * time.Duration(2))
	}
}

//Call doc
func (slf *RPCClient) Call(method string, param interface{}) error {
	data, err := proto.Marshal(param.(proto.Message))
	if err != nil {
		return err
	}

	data = codec.Encode(1, method, 0, codec.RPCRequest, proto.MessageName(param.(proto.Message)), data)
	return slf.SendTo(data)
}

//CallWait doc
func (slf *RPCClient) CallWait(method string, param interface{}) (interface{}, error) {
	if slf._responseWait != 0 {
		return nil, errors.New("call waitting")
	}

	data, err := proto.Marshal(param.(proto.Message))
	if err != nil {
		return nil, err
	}

	slf._responseWait = slf.incSerial()
	data = codec.Encode(1, method, slf._responseWait, codec.RPCRequest, proto.MessageName(param.(proto.Message)), data)
	if err := slf.SendTo(data); err != nil {
		slf._responseWait = 0
		return nil, err
	}

	for {
		select {
		case isStop := <-slf._responseStop:
			if isStop {
				slf._responseWait = 0
				return nil, errors.New("client close")
			}
		case result := <-slf._response:
			slf._responseWait = 0
			return result, nil
		}
	}
}

func (slf *RPCClient) onRequest(context actor.Context, sender *actor.PID, message interface{}) {
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

func (slf *RPCClient) onResponse(context actor.Context, sender *actor.PID, message interface{}) {
	response := message.(*responseEvent)
	if slf._responseWait != response._ser {
		slf.LogError("RPC Response error not request wait")
		return
	}

	select {
	case isStop := <-slf._responseStop:
		if isStop {
			slf._responseWait = 0
			slf.LogError("client closed")
			return
		}
	case slf._response <- response._return:
	}
}

func (slf *RPCClient) rpcDecode(context actor.Context, params ...interface{}) error {
	c := params[0].(*connector.NetConnector)

	if slf._auth == 0 {
		tmpAuth := c.ReadBuffer(1)
		if tmpAuth[0] != codec.ConstHandShakeCode {
			return errors.New("rpc connection unauthorized")
		}
		slf._auth = timer.Now()
	}

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

	dt := proto.MessageType(block.DName)
	if dt == nil {
		return code.ErrParamUndefined
	}

	data := reflect.New(dt.Elem()).Interface().(proto.Message)
	if err := proto.Unmarshal(block.Data, data); err != nil {
		return err
	}

	if block.Oper == codec.RPCRequest {
		actor.DefaultSchedulerContext.Send(context.Self(), &requestEvent{block.Method, method.Method, data, block.Ser})
	} else {
		actor.DefaultSchedulerContext.Send(context.Self(), &responseEvent{block.Method, data, block.Ser})
	}

	return net.ErrAnalysisSuccess
}

func (slf *RPCClient) incSerial() uint32 {
	slf._serial = ((slf._serial + 1) & 0xFFFFFFF)
	return slf._serial
}
