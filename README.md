# gin-scaffold

A small, reusable Gin engineering skeleton with clean layering, graceful
startup/shutdown, structured logging (zap), and JWT auth middleware. Clone it
as the starting point for new Gin services and extend the MySQL/GORM and Redis
layers for your own storage.

## What it includes

- Graceful startup/shutdown: signal-based shutdown with a bounded shutdown window.
- Logging: per-module zap loggers, daily rotation, and `request_id` correlation.
- JWT: login endpoint, Bearer auth middleware, and a protected `/api/v1/me`.
- Unified response envelope + typed `AppError` with business error codes.
- `X-Forwarded-For` is never trusted; `ClientIP` uses the direct socket address.
- Interface-based data access: `internal/dao` contract + MySQL/GORM implementation.
- Config files with an explicit `server.env: dev|test|prod` setting.
- MySQL client plus optional Redis-backed cache with connection pools, startup ping, and graceful close.

## Layout

```
main.go        entrypoint: load config -> init logger -> svc -> router -> serve
etc/config_<env>.yaml     runtime config; env field selects the environment
internal/
  config/                 config structs + Load() + defaults (viper: defaults<file<env)
  svc/                    ServiceContext: dependency-injection container
  router/                 gin engine + global middleware + explicit routes
  handler/                health, auth login, me (thin HTTP adapters)
  business/                  auth login (business orchestration)
  dao/                    DAO registry + interfaces + GORM implementations
  cache/                  application cache wrapper around the optional Redis client
  errors/                 application errors, business codes, and domain sentinels
  schema/                 request/response DTOs
common/                   public packages shared across services
  logger/                 per-module zap loggers + daily rotation + gorm adapter + request_id context
  middleware/             requestid, accesslog, recovery, auth (JWT)
  model/                  storage-side structs (db tags)
  response.go             unified response envelope and writers
  jwt.go                  JWT issue/parse (HS256 + custom claims)
  pagination.go           page/size + id cursor normalization
docs/ARCHITECTURE.md      design rationale
docs/MODULES.zh-CN.md     module-by-module review guide (Chinese)
migrations/               SQL schema migrations
```

## Layer rule of thumb

