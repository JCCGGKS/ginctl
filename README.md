# gin-scaffold

A small, reusable Gin engineering skeleton with clean layering, graceful
startup/shutdown, structured logging (zap), and JWT auth middleware. Clone it
as the starting point for new Gin services and swap the in-memory DAO for your
own storage.

## What it includes

- Graceful startup/shutdown: signal-based shutdown plus `http.Server` timeouts.
- Logging: `go.uber.org/zap` configured from `log.level` and `log.format`.
- JWT: login endpoint, Bearer auth middleware, and a protected `/api/v1/me`.
- Unified response envelope + typed `AppError` with business error codes.
- Login rate limiting, request-body limits, and proxy-aware client IP handling.
- Interface-based data access: `internal/dao` contract + in-memory demo store.
- Per-environment config files: `etc/config_<env>.yaml`.
- MySQL & Redis config stubs: pool settings + `DSN()` / `Addr()` helpers.

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
  dao/                    UserDAO interface + memory/ implementation
  model/                  storage-side structs (db tags)
  schema/                 request/response DTOs
infra/
  logger/                 zap level/format setup
  response/               unified envelope + AppError + business codes
  token/                  JWT issue/parse (HS256 + custom claims)
  pagination/             page/size + id cursor normalization
docs/ARCHITECTURE.md      design rationale
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
`MYSQL_PASSWORD`, `REDIS_HOST`, `REDIS_PASSWORD`. `config.MySQLConf.DSN()`
returns a go-sql-driver DSN and `config.RedisConf.Addr()` returns `host:port`
for wiring real clients in `internal/svc`.

## Auth demo

```bash
curl -s http://127.0.0.1:8080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'

curl -s http://127.0.0.1:8080/api/v1/me \
  -H "Authorization: Bearer <access_token>"
```

The demo user is seeded by the in-memory DAO with a bcrypt hash
(`admin` / `admin123`). Replace `internal/dao/memory` with a real `UserDAO`
(MySQL/gorm) and it plugs into `internal/svc` without touching business. The
`jwtSecret` is empty in prod and must be supplied via `AUTH_JWTSECRET`; the
server refuses to start without it. Prod does not seed the demo user and uses
an empty in-memory store until a real DAO is wired, so login returns 401 there
by design.

## Security defaults

- `auth.loginRateLimitPerMinute` limits login attempts per client IP (default 10;
  in-process, so use Redis/shared storage across multiple instances).
- `server.maxBodyBytes` caps request bodies (default 1 MiB).
- `server.trustedProxies` is empty by default, which disables `X-Forwarded-For`
  trust. Set explicit CIDRs only for a known load balancer.
- 404 and 405 responses use the same JSON envelope and business-code namespace
  as other endpoints.

## Response contract

Every business JSON endpoint returns `{code,msg,data,request_id,timestamp}`.
`code=0` means success; business codes are independent of HTTP status (e.g.
`40100` for unauthorized, `40400` for not found, `42900` for rate limiting,
`50000` for internal errors). `GET /healthz` is the deliberate exception: it
returns a minimal `{status,time}` body for infrastructure probes.
