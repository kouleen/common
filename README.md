# common

`common` 是一个给 Go 微服务复用的公共基础库，主要封装了：

- Kitex 服务端启动与注册
- Kitex 客户端创建
- RPC `traceId` / `userId` 透传
- MySQL 主从连接初始化
- Redis 单例客户端
- SQLite 单例客户端
- 固定业务 RPC 客户端封装

它不是独立应用，而是给业务服务直接 `import` 的基础依赖。

## 目录结构

```text
.
├── bootstrap/
│   ├── client.go
│   └── server.go
├── client/
│   └── client.go
├── middleware/
│   └── rpc.go
└── pkg/
    ├── ctxutil/
    │   └── ctx_meta.go
    ├── mysql/
    │   └── mysql_client.go
    ├── redis/
    │   └── redis_client.go
    └── sqlite/
        └── sqlite_client.go
```

## 依赖要求

- Go `1.25.0`
- CloudWeGo Kitex
- etcd
- MySQL
- Redis
- SQLite
- `github.com/kouleen/idl`

安装：

```bash
go get github.com/kouleen/common
```

## 使用前说明

这个仓库有几个比较重要的行为特点：

1. 多个包在 `init()` 阶段会直接读取环境变量，部分包会立即初始化外部连接。
2. 配置缺失或连接失败时，库内部会直接调用 `log.Fatal` / `log.Fatalf` 结束进程。
3. `client` 包虽然不再在 `init()` 中直接建连，但仍然依赖全局服务名，初始化顺序要注意。

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
| --- | --- | --- | --- |
| `REDIS_ADDR` | 否 | `redis:6379` | Redis 地址 |
| `REDIS_PASSWORD` | 否 | 空 | Redis 密码 |
| `REDIS_DB` | 是 | 无 | Redis DB 编号，必须是合法整数 |

### SQLite

| 变量名 | 是否必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `SQLITE_DATABASE` | 是 | 无 | SQLite 文件路径 |

## 推荐初始化顺序

建议按下面顺序接入：

1. 准备 `ETCD_ENDPOINTS`、数据库、缓存相关环境变量
2. 在程序启动早期调用 `bootstrap.SetCurrentServiceName(...)`
3. 启动服务端时使用 `bootstrap.Run(...)`
4. 业务里如果要使用预置 RPC 客户端，先设置服务名，再调用 `client.InitClientRpc()` 或 `client.GetUserRpc()`
5. 入口层写入 `traceId` / `userId`

一个典型例子：

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
}
```

## `bootstrap` 包

### 服务端启动

最常用入口：

```go
func Run(serviceName string, newServer func(...server.Option) server.Server, opts ...ServerOption)
```

它会：

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

如果你想自己控制 `NewServer` 和 `Run` 时机，也可以直接使用：

```go
opts := bootstrap.Options(
	"demo.rpc",
	bootstrap.WithServerMiddleware(middleware.RpcServerMiddleware),
)
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
- 写入 `ClientBasicInfo.ServiceName`
- 挂载客户端中间件
- 透传额外 `client.Option`

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

这个包封装了两个固定业务客户端：

- `GetUserRpc()`
- `GetSystemRpc()`

并提供显式初始化入口：

```go
func InitClientRpc() error
```

它们会在首次调用 `GetUserRpc()` / `GetSystemRpc()` 时自动初始化；如果你想更明确控制时机，推荐先设置服务名，再主动执行：

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

这个包依赖以下 IDL 产物：

- `github.com/kouleen/idl/kitex_gen/rpc`
- `github.com/kouleen/idl/kitex_gen/user/userservice`
- `github.com/kouleen/idl/kitex_gen/system/systemservice`

## `middleware` 包

### `RpcServerMiddleware`

服务端中间件职责：

- 从上下文读取 `traceId`
- 如果缺失则生成一个 UUID
- 打印请求日志
- 打印响应日志、耗时和错误

挂载方式：

```go
bootstrap.WithServerMiddleware(middleware.RpcServerMiddleware)
```

### `RpcClientMiddleware`

客户端中间件职责：

- 从上下文读取 `traceId` 和 `userId`
- 通过 `ctxutil.SetMeta(...)` 写入持久化元信息
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

提供方法：

```go
func GetTraceId(ctx context.Context) string
func GetUserId(ctx context.Context) int64
func SetMeta(ctx context.Context, traceId string, userId int64) context.Context
```

常见用法：

```go
ctx := context.Background()
ctx = ctxutil.SetMeta(ctx, "trace-123", 10001)
```

## `pkg/mysql` 包

这个包在 `init()` 中初始化数据库连接。

对外方法：

```go
func GetWriteMysqlDDB() *gorm.DB
func GetReadMysqlDDB() *gorm.DB
```

使用示例：

```go
package main

import "github.com/kouleen/common/pkg/mysql"

func main() {
	writeDB := mysql.GetWriteMysqlDDB()
	readDB := mysql.GetReadMysqlDDB()

	_ = writeDB
	_ = readDB
}
```

读库选择逻辑：

- 两个从库都存在时轮询
- 只有一个从库存在时返回该从库
- 都不存在时回退主库

连接池配置：

- `MaxIdleConns = 20`
- `MaxOpenConns = 100`
- `ConnMaxLifetime = 30s`
- `ConnMaxIdleTime = 30s`

## `pkg/redis` 包

这个包在 `init()` 中检查并初始化 Redis。

对外方法：

```go
func InitRedis(addr, password string, db int)
func Get(ctx context.Context, key string) (string, error)
func Set(ctx context.Context, key, value string, expiration time.Duration) error
func Del(ctx context.Context, key string) error
```

由于当前实现要求 `REDIS_DB` 必须存在，最简单的接入方式是直接在环境变量中配置好：

```bash
export REDIS_ADDR=127.0.0.1:6379
export REDIS_PASSWORD=
export REDIS_DB=0
```

## `pkg/sqlite` 包

这个包在 `init()` 中初始化 SQLite。

底层驱动使用：

```go
github.com/glebarez/sqlite
```

对外方法：

```go
func GetSqliteDb() *gorm.DB
```

使用示例：

```go
package main

import "github.com/kouleen/common/pkg/sqlite"

func main() {
	db := sqlite.GetSqliteDb()
	_ = db
}
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
