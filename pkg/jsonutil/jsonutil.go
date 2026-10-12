package jsonutil

import (
	"context"
	"encoding/json"

	"github.com/bytedance/gopkg/util/logger"
)

func FromJson(ctx context.Context, obj any) []byte {
	marshal, err := json.Marshal(obj)
	if err != nil {
		logger.CtxErrorf(ctx, "json marshal err: %v", err)
		panic(err)
	}
	return marshal
}

func FormObj(ctx context.Context, jsonByte []byte, obj any) {
	if err := json.Unmarshal(jsonByte, obj); err != nil {
		logger.CtxErrorf(ctx, "json unmarshal err: %v", err)
		panic(err)
	}
}
