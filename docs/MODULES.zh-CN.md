# gin-scaffold 当前骨架模块梳理

> 基线状态：当前工作区（在 b2c375a 基础上增加日志时区、大小分片、保留和压缩配置）
>
> 整理目标：按模块说明当前代码的职责、依赖边界、运行链路和审查重点。本文描述的是当前已实现状态，不把规划中的能力写成既有能力。

## 0. 快速结论

- 项目是一个可独立运行、默认不依赖 MySQL/Redis 的 Gin 服务骨架。
- 分层主线清晰：`handler -> business -> dao`，`middleware` 负责横切能力，`svc` 负责依赖装配，`infra` 放通用基础设施。
- 启动流程包含配置加载、配置校验、日志初始化、依赖装配、路由构建、HTTP 服务和优雅关闭。
- 已有能力：JWT 登录与鉴权、统一响应、错误码、访问日志、请求 ID、请求体限制、登录限流、panic 恢复、四模块日志、按天切割、日志压缩和保留清理。
- 当前仍是骨架而非完整生产实现：MySQL/Redis 只有配置，内存 DAO 是默认实现，prod 下没有真实用户数据源，限流和 JWT 都是单机/无状态实现。
- 日志已按 `app`、`business`、`gorm`、`access` 拆分，并通过 `request_id` 串联；当前没有真实 GORM 连接，因此 `gorm.log` 只会创建文件，接入数据库后才会产生 SQL 日志。

## 1. 模块地图

```text
.
├── main.go                    进程入口、HTTP Server 生命周期
├── etc/                       分环境 YAML 配置
├── internal/
│   ├── config/                配置结构、默认值、加载、校验
│   ├── svc/                   依赖注入容器 ServiceContext
│   ├── router/                路由注册和全局中间件装配
│   ├── middleware/            请求 ID、限流、鉴权、访问日志、恢复等
│   ├── handler/               HTTP 适配层
│   ├── business/              业务编排和业务错误
│   ├── dao/                   数据访问接口和内存实现
│   ├── model/                 存储侧模型
│   └── schema/                HTTP 请求/响应 DTO
├── infra/
│   ├── logger/                分模块 zap 日志、按天切割、GORM 适配
│   ├── requestid/             request_id 的 context 载体
│   ├── response/              统一响应包、AppError、业务码
│   ├── token/                 JWT 签发和校验
│   └── pagination/            分页归一化
└── docs/                      架构分析和本模块梳理
```

| 模块 | 主要文件 | 核心职责 |
|---|---|---|
| 进程入口 | `main.go` | 解析环境/配置、初始化日志、装配依赖、启动和关闭 HTTP Server |
| 配置 | `internal/config/config.go` | 配置结构、默认值、Viper 加载、启动前校验 |
| 依赖装配 | `internal/svc/service_context.go` | 构造 token manager、DAO 等共享依赖 |
| 路由 | `internal/router/*.go` | Gin 引擎、中间件顺序、公开/受保护路由、404/405 |
| 中间件 | `internal/middleware/*.go` | 请求 ID、请求体限制、访问日志、panic 恢复、限流、JWT 鉴权 |
| 处理器 | `internal/handler/*.go` | 绑定 HTTP 输入、调用业务、输出统一响应 |
| 业务 | `internal/business/*.go` | 登录编排、凭证校验、错误语义、业务日志 |
| DAO | `internal/dao/**` | 定义持久化接口，提供 dev/test 可运行的内存实现 |
| 领域/存储模型 | `internal/model/user.go` | 存储侧用户行结构 |
| API DTO | `internal/schema/auth.go` | HTTP 请求和响应结构 |
| 日志 | `infra/logger/*.go` | 模块 logger、按日文件、request_id 注入、GORM 适配 |
| 请求 ID | `infra/requestid/requestid.go` | 在 `context.Context` 中保存和读取请求 ID |
| 统一响应 | `infra/response/response.go` | 响应 envelope、AppError、业务码、错误日志 |
| JWT | `infra/token/token.go` | HS256 token 签发、解析、Bearer 提取 |
| 分页 | `infra/pagination/pagination.go` | page/size 和 id cursor 边界归一化 |

## 2. 依赖方向与分层规则

