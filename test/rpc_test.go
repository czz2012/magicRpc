package test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/yamakiller/magicNet/engine/actor"

	"github.com/yamakiller/magicLibs/logger"
	"github.com/yamakiller/magicLibs/util"
	"github.com/yamakiller/magicNet/core"
	"github.com/yamakiller/magicNet/core/boot"
	"github.com/yamakiller/magicNet/core/frame"
	"github.com/yamakiller/magicRpc/assembly/client"
	"github.com/yamakiller/magicRpc/assembly/server"
	"github.com/yamakiller/magicRpc/examples/helloworld"
)

type testEngine struct {
	core.DefaultBoot
	core.DefaultService
	core.DefaultWait

	_rpcServer *server.RPCServer
	_rpcClient *client.RPCClientPool
}

type testFunc struct {
}

func (slf *testFunc) A(context actor.Context, request *helloworld.HelloRequest) *helloworld.HelloReply {
	logger.Info(context.Self().ID, "Remote Call A Request:%s", request.Name)
	return &helloworld.HelloReply{Name: "test"}
}

func (slf *testEngine) InitService() error {
	addr := "0.0.0.0:8888"
	rpcSrv, err := server.New(server.SetName("testRpc"))
	if err != nil {
		return errors.New("创建RPC服务失败")
	}

	if err := rpcSrv.Listen(addr); err != nil {
		fmt.Println(err)
		return errors.New("监听RPC失败")
	}

	slf._rpcServer = rpcSrv
	slf._rpcServer.RegRPC(&testFunc{})

	logger.Info(0, "RPC监听%s成功", addr)

	logger.Info(0, "RPC开始创建Client")
	//启动客户端
	rpcCli, err := client.New(client.SetAddr("127.0.0.1:8888"))
	if err != nil && rpcCli != nil {
		return fmt.Errorf("创建RPC Client Fail%+v", err)
	}

	slf._rpcClient = rpcCli
	r := &helloworld.HelloReply{}
	err = rpcCli.Call("testFunc.A", &helloworld.HelloRequest{Name: "request - 1"}, r)
	if err != nil {
		logger.Error(0, "RPC调用失败:%+v", err)
		return nil
	}

	logger.Info(0, "1.RPC调用成功%+v,%p", r, r)

	err = rpcCli.Call("testFunc.A", &helloworld.HelloRequest{Name: "request - 1"}, r)

	logger.Info(0, "2.RPC调用成功%+v,%p", r, r)

	err = rpcCli.Call("testFunc.A", &helloworld.HelloRequest{Name: "request - 1"}, r)

	logger.Info(0, "3.RPC调用成功%+v,%p", r, r)
	logger.Info(0, "%s", util.SpawnUUID())

	return nil
}

func (slf *testEngine) CloseService() {
	logger.Info(0, "close service start")
	if slf._rpcClient != nil {
		slf._rpcClient.Shutdown()
		slf._rpcClient = nil
	}

	if slf._rpcServer != nil {
		slf._rpcServer.Shutdown()
		slf._rpcServer = nil
	}
	logger.Info(0, "close service complate")
}

func (slf *testEngine) testClosed() {
	d := time.Duration(time.Second * 2)

	t := time.NewTicker(d)
	go func() {
		defer t.Stop()
		for {
			<-t.C
			slf._rpcClient.Shutdown()
			slf._rpcClient = nil
			fmt.Println("timeout...")
			break
		}
	}()
}

func TestRPCServer(t *testing.T) {
	boot.Launch(func() frame.Framework {
		return &testEngine{}
	})
}
