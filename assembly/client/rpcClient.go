package client

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/yamakiller/magicNet/engine/actor"
	"github.com/yamakiller/magicNet/handler/implement/connector"
	"github.com/yamakiller/magicNet/handler/net"
	"github.com/yamakiller/magicNet/timer"
	"github.com/yamakiller/magicRpc/assembly/common"
	"github.com/yamakiller/magicRpc/code"
)

const (
	constClientIdle = 0
	constClientRun  = 1
	constClientDel  = 2
)

//RPCClient doc
type RPCClient struct {
	connector.NetConnector
	_parent       *RPCClientPool
	_auth         uint64
	_connTimeout  int64
	_timeOut      int64
	_idletime     int64
	_serial       uint32
	_response     chan *common.ResponseEvent
	_responseStop chan bool
	_responseWait uint32
	_closeWait    sync.WaitGroup
}

//Initial doc
//@Summary Initial RPC Client service initial
//@Method Initial
func (slf *RPCClient) Initial() {
	slf._response = make(chan *common.ResponseEvent)
	slf._responseStop = make(chan bool)
	slf.NetConnector.Initial()
	slf.RegisterMethod(&common.RequestEvent{}, slf.onRequest)
	slf.RegisterMethod(&common.ResponseEvent{}, slf.onResponse)
}

//Shutdown doc
//@Summary Close RPC Client
func (slf *RPCClient) Shutdown() {
	close(slf._responseStop)
	slf._closeWait.Wait()
	close(slf._response)
	slf.NetConnector.Shutdown()
	slf._parent = nil
	slf._serial = 0
	slf._responseWait = 0
	slf._auth = 0
}

//Connection doc
//@Summary Connection address
//@Method  Connection
//@Param   string address
//@Return  error
func (slf *RPCClient) Connection(addr string) error {
	startTime := time.Now().UnixNano()
	if err := slf.NetConnector.Connection(addr); err != nil {
		return err
	}

	ick := 0
	for {
		if slf._auth > 0 {
			return nil
		}

		if slf._connTimeout > 0 {
			currTime := time.Now().UnixNano() - startTime
			if (currTime / int64(time.Millisecond)) > slf._connTimeout {
				slf.Shutdown()
				return errors.New("RPC Connection time out")
			}
		}

		ick++
		if ick < 6 {
			continue
		}
		ick = 0
		time.Sleep(time.Millisecond * time.Duration(2))
	}
}

//Call doc
//@Summary Call remote function
//@Param   string  			method
//@Param   interface{}  	param
//@Return  error
func (slf *RPCClient) call(method string, param proto.Message) error {
	data, err := common.Call(method, param)
	if err != nil {
		return err
	}
	return slf.SendTo(data)

}

func (slf *RPCClient) callr(method string, param proto.Message) (proto.Message, error) {
	var data []byte
	var err error
	if slf._responseWait != 0 {
		return nil, errors.New("call waitting")
	}
	if param != nil {
		data, err = proto.Marshal(param)
		if err != nil {
			return nil, err
		}
	}
	slf._responseWait = slf.incSerial()
	data = common.Encode(common.ConstVersion, method, slf._responseWait, common.RPCRequest, proto.MessageName(param), data)
	if err = slf.SendTo(data); err != nil {
		slf._responseWait = 0
		return nil, err
	}

	slf._closeWait.Add(1)
	defer slf._closeWait.Done()
	for {
		select {
		case <-slf._responseStop:
			atomic.StoreUint32(&slf._responseWait, 0)
			return nil, code.ErrConnectClosed
		case result := <-slf._response:
			if atomic.CompareAndSwapUint32(&slf._responseWait, result.Ser, 0) {
				return result.Return, nil
			}

			continue
		case <-time.After(time.Duration(slf._timeOut) * time.Millisecond):
			atomic.StoreUint32(&slf._responseWait, 0)
			return nil, code.ErrTimeOut
		}
	}
}

func (slf *RPCClient) onRequest(context actor.Context, sender *actor.PID, message interface{}) {
	if err := common.RPCRequestProcess(context, slf.SendTo, message); err != nil {
		slf.LogError("%s", err)
		return
	}
}

func (slf *RPCClient) onResponse(context actor.Context, sender *actor.PID, message interface{}) {
	response := message.(*common.ResponseEvent)
	if slf._responseWait != response.Ser {
		slf.LogError("RPC Response error not request wait")
		return
	}

	slf._closeWait.Add(1)
	defer slf._closeWait.Done()

	select {
	case <-slf._responseStop:
		atomic.StoreUint32(&slf._responseWait, 0)
		slf.LogError("client closed")
		return
	case slf._response <- response:
	}
}

func (slf *RPCClient) rpcDecode(context actor.Context, params ...interface{}) error {
	c := params[0].(*connector.NetConnector)
	if slf._auth == 0 {
		tmpAuth := c.ReadBuffer(1)
		if tmpAuth[0] != common.ConstHandShakeCode {
			return errors.New("rpc connection unauthorized")
		}
		slf._auth = timer.Now()
	}

	data, err := common.RPCDecodeClient(slf._parent.getRPC, c)
	if err != nil {
		return err
	}

	actor.DefaultSchedulerContext.Send(context.Self(), data)

	return net.ErrAnalysisSuccess
}

func (slf *RPCClient) incSerial() uint32 {
	slf._serial = ((slf._serial + 1) & 0xFFFFFFF)
	return slf._serial
}
