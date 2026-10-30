#!/usr/bin/env bash
#
# Initialize a new project from this scaffold.
#
# Usage:
#   ./scripts/init-project.sh <target-dir> [module-path] [service-name] [db-name]
#
# Defaults:
#   module-path   basename of <target-dir>
#   service-name  basename of module-path
#   db-name       service-name with '.' and '-' replaced by '_', lowercased
#
# The script copies tracked files only, rewrites Go import paths and project
# names, then optionally initializes a fresh Git repository.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
GENERATOR_REL="scripts/init-project.sh"

log_info() { printf '\033[0;32m[INFO]\033[0m  %s\n' "$*"; }
log_warn() { printf '\033[0;33m[WARN]\033[0m  %s\n' "$*"; }
log_ok()   { printf '\033[0;32m[ OK ]\033[0m  %s\n' "$*"; }
log_err()  { printf '\033[0;31m[ERROR]\033[0m %s\n' "$*" >&2; }

die() {
  log_err "$*"
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  ./scripts/init-project.sh [--no-git] <target-dir> [module-path] [service-name] [db-name]

Examples:
  ./scripts/init-project.sh ../order-service
  ./scripts/init-project.sh ../order-service github.com/acme/order-service order-service order_service

Options:
  --no-git   Do not initialize a Git repository in the target directory.
  -h,--help  Show this help.

The target directory must not exist or must be empty. Only tracked files are
copied; generated binaries, runtime logs, and local config overrides are
excluded by the scaffold's .gitignore rules.
EOF
}

GIT_INIT=1
ARGS=()
for arg in "$@"; do
  case "$arg" in
    --no-git) GIT_INIT=0 ;;
    -h|--help) usage; exit 0 ;;
    *) ARGS+=("$arg") ;;
  esac
done
set -- "${ARGS[@]}"

if [[ $# -lt 1 || $# -gt 4 ]]; then
  usage
  exit 2
fi

command -v git >/dev/null 2>&1 || die "git is required"
command -v perl >/dev/null 2>&1 || die "perl is required for text rewriting"

TARGET_INPUT=$1
TARGET_PARENT="$(cd "$(dirname "$TARGET_INPUT")" 2>/dev/null && pwd)" || die "target parent directory does not exist: $(dirname "$TARGET_INPUT")"
TARGET="$TARGET_PARENT/$(basename "$TARGET_INPUT")"

case "$TARGET/" in
  "$ROOT_DIR/"*) die "target must be outside the scaffold repository: $TARGET" ;;
esac

if [[ -e "$TARGET" ]]; then
  [[ -d "$TARGET" ]] || die "target exists and is not a directory: $TARGET"
  if [[ -n "$(find "$TARGET" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
    die "target directory is not empty: $TARGET"
  fi
fi

MODULE_PATH=${2:-}
SERVICE_NAME=${3:-}
DB_NAME=${4:-}
MODULE_PATH=${MODULE_PATH%/}

if [[ -z "$MODULE_PATH" ]]; then
  MODULE_PATH="$(basename "$TARGET")"
fi
if [[ -z "$SERVICE_NAME" ]]; then
  SERVICE_NAME="$(basename "$MODULE_PATH")"
fi
if [[ -z "$DB_NAME" ]]; then
  DB_NAME="$(printf '%s' "$SERVICE_NAME" | tr '.-' '__' | tr '[:upper:]' '[:lower:]')"
fi

[[ "$MODULE_PATH" =~ ^[A-Za-z0-9][A-Za-z0-9._~/-]*$ ]] || die "invalid module path: $MODULE_PATH"
[[ "$MODULE_PATH" != *'//'* ]] || die "invalid module path: $MODULE_PATH"
[[ "$SERVICE_NAME" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || die "invalid service name: $SERVICE_NAME"
[[ "$DB_NAME" =~ ^[A-Za-z0-9_]+$ ]] || die "invalid database name: $DB_NAME"

if [[ -n "$(git -C "$ROOT_DIR" status --porcelain --untracked-files=no)" ]]; then
  log_warn "working tree has uncommitted tracked changes; their current contents will be copied"
fi

log_info "creating $TARGET"
mkdir -p "$TARGET"

while IFS= read -r -d '' rel; do
  [[ "$rel" == "$GENERATOR_REL" ]] && continue
  mkdir -p "$TARGET/$(dirname "$rel")"
  cp -p "$ROOT_DIR/$rel" "$TARGET/$rel"
done < <(git -C "$ROOT_DIR" ls-files -z --cached)

[[ -f "$TARGET/go.mod" ]] || die "copy failed: go.mod is missing"

replace_literal() {
  local old=$1 new=$2 file=$3
  OLD="$old" NEW="$new" perl -pi -e 's/\Q$ENV{OLD}\E/$ENV{NEW}/g' "$file"
}

is_text_file() {
  LC_ALL=C grep -Iq . "$1" 2>/dev/null
}

log_info "rewriting module=$MODULE_PATH service=$SERVICE_NAME db=$DB_NAME"

while IFS= read -r -d '' file; do
  replace_literal '"gin-scaffold"' "\"$SERVICE_NAME\"" "$file"
  replace_literal 'gin-scaffold' "$MODULE_PATH" "$file"
  replace_literal 'ginctl' "$SERVICE_NAME" "$file"
  replace_literal 'gin_scaffold' "$DB_NAME" "$file"
done < <(find "$TARGET" -type f -name '*.go' -print0)

replace_literal 'gin-scaffold' "$MODULE_PATH" "$TARGET/go.mod"

while IFS= read -r -d '' file; do
  is_text_file "$file" || continue
  replace_literal 'gin-scaffold' "$SERVICE_NAME" "$file"
  replace_literal 'ginctl' "$SERVICE_NAME" "$file"
  replace_literal 'gin_scaffold' "$DB_NAME" "$file"
done < <(find "$TARGET" -type f ! -name '*.go' ! -name 'go.mod' ! -name 'go.sum' -print0)

if [[ "$GIT_INIT" -eq 1 ]]; then
  git init -q "$TARGET"
  git -C "$TARGET" add -A
  log_ok "initialized Git repository"
fi

if command -v go >/dev/null 2>&1; then
  if (cd "$TARGET" && go build ./...); then
    log_ok "go build ./... passed"
  else
    log_warn "go build ./... failed; inspect the generated project before using it"
  fi
fi

cat <<EOF

Project created: $TARGET
  module:   $MODULE_PATH
  service:  $SERVICE_NAME
  database: $DB_NAME

Next steps:
  cd "$TARGET"
  edit etc/config_dev.yaml, migrations/, and README.md
  go run .
EOF

if [[ "$GIT_INIT" -eq 1 ]]; then
  printf '  git commit -m "chore: initialize project"\n'
fi
