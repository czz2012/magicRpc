package protocol

import (
	"bytes"
	"errors"
)

const (
	//data head size default 8 bit
	constHeadSize = 64
	//data head size byte size
	constHeadByte = (constHeadSize >> 3)
	//version size
	constVersionSize = 8
	//version start pos bit
	constVersionStart = 0
	//version mask
	constVersionMask = 0xF
	//version shift
	constVersionShift = constHeadSize - constVersionSize
	//data length size 8 bit
	constDataLengthSize = 16
	//data length start pos bit
	constDataLengthStart = constVersionStart + constVersionSize
	//data length mask
	constDataLengthMask = 0xFF
	//data length shift
	constDataLengthShift = constVersionShift - constDataLengthSize
	//command name length size 8 bit
	constCmdNameLengthSize = 8
	//command name start pos bit
	constCmdNameLengthStart = constDataLengthStart + constDataLengthSize
	//command name mask
	constCmdNameLengthMask = 0xF
	//command name shift
	constCmdNameLengthShift = constDataLengthShift - constCmdNameLengthSize
	//data serial of number size 32 bit
	constSerialSize = 32
	//data serial of number start pos bit
	constSerialStart = constCmdNameLengthStart + constCmdNameLengthSize
	//data serial of number mask
	constSerialMask = 0xFFFF
	//data serial of number shift == 0
	constSerialShift = constCmdNameLengthShift - constSerialSize
)

var (
	//ErrIncompleteData error
	ErrIncompleteData = errors.New("Incomplete data")
)

//Header data header
type Header uint64

func getVersion(d uint64) int {
	return int((d >> constVersionShift) & constVersionMask)
}

func getDataLength(d uint64) int {
	return int((d >> constDataLengthShift) & constDataLengthMask)
}

func getCommandLength(d uint64) int {
	return int((d >> constCmdNameLengthShift) & constCmdNameLengthMask)
}

func getSerial(d uint64) uint32 {
	return uint32(d & constSerialMask)
}

//Protocol data format====================================================================================
//--------------------------------------------------------------------------------------------------------
//  8 Bit Version  | 16 Bit Data length | 8 Bit Command name length | 32 Bit Serial Number | data packet |
//--------------------------------------------------------------------------------------------------------
//========================================================================================================

//Encode doc
func Encode(data *bytes.Buffer) (version int, name string, err error) {
	tmpData := data.Bytes()
	if len(tmpData) < (constHeadSize >> 3) {
		return version, name, ErrIncompleteData
	}

	return version, name, err
}
