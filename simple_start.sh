#!/usr/bin/env bash
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOGS=/tmp/flare-logs-simple
mkdir -p "$LOGS"
rm -f /tmp/flared.sock 2>/dev/null || true

pkill -9 flared 2>/dev/null || true
pkill -9 uvicorn 2>/dev/null || true
pkill -9 "manage.py runserver" 2>/dev/null || true
pkill -9 vite 2>/dev/null || true
sleep 1

# flared
/tmp/flared >"$LOGS/flared.log" 2>&1 &
echo "flared:$!"
sleep 0.5

# django
cd "$DIR/backend/django-flare"
python3 manage.py runserver 127.0.0.1:8000 >"$LOGS/django.log" 2>&1 &
echo "django:$!"
cd "$DIR"

# bridge
cd "$DIR/bridge/python-bridge"
export BRIDGE_TOKEN=localtest
export PYTHONPATH="$DIR/bridge/python-bridge"
lsof -ti:8443 2>/dev/null | xargs kill -9 2>/dev/null || true
python3 -m uvicorn app.main:app --host 127.0.0.1 --port 8443 --ssl-certfile cert.pem --ssl-keyfile key.pem >"$LOGS/bridge.log" 2>&1 &
echo "bridge:$!"
cd "$DIR"

# frontend
cd "$DIR/frontend/flare-ui"
npx --no-install vite --host 127.0.0.1 --port 5173 >"$LOGS/frontend.log" 2>&1 &
echo "frontend:$!"
cd "$DIR"

echo "---READY---"