```text
main
 ├── config
 ├── logger
 ├── svc ────────┬── dao/memory
 │               └── token
 └── router
      ├── middleware ──── response / token / logger / requestid
      └── handler ─────── business ──── dao / schema / svc
                            └────────── logger
```

| 层 | 应该负责 | 不应该负责 |
|---|---|---|
| `handler` | 解析 HTTP、调用 business、写统一响应 | 业务分支、SQL、复杂校验策略 |
| `business` | 业务流程、调用 DAO、组装响应、返回类型化错误 | 直接操作 `gin.Context`、拼接 HTTP 响应 |
| `dao` | 持久化接口和具体查询 | HTTP 解析、业务策略 |
| `middleware` | 跨路由 HTTP 横切逻辑 | 具体业务规则 |
| `svc` | 构造和共享依赖 | 业务流程 |
| `infra` | 可复用的通用基础设施 | 反向依赖 `internal/*` |

当前 `infra/*` 没有导入 `internal/*`，方向保持正确。业务层通过 `svc.ServiceContext` 获取 DAO 和 token manager，这是当前骨架的核心装配方式。

## 3. 进程入口与生命周期

文件：`main.go`

### 启动顺序

1. 解析 `-env`、`-c`、`-f` 参数，并允许 `ENV` 环境变量提供默认环境。
2. 决定配置文件：显式 `-f` 优先，否则使用 `etc/config_<env>.yaml`。
3. `config.Load`：默认值 < YAML 文件 < 环境变量。
4. `logger.Init`：按 `log.rootDir` 初始化 `app/business/gorm/access` 四个 logger。
5. `cfg.Validate`：拒绝端口、超时、JWT、日志等非法配置。
6. `svc.NewServiceContext`：构造 token manager 和 UserDAO。
7. `router.New`：设置 Gin 模式、可信代理、全局中间件和路由。
8. 构造 `http.Server`，设置读、写、Header、空闲和优雅关闭超时。
9. 监听 `SIGINT/SIGTERM`，在 goroutine 中启动 `ListenAndServe`。
10. 收到信号或监听错误后，使用 `Shutdown` 做优雅关闭。
11. 退出前 `logger.Close()`，刷新并关闭日志文件。

### 审查关注

- `config loaded` 日志发生在 `cfg.Validate` 之前；配置非法时仍会先记录一条“已加载”，最终错误由入口日志体现。
- `MaxHeaderBytes` 当前固定为 1 MiB，没有对应配置项。
- 服务没有独立 readiness 检查；`/healthz` 是轻量 liveness 探针，不检查数据库、Redis 等外部依赖。
- 关闭流程依赖 `ShutdownTimeoutSeconds`，目前没有额外的后台任务等待逻辑。

## 4. 配置模块

文件：`internal/config/config.go`、`etc/config.example.yaml`、`etc/config_dev.yaml`、`etc/config_test.yaml`、`etc/config_prod.yaml`

### 配置来源与优先级

```text
代码默认值 < etc/config_<env>.yaml < 环境变量
```

环境变量没有前缀，`.` 替换为 `_`，例如：

- `SERVER_PORT`
- `AUTH_JWTSECRET`
- `LOG_ROOTDIR`
- `MYSQL_PASSWORD`
- `REDIS_PASSWORD`

环境/文件选择顺序：`-f` > `-env` > `ENV` > `dev`。

### 配置分组

| 分组 | 关键字段 | 用途 |
|---|---|---|
| `server` | host、port、mode、maxBodyBytes、trustedProxies、各类超时 | HTTP Server 和 Gin 行为 |
| `auth` | jwtSecret、accessExpireSeconds、loginRateLimitPerMinute | JWT 和登录限流 |
| `log` | rootDir、level、format、timezone、maxSizeMB、maxAgeDays、compress | 日志根目录、级别、编码、切日时区、大小分片、保留和压缩 |
| `mysql` | 连接、库名、连接池、超时 | 仅配置和 DSN 生成，尚未建立连接 |
| `redis` | 地址、密码、DB、连接池、超时 | 仅配置和 Addr 生成，尚未建立客户端 |

### 启动校验

`Config.Validate` 会检查：

