package client

import (
	"log"

	"github.com/kouleen/common/bootstrap"
	"github.com/kouleen/common/middleware"
	"github.com/kouleen/idl/kitex_gen/rpc"
	"github.com/kouleen/idl/kitex_gen/system/systemservice"
	"github.com/kouleen/idl/kitex_gen/user/userservice"
)

var (
	userRpc   userservice.Client
	systemRpc systemservice.Client
)

func GetUserRpc() userservice.Client {
	return userRpc
}

func GetSystemRpc() systemservice.Client {
	return systemRpc
}

func init() {
	userClient, err := bootstrap.NewClient(rpc.USER_RPC_SERVER, userservice.NewClient, bootstrap.WithClientMiddleware(middleware.RpcClientMiddleware))
	if err != nil {
		log.Fatal(err)
	}
	userRpc = userClient
	systemClient, err := bootstrap.NewClient(rpc.SYSTEM_RPC_SERVER, systemservice.NewClient, bootstrap.WithClientMiddleware(middleware.RpcClientMiddleware))
	if err != nil {
		log.Fatal(err)
	}
	systemRpc = systemClient
}
