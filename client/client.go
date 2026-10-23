package client

import (
	"context"
	"sync"

	"github.com/kouleen/common/bootstrap"
	"github.com/kouleen/common/middleware"
	"github.com/kouleen/common/pkg/logutil"
	"github.com/kouleen/idl/kitex_gen/room/roomservice"
	"github.com/kouleen/idl/kitex_gen/rpc"
	"github.com/kouleen/idl/kitex_gen/system/systemservice"
	"github.com/kouleen/idl/kitex_gen/user/userservice"
	"github.com/kouleen/idl/kitex_gen/workflow/workflowservice"
)

var (
	userRpc        userservice.Client
	systemRpc      systemservice.Client
	roomRpc        roomservice.Client
	workflowRpc    workflowservice.Client
	clientInitOnce sync.Once
	clientInitErr  error
)

func GetUserRpc(ctx context.Context) userservice.Client {
	if err := InitClientRpc(); err != nil {
		logutil.CtxError(ctx, "failed to initialize user rpc client")
		panic(err)
	}
	return userRpc
}

func GetSystemRpc() systemservice.Client {
	if err := InitClientRpc(); err != nil {
		logutil.CtxError(context.Background(), "failed to initialize system rpc client")
		panic(err)
	}
	return systemRpc
}

func GetRoomRpc() roomservice.Client {
	if err := InitClientRpc(); err != nil {
		logutil.CtxError(context.Background(), "failed to initialize room rpc client")
		panic(err)
	}
	return roomRpc
}

func GetWorkflowRpc() workflowservice.Client {
	if err := InitClientRpc(); err != nil {
		logutil.CtxError(context.Background(), "failed to initialize workflow rpc client")
		panic(err)
	}
	return workflowRpc
}

func InitClientRpc() error {
	clientInitOnce.Do(func() {
		if userClient, err := bootstrap.NewClient(
			rpc.USER_RPC_SERVER,
			userservice.NewClient,
			bootstrap.WithClientMiddleware(middleware.RpcClientMiddleware),
		); err != nil {
			clientInitErr = err
			return
		} else {
			userRpc = userClient
		}
		if systemClient, err := bootstrap.NewClient(
			rpc.SYSTEM_RPC_SERVER,
			systemservice.NewClient,
			bootstrap.WithClientMiddleware(middleware.RpcClientMiddleware),
		); err != nil {
			clientInitErr = err
			return
		} else {
			systemRpc = systemClient
		}
		if roomClient, err := bootstrap.NewClient(
			rpc.ROOM_RPC_SERVER,
			roomservice.NewClient,
			bootstrap.WithClientMiddleware(middleware.RpcClientMiddleware),
		); err != nil {
			clientInitErr = err
			return
		} else {
			roomRpc = roomClient
		}
		if workflowClient, err := bootstrap.NewClient(
			rpc.WORKFLOW_RPC_SERVER,
			workflowservice.NewClient,
			bootstrap.WithClientMiddleware(middleware.RpcClientMiddleware),
		); err != nil {
			clientInitErr = err
			return
		} else {
			workflowRpc = workflowClient
		}
	})

	return clientInitErr
}
