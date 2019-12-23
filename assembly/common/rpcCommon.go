package common

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/yamakiller/magicNet/handler/net"

	"github.com/gogo/protobuf/proto"
	"github.com/yamakiller/magicRpc/code"
)

//GetRPCMethod Return Register rpc method
type GetRPCMethod func(name string) interface{}

//MethodSplit Return class.method
func methodSplit(name string) []string {
	return strings.Split(name, ".")
}

//Call Run Remote function
func Call(method string, param interface{}) ([]byte, error) {
	var data []byte
	var err error
	var dataName string
	if param != nil {
		data, err = proto.Marshal(param.(proto.Message))
		if err != nil {
			return nil, err
		}
		dataName = proto.MessageName(param.(proto.Message))
	}

	data = Encode(ConstVersion, method, 0, RPCRequest, dataName, data)

	return data, nil
}

func rpcDecode(rpcGet GetRPCMethod,
	bf net.INetReceiveBuffer) (*Block, interface{}, proto.Message, error) {
	block, err := Decode(bf)
	if err != nil {
		if err == code.ErrIncompleteData {
			return nil, nil, nil, net.ErrAnalysisProceed
		}
		return nil, nil, nil, err
	}

	methodName := methodSplit(block.Method)
	var mObj interface{}
	if block.Oper == RPCRequest {
		mObj = rpcGet(methodName[0])
		if mObj == nil || len(methodName) != 2 {
			return nil, nil, nil, code.ErrMethodUndefined
		}

		if !reflect.ValueOf(mObj).MethodByName(methodName[1]).IsValid() {
			return nil, nil, nil, code.ErrMethodUndefined
		}
	}

	var data proto.Message
	if block.DataName != "" {
		dt := proto.MessageType(block.DataName)
		if dt == nil {
			return nil, nil, nil, code.ErrParamUndefined
		}

		data = reflect.New(dt.Elem()).Interface().(proto.Message)
		if err := proto.Unmarshal(block.Data, data); err != nil {
			return nil, nil, nil, err
		}
	}
	return block, mObj, data, nil
}

//RPCDecodeServer RPC Server decode
func RPCDecodeServer(rpcGet GetRPCMethod,
	bf net.INetReceiveBuffer) (interface{}, error) {
	block, mObj, data, err := rpcDecode(rpcGet, bf)
	if err != nil {
		return nil, err
	}

	return &RequestEvent{block.Method, mObj, data, block.Ser}, nil
}

//RPCDecodeClient RPC Client decode
func RPCDecodeClient(rpcGet GetRPCMethod,
	bf net.INetReceiveBuffer) (interface{}, error) {
	block, mObj, data, err := rpcDecode(rpcGet, bf)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if block.Oper == RPCRequest {
		result = &RequestEvent{block.Method, mObj, data, block.Ser}
	} else {
		result = &ResponseEvent{block.Method, data, block.Ser}
	}
	return result, nil
}

//RPCRequestProcess doc
//@Summary RPC Request proccess
//@Method RPCRequest
//@Param  *event.RequestEvent
//@Return []byte
//@Return error
func RPCRequestProcess(c interface{},
	sendto func([]byte) error,
	message interface{}) error {

	request := message.(*RequestEvent)
	methodName := methodSplit(request.MethodName)
	method := reflect.ValueOf(request.Method).MethodByName(methodName[1])
	paramNumber := 1
	if request.Param != nil {
		paramNumber++
	}
	params := make([]reflect.Value, paramNumber)
	params[0] = reflect.ValueOf(c)
	if request.Param != nil {
		params[1] = reflect.ValueOf(request.Param)
	}

	rs := method.Call(params)
	if len(rs) > 0 {
		msgPb := rs[0].Interface().(proto.Message)
		data, err := proto.Marshal(msgPb)
		if err != nil {
			return fmt.Errorf("RPC Response error:%s  =>  %d[%+v]", request.Method, request.Ser, err)
		}

		data = Encode(ConstVersion, request.MethodName, request.Ser, RPCResponse, proto.MessageName(msgPb), data)
		if err := sendto(data); err != nil {
			return fmt.Errorf("RPC Response error:%s  => %d[%+v]", request.Method, request.Ser, err)
		}
	}
	return nil
}
