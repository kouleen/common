# common

`common` 是一个给 Go 微服务复用的公共基础库，当前主要包含：

- Kitex 服务端启动与服务发现
- Kitex 客户端创建与固定业务 RPC 客户端
- RPC `traceId` / `userId` 透传中间件
- RabbitMQ 发送与消费 SDK
- MySQL 主从连接
- Redis 全局客户端
- SQLite 全局客户端
- 本地内存缓存与编号生成能力

它不是独立应用，而是供业务项目直接 `import` 的基础依赖库。

## 目录结构

```text
.
├── bootstrap/
│   ├── client.go
│   └── server.go
├── client/
│   └── client.go
├── message/
│   └── rabbit.go
├── middleware/
│   └── rpc.go
└── pkg/
    ├── code/
    │   └── code_client.go
    ├── ctxutil/
    │   └── ctx_meta.go
    ├── mysql/
    │   ├── mysql.go
    │   └── mysql_client.go
    ├── redis/
    │   ├── redis.go
    │   └── redis_client.go
    ├── sqlite/
    │   ├── sqlite.go
    │   └── sqlite_client.go
    └── store/
        ├── store.go
        └── store_client.go
```

## 依赖要求

- Go `1.25.0`
- CloudWeGo Kitex
- etcd
- RabbitMQ
- MySQL
- Redis
- SQLite
- `github.com/kouleen/idl`

安装：

```bash
go get github.com/kouleen/common
```

## 使用前说明

这个仓库有几个很重要的行为特点：

1. 多个包在 `init()` 阶段会直接读取环境变量，部分包会立即初始化外部连接。
2. 配置缺失或连接失败时，部分包会直接 `log.Fatal` / `log.Fatalf` 终止进程。
3. `bootstrap`、`client`、`message` 都依赖初始化顺序，建议在宿主项目启动阶段统一接入。
4. `message` 包支持两种风格：
   - 简单发送：先初始化发送器，再直接按队列发送
   - SDK 接入：业务包 `init()` 注册消费者，宿主项目统一启动消费者

## 环境变量

### RPC / 服务注册

| 变量名 | 是否必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `ETCD_ENDPOINTS` | 否 | `etcd:2379` | Kitex 服务注册和发现使用的 etcd 地址 |
| `ADDRESS` | 服务端必填 | 无 | 当前服务监听地址，例如 `0.0.0.0:8888` |

### MySQL

主库：

| 变量名 | 是否必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `MYSQL_USERNAME` | 是 | 无 | 主库用户名 |
| `MYSQL_PASSWORD` | 是 | 无 | 主库密码 |
| `MYSQL_HOST` | 否 | `mysql` | 主库地址 |
| `MYSQL_PORT` | 否 | `3306` | 主库端口 |
| `MYSQL_DATABASE` | 是 | 无 | 主库库名 |

从库 1：

| 变量名 | 是否必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `MYSQL_SLAVE1_HOST` | 启用开关 | 无 | 非空时尝试初始化从库 1 |
| `MYSQL_SLAVE1_USERNAME` | 条件必填 | 无 | 从库 1 用户名 |
| `MYSQL_SLAVE1_PASSWORD` | 条件必填 | 无 | 从库 1 密码 |
| `MYSQL_SLAVE1_PORT` | 条件必填 | 无 | 从库 1 端口 |
| `MYSQL_SLAVE1_DATABASE` | 条件必填 | 无 | 从库 1 库名 |

从库 2：

| 变量名 | 是否必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `MYSQL_SLAVE2_HOST` | 启用开关 | 无 | 非空时尝试初始化从库 2 |
| `MYSQL_SLAVE2_USERNAME` | 条件必填 | 无 | 从库 2 用户名 |
| `MYSQL_SLAVE2_PASSWORD` | 条件必填 | 无 | 从库 2 密码 |
| `MYSQL_SLAVE2_PORT` | 条件必填 | 无 | 从库 2 端口 |
| `MYSQL_SLAVE2_DATABASE` | 条件必填 | 无 | 从库 2 库名 |

### Redis

| 变量名 | 是否必填 | 默认值 | 说明 |
| --- |-----|-----| --- |
| `REDIS_ADDR` | 否   | `redis:6379` | Redis 地址 |
| `REDIS_PASSWORD` | 否   | 空   | Redis 密码 |
| `REDIS_DB` | 否   | 0   | Redis DB 编号，必须是合法整数 |

说明：

- `pkg/redis` 在 `init()` 中会校验 `REDIS_DB`
- 如果 `REDIS_DB` 未设置，当前实现会直接报错退出

### SQLite

| 变量名 | 是否必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `SQLITE_DATABASE` | 是 | 无 | SQLite 文件路径 |

## 推荐接入顺序

建议宿主项目按下面顺序初始化：

1. 准备 etcd、MySQL、Redis、SQLite 相关环境变量
2. 尽早调用 `bootstrap.SetCurrentServiceName(...)`
3. 如果需要固定 RPC 客户端，调用 `client.InitClientRpc()`
4. 如果需要 MQ 发送，调用 `message.InitSender(...)`
5. 如果需要 MQ 消费，在业务包注册消费者，宿主启动时统一调用 `message.StartConsumers(...)`
6. 最后启动 Web / Kitex / HTTP 服务

