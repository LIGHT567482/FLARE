#!/usr/bin/env bash
set -euo pipefail

USER_HOME="${HOME:?HOME must be set}"
DATA_DIR="${FLARE_DATA_DIR:-${USER_HOME}/.flare-data}"
BACKUP_DIR="${FLARE_BACKUP_DIR:-${USER_HOME}/flare-backups}"
DB_FILE="${DATA_DIR}/local.db"
TIMESTAMP="$(date -u +"%Y%m%dT%H%M%SZ")"

if [[ ! -f "${DB_FILE}" ]]; then
  echo "FLARE database not found: ${DB_FILE}" >&2
  exit 1
fi

mkdir -p "${DATA_DIR}" "${BACKUP_DIR}"
chmod 700 "${DATA_DIR}"
chmod 700 "${BACKUP_DIR}"
SNAPSHOT_DIR="$(mktemp -d "${DATA_DIR}/.flare-snapshot.XXXXXX")"
SNAPSHOT="${SNAPSHOT_DIR}/snapshot.db"
ARCHIVE_TMP="$(mktemp "${BACKUP_DIR}/.flare-backup.XXXXXX")"
trap 'rm -f -- "${SNAPSHOT}" "${ARCHIVE_TMP}"; rmdir -- "${SNAPSHOT_DIR}"' EXIT

python3 - "${DB_FILE}" "${SNAPSHOT}" <<'PY'
import sqlite3
import sys

source = sqlite3.connect(sys.argv[1], timeout=30)
target = sqlite3.connect(sys.argv[2])
try:
    source.backup(target)
    result = target.execute("PRAGMA integrity_check").fetchone()
    if result != ("ok",):
        raise RuntimeError(f"database integrity check failed: {result}")
finally:
    target.close()
    source.close()
PY

FILES=()
for name in node_keys.ed25519 node_config.json; do
  if [[ -f "${DATA_DIR}/${name}" ]]; then
    FILES+=("${name}")
  fi
done

tar -cf "${ARCHIVE_TMP}" -C "${SNAPSHOT_DIR}" snapshot.db
if [[ ${#FILES[@]} -gt 0 ]]; then
  tar -rf "${ARCHIVE_TMP}" -C "${DATA_DIR}" "${FILES[@]}"
fi
gzip -f "${ARCHIVE_TMP}"
ARCHIVE_TMP="${ARCHIVE_TMP}.gz"
chmod 600 "${ARCHIVE_TMP}"
BACKUP_FILE="${BACKUP_DIR}/flare_snapshot_${TIMESTAMP}.tar.gz"
mv -- "${ARCHIVE_TMP}" "${BACKUP_FILE}"

echo "FLARE backup created and database integrity verified: ${BACKUP_FILE}"
