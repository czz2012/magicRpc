package test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/yamakiller/magicLibs/logger"
	"github.com/yamakiller/magicNet/core"
	"github.com/yamakiller/magicNet/core/boot"
	"github.com/yamakiller/magicNet/core/frame"

	"github.com/yamakiller/magicNet/engine/actor"
	"github.com/yamakiller/magicRpc/examples/helloworld"

	rpcclient "github.com/yamakiller/magicRpc/assembly/client"
	rpcserver "github.com/yamakiller/magicRpc/assembly/server"

	"net/http"
	_ "net/http/pprof"
)

type testBFunc struct {
}

func (slf *testBFunc) Atest(context actor.Context,
	request *helloworld.HelloRequest) *helloworld.HelloReply {

	return &helloworld.HelloReply{Name: "test"}
}

type testBEngine struct {
	core.DefaultBoot
	core.DefaultService
	core.DefaultWait

	_shutdown bool
	_srv      *rpcserver.RPCServer
	_cli      *rpcclient.RPCClientPool
	_wait     sync.WaitGroup
}

func (slf *testBEngine) InitService() error {
	logger.Info(0, "启动压力测试")

	srvAddr := "0.0.0.0:8888"
	rpcSrv, err := rpcserver.New(rpcserver.SetName("testRPC"))
	if err != nil {
		return fmt.Errorf("创建RPC服务错误：%+v", err)
	}

	if err = rpcSrv.Listen(srvAddr); err != nil {
		return fmt.Errorf("监听RPC服务错误: %+v", err)
	}

	slf._srv = rpcSrv
	slf._srv.RegRPC(&testBFunc{})
	logger.Info(0, "启动客户端连接池")
	rpcCli, err := rpcclient.New(
		rpcclient.SetAddr("127.0.0.1:8888"),
		rpcclient.SetActive(128),
		rpcclient.SetIdle(32),
		rpcclient.SetTimeout(30*1000),
		rpcclient.SetIdleTimeout(60*1000),
	)

	if err != nil {
		return fmt.Errorf("创建RPC客户端错误：%+v", err)
	}
	slf._cli = rpcCli
	logger.Info(0, "启动服务成功")

	//
	logger.Info(0, "开启客户端")
	clientNum := 40
	slf._shutdown = false
	rand.Seed(time.Now().Unix())
	slf._wait.Add(clientNum)
	for i := 0; i < clientNum; i++ {
		go slf.testCall(i + 1)
	}

	http.ListenAndServe("0.0.0.0:6060", nil)

	return nil
}

func (slf *testBEngine) CloseService() {
	logger.Info(0, "close service start")
	slf._shutdown = true
	slf._wait.Wait()
	if slf._cli != nil {
		slf._cli.Shutdown()
		slf._cli = nil
	}

	if slf._srv != nil {
		slf._srv.Shutdown()
		slf._srv = nil
	}
	logger.Info(0, "close service complate")
}

func (slf *testBEngine) testCall(n int) {
	defer slf._wait.Done()
	var err error

	
	name := fmt.Sprintf("request - %d", n)
	r := &helloworld.HelloReply{}
	for {
		if slf._shutdown {
			break
		}
		err = slf._cli.Call("testBFunc.Atest",
			&helloworld.HelloRequest{Name: name},
			r)
		if err != nil {
			fmt.Printf("调用函数testBFunc.Atest错误:%+v\n", err)
		}
		sleep:= rand.Intn(200)
		time.Sleep(time.Duration(sleep) * time.Millisecond)
	}
}

func BenchmarkRPCServer(b *testing.B) {
	boot.Launch(func() frame.Framework {
		return &testBEngine{}
	})
}