| layer | answers | must not |
|-------|---------|----------|
| handler | "how does HTTP enter?" | hold business branches |
| business | "when & why is each step done?" | touch HTTP or raw SQL |
| dao | "how is data stored/queried?" | parse HTTP / decide policy |
| middleware | "what applies across routes?" | hold business rules |
| svc | "how are deps assembled?" | hold business logic |
| common/* | "what generic thing is reused?" | import internal/* feature modules |

## Run

MySQL is a startup dependency. Redis is optional: the service still starts and
logs a warning when Redis is unavailable. Create the schema before starting the
server:

```bash
mysql -u app -p gin_scaffold < migrations/001_create_users.sql
```

Then start the service:

```bash
go run .                           # defaults to etc/config_dev.yaml
go run . -f etc/config_test.yaml
go run . -f etc/config_prod.yaml   # requires AUTH_JWTSECRET (>=32 bytes)
```

## Config file & gin mode

The service has one CLI flag: `-f` selects the config file and defaults to
`etc/config_dev.yaml`. The file's `server.env` field determines its runtime
environment:

| `server.env` | gin mode | intended use |
|--------------|----------|--------------|
| `dev` | `debug` | local development |
| `test` | `test` | automated or staging tests |
| `prod` | `release` | production |

`server.env` is validated at startup and also enables stricter production checks.

## MySQL & Redis config

Each config file ships `mysql` and `redis` sections. Values follow
the same precedence and can be overridden by env vars, e.g. `MYSQL_HOST`,
`MYSQL_PASSWORD`, `REDIS_HOST`, `REDIS_PASSWORD`. `internal/svc` opens MySQL
with `gorm.io/driver/mysql`, applies the configured pool limits, pings the
server before serving traffic, and closes the pool during shutdown. A failed
connection is a startup error rather than a deferred request failure.

Redis is opened with `go-redis`, configured with the pool and timeout values in
YAML, and pinged during startup. If the ping fails, startup continues with
an unavailable `ServiceContext.Cache`; no route currently uses the cache, and
the cache layer remains available for future shared state such as sessions or
counters.

`config.MySQLConf.DSN()` uses `parseTime=true`, local timezone interpretation,
a 5s dial timeout, and 30s read/write timeouts. `config.RedisConf.Addr()` is
used by the shared Redis client.

## Logging

`log.rootDir` is the only log path setting. `log.dailyRotate` defaults to
`true`; the service then creates one zap logger per module and writes one file
per module per day in the configured timezone:

```
logs/2026-10/2026-10-29_app.log
logs/2026-10/2026-10-29_gorm.log
logs/2026-10/2026-10-29_access.log
```

- `app`: internal application logs, including startup/shutdown, server errors, panic recovery, unhandled 5xx, and business events such as login success/rejection.
- `gorm`: SQL, rows affected, elapsed time, slow queries, and query failures.
- `access`: one structured line per HTTP request plus Gin framework debug/warning/error output.

With `log.dailyRotate: false`, date switching is disabled and the modules use
fixed files instead:

```
logs/app.log
logs/gorm.log
logs/access.log
```

`log.level` filters all modules. `log.format` selects `json` or `console` for
the file encoder. `log.timezone` controls the day boundary. `log.maxSizeMB`
splits an oversized file into numbered files such as `2026-10-29_app.1.log`
(daily mode) or `app.1.log` (fixed mode). `log.maxAgeDays` removes files older
than the configured number of days, and `log.compress` gzips completed files
from earlier days. Daily mode uses the date in the file name for these checks;
fixed mode uses the file modification time and never compresses or removes the
active file. Consequently, fixed mode requires `maxSizeMB > 0` when retention
or compression is enabled. Set `maxSizeMB` or `maxAgeDays` to `0` to disable
that policy.

The request-id middleware stores the id in the request `context.Context`, and
every module logger automatically adds `request_id`, so one request can be
followed across application, SQL, and access logs.

The MySQL connection is created in `internal/svc/database.go`. It wires the
dedicated GORM module logger like this:

```go
db, err := gorm.Open(mysql.Open(cfg.MySQL.DSN()), &gorm.Config{
    Logger: logger.NewGormLoggerWithOptions(logger.GormLoggerOptions{
        SlowThreshold: 200 * time.Millisecond,
    }),
})
```

Pass the request context with `db.WithContext(ctx)` so SQL logs carry the same
`request_id`. `NewGormLogger` uses GORM's parameter filter by default, so
`gorm.log` keeps SQL placeholders and omits parameter values. Use
`NewGormLoggerWithOptions(logger.GormLoggerOptions{LogSQLParameters: true})`
only when the SQL parameters are safe to retain; `LogRecordNotFound` can be
enabled separately for record-not-found diagnostics.

## Auth demo

The GORM DAO does not seed users. Generate a bcrypt hash for a development
password and insert it explicitly, for example:

```bash
cat >/tmp/hashpw.go <<'EOF'
package main

import (
    "fmt"
    "golang.org/x/crypto/bcrypt"
)

func main() {
    hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
    fmt.Println(string(hash))
}
EOF
go run /tmp/hashpw.go
```

```sql
INSERT INTO users (username, password_hash)
VALUES ('admin', '<bcrypt-hash-from-the-command-above>');
```

Then call the API:

```bash
curl -s http://127.0.0.1:8080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'

curl -s http://127.0.0.1:8080/api/v1/me \
  -H "Authorization: Bearer <access_token>"
```

The `users` table is defined in `migrations/001_create_users.sql`; the service
does not run `AutoMigrate`. The `jwtSecret` is empty in prod and must be
supplied via `AUTH_JWTSECRET`; the server refuses to start without it. In prod,
`mysql.password` and `redis.password` are also required and should be injected
via `MYSQL_PASSWORD` and `REDIS_PASSWORD`.

## Security defaults

- `X-Forwarded-For` is never trusted; `ClientIP` always uses the direct socket
  address.
- 404 and 405 responses use the same JSON envelope and business-code namespace
  as other endpoints.

## Response contract

Every business JSON endpoint returns `{code,msg,data,request_id,timestamp}`.
`code=0` means success; business codes are independent of HTTP status (e.g.
`40100` for unauthorized, `40400` for not found, and `50000` for internal
errors). `GET
/healthz` is the deliberate exception: it returns a minimal `{status,time}` body
for infrastructure probes.