- 端口范围合法。
- Gin mode 只能是 `debug`、`release`、`test`，prod 必须是 `release`。
- 请求体上限和各类 Server 超时必须为正数。
- JWT secret 必填；prod 长度至少 32 字节。
- access TTL 为正数，登录限流不能为负数。
- 日志级别只能是 `debug/info/warn/error`，格式只能是 `console/json`；rootDir 和 timezone 不能为空，timezone 必须可加载；maxSizeMB 和 maxAgeDays 不能为负数。

### 审查关注

- dev/test 配置包含演示 secret；只能用于本地或测试，不能复用到真实环境。
- MySQL/Redis 配置尚未被 `svc` 消费，当前没有连接池、健康检查或关闭逻辑。
- `MySQLConf.DSN` 固定 `parseTime=true&loc=Local`，没有 TLS、连接超时等参数；接入生产数据库时需要确认是否需要扩展。
- `loginRateLimitPerMinute` 允许配置为 `0`，但限流中间件内部把 `max <= 0` 回退为 10，因此“0 表示关闭限流”的直觉并不成立。若需要关闭限流，应显式调整实现或约定。
- `trustedProxies` 默认为空，即不信任 `X-Forwarded-For`。只有在明确知道入口代理 CIDR 时才应配置。

## 5. 依赖装配模块

文件：`internal/svc/service_context.go`

`ServiceContext` 当前包含：

```go
type ServiceContext struct {
    Config       *config.Config
    TokenManager *token.Manager
    UserDAO      dao.UserDAO
}
```

### 构造行为

- 拒绝空配置。
- 要求 `auth.jwtSecret` 非空，属于入口校验前的防御性检查。
- 按环境选择 UserDAO：
  - prod：空的 `memory.NewUserDAO()`，不注入演示账号，并记录 warning。
  - dev/test：`memory.NewDemoUserDAO()`，种子账号为 `admin/admin123`。
- 用配置构造 `token.Manager`。

### 审查关注

- 这里是接入 MySQL、Redis、gorm、外部客户端的最佳位置；当前尚未实现资源生命周期管理。
- prod 使用空内存 DAO 是显式的“未完成”状态，不会误放行演示账号，但登录在真实 DAO 接入前必然失败。
- 如果后续依赖需要关闭，建议为 ServiceContext 增加 `Close()`，并在 `main.go` 关闭流程中调用。

## 6. 路由模块

文件：`internal/router/router.go`、`internal/router/routes.go`

### 全局中间件顺序

当前注册顺序：

```text
RequestID -> BodyLimit -> AccessLog -> Recovery -> 路由中间件 -> handler
```

顺序含义：

- `RequestID` 最先执行，后续日志和响应都能拿到请求 ID。
- `BodyLimit` 在读取请求体前替换 `Request.Body`。
- `AccessLog` 包住后续链路，请求结束后记录最终状态和耗时。
- `Recovery` 位于访问日志内部，panic 被转换成 500 后，访问日志仍能记录 500。

### 路由表

| 方法 | 路径 | 中间件 | 说明 |
|---|---|---|---|
| `GET` | `/healthz` | 全局中间件 | 极简存活探针，刻意不走统一 envelope |
| `POST` | `/api/v1/login` | 全局 + 登录限流 | 用户名密码登录并签发 JWT |
| `GET` | `/api/v1/me` | 全局 + JWT 鉴权 | 返回 token 中的当前用户身份 |

### 异常路由

- `NoRoute` 返回统一 `404` envelope，业务码 `40400`。
- `HandleMethodNotAllowed = true`，`NoMethod` 返回统一 `405` envelope，业务码 `40500`。
- 可信代理配置非法时会记录 app error，并回退为不信任代理。

### 审查关注

- `AccessLog` 必须在 `Recovery` 外层；如果顺序反过来，panic 恢复后的访问日志可能缺失最终状态。
- 登录限流是路由级实例，生命周期与当前路由构建一致；多实例部署时各实例独立计数。
- 新增受保护路由时应放入 `protected` 分组，避免遗漏 `middleware.Auth`。

## 7. 中间件模块

目录：`internal/middleware/`

