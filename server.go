package common

import (
	"log"
	"net"
	"os"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
)

// Option 启动器选项
type Option func(*config)

type config struct {
	middlewares []endpoint.Middleware
	extraOpts   []server.Option
}

// WithMiddleware 挂载服务端中间件（可多次调用）
func WithMiddleware(mw ...endpoint.Middleware) Option {
	return func(c *config) {
		c.middlewares = append(c.middlewares, mw...)
	}
}

// WithServerOption 透传额外的 kitex bootstrap.Option
func WithServerOption(opts ...server.Option) Option {
	return func(c *config) {
		c.extraOpts = append(c.extraOpts, opts...)
	}
}

// Options 构建公共的 server.Option 列表。
// 适合想自己控制 NewServer 调用时机的场景。
func Options(serviceName string, opts ...Option) []server.Option {
	c := &config{}
	for _, o := range opts {
		o(c)
	}

	if os.Getenv("ETCD_ENDPOINTS") == "" {
		log.Fatal("ETCD_ENDPOINTS is empty")
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

// Run 启动一个 Kitex RPC 服务。
//
//	newServer: 闭包，接收公共选项，内部调用 IDL 生成的 NewServer。
//	           这样完全避开泛型推断，任何服务的 NewServer 都能适配。
func Run(serviceName string, newServer func(...server.Option) server.Server, opts ...Option) {
	SetCurrentServiceName(serviceName)
	svr := newServer(Options(serviceName, opts...)...)
	if err := svr.Run(); err != nil {
		log.Fatal(err)
	}
}
