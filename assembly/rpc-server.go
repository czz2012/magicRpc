package assembly

import "github.com/yamakiller/magicNet/handler/implement"

//RPCServer doc
//@Summary rpc server
//@Struct RPCServer
//@Inherit implement.NetListenService
type RPCServer struct {
	implement.NetListenService
}
