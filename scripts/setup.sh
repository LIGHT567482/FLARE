#!/usr/bin/env bash
set -e

USER_HOME=$(eval echo "~$LOGNAME")
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA_DIR="${USER_HOME}/.flare-data"
BIN_DIR="/usr/local/bin"
SERVICE_DIR="/etc/systemd/system"

echo "[*] Setting up FLARE..."
mkdir -p "${DATA_DIR}" "${BACKUP_DIR:-$USER_HOME/flare-backups}"

# Build Go daemon if needed
if [ -d "${PROJECT_DIR}/daemon/go-flared" ]; then
  echo "[*] Building flared (Go)..."
  (cd "${PROJECT_DIR}/daemon/go-flared" && go build -o /tmp/flared ./cmd/flared || true)
  sudo cp -f /tmp/flared "${BIN_DIR}/flared" || cp /tmp/flared "${BIN_DIR}/flared" 2>/dev/null || true
  sudo chmod +x "${BIN_DIR}/flared" 2>/dev/null || chmod +x "${BIN_DIR}/flared" 2>/dev/null || true
fi

# Install systemd service if available
if command -v systemctl >/dev/null 2>&1; then
  echo "[*] Installing systemd service..."
  sudo cp -f "${PROJECT_DIR}/deploy/systemd/flare.service" "${SERVICE_DIR}/flare.service" 2>/dev/null || true
  sudo systemctl daemon-reload 2>/dev/null || true
  sudo systemctl enable --now flare.service 2>/dev/null || true
  echo "[*] Service status:"
  sudo systemctl status flare.service --no-pager 2>/dev/null || true
fi

# Build C++ lib
if [ -d "${PROJECT_DIR}/native/cpp-crypto" ]; then
  echo "[*] Building flare-crypto..."
  (cd "${PROJECT_DIR}/native/cpp-crypto" && make clean && make) || true
fi

echo "===================================================="
echo "  [✓] FLARE setup complete"
echo "  [*] Data: ${DATA_DIR}"
echo "  [*] Socket: /tmp/flared.sock"
echo "  [*] Binary: ${BIN_DIR}/flared"
echo "===================================================="