| 中间件 | 文件 | 行为 | 失败输出 |
|---|---|---|---|
| `RequestID` | `requestid.go` | 读取 `X-Request-Id`，没有则生成 16 字节随机 hex；写入 Gin context、request context 和响应头 | 随机源失败时使用 `unknown` |
| `BodyLimit` | `bodylimit.go` | 用 `http.MaxBytesReader` 限制请求体 | 由 handler 绑定阶段映射为 413 |
| `AccessLog` | `accesslog.go` | 请求结束后记录 method/path/status/latency/client_ip/user_id | 写 `access` 模块日志 |
| `Recovery` | `recovery.go` | recover panic，记录 app error 并返回统一 500 | `50000` |
| `RateLimit` | `ratelimit.go` | 按 ClientIP 做固定窗口计数，超限返回 `Retry-After` | `42900` |
| `Auth` | `auth.go` | 校验 `Authorization: Bearer <token>`，解析后把 claims 放入 Gin context | `40100` |

### 关键行为

- `AccessLog` 自动从 request context 获取 `request_id`；鉴权成功时补充 `user_id`。
- `Auth` 只接受 HS256，并要求 secret 已配置。
- `RateLimit` 使用内存 map，默认窗口一分钟，默认上限 10；每 1024 次调用清理一次过期 key。
- `BodyLimit` 的失败响应不是在中间件中直接写出的，而是由 handler 检测 `*http.MaxBytesError` 后输出 413。

### 审查关注

- 固定窗口限流是单机实现，不能跨实例共享；生产多副本应换 Redis 等共享存储。
- 大量不同客户端 IP 会扩大限流 map；当前清理策略是周期性全表扫描，规模较大时需要评估性能和内存上限。
- 外部传入的 `X-Request-Id` 当前没有长度和字符白名单校验。若直接暴露公网，建议限制长度，或在不受信任场景强制生成服务端 ID。
- JWT 是无状态校验，没有 token 撤销、黑名单、刷新和登出语义。
- `Recovery` 会先记录一次 `panic recovered`，随后 `response.Fail` 还会记录一次 5xx 的 `request failed`，同一 panic 可能产生两条 app error 日志。

## 8. HTTP 处理器模块

目录：`internal/handler/`

### 登录处理器

文件：`internal/handler/auth.go`

流程：

1. `ShouldBindJSON` 绑定 `schema.LoginRequest`，必填字段由 binding tag 校验。
2. 如果是请求体超限，返回 413。
3. 其他绑定失败返回 400。
4. 构造 `business.NewAuthBusiness(c.Request.Context(), svcCtx)` 并调用 `Login`。
5. 按错误类型映射：
   - `business.ErrInvalidCredentials` -> 401，不暴露用户不存在或密码错误。
   - context 取消/超时 -> 408。
   - 其他错误 -> `response.Fail`，未知错误降级为统一 500。

### 当前用户处理器

文件：`internal/handler/me.go`

- 从 Gin context 读取鉴权中间件写入的 claims。
- 没有 claims 时返回 401。
- 有 claims 时只返回 `id` 和 `username`，不查询数据库。

### 健康检查

文件：`internal/handler/health.go`

- 直接返回 `{status:"ok", time:...}`。
- 故意不使用统一响应 envelope，便于基础设施探针做简单判断。

### 审查关注

- 登录输入目前只有 `required`，没有用户名/密码长度、格式和最大长度约束。
- handler 没有直接写业务错误文案，错误映射集中在 `response` 和少量 switch 中，边界清晰。
- `/api/v1/me` 信任 token 中的身份；如果引入用户禁用、角色变更等实时状态，需要重新设计为查询会话或用户状态。

## 9. 业务模块

目录：`internal/business/`

### `AuthBusiness`

文件：`internal/business/auth.go`

构造方式：`NewAuthBusiness(ctx, svcCtx)`。业务对象持有本次请求的 `context.Context` 和共享依赖，符合当前项目约定的业务层模式。

登录业务步骤：

1. 检查 context 是否已经取消。
2. 通过 `UserDAO.FindByUsername` 查询用户。
3. 用户不存在时执行一次 dummy bcrypt 比较，使未知用户和密码错误的耗时路径接近。
4. 用户存在时执行真实 bcrypt 比较。
5. 通过 `TokenManager.Issue` 签发访问 token。
6. 返回 `LoginResponse`，其中包含 token、过期时间和最小化用户 DTO。

### 错误语义

`ErrInvalidCredentials` 同时表示“用户不存在”和“密码错误”，handler 统一映射为 401，避免客户端枚举账号。

