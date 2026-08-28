package client

import (
	"log"
	"sync"

	"github.com/kouleen/common/bootstrap"
	"github.com/kouleen/common/middleware"
	"github.com/kouleen/idl/kitex_gen/rpc"
	"github.com/kouleen/idl/kitex_gen/system/systemservice"
	"github.com/kouleen/idl/kitex_gen/user/userservice"
)

var (
	userRpc        userservice.Client
	systemRpc      systemservice.Client
	clientInitOnce sync.Once
	clientInitErr  error
)

func GetUserRpc() userservice.Client {
	if err := InitClientRpc(); err != nil {
		log.Fatal(err)
	}
	return userRpc
}

func GetSystemRpc() systemservice.Client {
	if err := InitClientRpc(); err != nil {
		log.Fatal(err)
	}
	return systemRpc
}

func InitClientRpc() error {
	clientInitOnce.Do(func() {
		userClient, err := bootstrap.NewClient(
			rpc.USER_RPC_SERVER,
			userservice.NewClient,
			bootstrap.WithClientMiddleware(middleware.RpcClientMiddleware),
		)
		if err != nil {
			clientInitErr = err
			return
		}
		userRpc = userClient
		systemClient, err := bootstrap.NewClient(
			rpc.SYSTEM_RPC_SERVER,
			systemservice.NewClient,
			bootstrap.WithClientMiddleware(middleware.RpcClientMiddleware),
		)
		if err != nil {
			clientInitErr = err
			return
		}
		systemRpc = systemClient
	})

	return clientInitErr
}
