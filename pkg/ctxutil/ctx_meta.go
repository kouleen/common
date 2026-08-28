package ctxutil

import (
	"context"
	"strconv"

	"github.com/bytedance/gopkg/cloud/metainfo"
)

// meta key常量，统一管理
const (
	TraceID = "x-trace-id"
	UserID  = "x-user-id"
)

// GetTraceId 从ctx获取链路traceId
func GetTraceId(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, ok := metainfo.GetPersistentValue(ctx, TraceID)
	if !ok {
		return ""
	}
	return v
}

// GetUserId 从ctx获取登录用户id
func GetUserId(ctx context.Context) int64 {
	if ctx == nil {
		return -1
	}
	v, ok := metainfo.GetPersistentValue(ctx, UserID)
	if !ok {
		return -1
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return -1
	}
	return i
}

// SetMeta 批量设置 traceId、userId，返回新ctx
// 等价 WithPersistentValues，对外统一入口；网关/客户端中间件使用
func SetMeta(ctx context.Context, traceId string, userId int64) context.Context {
	if ctx == nil {
		return ctx
	}
	return metainfo.WithPersistentValues(ctx,
		TraceID, traceId,
		UserID, strconv.FormatInt(userId, 10),
	)
}
