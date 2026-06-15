# Architecture analysis: mysurl1 (go-zero) → gin skeleton

This document records the Phase-1 analysis that motivated this skeleton. The
source of study is the `mysurl1` go-zero short-link service; the goal was to
**extract reusable engineering ideas**, not to port the business.

## 1. Ideas worth keeping

- **Documented layer boundaries.** `AGENTS.md` pins each layer to one question
  (config = what is configurable, schema = what the API exchanges, model = one
  row, dao = how data is stored, logic = when/why, handler = how HTTP enters,
  middleware = cross-cutting, svc = how deps assemble, utils = reused generics).
- **Thin handlers.** Fixed shape: parse → new logic → call → unified write.
- **Logic constructor pattern** `(ctx, svcCtx)`; methods orchestrate
  validate → cache → dao → assemble.
- **Pure DAOs.** Raw SQL, soft delete, dynamic query builder, cursor pagination
  + count, transactions.
- **Unified response + typed error** (`HTTPError` mapped via `errors.As`).
- **ServiceContext as a DI container** (constructed once).
- **Auth claims passed via context**, injected by middleware, read in logic.
- **Cross-cutting via middleware**: JWT auth, operation log.
- **Strategy + registry** for pluggable short-code generators.
- **Background worker with interface segregation** (`VisitCountCache`).
- **Graceful degradation** when DB/Redis/Bloom are unavailable.

## 2. go-zero coupling points (and gin replacements)

| go-zero | role | gin replacement |
|---------|------|-----------------|
| `rest.Server`, `rest.Route`, `WithMiddlewares` | routing + server lifecycle | `gin.Engine` / groups |
| `config` embeds `rest.RestConf` | server/log/trace config | custom `ServerConf` + `http.Server` |
| `conf.MustLoad` | YAML + tag defaults | `spf13/viper` (per-env `config_<env>.yaml` + `SetDefault` + env vars + opt-in watch) |
| `httpx.Parse` / `httpx.WriteJsonCtx` | bind / write | `c.ShouldBind*` / `c.JSON` |
| `sqlx.SqlConn` | data access | `database/sql` / `sqlx` / `gorm` |
| `logx` | logging | `log/slog` |
| `stat`, `syncx.SingleFlight` | metrics / dedup | business-specific, add as needed |

Key insight: handlers and middleware use **standard `net/http` signatures**
(go-zero `Middleware` is just `func(http.HandlerFunc) http.HandlerFunc`), so the
middleware *ideas* port almost verbatim. Only routing, binding, config loading,
sqlx and logx are truly go-zero-bound.

## 3. Migrated into this skeleton

Layering & boundaries · unified response + typed error · logic constructor
pattern · ServiceContext DI · JWT auth + claims-in-context · pagination
normalization · operation-log middleware (de-fragilized) · viper config (per-env file + defaults + env overrides) ·
graceful shutdown.

## 4. Deliberately left in the business project

Short-code strategies (snowflake/base62/redis-incr/auto-increment) · URL
normalize/hash · Bloom reuse check · singleflight origin-merge · visit-count
Redis aggregation + async flush · the concrete oplog route→action table · the
`ExtData` "parasitic response body" coupling (replaced here by explicit context
values) · concrete SQL/schema.

## 5. Design principles of the skeleton

Few dependencies (gin + jwt + viper) · thin handler / thick logic / pure dao /
cross-cutting middleware · interface-based DAO · one response contract · deps
centralized in svc · observability (request id + structured access log + panic
guard) · externalized config with defaults + env overrides · graceful shutdown · runs with zero
external dependencies.

## 6. Request flow (`GET /healthz`)

```
RequestID -> AccessLog -> Recovery -> Auth(group) -> Handler.bind
  -> Logic.List: pagination.Normalize -> ResourceDAO.Count/List -> assemble DTO
  -> response.OK (writes envelope + stores biz outcome on context)
<- AccessLog logs status + request_id
(OperationLog, when attached, reads biz code + claims from context after Next)
```

## 7. Roadmap

- **M0 (done):** runnable skeleton, health only, no deps.
- **M1:** real MySQL DAO, user table, bcrypt login.
- **M2:** refresh-token issue/rotate/logout; DB-backed audit sink.
- **M3:** tracing/metrics, validation error i18n; config hot-reload done (opt-in `config.Watch`).
- **M4:** turn it into a code generator / `gonew` template.
