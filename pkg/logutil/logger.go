package logutil

import (
	"context"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/google/uuid"
	"github.com/kouleen/common/pkg/ctxutil"
)

// CtxFatal calls the default logger's CtxFatal method and then os.Exit(1).
func CtxFatal(ctx context.Context, format string, v ...interface{}) {
	traceId := ctxutil.GetTraceId(ctx)

	// 如果上游没有traceId，可以在这里生成新traceId（可选）
	if traceId == "" {
		traceId = uuid.NewString()
		ctx = ctxutil.SetMeta(ctx, traceId, -1)
	}
	newFormat := "[%s]-" + format
	newV := append([]interface{}{traceId}, v...)
	logger.CtxFatalf(ctx, newFormat, newV...)
}

// CtxError calls the default logger's CtxError method.
func CtxError(ctx context.Context, format string, v ...interface{}) {
	traceId := ctxutil.GetTraceId(ctx)

	// 如果上游没有traceId，可以在这里生成新traceId（可选）
	if traceId == "" {
		traceId = uuid.NewString()
		ctx = ctxutil.SetMeta(ctx, traceId, -1)
	}
	newFormat := "[%s]-" + format
	newV := append([]interface{}{traceId}, v...)
	logger.CtxErrorf(ctx, newFormat, newV...)
}

// CtxWarn calls the default logger's CtxWarn method.
func CtxWarn(ctx context.Context, format string, v ...interface{}) {
	traceId := ctxutil.GetTraceId(ctx)

	// 如果上游没有traceId，可以在这里生成新traceId（可选）
	if traceId == "" {
		traceId = uuid.NewString()
		ctx = ctxutil.SetMeta(ctx, traceId, -1)
	}
	newFormat := "[%s]-" + format
	newV := append([]interface{}{traceId}, v...)
	logger.CtxWarnf(ctx, newFormat, newV...)
}

// CtxNotice calls the default logger's CtxNotice method.
func CtxNotice(ctx context.Context, format string, v ...interface{}) {
	traceId := ctxutil.GetTraceId(ctx)

	// 如果上游没有traceId，可以在这里生成新traceId（可选）
	if traceId == "" {
		traceId = uuid.NewString()
		ctx = ctxutil.SetMeta(ctx, traceId, -1)
	}
	newFormat := "[%s]-" + format
	newV := append([]interface{}{traceId}, v...)
	logger.CtxNoticef(ctx, newFormat, newV...)
}

// CtxInfo calls the default logger's CtxInfo method.
func CtxInfo(ctx context.Context, format string, v ...interface{}) {
	traceId := ctxutil.GetTraceId(ctx)

	// 如果上游没有traceId，可以在这里生成新traceId（可选）
	if traceId == "" {
		traceId = uuid.NewString()
		ctx = ctxutil.SetMeta(ctx, traceId, -1)
	}
	newFormat := "[%s]-" + format
	newV := append([]interface{}{traceId}, v...)
	logger.CtxInfof(ctx, newFormat, newV...)
}

// CtxDebug calls the default logger's CtxDebug method.
func CtxDebug(ctx context.Context, format string, v ...interface{}) {
	traceId := ctxutil.GetTraceId(ctx)

	// 如果上游没有traceId，可以在这里生成新traceId（可选）
	if traceId == "" {
		traceId = uuid.NewString()
		ctx = ctxutil.SetMeta(ctx, traceId, -1)
	}
	newFormat := "[%s]-" + format
	newV := append([]interface{}{traceId}, v...)
	logger.CtxDebugf(ctx, newFormat, newV...)
}

// CtxTrace calls the default logger's CtxTrace method.
func CtxTrace(ctx context.Context, format string, v ...interface{}) {
	traceId := ctxutil.GetTraceId(ctx)

	// 如果上游没有traceId，可以在这里生成新traceId（可选）
	if traceId == "" {
		traceId = uuid.NewString()
		ctx = ctxutil.SetMeta(ctx, traceId, -1)
	}
	newFormat := "[%s]-" + format
	newV := append([]interface{}{traceId}, v...)
	logger.CtxTracef(ctx, newFormat, newV...)
}
