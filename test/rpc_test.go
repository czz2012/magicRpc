package test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/yamakiller/magicNet/engine/actor"

	"github.com/yamakiller/magicLibs/logger"
	"github.com/yamakiller/magicNet/core"
	"github.com/yamakiller/magicNet/core/boot"
	"github.com/yamakiller/magicNet/core/frame"
	"github.com/yamakiller/magicRpc/assembly/client"
	"github.com/yamakiller/magicRpc/assembly/server"
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

func (slf *testFunc) A(context actor.Context) {
	logger.Info(context.Self().ID, "Remote Call A")
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

	rpcCli.Call("testFunc.A", nil)

	logger.Info(0, "RPC调用成功")

	return nil
}

func (slf *testEngine) CloseService() {
	if slf._rpcClient != nil {
		slf._rpcClient.Shutdown()
		slf._rpcClient = nil
	}

	if slf._rpcServer != nil {
		slf._rpcServer.Shutdown()
		slf._rpcServer = nil
	}
}

func TestRPCServer(t *testing.T) {
	boot.Launch(func() frame.Framework {
		return &testEngine{}
	})
}