## `bootstrap` 包

### 服务端启动

最常用入口：

```go
func Run(serviceName string, newServer func(...server.Option) server.Server, opts ...ServerOption)
```

它会自动：

- 设置当前服务名
- 生成公共 `server.Option`
- 启用 TTHeader 元信息处理
- 配置 etcd 注册
- 绑定监听地址
- 启动 Kitex 服务

示例：

```go
package main

import (
	"github.com/cloudwego/kitex/server"
	"github.com/kouleen/common/bootstrap"
	"github.com/kouleen/common/middleware"
	"github.com/your-org/your-idl/kitex_gen/demo/demoservice"
)

type DemoServiceImpl struct{}

func main() {
	bootstrap.Run(
		"demo.rpc",
		func(opts ...server.Option) server.Server {
			return demoservice.NewServer(new(DemoServiceImpl), opts...)
		},
		bootstrap.WithServerMiddleware(middleware.RpcServerMiddleware),
	)
}
```

### 客户端创建

统一入口：

```go
func NewClient[T any](
	destService string,
	newClient func(string, ...client.Option) (T, error),
	opts ...ClientOption,
) (T, error)
```

它会自动：

- 创建 etcd resolver
- 启用 `transmeta.ClientTTHeaderHandler`
- 使用 `transport.TTHeaderFramed`
- 填充调用方 `ServiceName`
- 挂载客户端中间件

示例：

```go
package main

import (
	"github.com/kouleen/common/bootstrap"
	"github.com/kouleen/common/middleware"
	"github.com/your-org/your-idl/kitex_gen/rpc"
	"github.com/your-org/your-idl/kitex_gen/user/userservice"
)

func main() {
	bootstrap.SetCurrentServiceName("order.rpc")

	userClient, err := bootstrap.NewClient(
		rpc.USER_RPC_SERVER,
		userservice.NewClient,
		bootstrap.WithClientMiddleware(middleware.RpcClientMiddleware),
	)
	if err != nil {
		panic(err)
	}

	_ = userClient
}
```

说明：

- 调用方服务名来自 `bootstrap.SetCurrentServiceName(...)`
- `SetCurrentServiceName(...)` 内部使用 `sync.Once`，第一次设置后不会再变

## `client` 包

这个包封装了两个固定业务 RPC 客户端：

- `GetUserRpc()`
- `GetSystemRpc()`

并提供显式初始化入口：

```go
func InitClientRpc() error
```

推荐在宿主项目启动时，先设置服务名，再初始化：

```go
package main

import (
	commonClient "github.com/kouleen/common/client"
	"github.com/kouleen/common/bootstrap"
)

func main() {
	bootstrap.SetCurrentServiceName("order.rpc")

	if err := commonClient.InitClientRpc(); err != nil {
		panic(err)
	}

	userRpc := commonClient.GetUserRpc()
	_ = userRpc
}
```

## `message` 包

`message` 是当前项目内置的 RabbitMQ SDK，支持发送和消费两部分能力。

### 1. 简单发送

如果业务只需要发送队列消息，推荐：

```go
package main

import (
	"context"

	"github.com/kouleen/common/message"
)

func main() {
	if err := message.InitSender("amqps://user:pass@host/vhost"); err != nil {
		panic(err)
	}
	defer message.CloseSenders()

	err := message.SendToQueue(context.Background(), "user-login-log", &message.Message{
		TraceID:   "trace-123",
		MessageID: "msg-1",
		Body:      []byte(`{"userId":1001}`),
	})
	if err != nil {
		panic(err)
	}
}
```

如果需要自定义 exchange / routing key，也可以使用：

```go
err := message.Send(context.Background(), message.SendOptions{
	URL:        "amqps://user:pass@host/vhost",
	Exchange:   "user.exchange",
	RoutingKey: "user.login",
}, &message.Message{
	Body: []byte(`{"userId":1001}`),
})
```

### 2. 消费接口

消费者只需要实现：

```go
type Consumer interface {
	Queue() string
	HandleMessage(ctx context.Context, msg *Message) error
}
```

如果还需要自动绑定 exchange / routing key，可以额外实现：

```go
type BindingConsumer interface {
	Consumer
	Exchange() string
	RoutingKey() string
}
```

### 3. SDK 风格推荐接法

如果这是给其他业务项目依赖的 SDK，推荐：

1. 业务包 `init()` 中注册消费者
2. 宿主项目统一启动消费者

业务包：

```go
package loginlog

import (
	"context"
	"log"

	"github.com/kouleen/common/message"
)

type LoginLogConsumer struct{}

func (LoginLogConsumer) Queue() string {
	return "user-login-log"
}

func (LoginLogConsumer) HandleMessage(ctx context.Context, msg *message.Message) error {
	log.Printf("receive message: %s", string(msg.Body))
	return nil
}

func init() {
	message.MustRegisterConsumer(LoginLogConsumer{})
}
```

