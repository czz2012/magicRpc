package assembly

import "github.com/gogo/protobuf/proto"

type requestEvent struct {
	_method interface{}
	_param  proto.Message
	_ser    uint32
}

type responseEvent struct {
	_return proto.Message
	_ser    uint32
}
