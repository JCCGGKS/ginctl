#!/usr/bin/env bash
#
# ginctl 服务启停脚本
#
# 用法:
#   ./start.sh                         默认构建并启动 dev 环境
#   ./start.sh [start|stop|restart|status|build] [dev|test|prod]
#   ./start.sh [dev|test|prod]         等价于 start <env>
#
# 设计要点:
#   - 构建产物落在 scripts/bin，pid 和运行元数据落在 scripts/run
#   - 配置文件固定为 etc/config_<env>.yaml，服务端口的 SERVER_PORT 环境变量可覆盖配置
#   - 日志由服务自身按 log.rootDir 配置落盘，启动脚本不额外捕获/记录日志
#   - 启动后轮询进程存活和端口监听，确认服务是否真正启动成功
#   - 停止时优先清理 pidfile 中的进程，并清理占用端口的同二进制孤儿进程

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR" || exit 1

BIN_DIR="$SCRIPT_DIR/bin"
RUN_DIR="$SCRIPT_DIR/run"
SERVICE_NAME="ginctl"
SERVICE_BIN="$BIN_DIR/$SERVICE_NAME"
PID_FILE="$RUN_DIR/$SERVICE_NAME.pid"
META_FILE="$RUN_DIR/$SERVICE_NAME.meta"
START_TIMEOUT_SECONDS=30

DEFAULT_ENV="dev"
ALL_ENVS=(dev test prod)

mkdir -p "$BIN_DIR" "$RUN_DIR"

log_info() { printf '\033[0;32m[INFO]\033[0m  %s\n' "$*"; }
log_warn() { printf '\033[0;33m[WARN]\033[0m  %s\n' "$*"; }
log_err()  { printf '\033[0;31m[ERROR]\033[0m %s\n' "$*" >&2; }
log_ok()   { printf '\033[0;32m[ OK ]\033[0m  %s\n' "$*"; }
log_fail() { printf '\033[0;31m[FAIL]\033[0m  %s\n' "$*" >&2; }

meta_value() {
  local key=$1 meta_key value
  [[ -f "$META_FILE" ]] || return 1
  while IFS='=' read -r meta_key value; do
    if [[ "$meta_key" == "$key" ]]; then
      printf '%s\n' "$value"
      return 0
    fi
  done < "$META_FILE"
  return 1
}

is_valid_env() {
  local candidate=$1 env
  for env in "${ALL_ENVS[@]}"; do
    [[ "$candidate" == "$env" ]] && return 0
  done
  return 1
}

config_for_env() {
  local env=$1
  is_valid_env "$env" || return 1
  printf '%s/etc/config_%s.yaml\n' "$ROOT_DIR" "$env"
}

read_server_port() {
  local config_file=$1
  awk '
    /^server:[[:space:]]*$/ { in_server = 1; next }
    in_server && /^[^[:space:]]/ { in_server = 0 }
    in_server && $1 == "port:" {
      value = $2
      sub(/#.*/, "", value)
      gsub(/[[:space:]"]/, "", value)
      print value
      exit
    }
  ' "$config_file"
}

resolve_port() {
  local config_file=$1 port

  if [[ -n "${SERVER_PORT:-}" ]]; then
    if [[ ! "$SERVER_PORT" =~ ^[0-9]+$ ]]; then
      log_err "SERVER_PORT 不是合法端口: $SERVER_PORT"
      return 1
    fi
    printf '%s\n' "$SERVER_PORT"
    return 0
  fi

  port=$(read_server_port "$config_file")
  if [[ ! "$port" =~ ^[0-9]+$ ]]; then
    log_err "无法从配置解析 server.port: $config_file"
    return 1
  fi
  printf '%s\n' "$port"
}

resolve_env() {
  local env="${TARGET_ENV:-}"

  if [[ -n "$env" ]]; then
    is_valid_env "$env" || return 1
    printf '%s\n' "$env"
    return 0
  fi

  case "$ACTION" in
    start|build)
      env="$DEFAULT_ENV"
      ;;
    *)
      env=$(meta_value env 2>/dev/null || true)
      env="${env:-$DEFAULT_ENV}"
      ;;
  esac

  is_valid_env "$env" || return 1
  printf '%s\n' "$env"
}

pids_on_port() {
  local port=$1 pids=""

  if command -v lsof >/dev/null 2>&1; then
    pids=$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null)
  fi
  if [[ -z "$pids" ]] && command -v ss >/dev/null 2>&1; then
    pids=$(ss -ltnp "sport = :$port" 2>/dev/null | grep -oP 'pid=\K[0-9]+')
  fi
  if [[ -z "$pids" ]] && command -v fuser >/dev/null 2>&1; then
    pids=$(fuser "${port}/tcp" 2>/dev/null | tr -s ' ')
  fi
  printf '%s\n' "$pids"
}

