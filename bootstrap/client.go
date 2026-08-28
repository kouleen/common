package bootstrap

import (
	"log"
	"os"
	"sync"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/transmeta"
	etcd "github.com/kitex-contrib/registry-etcd"
)

// currentServiceName 当前进程的服务名，由 Run() 启动时自动写入。
// 客户端用它填充 ClientBasicInfo，这样 tracing/metrics 里能看到调用方是谁。
var (
	currentServiceName string
	currentServiceOnce sync.Once
)

func SetCurrentServiceName(name string) {
	currentServiceOnce.Do(func() {
		currentServiceName = name
	})
}

// ClientOption 客户端选项
type ClientOption func(*clientConfig)

type clientConfig struct {
	middlewares []endpoint.Middleware
	extraOpts   []client.Option
}

// WithClientMiddleware 挂载客户端中间件
func WithClientMiddleware(mw ...endpoint.Middleware) ClientOption {
	return func(c *clientConfig) {
		c.middlewares = append(c.middlewares, mw...)
	}
}

// WithClientOption 透传额外的 kitex client.Option
func WithClientOption(opts ...client.Option) ClientOption {
	return func(c *clientConfig) {
		c.extraOpts = append(c.extraOpts, opts...)
	}
}

// NewClient 创建一个 Kitex RPC 客户端。
//
//	T:  IDL 生成的 Client 接口类型（自动推断）
//	destService: 目标服务名，如 rpc.SYSTEM_RPC_SERVER
//	newClient:   IDL 生成的构造函数，如 systemservice.NewClient
//
// 客户端泛型是安全的——T 只从 newClient 的返回值推断，没有冲突。
func NewClient[T any](destService string, newClient func(string, ...client.Option) (T, error), opts ...ClientOption) (T, error) {
	c := &clientConfig{}
	for _, o := range opts {
		o(c)
	}
	if os.Getenv("ETCD_ENDPOINTS") == "" {
		if err := os.Setenv("ETCD_ENDPOINTS", "etcd:2379"); err != nil {
			log.Fatal(err)
		}
	}
	r, err := etcd.NewEtcdResolver([]string{os.Getenv("ETCD_ENDPOINTS")})
	if err != nil {
		var zero T
		return zero, err
	}
	clientOpts := []client.Option{
		client.WithResolver(r),
		client.WithMetaHandler(transmeta.ClientTTHeaderHandler),
		client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: currentServiceName, // 调用方服务名，自动填充
		}),
	}
	for _, mw := range c.middlewares {
		clientOpts = append(clientOpts, client.WithMiddleware(mw))
	}
	clientOpts = append(clientOpts, c.extraOpts...)

	return newClient(destService, clientOpts...)
}
