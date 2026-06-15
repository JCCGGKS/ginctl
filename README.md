# gin-scaffold

A small, **reusable gin engineering skeleton** distilled from the layered
architecture of the `mysurl1` go-zero service. It is *not* the short-link
business — it is a clean starting point for new gin projects.

The program entrypoint is [`main.go`](./main.go). It boots with **zero external
dependencies** (in-memory demo DAO), so you can `go run` it immediately and
replace pieces as you grow.

## Layout

```
main.go        entrypoint: load config -> svc -> router -> graceful shutdown
etc/config_<env>.yaml     per-environment runtime config (dev|test|prod)
internal/
  config/                 config structs + Load() + defaults (viper: defaults<file<env)
  svc/                    ServiceContext: dependency-injection container
  router/                 routes + per-group middleware wiring (hand-written routes.go)
  middleware/             requestid, accesslog, recovery, auth (JWT), oplog (audit)
  handler/                health, auth, resource  (thin HTTP adapters)
  logic/                  auth, resource          (business orchestration)
  dao/                    resource DAO interface + in-memory impl
  model/                  storage structs (db tags)
  schema/                 request/response DTOs (json/form/binding tags)
pkg/
  response/               unified envelope {code,msg,data,request_id,timestamp} + AppError
  token/                  JWT issue/parse (HS256 + custom claims)
  pagination/             page/size normalization
docs/ARCHITECTURE.md      analysis of mysurl1 + design rationale
```

## Layer rule of thumb

| layer | answers | must not |
|-------|---------|----------|
| handler | "how does HTTP enter?" | hold business branches |
| logic | "when & why is each step done?" | touch HTTP or raw SQL |
| dao | "how is data stored/queried?" | parse HTTP / decide policy |
| middleware | "what applies across routes?" | hold business rules |
| svc | "how are deps assembled?" | hold business logic |
| pkg/* | "what generic thing is reused?" | import internal/* |

Every handler is the same four steps: **bind → call logic → on error `response.Fail` → else `response.OK`**.

## Run

```bash
# defaults to the dev environment -> etc/config_dev.yaml
go run .

# pick an environment -> loads etc/config_<env>.yaml
go run . -env prod
ENV=test go run .

# build + run, or point at an explicit file (overrides -env)
go build -o bin/server .
./bin/server -env prod
go run . -f etc/config_dev.yaml
```

The dev config listens on `0.0.0.0:8080` (see `etc/config_dev.yaml`).

### Environment selection

The active environment selects the file `etc/config_<env>.yaml`. Resolution
precedence is **`-f` (explicit file) > `-env` flag > `ENV` > `dev`**:

| flags / env             | file loaded                 |
|-------------------------|-----------------------------|
| (none)                  | `etc/config_dev.yaml`       |
| `-env prod`             | `etc/config_prod.yaml`      |
| `ENV=test`              | `etc/config_test.yaml`      |
| `-c /etc/app -env prod` | `/etc/app/config_prod.yaml` |
| `-f path/to.yaml`       | `path/to.yaml`              |

### Config precedence (viper)

Configuration loads as **defaults < YAML file < environment**. Any key can be
overridden by an env var with `.` replaced by `_` (so `server.port` ->
`SERVER_PORT`):

```bash
# override server.port without editing the file
SERVER_PORT=9090 go run . -env dev

# override auth.jwtSecret and log.level on top of the prod file
AUTH_JWTSECRET='real-secret' LOG_LEVEL=debug ./bin/server -env prod
```

Opt-in hot-reload is available via `config.Watch(path, onChange)`.

## Try the endpoints

```bash
# health (plain body, intentionally outside the unified envelope)
curl -s http://127.0.0.1:8080/healthz

# protected route without a token -> 401 unified error
curl -s http://127.0.0.1:8080/api/v1/resources

# login (demo creds admin/admin123) -> returns an access_token
curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'

# call a protected, paginated, filterable endpoint
TOKEN="<access_token from login>"
curl -s "http://127.0.0.1:8080/api/v1/resources?page=1&page_size=3&keyword=resource-1" \
  -H "Authorization: Bearer $TOKEN"

# single resource (404 demo: id 999)
curl -s http://127.0.0.1:8080/api/v1/resources/2 -H "Authorization: Bearer $TOKEN"
```

Every JSON response shares the same shape:

```json
{ "code": 0, "msg": "ok", "data": { }, "request_id": "…", "timestamp": 1700000000 }
```

## How to extend

**Add an endpoint** (e.g. `POST /api/v1/resources`):
1. `schema/resource.go`: add the request/response DTOs.
2. `logic/resource.go`: add a method that validates + calls the DAO.
3. `handler/resource.go`: add a 4-step handler.
4. `router/router.go`: register the route (and any middleware) in the right group.

**Swap the in-memory store for MySQL:**
- Keep the `dao.ResourceDAO` interface; add a `sqlResourceDAO` implementation
  (database/sql, jmoiron/sqlx, or gorm) and construct it in `svc`.
- Logic and handler do not change — that is the point of the interface.

**Persist audit logs:** implement `middleware.OperationSink` against a DB and
pass it into `middleware.OperationLog("action", sink)`.

**Rename the module** for your project:
```bash
go mod edit -module github.com/you/yourapp
grep -rl 'gin-scaffold/' . | xargs sed -i 's#gin-scaffold/#github.com/you/yourapp/#g'
```

## Capabilities included

- `go run`-able gin server with graceful shutdown (SIGINT/SIGTERM)
- viper config: YAML file + defaults + env overrides + `-f` flag + opt-in hot-reload (`config.Watch`)
- ServiceContext dependency injection
- Unified response envelope + typed `AppError` → HTTP status mapping
- JWT issue/verify + auth middleware (claims on context)
- Global middleware: request id, structured access log (`log/slog`), panic recovery
- Operation-log middleware (reads outcome from context — no fragile body capture)
- Pagination normalization
- Interface-based DAO with an in-memory demo
- Demo endpoints: health, login, resource list/get

## Intentionally NOT included (to become a real scaffold)

- Real DB/Redis wiring, migrations, a user table + bcrypt login
- Refresh-token issue/rotate/logout
- Request validation with friendly i18n error messages
- Tracing/metrics, rate limiting, CORS
- A code generator / `gonew` template to stamp out new modules

## Notes

- Module path is `gin-scaffold` while the folder is `mysurl-gin`; rename as above when forking.
- `gin v1.12` requires Go ≥ 1.25, so `go mod tidy` set `go 1.25.0` and the toolchain auto-upgraded.
