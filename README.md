# gin-scaffold

A small, reusable Gin engineering skeleton with clean layering, graceful
startup/shutdown, structured logging (zap), and JWT auth middleware. Clone it
as the starting point for new Gin services and extend the MySQL/GORM and Redis
layers for your own storage.

## What it includes

- Graceful startup/shutdown: signal-based shutdown plus `http.Server` timeouts.
- Logging: per-module zap loggers, daily rotation, and `request_id` correlation.
- JWT: login endpoint, Bearer auth middleware, and a protected `/api/v1/me`.
- Unified response envelope + typed `AppError` with business error codes.
- Login rate limiting, request-body limits, and proxy-aware client IP handling.
- Interface-based data access: `internal/dao` contract + MySQL/GORM implementation.
- Per-environment config files: `etc/config_<env>.yaml`.
- MySQL and Redis clients with connection pools, startup ping, and graceful close.

## Layout

```
main.go        entrypoint: load config -> init logger -> svc -> router -> serve
etc/config_<env>.yaml     runtime config for dev|test|prod
internal/
  config/                 config structs + Load() + defaults (viper: defaults<file<env)
  svc/                    ServiceContext: dependency-injection container
  router/                 gin engine + global middleware + explicit routes
  middleware/             requestid, bodylimit, accesslog, recovery, ratelimit, auth (JWT)
  handler/                health, auth login, me (thin HTTP adapters)
  business/                  auth login (business orchestration)
  dao/                    UserDAO interface + gorm/ implementation
  model/                  storage-side structs (db tags)
  schema/                 request/response DTOs
infra/
  logger/                 per-module zap loggers + daily rotation + gorm adapter
  requestid/              request id context carrier
  response/               unified envelope + AppError + business codes
  ratelimit/              shared limiter interface + Redis/memory backends
  token/                  JWT issue/parse (HS256 + custom claims)
  pagination/             page/size + id cursor normalization
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
| infra/* | "what generic thing is reused?" | import internal/* |

## Run

MySQL and Redis are startup dependencies. Create the schema before starting the
server:

```bash
mysql -u app -p gin_scaffold < migrations/001_create_users.sql
```

Then start the service:

```bash
go run .                  # defaults to dev -> etc/config_dev.yaml
go run . -env prod        # requires AUTH_JWTSECRET (>=32 bytes)
ENV=test go run .         # etc/config_test.yaml
go run . -f etc/config_dev.yaml
```

## Environment selection & gin mode

| flags / env | file loaded | gin mode | log format |
|-------------|-------------|----------|------------|
| (none) | `etc/config_dev.yaml` | `debug` | `console` |
| `-env test` / `ENV=test` | `etc/config_test.yaml` | `test` | `console` |
| `-env prod` / `ENV=prod` | `etc/config_prod.yaml` | `release` | `json` |

Resolution precedence is `-f` (explicit file) > `-env` flag > `ENV` > `dev`.

## MySQL & Redis config

Each `etc/config_<env>.yaml` ships `mysql` and `redis` sections. Values follow
the same precedence and can be overridden by env vars, e.g. `MYSQL_HOST`,
`MYSQL_PASSWORD`, `REDIS_HOST`, `REDIS_PASSWORD`. `internal/svc` opens MySQL
with `gorm.io/driver/mysql`, applies the configured pool limits, pings the
server before serving traffic, and closes the pool during shutdown. A failed
connection is a startup error rather than a deferred request failure.

Redis is opened with `go-redis`, configured with the pool and timeout values in
YAML, and pinged before startup. Login rate limits use an atomic fixed-window
Lua script under `ratelimit:login:<client-ip>`; counters are shared across
instances. If Redis fails during a request, the login endpoint fails closed with
`503` rather than silently disabling brute-force protection.

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

- `auth.loginRateLimitPerMinute` limits login attempts per client IP (default 10)
  with shared Redis counters, so multiple instances enforce one limit.
- `server.maxBodyBytes` caps request bodies (default 1 MiB).
- `server.trustedProxies` is empty by default, which disables `X-Forwarded-For`
  trust. Set explicit CIDRs only for a known load balancer.
- 404 and 405 responses use the same JSON envelope and business-code namespace
  as other endpoints.

## Response contract

Every business JSON endpoint returns `{code,msg,data,request_id,timestamp}`.
`code=0` means success; business codes are independent of HTTP status (e.g.
`40100` for unauthorized, `40400` for not found, `42900` for rate limiting,
`50000` for internal errors, `50300` for unavailable dependencies). `GET
/healthz` is the deliberate exception: it returns a minimal `{status,time}` body
for infrastructure probes.
