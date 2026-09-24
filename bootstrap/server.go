package bootstrap

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/kouleen/common/message"
)

// ServerOption 启动器选项
type ServerOption func(*serverConfig)

type serverConfig struct {
	middlewares []endpoint.Middleware
	extraOpts   []server.Option
}

// WithServerMiddleware 挂载服务端中间件（可多次调用）
func WithServerMiddleware(mw ...endpoint.Middleware) ServerOption {
	return func(c *serverConfig) {
		c.middlewares = append(c.middlewares, mw...)
	}
}

// WithServerOption 透传额外的 kitex bootstrap.Option
func WithServerOption(opts ...server.Option) ServerOption {
	return func(c *serverConfig) {
		c.extraOpts = append(c.extraOpts, opts...)
	}
}

// Options 构建公共的 server.Option 列表。
// 适合想自己控制 NewServer 调用时机的场景。
func Options(serviceName string, opts ...ServerOption) []server.Option {
	c := &serverConfig{}
	for _, o := range opts {
		o(c)
	}

	if os.Getenv("ETCD_ENDPOINTS") == "" {
		if err := os.Setenv("ETCD_ENDPOINTS", "etcd:2379"); err != nil {
			log.Fatal(err)
		}
	}
	if os.Getenv("ADDRESS") == "" {
		log.Fatal("ADDRESS is empty")
	}

	r, err := etcd.NewEtcdRegistry([]string{os.Getenv("ETCD_ENDPOINTS")})
	if err != nil {
		log.Fatalf("etcd registry init failed: %v", err)
	}

	addr, err := net.ResolveTCPAddr("tcp", os.Getenv("ADDRESS"))
	if err != nil {
		log.Fatalf("resolve addr failed: %v", err)
	}

	serverOpts := []server.Option{
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: serviceName,
		}),
		server.WithMetaHandler(transmeta.ServerTTHeaderHandler),
		server.WithRegistry(r),
		server.WithServiceAddr(addr),
	}
	for _, mw := range c.middlewares {
		serverOpts = append(serverOpts, server.WithMiddleware(mw))
	}
	serverOpts = append(serverOpts, c.extraOpts...)

	return serverOpts
}

// Run 启动 Kitex RPC 服务，自动管理RabbitMQ生命周期
//
//	newServer: 闭包，接收公共选项，内部调用 IDL 生成的 NewServer。
func Run(serviceName string, newServer func(...server.Option) server.Server, opts ...ServerOption) {
	SetCurrentServiceName(serviceName)

	var needCloseMQ bool
	var consumeCancel context.CancelFunc

	// 全局信号ctx，统一优雅关闭
	rootCtx, rootCancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("receive shutdown signal, starting graceful exit")
		rootCancel()
	}()

	consumeEnable := strings.ToLower(os.Getenv("RABBITMQ_CONSUME_ENABLE")) == "true"

	// 有MQ地址 → 初始化连接，生产者可用
	if rabbitUrl := os.Getenv("RABBITMQ_URL"); rabbitUrl != "" {
		if err := message.InitRabbitMQ(rabbitUrl); err != nil {
			log.Fatalf("rabbitmq init failed: %v", err)
		}
		needCloseMQ = true
		log.Println("rabbitmq connection initialized, producer ready")

		// 消费开关开启，才启动消费协程
		if consumeEnable {
			consumeCtx, cancel := context.WithCancel(rootCtx)
			consumeCancel = cancel
			if err := message.StartConsume(consumeCtx); err != nil {
				log.Fatalf("rabbitmq start consume failed: %v", err)
			}
			log.Println("rabbitmq consumer started")
		}
	}

	// defer 统一清理
	defer func() {
		if consumeCancel != nil {
			consumeCancel()
		}
		if needCloseMQ {
			log.Println("shutdown: close rabbitmq connection")
			_ = message.Close()
		}
	}()

	svr := newServer(Options(serviceName, opts...)...)
	log.Printf("kitex service [%s] running", serviceName)
	if err := svr.Run(); err != nil {
		log.Fatalf("kitex server run error: %v", err)
	}
}
