# FLARE (Working Implementation)

This is a concrete implementation of the FLARE spec across Python (Django), Go (daemon), C++ (crypto), and React + TypeScript (frontend).

## Quick start

### Prereqs
- Python 3.10+
- Go 1.20+
- Node 18+ (pnpm/npm)
- CMake (for C++ crypto)
- sqlite3, systemd (Linux)

## Layout
- backend/django-flare/  Django REST API + models
- daemon/go-flared/      flared background daemon (Unix socket)
- native/cpp-crypto/     Ed25519/AES-256-GCM + hwid
- frontend/flare-ui/     React + Vite + Tailwind + Dexie.js
- scripts/               setup.sh, flare-backup.sh
- deploy/systemd/        flare.service
# FLARE
