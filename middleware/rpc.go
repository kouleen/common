package middleware

import (
	"context"
	"time"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/google/uuid"
	"github.com/kouleen/common/pkg/ctxutil"
)

const (
	HeaderTraceID = "x‑trace‑id"
	HeaderUserID  = "x‑user‑id"
)

// RpcServerMiddleware 【服务端中间件：被别人调用】
// 接收上游传来 x‑trace‑id、x‑user‑id；存入持久meta；打印请求响应耗时日志
func RpcServerMiddleware(next endpoint.Endpoint) endpoint.Endpoint {
	return func(ctx context.Context, req, resp interface{}) (err error) {
		// 1. 从RPC元数据读取上游透传过来的值
		traceId := ctxutil.GetTraceId(ctx)
		userId := ctxutil.GetUserId(ctx)

		// 如果上游没有traceId，可以在这里生成新traceId（可选）
		if traceId == "" {
			traceId = uuid.NewString()
		}
		ri := rpcinfo.GetRPCInfo(ctx)
		// 服务端：ri.From() = 调用方; ri.To() = 当前本服务
		svcName := ri.To().ServiceName()
		method := ri.To().Method()

		logger.CtxInfof(ctx, "[%s][uid:%d]-RpcServer ServiceName:[%s] Method:[%s] request: %#v",
			traceId, userId, svcName, method, req)

		startTime := time.Now()
		err = next(ctx, req, resp)
		costMs := float64(time.Since(startTime).Nanoseconds()) / 1e6

		logger.CtxInfof(ctx, "[%s][uid:%d]-RpcServer ServiceName:[%s] Method:[%s] cost:%.2fms err:%v response: %#v",
			traceId, userId, svcName, method, costMs, err, resp)

		return err
	}
}

// RpcClientMiddleware 【客户端中间件：调用别的服务】
// 从ctx读取 x‑trace‑id、x‑user‑id，封装进持久元数据，传递给远端服务端；打印调用耗时
func RpcClientMiddleware(next endpoint.Endpoint) endpoint.Endpoint {
	return func(ctx context.Context, req, resp interface{}) (err error) {
		// ==========【客户端：从本地ctx读取已有链路信息，打包发送给远端】==========
		traceId := ctxutil.GetTraceId(ctx)
		userId := ctxutil.GetUserId(ctx)

		// 关键：写入Persistent，kitex底层会把这两个key序列化放到rpc请求header传给对端服务
		ctx = ctxutil.SetMeta(ctx, traceId, userId)

		ri := rpcinfo.GetRPCInfo(ctx)
		svcName := ri.To().ServiceName()
		method := ri.To().Method()

		logger.CtxInfof(ctx, "[%s][uid:%d]-RpcClient Call Service:[%s] Method:[%s] request:%#v",
			traceId, userId, svcName, method, req)

		start := time.Now()
		err = next(ctx, req, resp)
		costMs := float64(time.Since(start).Nanoseconds()) / 1e6

		logger.CtxInfof(ctx, "[%s][uid:%d]-RpcClient Call Service:[%s] Method:[%s] cost:%.2fms err:%v response:%#v",
			traceId, userId, svcName, method, costMs, err, resp)

		return err
	}
}
