package assembly

import (
	"errors"
	"reflect"

	"github.com/yamakiller/magicNet/handler/implement"
)

//RPCServer doc
//@Summary rpc server
//@Struct RPCServer
//@Inherit implement.NetListenService
type RPCServer struct {
	implement.NetListenService

	_rpcs        map[string]interface{}
	_messageType func(name string) reflect.Type
	_messageName func(v reflect.Type) string
}

//Initial doc
//@Summary initialize rpc server
//@Method Initial
func (slf *RPCServer) Initial() {
	slf._rpcs = make(map[string]interface{})
	slf.NetListenService.Initial()
}

func (slf *RPCServer) getRPCMethod(name string) interface{} {
	f, ok := slf._rpcs[name]
	if !ok {
		return nil
	}
	return f
}

//RegisterRPCMethod doc
//@Summary register rpc method function
//@Param string is method name
//@Param interface{} method
//@Return error
func (slf *RPCServer) RegisterRPCMethod(name string, method interface{}) error {
	if reflect.ValueOf(method).Type().Kind() != reflect.Func {
		return errors.New("need method is function")
	}
	slf._rpcs[name] = method
	return nil
}
