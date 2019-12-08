package client

import (
	"github.com/gogo/protobuf/proto"
	"github.com/yamakiller/magicNet/handler/implement/connector"
)

type Options struct {
	TimeOut     int
	BufferLimit int
	OutChanSize int
}

//RPCClient doc
type RPCClient struct {
	connector.NetConnector

	_serial       uint32
	_response     chan proto.Message
	_responseStop chan bool
	_responseWait uint32
}
