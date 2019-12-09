package client

import (
	"errors"
	"reflect"

	"github.com/yamakiller/magicRpc/assembly/codec"
	"github.com/yamakiller/magicRpc/assembly/method"
)

//RPCClientPool doc
type RPCClientPool struct {
	//客户端队列
	_sz   int
	_rpcs map[string]*method.RPCMethod
}

func (slf *RPCClientPool) getRPC(name string) *method.RPCMethod {
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
func (slf *RPCClientPool) RegRPC(oper codec.RPCOper, met interface{}) error {
	if reflect.ValueOf(met).Type().Kind() != reflect.Func {
		return errors.New("need method is function")
	}
	slf._rpcs[reflect.TypeOf(met).Name()] = &method.RPCMethod{Method: met, Oper: oper}
	return nil
}