port_listening() {
  local port=$1 pids
  pids=$(pids_on_port "$port")
  [[ -n "$pids" ]]
}

is_our_service_pid() {
  local pid=$1 command
  command=$(ps -p "$pid" -o args= 2>/dev/null) || return 1
  [[ "$command" == *"$SERVICE_BIN"* ]]
}

service_pids_on_port() {
  local port=$1 pid
  for pid in $(pids_on_port "$port"); do
    if is_our_service_pid "$pid"; then
      printf '%s\n' "$pid"
    fi
  done
}

pid_alive() {
  local pid
  [[ -f "$PID_FILE" ]] || return 1
  pid=$(cat "$PID_FILE" 2>/dev/null)
  [[ "$pid" =~ ^[0-9]+$ ]] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  is_our_service_pid "$pid"
}

kill_pid() {
  local pid=$1 attempt
  [[ -n "$pid" ]] || return 0
  kill "$pid" 2>/dev/null
  for attempt in $(seq 1 5); do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 1
  done
  kill -9 "$pid" 2>/dev/null
}

write_meta() {
  local env=$1 config_file=$2 port=$3
  {
    printf 'env=%s\n' "$env"
    printf 'config=%s\n' "$config_file"
    printf 'port=%s\n' "$port"
  } > "$META_FILE"
}

build_service() {
  log_info "构建 $SERVICE_NAME -> $SERVICE_BIN"
  if ! CGO_ENABLED=0 go build -o "$SERVICE_BIN" .; then
    log_fail "$SERVICE_NAME 构建失败"
    return 1
  fi
  log_ok "$SERVICE_NAME 构建成功: $SERVICE_BIN"
}

start_service() {
  local env=$1 config_file=$2 port=$3 pid ready attempt

  if [[ ! -f "$config_file" ]]; then
    log_fail "配置文件不存在: $config_file"
    return 1
  fi

  if pid_alive; then
    log_warn "$SERVICE_NAME 已在运行，跳过 (PID=$(cat "$PID_FILE"), 如需重启请使用 restart)"
    return 0
  fi

  if port_listening "$port"; then
    log_fail "端口 $port 已被占用，无法启动 $SERVICE_NAME"
    return 1
  fi

  if ! build_service; then
    return 1
  fi

  log_info "启动 $SERVICE_NAME (env=$env, 配置=$config_file, 端口=$port) ..."
  nohup "$SERVICE_BIN" -f "$config_file" >/dev/null 2>&1 &
  pid=$!
  printf '%s\n' "$pid" > "$PID_FILE"
  write_meta "$env" "$config_file" "$port"

  ready=0
  for attempt in $(seq 1 "$START_TIMEOUT_SECONDS"); do
    sleep 1
    if ! kill -0 "$pid" 2>/dev/null; then
      log_fail "$SERVICE_NAME 进程已退出"
      rm -f "$PID_FILE" "$META_FILE"
      return 1
    fi
    if port_listening "$port"; then
      ready=1
      break
    fi
  done

  if [[ "$ready" -eq 1 ]]; then
    log_ok "$SERVICE_NAME 启动成功 (PID=$pid, env=$env, 端口=$port)"
    return 0
  fi

  log_fail "$SERVICE_NAME 在 ${START_TIMEOUT_SECONDS}s 内未监听端口 $port"
  kill_pid "$pid"
  rm -f "$PID_FILE" "$META_FILE"
  return 1
}

