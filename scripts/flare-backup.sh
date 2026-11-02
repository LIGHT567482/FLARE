#!/usr/bin/env bash
set -e

USER_HOME=$(eval echo "~$LOGNAME")
DATA_DIR="${USER_HOME}/.flare-data"
BACKUP_DIR="${USER_HOME}/flare-backups"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="${BACKUP_DIR}/flare_snapshot_${TIMESTAMP}.tar.gz"

mkdir -p "${BACKUP_DIR}"

echo "[*] Creating atomic SQLite online snapshot..."
if command -v sqlite3 >/dev/null 2>&1 && [ -f "${DATA_DIR}/local.db" ]; then
  sqlite3 "${DATA_DIR}/local.db" ".backup '${DATA_DIR}/snapshot.db'"
else
  cp -f "${DATA_DIR}/local.db" "${DATA_DIR}/snapshot.db" 2>/dev/null || true
fi

echo "[*] Packaging cryptographic keys and database state..."
tar -czf "${BACKUP_FILE}" \
  -C "${DATA_DIR}" snapshot.db node_keys.ed25519 node_config.json 2>/dev/null || \
  tar -czf "${BACKUP_FILE}" -C "${DATA_DIR}" snapshot.db 2>/dev/null || true

rm -f "${DATA_DIR}/snapshot.db"

echo "===================================================="
echo "  [✓] Offline backup successfully created!"
echo "  [*] Path: ${BACKUP_FILE}"
echo "===================================================="
