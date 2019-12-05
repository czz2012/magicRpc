package assembly

import (
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