stop_service() {
  local env=$1 config_file=$2 port=$3 pid orphan_pids

  log_info "停止 $SERVICE_NAME (env=$env, 配置=$config_file, 端口=$port) ..."

  if pid_alive; then
    pid=$(cat "$PID_FILE")
    kill_pid "$pid"
  else
    log_warn "pidfile 不存在或进程已退出: $PID_FILE"
  fi

  orphan_pids=$(service_pids_on_port "$port")
  if [[ -n "$orphan_pids" ]]; then
    log_warn "清理占用端口 $port 的 $SERVICE_NAME 进程: $orphan_pids"
    for pid in $orphan_pids; do
      kill_pid "$pid"
    done
  fi

  rm -f "$PID_FILE" "$META_FILE"

  sleep 1
  if port_listening "$port"; then
    log_fail "端口 $port 仍被其他进程占用，请检查: $(pids_on_port "$port")"
    return 1
  fi

  log_ok "$SERVICE_NAME 已停止"
}

status_service() {
  local env=$1 config_file=$2 port=$3 pid listening="否" orphan_pids

  if pid_alive; then
    pid=$(cat "$PID_FILE")
    port_listening "$port" && listening="是"
    log_ok "$SERVICE_NAME 运行中 (PID=$pid, env=$env, 配置=$config_file, 端口=$port, 监听=$listening)"
    return 0
  fi

  orphan_pids=$(service_pids_on_port "$port")
  if [[ -n "$orphan_pids" ]]; then
    log_warn "$SERVICE_NAME 存在未登记的运行进程 (PID=$orphan_pids, 端口=$port)"
    return 0
  fi

  if port_listening "$port"; then
    log_warn "端口 $port 被其他进程占用 (env=$env, 配置=$config_file)"
    return 0
  fi

  log_warn "$SERVICE_NAME 已停止 (env=$env, 配置=$config_file, 端口=$port)"
}

usage() {
  cat <<EOF
用法:
  $0                              默认构建并启动 dev 环境
  $0 [start|stop|restart|status|build] [dev|test|prod]
  $0 [dev|test|prod]              等价于 start <env>

动作:
  start    构建并启动(默认)
  stop     停止当前环境的服务
  restart  停止后启动
  status   查看运行状态
  build    仅构建二进制(不启动)

环境:
  dev      etc/config_dev.yaml (默认)
  test     etc/config_test.yaml
  prod     etc/config_prod.yaml
EOF
}

ACTION="start"
TARGET_ENV=""

if [[ $# -gt 0 ]]; then
  case "$1" in
    start|stop|restart|status|build)
      ACTION=$1
      shift
      ;;
    -h|--help|help)
      usage
      exit 0
      ;;
    dev|test|prod)
      TARGET_ENV=$1
      shift
      ;;
    *)
      log_err "未知参数: $1"
      usage
      exit 2
      ;;
  esac

  if [[ $# -gt 0 ]]; then
    if [[ -n "$TARGET_ENV" ]]; then
      log_err "参数过多: $*"
      usage
      exit 2
    fi
    TARGET_ENV=$1
    shift
  fi

  if [[ $# -gt 0 ]]; then
    log_err "参数过多: $*"
    usage
    exit 2
  fi
fi

SERVICE_ENV=$(resolve_env) || {
  log_err "环境必须是: ${ALL_ENVS[*]}"
  exit 2
}
SERVICE_CONFIG=$(config_for_env "$SERVICE_ENV")
SERVICE_PORT=""
if [[ "$ACTION" != "build" ]]; then
  SERVICE_PORT=$(resolve_port "$SERVICE_CONFIG") || exit 1
fi

case "$ACTION" in
  start)
    start_service "$SERVICE_ENV" "$SERVICE_CONFIG" "$SERVICE_PORT"
    ;;
  stop)
    stop_service "$SERVICE_ENV" "$SERVICE_CONFIG" "$SERVICE_PORT"
    ;;
  restart)
    stop_service "$SERVICE_ENV" "$SERVICE_CONFIG" "$SERVICE_PORT" || exit 1
    start_service "$SERVICE_ENV" "$SERVICE_CONFIG" "$SERVICE_PORT"
    ;;
  status)
    status_service "$SERVICE_ENV" "$SERVICE_CONFIG" "$SERVICE_PORT"
    ;;
  build)
    build_service
    ;;
esac
