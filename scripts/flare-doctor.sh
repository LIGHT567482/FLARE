#!/usr/bin/env bash
set -u

USER_HOME="${HOME:?HOME must be set}"
DATA_DIR="${FLARE_DATA_DIR:-${USER_HOME}/.flare-data}"
SOCKET_PATH="${FLARED_SOCKET:-${DATA_DIR}/flared.sock}"
failures=0

ok() { printf '[ok] %s\n' "$1"; }
warn() { printf '[warn] %s\n' "$1"; }
fail() { printf '[fail] %s\n' "$1"; failures=$((failures + 1)); }

for tool in go python3 node npm git openssl; do
  if command -v "${tool}" >/dev/null 2>&1; then
    ok "${tool} is installed"
  else
    fail "${tool} is missing"
  fi
done

if [[ -d "${DATA_DIR}" ]]; then
  mode="$(stat -c '%a' "${DATA_DIR}")"
  if (( (8#${mode} & 077) == 0 )); then
    ok "data directory permissions are private (${mode})"
  else
    fail "data directory ${DATA_DIR} is accessible to group/other (mode ${mode}); use chmod 700"
  fi
else
  warn "data directory does not exist yet: ${DATA_DIR}"
fi

if [[ -S "${SOCKET_PATH}" ]]; then
  if python3 - "${SOCKET_PATH}" <<'PY'
import json
import socket
import sys

client = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
client.settimeout(2)
try:
    client.connect(sys.argv[1])
    client.sendall(b'{"cmd":"status"}\n')
    response = json.loads(client.makefile("rb").readline())
    if response.get("engine") != "ok":
        raise RuntimeError(f"unexpected daemon status: {response}")
except (OSError, ValueError, RuntimeError) as error:
    print(error, file=sys.stderr)
    raise SystemExit(1)
finally:
    client.close()
PY
  then
    ok "daemon IPC socket responds"
  else
    fail "daemon IPC socket exists but does not respond"
  fi
else
  warn "daemon socket not found: ${SOCKET_PATH}"
fi

if [[ -f "${DATA_DIR}/local.db" ]]; then
  if python3 - "${DATA_DIR}/local.db" <<'PY'
import sqlite3
import sys

with sqlite3.connect(sys.argv[1]) as database:
    result = database.execute("PRAGMA integrity_check").fetchone()
    if result != ("ok",):
        raise SystemExit(f"SQLite integrity check failed: {result}")
PY
  then
    ok "local SQLite database passes integrity check"
  else
    fail "local SQLite database integrity check failed"
  fi
else
  warn "local database not found: ${DATA_DIR}/local.db"
fi

if command -v curl >/dev/null 2>&1 && curl --silent --show-error --fail --max-time 3 \
  --insecure https://127.0.0.1:8443/health >/dev/null 2>&1; then
  ok "local HTTPS bridge health endpoint responds"
else
  warn "local HTTPS bridge did not respond at 127.0.0.1:8443"
fi

if [[ -n "${FLARE_SYNC_URL:-}" && -n "${FLARE_SYNC_TOKEN:-}" ]]; then
  ok "remote sync environment is configured (token value not displayed)"
elif [[ -n "${FLARE_SYNC_URL:-}" || -n "${FLARE_SYNC_TOKEN:-}" ]]; then
  fail "set both FLARE_SYNC_URL and FLARE_SYNC_TOKEN or unset both"
else
  warn "remote sync is not configured; local queue remains available"
fi

if [[ "${FLARE_GIT_AUTO_COMMIT:-false}" == "true" && -n "${FLARE_GIT_REPOS:-}" ]]; then
  if [[ "${FLARE_GIT_AUTO_PUSH:-false}" == "true" ]]; then
    ok "automatic Git commit-and-push is enabled for configured paths"
  else
    ok "automatic local Git checkpoints are enabled for configured paths"
  fi
elif [[ "${FLARE_GIT_AUTO_PUSH:-false}" == "true" ]]; then
  fail "FLARE_GIT_AUTO_PUSH requires FLARE_GIT_AUTO_COMMIT=true and FLARE_GIT_REPOS"
else
  warn "automatic Git checkpoints are disabled"
fi

if (( failures > 0 )); then
  printf '\nFLARE diagnostics found %d failure(s).\n' "${failures}" >&2
  exit 1
fi
printf '\nFLARE diagnostics completed.\n'
