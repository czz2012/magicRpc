package assembly

import (
	"github.com/yamakiller/magicNet/engine/actor"
	"github.com/yamakiller/magicNet/handler/implement"
)

type rpcServerDeleate struct {
}

//Handshake doc
//@Summary accept client handshake
//@Method Handshake
//@Param implement.INetClient client interface
//@Return error
func (slf *rpcServerDeleate) Handshake(c implement.INetClient) error {
	return nil
}

//Decode doc
//@Summary client receive decode
//@Method Decode
//@Param actor.Context listen service context
//@Param *implement.NetListenService listen service
//@Param implement.INetClient client
//@Return error
func (slf *rpcServerDeleate) Decode(context actor.Context,
	nets *implement.NetListenService,
	c implement.INetClient) error {
	return nil
}

//UnOnlineNotification doc
//@Summary  Offline notification
//@Method UnOnlineNotification
//@Param  (uint64) client handle
//@Return error
func (slf *rpcServerDeleate) UnOnlineNotification(h uint64) error {
	return nil
}
