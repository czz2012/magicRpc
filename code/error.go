package code

import "errors"

var (
	//ErrIncompleteData error
	ErrIncompleteData = errors.New("Incomplete data")
	//ErrMethodName  error
	ErrMethodName = errors.New("Protocol exception method name exceeded")
	//ErrDataOverflow error
	ErrDataOverflow = errors.New("Data overflow")
	//ErrMethodUndefined error
	ErrMethodUndefined = errors.New("RPC method undefined")
	//ErrMethodDefinedResponse error
	ErrMethodDefinedResponse = errors.New("RPC method defined Request need Response")
	//ErrParamUndefined error
	ErrParamUndefined = errors.New("RPC param undefined")
	//ErrConnectFull error
	ErrConnectFull = errors.New("Connection is full")
)