### 业务日志

- 登录成功：`info`，包含 `user_id`、`username`。
- 凭证错误：`warn`，包含 `username` 和 `reason=invalid_credentials`。
- DAO 查询失败、token 签发失败：`error`。
- 不记录密码、密码哈希和 token。

### 审查关注

- dummy bcrypt 只能均衡未知用户与已知用户密码错误的时间差，不能替代完整的安全审计。
- 当前没有登录失败次数累计、账号锁定、验证码、刷新 token 和登出。
- 业务层直接依赖 `svc.ServiceContext`；如果未来需要复用业务逻辑到非 HTTP 场景，可考虑缩小为更精确的依赖接口。

## 10. 数据访问与模型模块

### DAO 接口

文件：`internal/dao/user.go`

```go
type UserDAO interface {
    FindByUsername(ctx context.Context, username string) (*model.User, error)
}
```

- 业务层只依赖接口。
- `ErrUserNotFound` 是 DAO 层 sentinel error。
- 接入 MySQL/gorm 时只需要提供新的实现，并在 `svc` 中替换。

### 内存实现

目录：`internal/dao/memory/`

- `UserDAO` 使用 `sync.RWMutex` 保护 map，并发安全。
- `FindByUsername` 返回用户副本，避免调用方修改内部数据。
- `NewDemoUserDAO` 在启动时生成 `admin123` 的 bcrypt 哈希，并写入演示用户。
- `NewUserDAO` 创建空存储，prod 当前使用该实现。

### 模型

文件：`internal/model/user.go`

- `PasswordHash` 带 `json:"-"`，不会通过 JSON 输出。
- 当前只有 `db` tag；接入 gorm 时可评估是否需要补充 gorm 专属 tag 或迁移到独立 model。

### 审查关注

- 还没有真实数据库、迁移脚本、事务和查询超时。
- 内存 DAO 适合 dev/test 和骨架演示，不适合作为生产持久层。
- 当前 `UserDAO` 只有按用户名查询，没有创建、更新、禁用等生命周期接口。

## 11. API DTO 模块

文件：`internal/schema/auth.go`

| 类型 | 用途 |
|---|---|
| `LoginRequest` | 登录输入，用户名和密码均必填 |
| `LoginResponse` | 登录输出，包含 access token、过期时间和用户 DTO |
| `UserDTO` | 对外用户视图，仅包含 id 和 username |

该模块与 `model` 分离：HTTP 结构不直接复用存储结构，避免数据库字段和 API 字段互相污染。

## 12. 通用基础设施

### 12.1 日志

目录：`infra/logger/`

当前使用一个 `log.rootDir` 配置，按模块创建四个独立 zap logger：

| 模块 | 文件 | 用途 |
|---|---|---|
| `app` | `<rootDir>/<YYYY-MM>/<YYYY-MM-DD>_app.log` | 启动、关闭、Server 错误、panic、未处理 5xx |
| `business` | `<rootDir>/<YYYY-MM>/<YYYY-MM-DD>_business.log` | 登录成功/拒绝等业务事件 |
| `gorm` | `<rootDir>/<YYYY-MM>/<YYYY-MM-DD>_gorm.log` | SQL、耗时、影响行数、慢查询、SQL 错误 |
| `access` | `<rootDir>/<YYYY-MM>/<YYYY-MM-DD>_access.log` | 每请求一条访问日志 |

关键实现：

- 每个模块一个独立 `*zap.Logger` 和一个独立 core。
- 所有 logger 共享同一个 rootDir，不暴露四个路径配置。
- `dailyWriter` 使用 `log.timezone` 检查日期，跨天后自动关闭旧文件并打开新日期文件。
- 日志文件使用追加写，启动时会创建当天文件和目录。
- `log.maxSizeMB` 为正数时，单个文件达到上限后继续生成 `<日期>_<模块>.<序号>.log`；设为 `0` 可关闭大小分片。
- `log.maxAgeDays` 为正数时，启动和跨天时清理早于保留窗口的历史文件；设为 `0` 可关闭按天保留。
- `log.compress=true` 时，今天之前已关闭的 `.log` 文件会压缩为 `.gz`，启动和跨天时异步执行。
- `log.level` 是四个模块共同的最低级别。
- `log.format=json` 使用 JSON 编码；`console` 使用无颜色 console 编码写入文件。
- 请求级日志方法接收 `context.Context`，通过 `infra/requestid` 自动补充 `request_id`。
- 初始化前或初始化失败时使用 stderr fallback logger，保证入口错误仍可见。
- Gin 默认输出和 `http.Server.ErrorLog` 都汇入 `app` 模块。
- `NewGormLogger` 把业务侧 GORM 日志适配到 `gorm` 模块：普通 SQL 为 info，慢查询为 warn，失败为 error。

