package common

import "github.com/gogo/protobuf/proto"

//requestEvent doc
//@Summary RPC Request event
//@Member string Request method name
//@Member interface{} Request method object
//@Member uint32      Request serial
type RequestEvent struct {
	MethodName string
	Method     interface{}
	Param      proto.Message
	Ser        uint32
}

//ResponseEvent doc
//@Summary RPC Response event
//@Member string  Request Method Name
//@Member proto.Message  Request Return Data
//@Member uint32         Request serial
type ResponseEvent struct {
	MethodName string
	Return     proto.Message
	Ser        uint32
}
