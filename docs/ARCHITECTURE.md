# Architecture analysis: mysurl1 (go-zero) → gin skeleton

This document records the Phase-1 analysis that motivated this skeleton. The
source of study is the `mysurl1` go-zero short-link service; the goal was to
**extract reusable engineering ideas**, not to port the business.

## 1. Ideas worth keeping

- **Documented layer boundaries.** Each layer answers one question (config =
  what is configurable, schema = what the API exchanges, model = one row, dao =
  how data is stored, business = when/why, handler = how HTTP enters, middleware =
  cross-cutting, svc = how deps assemble, utils = reused generics).
- **Thin handlers.** Fixed shape: parse → new business → call → unified write.
- **Logic constructor pattern** `(ctx, svcCtx)`; methods orchestrate
  validate → cache → dao → assemble.
- **Pure DAOs.** Raw SQL, soft delete, dynamic query builder, cursor pagination
  + count, transactions.
- **Unified response + typed error** (`AppError` mapped via `errors.As`).
- **ServiceContext as a DI container** (constructed once).
- **Auth claims passed via context**, injected by middleware, read in handlers.
- **Cross-cutting via middleware**: JWT auth.
- **Strategy + registry** for pluggable short-code generators.
- **Background worker with interface segregation** (`VisitCountCache`).
- **Graceful degradation** when DB/Redis/Bloom are unavailable.

## 2. go-zero coupling points (and gin replacements)

| go-zero | role | gin replacement |
|---------|------|-----------------|
| `rest.Server`, `rest.Route`, `WithMiddlewares` | routing + server lifecycle | `gin.Engine` / groups |
| `config` embeds `rest.RestConf` | server/log/trace config | custom `ServerConf` + `http.Server` |
| `conf.MustLoad` | YAML + tag defaults | `spf13/viper` (per-env `config_<env>.yaml` + `SetDefault` + env vars) |
| `httpx.Parse` / `httpx.WriteJsonCtx` | bind / write | `c.ShouldBind*` / `c.JSON` |
| `sqlx.SqlConn` | data access | `database/sql` / `sqlx` / `gorm` |
| `logx` | logging | `go.uber.org/zap` |
| `stat`, `syncx.SingleFlight` | metrics / dedup | business-specific, add as needed |

Handler and middleware *ideas* port directly, but the concrete signatures are
gin-specific (`gin.HandlerFunc`). Routing, binding, config loading, sqlx and
logx remain the go-zero-bound pieces.

## 3. Migrated into this skeleton

Layering & boundaries · unified response + typed error · business constructor
pattern · ServiceContext DI · JWT auth + claims-in-context · per-module zap
logging with request-id correlation and daily rotation · interface-based
UserDAO (MySQL/GORM) + bcrypt login · migration-based schema · pagination
normalization · viper config (per-env file + defaults + env overrides) ·
graceful shutdown · Redis-backed shared login rate limiting.

## 4. Deliberately left in the business project

Short-code strategies (snowflake/base62/redis-incr/auto-increment) · URL
normalize/hash · Bloom reuse check · singleflight origin-merge · visit-count
Redis aggregation + async flush · concrete SQL/schema.

## 5. Design principles of the skeleton

Small dependency set (gin + jwt + viper + zap + gorm/mysql + redis) · thin handler / thick business /
cross-cutting middleware · interface-based DAO · one response contract · deps
centralized in svc · observability (request id + app/gorm/access
logs + panic guard + 5xx error logs) · externalized config with defaults + env
overrides and startup validation · shared Redis login rate limiting + fixed proxy
trust policy · business errors decoupled from HTTP status · graceful
shutdown · fail-fast MySQL and Redis connectivity with bounded connection pools
and startup pings.

## 6. Request flows

Public liveness probe (`GET /healthz`):

```
RequestID -> AccessLog -> Recovery -> handler.Health
  -> 200 {status:"ok"}
```

Login and protected read (`POST /api/v1/login`, `GET /api/v1/me`):

```
POST /api/v1/login:
  RequestID -> AccessLog -> Recovery -> RateLimit(Redis Lua) -> handler.Login.bind
    -> business.Login (dao lookup + timing-equalized bcrypt compare) -> token.Issue
    -> response.OK {access_token, user}

GET /api/v1/me:
  RequestID -> AccessLog -> Recovery -> middleware.Auth (verify Bearer + set claims)
    -> handler.Me (CurrentClaims) -> response.OK {id, username}
<- AccessLog logs status + request_id (+ user_id when authenticated)

Unknown route / wrong method:
  RequestID -> AccessLog -> Recovery -> NoRoute/NoMethod
    -> response.Fail (40400 / 40500)
```

## 7. Roadmap

- **M0 (done):** runnable skeleton: health, bcrypt login, JWT middleware, zap, config validation, rate limiting, unified 404/405.
- **M1 (done):** real MySQL DAO, user table migration, startup ping, pool limits, and shutdown close.
- **M2 (done):** Redis client lifecycle and shared atomic login rate limiting.
- **M3:** refresh-token issue/rotate/logout.
- **M4:** tracing/metrics, validation error i18n.
- **M5:** turn it into a code generator / `gonew` template.