接入 GORM 时需要注意：

- 在 `logger.Init` 之后创建 GORM logger，避免它捕获 fallback logger。
- 使用 `db.WithContext(ctx)` 执行查询，GORM 日志才能拿到同一个 `request_id`。
- GORM 默认可能输出带参数值的 SQL；包含敏感参数时开启 `gorm.Config.ParameterizedQueries` 或自行脱敏。

当前运维边界：

- 保留和压缩只会处理符合当前日期文件命名规则的文件；升级前旧路径下的日志不会自动迁移或清理。
- 清理不会主动删除空的月份目录，只会删除过期文件和压缩文件。
- 日志只写本地文件，不输出 stdout，也没有远程采集接入。
- 切日和保留窗口使用 `log.timezone`；容器部署时应显式配置 `Asia/Shanghai` 或其他业务时区。

### 12.2 请求 ID

文件：`infra/requestid/requestid.go`

- 使用私有 context key 保存请求 ID。
- 不依赖 Gin 或 `internal/*`。
- `RequestID` 中间件同时维护 Gin context、request context 和 `X-Request-Id` 响应头，三者可互相兼容。
- `response` 和 `logger` 都可以从 request context 读取同一个 ID。

### 12.3 统一响应与错误

文件：`infra/response/response.go`

成功响应：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {},
  "request_id": "...",
  "timestamp": 0
}
```

业务码是独立于 HTTP 状态的稳定命名空间：

| 业务码 | 含义 |
|---|---|
| `0` | 成功 |
| `40000` | 请求参数错误 |
| `40100` | 未认证 |
| `40400` | 资源不存在 |
| `40500` | 方法不允许 |
| `40800` | 超时/请求取消 |
| `41300` | 请求体过大 |
| `42900` | 限流 |
| `50000` | 内部错误 |

错误处理规则：

- 已知 `*AppError` 保留 HTTP 状态和业务码。
- 5xx 的 `AppError` 会写入 `app` error 日志。
- 未知错误统一降级为 `50000` 和通用文案，原始错误只写日志，不返回客户端。
- 4xx 不写 app error 日志，由 access 日志记录访问结果。

### 12.4 JWT

文件：`infra/token/token.go`

- 使用 HS256。
- `AuthClaims` 包含 `uid`、`username` 和标准 RegisteredClaims。
- `Issue` 返回 token 和过期时间；TTL 非正数时回退 15 分钟。
- `ParseInto` 强制只接受 HS256，并使用 `WithValidMethods` 再限制一次。
- `ExtractBearer` 只接受 `Bearer <token>` 形式，大小写不敏感。

当前没有 issuer、audience、not-before、刷新 token、撤销和密钥轮换。

### 12.5 分页

文件：`infra/pagination/pagination.go`

- 页码分页：默认第 1 页、每页 10 条、最大 100 条，提供 `Offset()` 和 `Limit()`。
- Cursor 分页：按递增 id 的方向提供 `LastID`、归一化后的 limit 和 `HasCursor()`。
- 当前路由尚未使用该模块，属于给后续列表接口预留的通用能力。

## 13. 请求链路

### 13.1 登录

```text
POST /api/v1/login
  RequestID
  -> BodyLimit
  -> AccessLog
  -> Recovery
  -> RateLimit
  -> handler.Login
       -> ShouldBindJSON
       -> business.AuthBusiness.Login
            -> UserDAO.FindByUsername
            -> bcrypt 校验
            -> TokenManager.Issue
            -> business 日志
       -> response.OK
  -> AccessLog 写 access 日志
