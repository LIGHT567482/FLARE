#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "Usage: $0 BACKUP.tar.gz [--yes]" >&2
  exit 2
fi
ARCHIVE="$(realpath -- "$1")"
if [[ ! -f "${ARCHIVE}" ]]; then
  echo "Backup archive not found: ${ARCHIVE}" >&2
  exit 1
fi
if [[ "${2:-}" != "--yes" ]]; then
  read -r -p "Restore ${ARCHIVE} over the current FLARE data? Stop flared first. [y/N] " answer
  if [[ "${answer}" != "y" && "${answer}" != "Y" ]]; then
    echo "Restore cancelled."
    exit 1
  fi
fi

USER_HOME="${HOME:?HOME must be set}"
DATA_DIR="${FLARE_DATA_DIR:-${USER_HOME}/.flare-data}"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
mkdir -p "${DATA_DIR}"
chmod 700 "${DATA_DIR}"
STAGING="$(mktemp -d "${DATA_DIR}/.flare-restore.XXXXXX")"
trap 'rm -rf -- "${STAGING}"' EXIT

python3 - "${ARCHIVE}" "${STAGING}" <<'PY'
import os
import shutil
import sys
import tarfile

archive, destination = sys.argv[1:]
allowed = {"snapshot.db", "node_keys.ed25519", "node_config.json"}
with tarfile.open(archive, "r:gz") as source:
    members = source.getmembers()
    names = [member.name for member in members]
    if len(names) != len(set(names)) or not names or set(names) - allowed:
        raise ValueError("archive contains missing, duplicate, or unexpected paths")
    if "snapshot.db" not in names:
        raise ValueError("archive does not contain snapshot.db")
    for member in members:
        if not member.isfile() or member.size > 2 * 1024 * 1024 * 1024:
            raise ValueError(f"archive entry is not a permitted regular file: {member.name}")
        source_file = source.extractfile(member)
        if source_file is None:
            raise ValueError(f"unable to read archive entry: {member.name}")
        with source_file, open(os.path.join(destination, member.name), "xb") as target:
            shutil.copyfileobj(source_file, target)

import sqlite3
database = sqlite3.connect(os.path.join(destination, "snapshot.db"))
try:
    result = database.execute("PRAGMA integrity_check").fetchone()
    if result != ("ok",):
        raise RuntimeError(f"restored database integrity check failed: {result}")
finally:
    database.close()
PY

bash "${SCRIPT_DIR}/flare-backup.sh"
mv -- "${STAGING}/snapshot.db" "${DATA_DIR}/local.db"
for name in node_keys.ed25519 node_config.json; do
  if [[ -f "${STAGING}/${name}" ]]; then
    chmod 600 "${STAGING}/${name}"
    mv -- "${STAGING}/${name}" "${DATA_DIR}/${name}"
  else
    rm -f -- "${DATA_DIR:?}/${name}"
  fi
done
chmod 600 "${DATA_DIR}/local.db"
echo "FLARE restore completed. Restart flared to use the restored database."
