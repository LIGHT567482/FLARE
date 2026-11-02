# FLARE Unix-Socket -> HTTPS Bridge (robust)

Features:
- FastAPI + uvicorn
- CORS restricted to localhost dev
- Per-request timeout + concurrency limits
- Line-based JSON IPC to flared over Unix socket
- Optional mTLS-like local HTTPS (self-signed certs)
- Health + /ipc + /flare-ipc endpoints
- Config via env

## Quick start
cd bridge/python-bridge
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
./gen_certs.sh
FLARED_SOCKET=/tmp/flared.sock BRIDGE_PORT=8443 python run.py