```

成功时，业务日志和 access 日志共享同一个 `request_id`。若后续登录业务改为数据库 DAO，并且调用链使用 `db.WithContext(ctx)`，GORM 日志也会共享同一个 ID。

### 13.2 当前用户

```text
GET /api/v1/me
  RequestID -> BodyLimit -> AccessLog -> Recovery
  -> middleware.Auth
       -> ExtractBearer
       -> TokenManager.ParseInto
       -> 写入 claims
  -> handler.Me
       -> 读取 claims
       -> response.OK
  -> AccessLog（鉴权成功时附带 user_id）
```

### 13.3 404 / 405

```text
RequestID -> BodyLimit -> AccessLog -> Recovery -> NoRoute / NoMethod
  -> response.Fail
  -> AccessLog
```

404 和 405 也使用统一 envelope，便于客户端和服务端日志保持一致。

## 14. 日志关联示例

一次请求的典型关联关系：

```text
access.log   request_id=abc123 path=/api/v1/login status=200
business.log request_id=abc123 login succeeded user_id=1 username=admin
gorm.log     request_id=abc123 gorm query elapsed=2ms rows_affected=1 sql=...
```

关联依赖两个条件：

1. 请求经过 `middleware.RequestID`，ID 被写入 request context。
2. 业务和 GORM 调用使用同一个 request context；GORM 场景必须使用 `db.WithContext(ctx)`。

## 15. 当前未完成事项与风险

### 数据与持久化

- MySQL/Redis 只有配置和辅助方法，没有连接、连接池、健康检查、迁移或关闭逻辑。
- 默认 UserDAO 是内存实现；prod 使用空内存存储，真实登录尚不可用。
- 没有事务、锁、软删除和真实 SQL 查询。

### 安全

- 登录只做必填校验，没有密码长度、用户名格式和请求频率之外的防爆破措施。
- JWT 没有刷新、撤销、黑名单、issuer/audience 校验和密钥轮换。
- `X-Request-Id` 未限制外部输入长度和字符。
- GORM 日志可能包含 SQL 参数，敏感字段需要参数化或脱敏。
- 限流是单进程内存实现，多副本下不共享计数。

### 可观测性

- 没有 metrics、trace、慢请求采样和依赖健康检查。
- 日志没有保留期、压缩和清理策略。
- 当前只写文件；容器化环境若依赖 stdout 采集，需要增加 stdout sink 或 sidecar 采集。

### 工程能力

- 没有测试文件，当前验证依赖 `go build`、`go vet`、运行期手工请求。
- 没有 CI、代码生成、数据库迁移工具和容器化部署文件。
- 分页模块尚未被任何路由使用。

## 16. 审查清单

建议按以下顺序审查当前骨架：

1. **启动链路是否正确**：`main.go` 的配置加载、日志初始化、校验、服务装配、信号关闭顺序。
2. **配置是否符合部署方式**：环境变量名、prod secret、trusted proxies、日志目录和超时。
3. **分层依赖是否越界**：handler 是否只做适配，business 是否直接操作 HTTP，infra 是否反向依赖 internal。
4. **认证与错误语义是否安全**：账号枚举、JWT 校验、401 文案、未知错误是否泄露内部信息。
5. **中间件顺序是否正确**：RequestID、BodyLimit、AccessLog、Recovery、限流和鉴权的作用范围。
6. **日志是否能串联请求**：请求 ID 是否贯穿 access、business、gorm，敏感信息是否被排除。
7. **生产缺口是否明确**：真实 DAO、MySQL/Redis、共享限流、日志保留和监控。
8. **扩展点是否清晰**：新增路由、业务、DAO、依赖和日志模块时是否只需改局部装配。

## 17. 推荐后续迭代

按当前骨架的依赖边界，建议优先顺序如下：

1. 接入真实 UserDAO：MySQL + gorm，补充迁移和连接关闭。
2. 在 `svc` 中初始化并注入 gorm logger，验证 `request_id` 贯穿 SQL 日志。
3. 为登录输入增加长度和格式约束，并评估账号锁定/验证码策略。
4. 将限流抽象成接口，提供内存实现和 Redis 实现。
5. 增加日志保留、压缩和 stdout/远程采集策略。
6. 增加 metrics、trace、依赖健康检查和 readiness 路由。
7. 补充核心路径测试；如项目要求“不保留测试文件”，则至少保留 CI 中的构建和静态检查。
