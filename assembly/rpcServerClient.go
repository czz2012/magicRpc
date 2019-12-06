package assembly

import (
	"reflect"

	"github.com/gogo/protobuf/proto"
	"github.com/yamakiller/magicNet/engine/actor"
	"github.com/yamakiller/magicNet/handler/implement"
	"github.com/yamakiller/magicNet/network"
)

//RPCSrvClient doc
//@Summary
//@Inherit  implement.NetClientService
type RPCSrvClient struct {
	implement.NetClientService
	_parent *RPCServer
	_handle uint64
	_socket int32
	_serial uint32
}

//Initial doc
//@Summary initialize rpc server client
//@Method Initial
func (slf *RPCSrvClient) Initial() {
	slf.NetClientService.Initial()
	slf.RegisterMethod(&requestEvent{}, slf.onRequest)
	slf.RegisterMethod(&responseEvent{}, slf.onResponse)
}

//WithParent doc
//@Summary with parent server
//@Method WithParent
//@Param *RPCServer parent server
func (slf *RPCSrvClient) WithParent(srv *RPCServer) {
	slf._parent = srv
}

//SetID doc
//@Summary Setting the client ID
//@Method  SetID desc
//@Param  (uint64) id
func (slf *RPCSrvClient) SetID(v uint64) {
	slf._handle = v
}

//GetID doc
//@Summary Returns the client ID
//@Method GetID
//@Return (uint64) return current  rpc server client handle
func (slf *RPCSrvClient) GetID() uint64 {
	return slf._handle
}

//GetSocket doc
//@Summary Returns the gateway client socket
//@Method GetSocket
//@Return Returns the gateway client socket
func (slf *RPCSrvClient) GetSocket() int32 {
	return slf._socket
}

//SetSocket doc
//@Summary Setting the gateway client socket
//@Method  SetSocket
//@Param   (int32) a socket id
func (slf *RPCSrvClient) SetSocket(sock int32) {
	slf._socket = sock
}

//Write doc
//@Summary Send data to the client
//@Method  Write
//@Param  ([]byte) a need send data
//@Param  (int) need send data length
func (slf *RPCSrvClient) Write(d []byte, len int) {
	sock := slf.GetSocket()
	if sock <= 0 {
		slf.LogError("Write Data error:socket[0]")
		return
	}

	if err := network.OperWrite(sock, d, len); err != nil {
		slf.LogError("Write Data error:%+v", err)
	}
}

//SetAuth doc
//@Summary Setting the time for authentication
//@Method SetAuth desc: Setting author time
//@Param (uint64) a author time
func (slf *RPCSrvClient) SetAuth(auth uint64) {

}

//GetAuth doc
//@Summary Returns the client author time
//@Method GetAuth
//@Return (uint64) the client author time
func (slf *RPCSrvClient) GetAuth() uint64 {
	return 0
}

//GetKeyPair doc
//@Summary Returns the client key pairs
//@Method GetKeyPair
//@Return (interface{}) the client key pairs
func (slf *RPCSrvClient) GetKeyPair() interface{} {
	return nil
}

//BuildKeyPair doc
//@Summary Building the client key pairs
//@Method BuildKeyPair
func (slf *RPCSrvClient) BuildKeyPair() {

}

//GetKeyPublic doc
//@Summary Returns the client public key
//@Method GetKeyPublic
//@Return (string) a public key
func (slf *RPCSrvClient) GetKeyPublic() string {
	return ""
}

//Shutdown doc
//@Summary Terminate this client
//@Method Shutdown
func (slf *RPCSrvClient) Shutdown() {
	slf.NetClientService.Shutdown()
	slf._parent = nil
}

func (slf *RPCSrvClient) onRequest(context actor.Context, sender *actor.PID, message interface{}) {
	request := message.(*requestEvent)
	method := reflect.ValueOf(request._method)
	params := make([]reflect.Value, 1)
	params[0] = reflect.ValueOf(request._param)

	rs := method.Call(params)
	if len(rs) > 0 {
		msgPb := rs[0].Interface().(proto.Message)
		data, err := proto.Marshal(msgPb)
		if err != nil {
			slf.LogError("RPC Response error:%d-%+v", request._ser, err)
			return
		}
		data = Encode(1, "", request._ser, RPCResponse, slf._parent._messageName(rs[0].Type()), data)
		slf.Write(data, len(data))
		return
	}
	return
}

func (slf *RPCSrvClient) onResponse(context actor.Context, sender *actor.PID, message interface{}) {

}