宿主项目：

```go
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	_ "github.com/your-org/your-app/internal/loginlog"
	"github.com/kouleen/common/message"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := message.StartConsumers(ctx, message.ClientOptions{
		URL: "amqps://user:pass@host/vhost",
	}); err != nil {
		log.Fatal(err)
	}
	defer message.CloseConsumers()

	select {}
}
```

### 4. 全局消费者入口

当前 `message` 包提供：

- `RegisterConsumer(...)`
- `MustRegisterConsumer(...)`
- `InitConsumers(opts)`
- `StartConsumers(ctx, opts)`
- `CloseConsumers()`

说明：

- `InitConsumers(opts)` 等价于 `StartConsumers(nil, opts)`，适合无宿主生命周期上下文的场景
- Web / Kitex 项目更推荐 `StartConsumers(ctx, opts)`，把消费者退出和宿主生命周期绑定
- 不要在业务包 `init()` 里调用 `CloseConsumers()` 或取消上下文

## `middleware` 包

### `RpcServerMiddleware`

服务端中间件职责：

- 从上下文读取 `traceId`
- 如果缺失则自动生成一个 UUID，并写回持久元信息
- 打印请求、响应、耗时日志

挂载方式：

```go
bootstrap.WithServerMiddleware(middleware.RpcServerMiddleware)
```

### `RpcClientMiddleware`

客户端中间件职责：

- 从上下文读取 `traceId` 和 `userId`
- 调用 `ctxutil.SetMeta(...)` 写入持久化元信息
- 让 Kitex 通过 TTHeader 把元信息透传到下游
- 打印调用日志

挂载方式：

```go
bootstrap.WithClientMiddleware(middleware.RpcClientMiddleware)
```

## `pkg/ctxutil` 包

固定元信息 key：

- `x-trace-id`
- `x-user-id`

对外方法：

```go
func GetTraceId(ctx context.Context) string
func GetUserId(ctx context.Context) int64
func SetMeta(ctx context.Context, traceId string, userId int64) context.Context
```

使用示例：

```go
ctx := context.Background()
ctx = ctxutil.SetMeta(ctx, "trace-123", 10001)
```

## `pkg/mysql` 包

这个包在 `init()` 中初始化主库和可选从库连接。

对外方法：

```go
func GetWriteMysqlDDB() *gorm.DB
func GetReadMysqlDDB() *gorm.DB
```

读库选择逻辑：

- 两个从库都存在时轮询
- 只有一个从库存在时直接返回该从库
- 都不存在时回退主库

连接池配置：

- `MaxIdleConns = 20`
- `MaxOpenConns = 100`
- `ConnMaxLifetime = 30s`
- `ConnMaxIdleTime = 30s`

使用示例：

```go
writeDB := mysql.GetWriteMysqlDDB()
readDB := mysql.GetReadMysqlDDB()
```

## `pkg/redis` 包

这个包在 `init()` 中初始化 Redis。

对外方法：

```go
func InitRedis(addr, password string, db int)
func GetRedisClient() *redis.Client
func Get(ctx context.Context, key string) (string, error)
func Set(ctx context.Context, key, value string, expiration time.Duration) error
func Del(ctx context.Context, key string) error
func Ttl(ctx context.Context, key string) (time.Duration, error)
```

还提供基于 Redis 的编号生成实现：

```go
type CodeProcess struct{}
```

它实现了 `pkg/code.Process` 接口，可配合业务规则生成按日期递增的编号。

## `pkg/store` 包

`pkg/store` 提供一个本地内存版本的过期 KV 存储，适合不依赖 Redis 的场景。

对外方法：

```go
func GetCacheStore() *ExpireMap
func Get(ctx context.Context, key string) (string, error)
func Set(ctx context.Context, key, value string, expiration time.Duration) error
func Del(ctx context.Context, key string) error
func Ttl(ctx context.Context, key string) (time.Duration, error)
```

它同样提供一个本地编号生成实现：

```go
type CodeProcess struct{}
```

`ExpireMap` 还支持：

- `Incr`
- `IncrBy`
- `Expire`
- `GetNum`

## `pkg/code` 包

`pkg/code` 本身只定义接口：

```go
type Rule interface {
	GetPrefix() string
	GetPattern() string
	GetDigit() int
}

type Process interface {
	GenerateCode(ctx context.Context, rule Rule) string
}
```

你可以搭配：

- `redis.CodeProcess`
- `store.CodeProcess`

来生成业务编号。

## `pkg/sqlite` 包

这个包在 `init()` 中初始化 SQLite。

底层驱动使用：

```go
github.com/glebarez/sqlite
```

对外方法：

```go
func GetSqliteClient() *gorm.DB
```

实现特点：

- 自动创建数据库文件
- 使用 WAL 模式
- 最大连接数为 `1`
- 最大空闲连接数为 `1`

## 当前验证

当前仓库执行以下命令可以通过：

```bash
go test ./...
```
