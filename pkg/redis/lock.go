package redis

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

type Lock struct {
	client *redis.Client
	key    string        // 锁key
	val    string        // 唯一标识，用来判断是不是自己的锁
	ttl    time.Duration // 锁过期时间
}

// NewLock 新建锁
func NewLock(key string, ttl time.Duration) *Lock {
	// 随机串，防止释放别人的锁
	val := fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Int63())
	return &Lock{
		client: GetRedisClient(),
		key:    key,
		val:    val,
		ttl:    ttl,
	}
}

// TryLock 尝试获取锁，非阻塞，拿到返回true
func (l *Lock) TryLock(ctx context.Context) (bool, error) {
	// SET key value NX EX ttl
	ok, err := l.client.SetNX(ctx, l.key, l.val, l.ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// Lock 阻塞获取，循环重试（可选）
func (l *Lock) Lock(ctx context.Context, retryInterval time.Duration) error {
	for {
		ok, err := l.TryLock(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryInterval):
		}
	}
}

// Unlock 释放锁：Lua脚本，先判断value相等再删除，原子操作
const unlockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`

func (l *Lock) Unlock(ctx context.Context) (bool, error) {
	res, err := l.client.Eval(ctx, unlockScript, []string{l.key}, l.val).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func (l *Lock) StartWatchDog(ctx context.Context) (context.CancelFunc, <-chan struct{}, error) {

	// 同步前置检查：看看当前锁是不是还在
	existVal, err := l.client.Get(ctx, l.key).Result()
	if err != nil {
		return nil, nil, err
	}
	if existVal != l.val {
		return nil, nil, errors.New("lock already lost before start watchdog")
	}
	watchCtx, cancel := context.WithCancel(ctx)
	lostCh := make(chan struct{}, 1)

	go func() {
		defer close(lostCh)
		ticker := time.NewTicker(l.ttl / 3)
		defer ticker.Stop()
		renewScript := `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
		    return redis.call("EXPIRE", KEYS[1], ARGV[2])
		else
		    return 0
		end
		`
		sec := int64(l.ttl.Seconds())
		for {
			select {
			case <-watchCtx.Done():
				return
			case <-ticker.C:
				r, err := l.client.Eval(watchCtx, renewScript, []string{l.key}, l.val, sec).Int64()
				if err != nil || r != 1 {
					lostCh <- struct{}{}
					return
				}
			}
		}
	}()
	return cancel, lostCh, nil
}
