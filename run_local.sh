#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PY=python3
VENV="${DIR}/.venv"

start_flared() {
  if [ -f /tmp/flared ]; then
    /tmp/flared &
    echo $! > /tmp/flared.pid
    echo "[flared] started (pid=$(cat /tmp/flared.pid))"
  fi
}

start_bridge() {
  BRIDGE_DIR="${DIR}/bridge/python-bridge"
  if [ ! -f "${BRIDGE_DIR}/cert.pem" ]; then
    (cd "$BRIDGE_DIR" && ./gen_certs.sh >/dev/null 2>&1)
  fi
  if [ -z "${BRIDGE_TOKEN:-}" ]; then
    BRIDGE_TOKEN=$(python3 -c "import secrets; print(secrets.token_urlsafe(48))")
    export BRIDGE_TOKEN
  fi
  cd "$BRIDGE_DIR"
  PYTHONPATH="$BRIDGE_DIR" "$PY" run.py &
  echo $! > /tmp/flare-bridge.pid
  echo "[bridge] https://127.0.0.1:8443 (pid=$(cat /tmp/flare-bridge.pid))"
  cd "$DIR"
}

start_django() {
  DDIR="${DIR}/backend/django-flare"
  cd "$DDIR"
  "$PY" manage.py migrate --noinput >/dev/null 2>&1 || true
  "$PY" manage.py runserver 127.0.0.1:8000 &
  echo $! > /tmp/flare-django.pid
  echo "[django] http://127.0.0.1:8000 (pid=$(cat /tmp/flare-django.pid))"
  cd "$DIR"
}

start_frontend() {
  FDIR="${DIR}/frontend/flare-ui"
  cd "$FDIR"
  npx vite --host 127.0.0.1 --port 5173 &
  echo $! > /tmp/flare-frontend.pid
  echo "[frontend] http://127.0.0.1:5173 (pid=$(cat /tmp/flare-frontend.pid))"
  cd "$DIR"
}

stop_all() {
  for p in flared.pid flare-bridge.pid flare-django.pid flare-frontend.pid; do
    f="/tmp/$p"
    if [ -f "$f" ]; then
      kill "$(cat "$f")" 2>/dev/null || true; rm -f "$f"
    fi
  done
  pkill -f "uvicorn.*app.main" 2>/dev/null || true
  pkill -f "vite" 2>/dev/null || true
  pkill flared 2>/dev/null || true
  echo "[stop] all stopped"
}

case "${1:-start}" in
  start)
    trap stop_all SIGINT SIGTERM
    start_flared
    start_bridge
    start_django
    start_frontend
    echo "--- FLARE running locally ---"
    echo "Frontend:  http://127.0.0.1:5173"
    echo "Django:    http://127.0.0.1:8000"
    echo "Bridge:    https://127.0.0.1:8443 (TLS)"
    echo "Flared:    /tmp/flared.sock"
    wait
    ;;
  stop)
    stop_all
    ;;
  *)
    echo "usage: $0 [start|stop]"
    ;;
esac
