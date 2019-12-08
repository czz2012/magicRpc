package server

import "github.com/gogo/protobuf/proto"

type requestEvent struct {
	_methodName string
	_method     interface{}
	_param      proto.Message
	_ser        uint32
}

type responseEvent struct {
	_methodName string
	_return     proto.Message
	_ser        uint32
}
