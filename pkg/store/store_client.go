package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/kouleen/common/pkg/code"
)

type CodeProcess struct{}

func GetCacheStore() *ExpireMap {
	return cacheStore
}

func Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", errors.New("key is empty")
	}
	return GetCacheStore().get(ctx, key), nil
}

func Set(ctx context.Context, key, value string, expiration time.Duration) error {
	if key == "" {
		return errors.New("key is empty")
	}
	GetCacheStore().set(ctx, key, value, expiration)
	return nil
}

func Del(ctx context.Context, key string) error {
	GetCacheStore().del(ctx, key)
	return nil
}

func (p *CodeProcess) GenerateCode(ctx context.Context, rule code.Rule) string {
	prefix := rule.GetPrefix()
	date := time.Now().Format(rule.GetPattern())
	orderCode := prefix + date
	countStr := generateCode(rule.GetPrefix(), rule.GetPattern())
	orderCode += buildOrderCode(rule.GetDigit(), countStr)
	return orderCode
}

func generateCode(keyPrefix, pattern string) string {
	now := time.Now()
	date := now.Format(pattern)

	key := fmt.Sprintf("CODE:%s:%s", keyPrefix, date)

	count, err := GetCacheStore().Incr(key).Result()
	if err != nil {
		return ""
	}
	if count == 1 {
		GetCacheStore().Expire(key, 24*time.Hour)
	}
	return strconv.FormatInt(count, 10)
}

func buildOrderCode(digit int, countStr string) string {
	count := digit - len(countStr)
	if count < 0 {
		digit++
		return buildOrderCode(digit, countStr)
	}
	orderCodeBuild := ""
	for i := 0; i < count; i++ {
		orderCodeBuild += "0"
	}
	orderCodeBuild += countStr
	return orderCodeBuild
}
