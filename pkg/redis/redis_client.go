package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/kouleen/common/pkg/code"
	"github.com/redis/go-redis/v9"
)

type CodeProcess struct{}

func GetRedisClient() *redis.Client {
	return redisClient
}

func Get(ctx context.Context, key string) (string, error) {
	return redisClient.Get(ctx, key).Result()
}

func Set(ctx context.Context, key, value string, expiration time.Duration) error {
	return redisClient.Set(ctx, key, value, expiration).Err()
}

func Del(ctx context.Context, key string) error {
	return redisClient.Del(ctx, key).Err()
}

func Ttl(ctx context.Context, key string) (time.Duration, error) {
	return redisClient.TTL(ctx, key).Result()
}

func (p *CodeProcess) GenerateCode(ctx context.Context, rule code.Rule) string {
	prefix := rule.GetPrefix()
	localDateTime := time.Now()
	date := localDateTime.Format(rule.GetPattern())
	orderCode := prefix + date
	countStr := generateCode(ctx, prefix, rule.GetPattern())
	orderCode += buildOrderCode(rule.GetDigit(), countStr)
	return orderCode
}

func generateCode(ctx context.Context, keyPrefix, pattern string) string {
	date := time.Now().Format(pattern)

	key := fmt.Sprintf("CODE:%s:%s", keyPrefix, date)

	count, err := GetRedisClient().Incr(ctx, key).Result()
	if err != nil {
		return ""
	}
	if count == 1 {
		GetRedisClient().Expire(ctx, key, 24*time.Hour)
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
