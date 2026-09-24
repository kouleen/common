package message

import (
	"context"
	"errors"
	"log"
	"sync"
)

// Message 消息结构
type Message struct {
	TraceID string                 `json:"traceId"`
	Body    []byte                 `json:"body"`
	Headers map[string]interface{} `json:"headers"`
}

// ConsumerHandler 消费回调
type ConsumerHandler func(ctx context.Context, msg *Message) error

var (
	producer    *rabbitProducer
	consumer    *rabbitConsumer
	initOnce    sync.Once
	initialized bool
)

// InitRabbitMQ 初始化rabbit，main调用一次，全局生效
func InitRabbitMQ(url string) error {
	var err error
	initOnce.Do(func() {
		p, e := newRabbitProducer(url)
		if e != nil {
			err = e
			return
		}
		c := newRabbitConsumer(url)
		producer = p
		consumer = c
		initialized = true
		log.Println("mq init success")
	})
	return err
}

// Publish 【直接调用】mq.Publish，不用传实例
func Publish(ctx context.Context, exchange, routingKey string, msg *Message) error {
	if !initialized {
		return ErrNotInit
	}
	return producer.publish(ctx, exchange, routingKey, msg)
}

// RegisterConsumer 注册消费队列和回调，直接调用
func RegisterConsumer(queue string, handler ConsumerHandler) error {
	if !initialized {
		return ErrNotInit
	}
	return consumer.register(queue, handler)
}

// StartConsume 启动所有注册好的消费者后台协程
func StartConsume(ctx context.Context) error {
	if !initialized {
		return ErrNotInit
	}
	return consumer.start(ctx)
}

// Close 关闭mq资源
func Close() error {
	if !initialized {
		return nil
	}
	_ = producer.close()
	return consumer.close()
}

var ErrNotInit = errors.New("mq not initialized, call mq.Init first")
