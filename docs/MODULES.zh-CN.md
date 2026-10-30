# gin-scaffold 当前骨架模块梳理

> 基线状态：当前工作区（已完成日志改造、MySQL + GORM、可选 Redis 客户端接入；登录限流已移除）
>
> 整理目标：按模块说明当前代码的职责、依赖边界、运行链路和审查重点。本文描述的是当前已实现状态，不把规划中的能力写成既有能力。

## 0. 快速结论

- 项目是一个 Gin 服务骨架，启动时依赖可用的 MySQL；Redis 是可选依赖，连接失败只会记录 warning，不阻断启动。
- 分层主线清晰：`handler -> business -> dao`，`middleware` 负责横切能力，`svc` 负责依赖装配，`infra` 放通用基础设施。
- 启动流程包含配置加载、配置校验、日志初始化、依赖装配、路由构建、HTTP 服务和优雅关闭。
- 已有能力：JWT 登录与鉴权、统一响应、错误码、访问日志、请求 ID、panic 恢复、三模块日志、可配置的按天切割、日志压缩和保留清理、MySQL 连接池、可选 Redis 连接池、启动 ping 和关闭逻辑。
- 当前仍是骨架而非完整生产实现：用户表需要手动迁移，JWT 仍是单机无状态校验，没有刷新、撤销和黑名单。
- 日志已按 `app`、`gorm`、`access` 拆分，并通过 `request_id` 串联；业务内部日志统一写入 `app`，GORM 查询通过 `db.WithContext(ctx)` 写入 `gorm`，HTTP 请求和 Gin 输出写入 `access`。

## 1. 模块地图

```text
.
├── main.go                    进程入口、HTTP Server 生命周期
├── etc/                       分环境 YAML 配置
├── internal/
│   ├── config/                配置结构、默认值、加载、校验
│   ├── svc/                   依赖注入容器 ServiceContext
│   ├── router/                路由注册和全局中间件装配
│   ├── middleware/            请求 ID、鉴权、访问日志、恢复等
│   ├── handler/               HTTP 适配层
│   ├── business/              业务编排和业务错误
│   ├── dao/                   数据访问接口和 GORM 实现
│   ├── model/                 存储侧模型
│   └── schema/                HTTP 请求/响应 DTO
├── infra/
│   ├── logger/                分模块 zap 日志、按天/固定文件、GORM 适配、request_id context
│   ├── response.go            统一响应包、AppError、业务码
│   ├── jwt.go                 JWT 签发和校验
│   └── pagination.go          分页归一化
├── migrations/                MySQL 建表脚本
└── docs/                      架构分析和本模块梳理
```

| 模块 | 主要文件 | 核心职责 |
|---|---|---|
| 进程入口 | `main.go` | 解析环境/配置、初始化日志、装配依赖、启动和关闭 HTTP Server |
| 配置 | `internal/config/config.go` | 配置结构、默认值、Viper 加载、启动前校验 |
| 依赖装配 | `internal/svc/service_context.go` | 构造 JWT manager、DAO 等共享依赖 |
| 路由 | `internal/router/*.go` | Gin 引擎、中间件顺序、公开/受保护路由、404/405 |
| 中间件 | `internal/middleware/*.go` | 请求 ID、访问日志、panic 恢复、JWT 鉴权 |
| 处理器 | `internal/handler/*.go` | 绑定 HTTP 输入、调用业务、输出统一响应 |
| 业务 | `internal/business/*.go` | 登录编排、凭证校验、错误语义、业务日志 |
| DAO | `internal/dao/**` | 定义持久化接口，提供 MySQL/GORM 实现 |
| 领域/存储模型 | `internal/model/user.go` | 存储侧用户行结构 |
| API DTO | `internal/schema/auth.go` | HTTP 请求和响应结构 |
| 日志 | `infra/logger/*.go` | 模块 logger、按日文件、request_id 注入、GORM 适配 |
| 请求 ID | `infra/logger/requestid.go` | 在 `context.Context` 中保存和读取请求 ID |
| 统一响应 | `infra/response.go` | 响应 envelope、AppError、业务码、错误日志 |
| JWT | `infra/jwt.go` | HS256 JWT 签发、解析、Bearer 提取 |
| 分页 | `infra/pagination.go` | page/size 和 id cursor 边界归一化 |

