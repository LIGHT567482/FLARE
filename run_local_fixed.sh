#!/usr/bin/env bash
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PY=python3
PIDS=/tmp/flare-pids
mkdir -p "$PIDS"
LOGS=/tmp/flare-logs
mkdir -p "$LOGS"

stop_all() {
  for f in "$PIDS"/*.pid; do
    [ -f "$f" ] && kill "$(cat "$f")" 2>/dev/null || true; rm -f "$f"
  done
  pkill -9 -f flared 2>/dev/null || true
  pkill -9 -f uvicorn 2>/dev/null || true
  pkill -9 -f "manage.py runserver" 2>/dev/null || true
  pkill -9 -f vite 2>/dev/null || true
  rm -f /tmp/flared.sock 2>/dev/null || true
  sleep 0.8
}

start_flared() {
  F="/tmp/flared"
  if [ ! -x "$F" ] && [ ! -f "${DIR}/daemon/go-flared/flared" ]; then
    (cd "${DIR}/daemon/go-flared" && go build -o /tmp/flared ./cmd/flared)
  elif [ -f "${DIR}/daemon/go-flared/flared" ] && [ ! -x "$F" ]; then
    cp "${DIR}/daemon/go-flared/flared" /tmp/flared 2>/dev/null || true
    chmod +x /tmp/flared 2>/dev/null || true
  fi
  /tmp/flared >"$LOGS/flared.log" 2>&1 &
  echo $! > "$PIDS/flared.pid"
}

start_django() {
  cd "${DIR}/backend/django-flare"
  $PY manage.py migrate --noinput >/dev/null 2>&1 || true
  $PY manage.py runserver 127.0.0.1:8000 >"$LOGS/django.log" 2>&1 &
  echo $! > "$PIDS/django.pid"
  cd "$DIR"
}

start_bridge() {
  BR="${DIR}/bridge/python-bridge"
  [ -f "${BR}/cert.pem" ] || (cd "$BR" && ./gen_certs.sh >/dev/null 2>&1)
  export BRIDGE_TOKEN="${BRIDGE_TOKEN:-localtest}"
  export PYTHONPATH="$BR"
  lsof -ti:8443 2>/dev/null | xargs kill -9 2>/dev/null || true
  cd "$BR"
  $PY -m uvicorn app.main:app --host 127.0.0.1 --port 8443 --ssl-certfile cert.pem --ssl-keyfile key.pem >"$LOGS/bridge.log" 2>&1 &
  echo $! > "$PIDS/bridge.pid"
  cd "$DIR"
}

start_frontend() {
  cd "${DIR}/frontend/flare-ui"
  npx --no-install vite --host 127.0.0.1 --port 5173 >"$LOGS/frontend.log" 2>&1 &
  echo $! > "$PIDS/frontend.pid"
  cd "$DIR"
}

case "${1:-start}" in
  start) stop_all; start_flared; sleep 1; start_django; sleep 0.5; start_bridge; start_frontend; sleep 1; echo "OK" ;;
  stop) stop_all; echo "STOPPED" ;;
esac
