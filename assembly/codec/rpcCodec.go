package codec

import (
	"encoding/binary"
	"unicode/utf8"

	"github.com/yamakiller/magicNet/handler/net"
	"github.com/yamakiller/magicRpc/code"
)

const (
	//data head size default 8 bit
	constHeadSize = 64
	//data head size byte size
	constHeadByte = (constHeadSize >> 3)
	//version size
	constVersionSize = 7
	//version start pos bit
	constVersionStart = 0
	//version mask
	constVersionMask = 0x7F
	//version shift
	constVersionShift = constHeadSize - constVersionSize
	//oper size
	constOperSize = 1
	//oper start pos bit
	constOperStart = constVersionSize
	//oper mask
	constOperMask = 0x1
	//oper shift
	constOperShift = constVersionShift - constOperSize
	//data length size 8 bit
	constDataLengthSize = 16
	//data length start pos bit
	constDataLengthStart = constOperStart + constOperStart
	//data length mask
	constDataLengthMask = 0xFFFF
	//data length shift
	constDataLengthShift = constOperShift - constDataLengthSize
	//rpc method name length size 8 bit
	constMethodNameLengthSize = 6
	//rpc method name start pos bit
	constMethodNameLengthStart = constDataLengthStart + constDataLengthSize
	//rpc method name mask
	constMethodNameLengthMask = 0x3F
	//rpc method name shift
	constMethodNameLengthShift = constDataLengthShift - constMethodNameLengthSize
	//
	constDataNameLengthSize = 6
	//
	constDataNameLengthStart = constMethodNameLengthStart + constDataNameLengthSize
	//
	constDataNameLengthMask = 0x3F
	//
	constDataNameLengthShift = constMethodNameLengthShift - constDataNameLengthSize
	//data serial of number size 32 bit
	constSerialSize = 28
	//data serial of number start pos bit
	constSerialStart = constDataNameLengthStart + constDataNameLengthSize
	//data serial of number mask
	constSerialMask = 0xFFFFFFF
	//data serial of number shift == 0
	constSerialShift = constDataNameLengthShift - constSerialSize
	//rpc name length limit
	constNameLimit = 65
)

const (
	//rpc handshake code
	ConstHandShakeCode = 0xBF
)

//Header data header
type Header uint64

//RPCOper rpc oper mode 1.request 0.response
type RPCOper int

const (
	//RPCRequest request
	RPCRequest RPCOper = 1
	//RPCResponse response
	RPCResponse RPCOper = 0
)

//Block doc
//@Summary data block
//@Struct Block
//@Member int 		version
//@Member string    method name
//@Member int       param  data length
//@Member uint32    call serial of number
type Block struct {
	Ver    int
	Oper   RPCOper
	Method string
	Ser    uint32
	DName  string
	Data   []byte
}

func getVersion(d uint64) int {
	return int((d >> constVersionShift) & constVersionMask)
}

func getOper(d uint64) int {
	return int((d >> constOperShift) & constOperMask)
}

func getDataLength(d uint64) int {
	return int((d >> constDataLengthShift) & constDataLengthMask)
}

func getMethodLength(d uint64) int {
	return int((d >> constMethodNameLengthShift) & constMethodNameLengthMask)
}

func getDataNameLength(d uint64) int {
	return int((d >> constDataNameLengthShift) & constDataNameLengthMask)
}

func getSerial(d uint64) uint32 {
	return uint32(d & constSerialMask)
}

//Protocol data format===================================================================================================================================
//-------------------------------------------------------------------------------------------------------------------------------------------------------
//  7 Bit Version  | 1 Bit Operation mode| 16 Bit Data length | 6 Bit Method name length | 6 Bit data name length | 28 Bit Serial Number | data packet |
//------------------------------------------------------------------------------------------------------------------------------------------------------
//=======================================================================================================================================================

//Decode doc
//@Summary rpc network data decode
//@Method Decode
//@Param  *bytes.Buffer   recvice data buffer
//@Return *Block network data block
//@Return error
func Decode(data net.INetReceiveBuffer) (*Block, error) {
	if data.GetBufferLen() < constHeadByte {
		return nil, code.ErrIncompleteData
	}

	tmpHeader := binary.BigEndian.Uint64(data.GetBufferBytes()[:constHeadByte])
	tmpDataLength := getDataLength(tmpHeader)
	tmpMethodNameLength := getMethodLength(tmpHeader)
	tmpDataNameLength := getDataNameLength(tmpHeader)

	if data.GetBufferLen() < (constHeadByte + tmpDataLength + tmpMethodNameLength + tmpDataNameLength) {
		return nil, code.ErrIncompleteData
	}

	if tmpMethodNameLength > constNameLimit || tmpDataNameLength > constNameLimit {
		return nil, code.ErrMethodName
	}

	if (constHeadByte + tmpDataLength + tmpMethodNameLength + tmpDataNameLength) > (data.GetBufferCap() << 1) {
		return nil, code.ErrDataOverflow
	}

	data.TrunBuffer(constHeadByte)

	result := &Block{Ver: getVersion(tmpHeader),
		Oper:   RPCOper(getOper(tmpHeader)),
		Method: string(data.ReadBuffer(tmpMethodNameLength)),
		DName:  string(data.ReadBuffer(tmpDataNameLength)),
		Ser:    getSerial(tmpHeader),
		Data:   data.ReadBuffer(tmpDataLength)}

	return result, nil
}

//Encode doc
//@Summary rpc network data encode
//@Method Encode
//@Param  int 		version
//@Param  string 	method name
//@Param  uint32    serial
//@Param  int       oper  1.request 0.response
//@Param  string	param name/return name
//@Param  []byte    params/return
//@Return []byte
func Encode(ver int, methodName string, ser uint32, oper RPCOper, dataName string, data []byte) []byte {
	tmpMethodNameLength := utf8.RuneCountInString(methodName)
	tmpDataNameLength := utf8.RuneCountInString(dataName)
	tmpDataLength := 0
	if data != nil {
		tmpDataLength = len(data)
	}
	tmpData := make([]byte, constHeadByte)
	tmpHeader := ((uint64(ver) & constVersionMask) << constVersionShift) |
		((uint64(oper) & constOperMask) << constOperShift) |
		((uint64(tmpDataLength) & constDataLengthMask) << constDataLengthShift) |
		((uint64(tmpMethodNameLength) & constMethodNameLengthMask) << constMethodNameLengthShift) |
		((uint64(tmpDataNameLength) & constDataNameLengthMask) << constDataNameLengthShift) |
		(uint64(ser) & constSerialMask)

	binary.BigEndian.PutUint64(tmpData, tmpHeader)

	tmpData = append(tmpData, []byte(methodName)...)
	if data != nil {
		tmpData = append(tmpData, data...)
	}

	return tmpData
}