## 2. 依赖方向与分层规则

```text
main
 ├── config
 ├── logger
 ├── svc ────────┬── dao/gorm ─── gorm.io/gorm
 │               ├── redis ────── go-redis
 │               └── infra
 └── router
      ├── middleware ──── infra / logger
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

当前 `infra/*` 没有导入 `internal/*`，方向保持正确。业务层通过 `svc.ServiceContext` 获取 DAO 和 JWT manager，这是当前骨架的核心装配方式。

## 3. 进程入口与生命周期

文件：`main.go`

### 启动顺序

1. 解析唯一参数 `-f`，默认加载 `etc/config_dev.yaml`。
2. `config.Load`：默认值 < YAML 文件 < 环境变量。
3. `cfg.Validate`：校验配置中的 `server.env`、端口、JWT、日志等字段。
4. `logger.Init`：按 `log.rootDir` 初始化 `app/gorm/access` 三个 logger。
5. `svc.NewServiceContext`：建立 MySQL 连接并执行启动 ping；尝试建立 Redis 连接，失败时记录 warning 并继续；构造 JWT manager 和 GORM UserDAO。
6. `router.New`：根据配置中的 `server.env` 映射 Gin 模式，设置固定不信任代理、全局中间件和路由。
7. 构造 `http.Server`，只设置监听地址、Handler 和错误日志；请求超时使用 `net/http` 默认值。
8. 监听 `SIGINT/SIGTERM`，在 goroutine 中启动 `ListenAndServe`。
9. 收到信号或监听错误后，使用 `Shutdown` 做优雅关闭，并等待监听 goroutine 返回；第二次信号可强制结束进程。
10. `svc.ServiceContext.Close()` 关闭数据库和可用的 Redis 客户端；最后 `logger.Close()` 刷新并关闭日志文件。

### 审查关注

- `cfg.Validate` 在 `logger.Init` 之前执行；配置非法时不会创建日志文件，错误由入口的 fallback logger 输出到 stderr。
- 服务没有独立 readiness 检查；`/healthz` 是轻量 liveness 探针，不检查数据库、Redis 等外部依赖。
- 优雅关闭使用 `main.go` 中 `httpShutdownTimeout` 常量限制为 10 秒；`Shutdown` 返回后读取监听结果，关闭失败和监听错误会记录 error 并合并返回。

## 4. 配置模块

文件：`internal/config/config.go`、`etc/config.example.yaml`、`etc/config_dev.yaml`、`etc/config_test.yaml`、`etc/config_prod.yaml`

### 配置来源与优先级

```text
代码默认值 < -f 指定的 YAML 文件 < 环境变量
```

环境变量没有前缀，`.` 替换为 `_`，例如：

- `SERVER_PORT`
- `AUTH_JWTSECRET`
- `LOG_ROOTDIR`
- `MYSQL_PASSWORD`
- `REDIS_PASSWORD`

CLI 只保留 `-f`，默认值为 `etc/config_dev.yaml`。环境由配置文件的 `server.env` 字段决定，不能再用 `-env` 或 `-c` 选择。

### 配置分组

| 分组 | 关键字段 | 用途 |
|---|---|---|
| `server` | env、host、port | `env` 取 dev/test/prod 并映射 Gin mode；host/port 为 HTTP 监听地址，请求超时使用 `net/http` 默认值 |
| `auth` | jwtSecret、accessExpireSeconds | JWT 签发和有效期 |
| `log` | rootDir、level、format、dailyRotate、timezone、maxSizeMB、maxAgeDays、compress | 日志根目录、级别、编码、按天开关、切日时区、大小分片、保留和压缩 |
| `mysql` | 连接、库名、连接池、超时 | 建立 GORM 连接、启动 ping、连接池和优雅关闭 |
| `redis` | 地址、密码、DB、连接池、超时 | 尝试建立 go-redis 客户端、启动 ping、连接池和优雅关闭；失败不阻断启动 |

### 启动校验

`Config.Validate` 会检查：

- 环境名只能是 `dev`、`test`、`prod`。
- 端口范围合法。
- Gin mode 由 `server.env` 自动映射，不再单独配置；prod 固定映射为 `release`。
- JWT secret 必填；prod 长度至少 32 字节。
- access TTL 为正数。
- MySQL host/port/user/database/charset 必填且格式合法；prod 必须提供非空密码；连接池必须为正数或合法非负值，`maxIdleConns` 不能超过 `maxOpenConns`。
- Redis host/port 必填；prod 必须提供非空密码；DB 不能为负数；连接池和各类超时必须为正数，`minIdleConns` 不能超过 `poolSize`。
- 日志级别只能是 `debug/info/warn/error`，格式只能是 `console/json`；rootDir 和 timezone 不能为空，timezone 必须可加载；maxSizeMB 和 maxAgeDays 不能为负数；`dailyRotate=false` 且启用保留或压缩时，maxSizeMB 必须大于 `0`。

### 审查关注

- dev/test 配置包含演示 secret；只能用于本地或测试，不能复用到真实环境。
- MySQL 和 Redis 配置都已被 `svc` 消费；MySQL 连接失败会阻止进程启动，Redis 连接失败只记录 warning 并让 `ServiceContext.Redis` 保持 `nil`。
- `MySQLConf.DSN` 固定 `parseTime=true&loc=Local`，并包含 5 秒拨号超时、30 秒读超时和 30 秒写超时；当前没有 TLS 参数，生产启用 TLS 时需要扩展 DSN。
- `X-Forwarded-For` 固定不信任，`ClientIP` 始终使用直连地址；如果后续部署在反向代理后并需要真实客户端 IP，需要重新引入可信代理配置。

## 5. 依赖装配模块

文件：`internal/svc/service_context.go`

`ServiceContext` 当前包含：

```go
type ServiceContext struct {
    Config       *config.Config
    JWTManager   *infra.JWTManager
    UserDAO      dao.UserDAO
    DB           *gorm.DB
    Redis        *redis.Client
}
```

### 构造行为

- 拒绝空配置。
- 要求 `auth.jwtSecret` 非空，属于入口校验前的防御性检查。
- 调用 `openDatabase` 打开 MySQL：使用 GORM MySQL dialector，注入 `gorm` 模块 logger，设置连接池上限、生命周期和空闲时间，并在启动阶段 ping。
- 调用 `openRedis` 尝试打开 Redis：设置密码、DB、连接池、拨号和读写超时，并在启动阶段 ping；失败时记录 warning、关闭临时客户端并保持 `ServiceContext.Redis == nil`，不会关闭 MySQL。
- 构造 `gormdao.NewUserDAO(db)`，不再按环境切换 DAO 实现。
- 用配置构造 `infra.JWTManager`。
- 提供幂等的 `Close()`，关闭底层 `*sql.DB` 和 Redis 客户端。

### 审查关注

- 这里是继续接入外部客户端和更多 DAO 的最佳位置。
- MySQL 连接失败会阻止进程启动；Redis 连接失败不会阻止启动，调用方在使用 `ServiceContext.Redis` 前必须容忍其为 `nil`。当前没有路由消费 Redis。
- 如果后续增加更多需要关闭的资源，应纳入 `ServiceContext.Close()`，并在 `main.go` 的 defer 中统一释放。

## 6. 路由模块

文件：`internal/router/router.go`、`internal/router/routes.go`

### 全局中间件顺序

当前注册顺序：

```text
RequestID -> AccessLog -> Recovery -> 路由中间件 -> handler
```

顺序含义：

- `RequestID` 最先执行，后续日志和响应都能拿到请求 ID。
- `AccessLog` 包住后续链路，请求结束后记录最终状态和耗时。
- `Recovery` 位于访问日志内部，panic 被转换成 500 后，访问日志仍能记录 500。

### 路由表

| 方法 | 路径 | 中间件 | 说明 |
|---|---|---|---|
| `GET` | `/healthz` | 全局中间件 | 极简存活探针，刻意不走统一 envelope |
| `POST` | `/api/v1/login` | 全局中间件 | 用户名密码登录并签发 JWT |
| `GET` | `/api/v1/me` | 全局 + JWT 鉴权 | 返回 JWT claims 中的当前用户身份 |

### 异常路由

- `NoRoute` 返回统一 `404` envelope，业务码 `40400`。
- `HandleMethodNotAllowed = true`，`NoMethod` 返回统一 `405` envelope，业务码 `40500`。

### 审查关注

- `AccessLog` 必须在 `Recovery` 外层；如果顺序反过来，panic 恢复后的访问日志可能缺失最终状态。
- 新增受保护路由时应放入 `protected` 分组，避免遗漏 `middleware.Auth`。

## 7. 中间件模块

目录：`internal/middleware/`

| 中间件 | 文件 | 行为 | 失败输出 |
|---|---|---|---|
| `RequestID` | `requestid.go` | 读取 `X-Request-Id`，没有则生成 16 字节随机 hex；写入 Gin context、request context 和响应头 | 随机源失败时使用 `unknown` |
| `AccessLog` | `accesslog.go` | 请求结束后记录 method/path/status/latency/client_ip/user_id | 写 `access` 模块日志 |
| `Recovery` | `recovery.go` | recover panic，记录 app error 并返回统一 500 | `50000` |
| `Auth` | `auth.go` | 校验 `Authorization: Bearer <JWT>`，解析后把 claims 放入 Gin context | `40100` |

### 关键行为

- `AccessLog` 自动从 request context 获取 `request_id`；鉴权成功时补充 `user_id`。
- `Auth` 只接受 HS256，并要求 secret 已配置。

### 审查关注

- `ClientIP` 使用直连地址，部署在反向代理后需要重新评估真实 IP 获取方式。
- 外部传入的 `X-Request-Id` 当前没有长度和字符白名单校验。若直接暴露公网，建议限制长度，或在不受信任场景强制生成服务端 ID。
- JWT 是无状态校验，没有 JWT 撤销、黑名单、刷新和登出语义。
- `Recovery` 会先记录一次 `panic recovered`，随后 `response.Fail` 还会记录一次 5xx 的 `request failed`，同一 panic 可能产生两条 app error 日志。

## 8. HTTP 处理器模块

目录：`internal/handler/`

### 登录处理器

文件：`internal/handler/auth.go`

流程：

1. `ShouldBindJSON` 绑定 `schema.LoginRequest`，必填字段由 binding tag 校验。
2. 绑定失败返回 400。
3. 构造 `business.NewAuthBusiness(c.Request.Context(), svcCtx)` 并调用 `Login`。
4. 按错误类型映射：
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
- `/api/v1/me` 信任 JWT claims 中的身份；如果引入用户禁用、角色变更等实时状态，需要重新设计为查询会话或用户状态。

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
5. 通过 `JWTManager.Issue` 签发访问 JWT。
6. 返回 `LoginResponse`，其中包含 JWT、过期时间和最小化用户 DTO。

### 错误语义

`ErrInvalidCredentials` 同时表示“用户不存在”和“密码错误”，handler 统一映射为 401，避免客户端枚举账号。

### 业务日志

- 登录成功：`info`，包含 `user_id`、`username`。
- 凭证错误：`warn`，包含 `username` 和 `reason=invalid_credentials`。
- DAO 查询失败、JWT 签发失败：`error`。
- 不记录密码、密码哈希和 JWT。

### 审查关注

- dummy bcrypt 只能均衡未知用户与已知用户密码错误的时间差，不能替代完整的安全审计。
- 当前没有登录失败次数累计、账号锁定、验证码、刷新 JWT 和登出。
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
- 数据库实现位于 `internal/dao/gorm/user.go`，由 `svc` 注入。

### GORM 实现

目录：`internal/dao/gorm/`

- `UserDAO` 持有 `*gorm.DB`，由 `NewUserDAO` 构造。
- `FindByUsername` 使用 `db.WithContext(ctx)` 执行参数化查询，保证查询能被请求取消，并让 GORM 日志拿到同一个 `request_id`。
- `gorm.ErrRecordNotFound` 转换为 `dao.ErrUserNotFound`，其他数据库错误带上下文返回。
- 不记录明文密码、密码哈希或 JWT。

### 表结构

目录：`migrations/`

- `001_create_users.sql` 创建 `users` 表，包含用户名唯一索引、密码哈希和审计时间字段。
- 表结构由迁移脚本管理，不使用 GORM `AutoMigrate` 自动修改生产表。
- 服务启动只验证数据库连通性，不自动建库、建表或写入种子用户。

### 模型

文件：`internal/model/user.go`

- `PasswordHash` 带 `json:"-"`，不会通过 JSON 输出。
- 模型同时带 `db` 和 `gorm` tag，`TableName()` 固定为 `users`，避免类型重命名改变表名。

### 审查关注

- 还没有事务封装、软删除和业务级查询超时；当前只提供一个按用户名查询。
- 迁移脚本没有接入自动执行工具，部署流程需要显式执行。
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

当前使用一个 `log.rootDir` 配置，按模块创建三个独立 zap logger。`log.dailyRotate=true`（默认）时使用日期目录和日期文件：

| 模块 | 文件 | 用途 |
|---|---|---|
| `app` | `<rootDir>/<YYYY-MM>/<YYYY-MM-DD>_app.log` | 启动、关闭、Server 错误、panic、未处理 5xx、登录成功/拒绝等内部业务日志 |
| `gorm` | `<rootDir>/<YYYY-MM>/<YYYY-MM-DD>_gorm.log` | SQL、耗时、影响行数、慢查询、SQL 错误 |
| `access` | `<rootDir>/<YYYY-MM>/<YYYY-MM-DD>_access.log` | 每请求一条访问日志，以及 Gin 框架自身的 debug/warning/error 输出 |

`log.dailyRotate=false` 时关闭日期切换，三个模块使用固定文件：`<rootDir>/app.log`、`gorm.log`、`access.log`。大小分片分别为 `<日期>_<模块>.<序号>.log` 和 `<模块>.<序号>.log`。

关键实现：

- `logger.Init(cfg.Log)` 直接接收 `logger.Config`，无需在入口逐字段搬运配置。
- 每个模块一个独立 `*zap.Logger` 和一个独立 core。
- 所有 logger 共享同一个 rootDir，不暴露独立路径配置。
- `dailyRotate=true` 时，`dailyWriter` 使用 `log.timezone` 检查日期，跨天后自动关闭旧文件并打开新日期文件；`false` 时不再按日期切换，启动和大小轮转时选择可追加的最新固定分片。
- 日志文件使用追加写，启动时会创建活动文件或当天文件和目录。
- `log.maxSizeMB` 为正数时，单个文件达到上限后生成下一个序号文件；设为 `0` 可关闭大小分片。
- `log.maxAgeDays` 为正数时异步清理过期文件：按天模式使用文件名日期，固定文件模式使用文件修改时间；设为 `0` 可关闭保留清理。
- `log.compress=true` 时，今天之前的已关闭 `.log` 文件会压缩为 `.gz`；固定文件模式跳过当前活动文件，因此需要配合大小分片才能持续压缩历史文件。
- `log.level` 是三个模块共同的最低级别。
- `log.format=json` 使用 JSON 编码；`console` 使用无颜色 console 编码写入文件。
- 请求级日志方法接收 `context.Context`，通过 `infra/logger` 的 context 辅助函数自动补充 `request_id`。
- 初始化前或初始化失败时使用 stderr fallback logger，保证入口错误仍可见。
- Gin 默认输出汇入 `access` 模块；`http.Server.ErrorLog` 仍属于应用内部错误，汇入 `app` 模块。
- `NewGormLogger` 把业务侧 GORM 日志适配到 `gorm` 模块：普通 SQL 为 info，慢查询为 warn，失败为 error。
- GORM 适配器实现 `ParamsFilter`：默认保留 SQL 占位符、丢弃参数值，避免敏感参数进入 `gorm.log`；默认忽略 `ErrRecordNotFound`。需要参数值或未命中诊断时使用 `NewGormLoggerWithOptions` 显式开启。

当前 GORM 接入注意事项：

- `internal/svc/database.go` 在 `logger.Init` 之后创建 GORM logger，避免它捕获 fallback logger。
- 使用 `db.WithContext(ctx)` 执行查询，GORM 日志才能拿到同一个 `request_id`。
- 默认不记录 SQL 参数值；如显式开启 `LogSQLParameters`，必须确认日志访问权限和保留策略符合敏感数据要求，或者在查询层做脱敏。
- 当前慢查询阈值固定为 200ms，默认不记录 SQL 参数，默认忽略 `ErrRecordNotFound`；这些是安全默认值，未暴露成 YAML 配置项。

当前运维边界：

- 保留和压缩只会处理当前模式对应的命名规则：按天模式为日期文件，固定文件模式为 rootDir 下的固定文件名；升级前其他路径下的日志不会自动迁移或清理。
- 清理不会主动删除空的月份目录，只会删除过期文件和压缩文件。
- 固定文件模式的活动文件不会因过期或压缩策略被删除；若 `maxSizeMB=0` 且启用保留或压缩，配置校验会直接拒绝。
- 日志只写本地文件，不输出 stdout，也没有远程采集接入。
- 切日和保留窗口使用 `log.timezone`；容器部署时应显式配置 `Asia/Shanghai` 或其他业务时区。

### 12.2 请求 ID

文件：`infra/logger/requestid.go`

- 使用私有 context key 保存请求 ID。
- 不依赖 Gin 或 `internal/*`。
- `RequestID` 中间件同时维护 Gin context、request context 和 `X-Request-Id` 响应头，三者可互相兼容。
- `response` 的 RequestID 辅助函数和 `logger` 都从同一个 request context 读取 ID。

### 12.3 统一响应与错误

文件：`infra/response.go`

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
| `50000` | 内部错误 |

错误处理规则：

- 已知 `*AppError` 保留 HTTP 状态和业务码。
- 5xx 的 `AppError` 会写入 `app` error 日志。
- 未知错误统一降级为 `50000` 和通用文案，原始错误只写日志，不返回客户端。
- 4xx 不写 app error 日志，由 access 日志记录访问结果。

### 12.4 JWT

文件：`infra/jwt.go`

- 使用 HS256。
- `AuthClaims` 包含 `uid`、`username` 和标准 RegisteredClaims。
- `Issue` 返回 JWT 和过期时间；TTL 非正数时回退 15 分钟。
- `ParseInto` 强制只接受 HS256，并使用 `WithValidMethods` 再限制一次。
- `ExtractBearer` 只接受 `Bearer <JWT>` 形式，大小写不敏感。

当前没有 issuer、audience、not-before、刷新 JWT、撤销和密钥轮换。

### 12.5 分页

文件：`infra/pagination.go`

- 页码分页：默认第 1 页、每页 10 条、最大 100 条，提供 `Offset()` 和 `Limit()`。
- Cursor 分页：按递增 id 的方向提供 `LastID`、归一化后的 limit 和 `HasCursor()`。
- 当前路由尚未使用该模块，属于给后续列表接口预留的通用能力。

## 13. 请求链路

### 13.1 登录

```text
POST /api/v1/login
  RequestID
  -> AccessLog
  -> Recovery
  -> handler.Login
       -> ShouldBindJSON
       -> business.AuthBusiness.Login
            -> UserDAO.FindByUsername
            -> bcrypt 校验
            -> JWTManager.Issue
            -> app 日志（业务事件）
       -> response.OK
  -> AccessLog 写 access 日志
```

成功时，app 中的业务日志、GORM 查询日志和 access 日志共享同一个 `request_id`；DAO 使用 `db.WithContext(ctx)` 把请求上下文传给 GORM。

### 13.2 当前用户

```text
GET /api/v1/me
  RequestID -> AccessLog -> Recovery
  -> middleware.Auth
       -> ExtractBearer
       -> JWTManager.ParseInto
       -> 写入 claims
  -> handler.Me
       -> 读取 claims
       -> response.OK
  -> AccessLog（鉴权成功时附带 user_id）
```

### 13.3 404 / 405

```text
RequestID -> AccessLog -> Recovery -> NoRoute / NoMethod
  -> response.Fail
  -> AccessLog
```

404 和 405 也使用统一 envelope，便于客户端和服务端日志保持一致。

## 14. 日志关联示例

一次请求的典型关联关系：

```text
access.log request_id=abc123 path=/api/v1/login status=200
app.log    request_id=abc123 login succeeded user_id=1 username=admin
gorm.log   request_id=abc123 gorm query elapsed=2ms rows_affected=1 sql=...
```

关联依赖两个条件：

1. 请求经过 `middleware.RequestID`，ID 被写入 request context。
2. 业务和 GORM 调用使用同一个 request context；GORM 场景必须使用 `db.WithContext(ctx)`。

## 15. 当前未完成事项与风险

### 数据与持久化

- MySQL 已接入 GORM、连接池、启动 ping 和关闭逻辑；Redis 已接入客户端、连接池、启动 ping 和关闭逻辑，但它是可选依赖，连接失败只记录 warning，且当前没有路由使用 Redis。
- 用户表需要先执行 `migrations/001_create_users.sql`，且没有内置种子账号。
- 没有事务、锁、软删除、迁移工具和查询超时封装；当前只有一条按用户名查询。

### 安全

- 登录只做必填校验，没有密码长度、用户名格式和频率限制等防爆破措施。
- JWT 没有刷新、撤销、黑名单、issuer/audience 校验和密钥轮换。
- `X-Request-Id` 未限制外部输入长度和字符。
- GORM 日志默认只保留 SQL 占位符、不记录参数；如显式开启参数记录，敏感字段需要脱敏。

### 可观测性

- 没有 metrics、trace、慢请求采样和依赖健康检查。
- 本地日志已具备保留期、压缩和清理策略，但没有 stdout sink、集中式日志采集、metrics 或告警。
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
5. **中间件顺序是否正确**：RequestID、AccessLog、Recovery 和鉴权的作用范围。
6. **日志是否能串联请求**：请求 ID 是否贯穿 access、app、gorm，敏感信息是否被排除。
7. **生产缺口是否明确**：迁移流程、Redis 高可用、JWT 生命周期、日志保留和监控。
8. **扩展点是否清晰**：新增路由、业务、DAO、依赖和日志模块时是否只需改局部装配。

## 17. 推荐后续迭代

按当前骨架的依赖边界，建议优先顺序如下：

1. 将迁移脚本接入部署流程，并建立开发/生产的种子用户管理方式。
2. 验证 `request_id` 在 MySQL/GORM 查询日志中的贯穿，并补充数据库错误分类。
3. 为登录输入增加长度和格式约束，并评估账号锁定/验证码策略。
4. 增加 stdout/远程日志采集，并在部署环境验证保留、压缩和磁盘容量策略。
5. 增加 metrics、trace、依赖健康检查和 readiness 路由。
6. 补充核心路径测试；如项目要求“不保留测试文件”，则至少保留 CI 中的构建和静态检查。
