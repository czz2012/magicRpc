package assembly

import (
	"encoding/binary"
	"reflect"

	"github.com/gogo/protobuf/proto"
	"github.com/yamakiller/magicNet/engine/actor"
	"github.com/yamakiller/magicNet/handler/implement"
	"github.com/yamakiller/magicRpc/code"
)

type rpcServerDeleate struct {
	_parent *RPCServer
}

//Handshake doc
//@Summary accept client handshake
//@Method Handshake
//@Param implement.INetClient client interface
//@Return error
func (slf *rpcServerDeleate) Handshake(c implement.INetClient) error {
	x := make([]byte, 4)
	binary.BigEndian.PutUint32(x, constHandShakeCode)
	c.(*RPCSrvClient).Write(x, len(x))
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
	block, err := Decode(c.GetRecvBuffer())
	if err != nil {
		if err == code.ErrIncompleteData {
			return implement.ErrAnalysisProceed
		}
		return err
	}

	method := slf._parent.getRPCMethod(block.Method)
	if method == nil {
		return code.ErrMethodUndefined
	}

	dataType := slf._parent._messageType(block.DName)
	if dataType == nil {
		return code.ErrParamUndefined
	}

	data := reflect.New(dataType.Elem()).Interface().(proto.Message)
	if err := proto.Unmarshal(block.Data, data); err != nil {
		return err
	}

	if block.Oper == RPCRequest {
		actor.DefaultSchedulerContext.Send((c.(*RPCSrvClient)).GetPID(), &requestEvent{method, data, block.Ser})
	} else {
		actor.DefaultSchedulerContext.Send((c.(*RPCSrvClient)).GetPID(), &responseEvent{})
	}

	return implement.ErrAnalysisSuccess
}

//UnOnlineNotification doc
//@Summary  Offline notification
//@Method UnOnlineNotification
//@Param  (uint64) client handle
//@Return error
func (slf *rpcServerDeleate) UnOnlineNotification(h uint64) error {
	return nil
}
