package assembly

import "github.com/yamakiller/magicNet/handler/implement"

/*
	GetName() string
	GetAddr() string
	GetOutSize() int
	IsTimeout() uint64
	RestTimeout()
	SetStatus(stat NetConnectStatus)
	GetStatus() NetConnectStatus
*/

type rpcTarget struct {
	_addr string
}

func (slf *rpcTarget) GetAddr() string {
	return slf._addr
}

//RPCClient doc
type RPCClient struct {
	implement.NetConnectService
}
