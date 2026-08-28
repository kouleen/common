package redis

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client

func init() {
	if os.Getenv("REDIS_ADDR") == "" {
		if err := os.Setenv("REDIS_ADDR", "redis:6379"); err != nil {
			log.Fatal(err)
		}
	}
	if v := os.Getenv("REDIS_DB"); v != "" {
		if iv, err := strconv.Atoi(v); err == nil {
			InitRedis(os.Getenv("REDIS_ADDR"), os.Getenv("REDIS_PASSWORD"), iv)
			return
		}
	}
	log.Fatalf("invalid redis db value")
}

// InitRedis 初始化Redis连接（全局单例）
func InitRedis(addr, password string, db int) {
	redisClient = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     10,
	})

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("connect redis failed: %v", err)
	}
	log.Println("redis initialized successfully")
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
