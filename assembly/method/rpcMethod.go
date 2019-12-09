package method

import "github.com/yamakiller/magicRpc/assembly/codec"

//RPCMethod Rpc Method
type RPCMethod struct {
	Method interface{}
	Oper   codec.RPCOper
}
