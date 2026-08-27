package middleware

import (
	"context"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/bytedance/gopkg/util/logger"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

// RpcClientMiddleware 自定义RPC拦截中间件
func RpcClientMiddleware(next endpoint.Endpoint) endpoint.Endpoint {
	return func(ctx context.Context, req, resp interface{}) (err error) {
		traceId, _ := metainfo.GetPersistentValue(ctx, "x-trace-id")
		// =========调用前（拦截前置逻辑）=========
		ri := rpcinfo.GetRPCInfo(ctx)
		logger.CtxInfof(ctx, "[%s]-Rpc client ServiceName:[%ss] Method:[%s]  request: %#v", traceId, ri.To().ServiceName(), ri.To().Method(), req)
		// 1. 打印日志、埋点
		// 2. 往ctx注入traceId、transmeta ttheader元数据
		// 3. 权限校验，不满足直接 return err，截断RPC调用，不走到next()
		startTime := time.Now()
		// 执行真正RPC调用
		err = next(ctx, req, resp)
		costMs := float64(time.Since(startTime).Nanoseconds()) / 1e6

		// =========调用返回后（后置逻辑）=========
		// 处理返回resp、err；统计耗时、错误码
		logger.CtxInfof(ctx, "[%s]-Rpc client ServiceName:[%s] Method:[%s] cost:%.2fms err:%v response: %#v",
			traceId, ri.To().ServiceName(), ri.To().Method(), costMs, err, resp)
		return err
	}
}
