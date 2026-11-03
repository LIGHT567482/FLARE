<table align="center" width="100%">
  <tr>
    <td align="center" bgcolor="#111827">
      <img src="./white.svg" alt="FLARE" width="128">
    </td>
  </tr>
</table>

# FLARE

FLARE is a local-first application prototype built from a React/TypeScript UI, a Go daemon, a Django API, a FastAPI IPC bridge, and a C++ cryptography library.

> **Status:** the local queue, authenticated self-hosted event upload, tracked-file checkpoints, basic in-app conflict review/resolution, and crypto primitives are implemented. A production deployment still needs operator-managed secrets, HTTPS, and target-specific validation. Node-locked licensing, application-wide encrypted storage, bidirectional replication, and a full collaborative merge workflow are not implemented; do not treat the prototype as production-secure.

## Components

- `frontend/flare-ui/` — React + Vite dashboard with IndexedDB local storage
- `daemon/go-flared/` — Go daemon, private Unix socket, SQLite queue, retrying sync worker, optional Git checkpoints
- `bridge/python-bridge/` — local HTTPS bridge from the browser to the daemon socket
- `backend/django-flare/` — self-hostable sync receiver and Django admin
- `native/cpp-crypto/` — OpenSSL-backed Ed25519 and AES-256-GCM primitives
- `scripts/` — setup, diagnostics, and verified SQLite backup/restore scripts
- `deploy/systemd/` — daemon and bridge service units

## Development

Prerequisites: Go 1.20+, Python 3.10+, Node 18+, a C++17 compiler, OpenSSL development headers, and Git.

Create a private bridge token once and load it in the bridge and Vite terminals:

```sh
install -d -m 700 ~/.config/flare
printf 'BRIDGE_AUTH_TOKEN=%s\n' "$(openssl rand -hex 32)" > ~/.config/flare/bridge.env
chmod 600 ~/.config/flare/bridge.env
```

Run each component in a separate terminal:

```sh
# Local daemon
cd daemon/go-flared
go test ./...
go run ./cmd/flared

# Local bridge (expects the bridge certificates in bridge/python-bridge/)
cd bridge/python-bridge
python3 -m pip install -r requirements.txt
set -a; source ~/.config/flare/bridge.env; set +a
python3 run.py

# Frontend
cd frontend/flare-ui
npm ci
set -a; source ~/.config/flare/bridge.env; set +a
export FLARE_BRIDGE_TOKEN="$BRIDGE_AUTH_TOKEN"
npm run dev

# Backend and self-hosted sync receiver
cd backend/django-flare
python3 -m pip install -r requirements.txt
python3 manage.py migrate
export FLARE_SYNC_TOKEN='replace-with-the-same-token-configured-on-flared'
python3 manage.py runserver 127.0.0.1:8000
```

The Vite development proxy sends `/api/*` to the local HTTPS bridge at `https://127.0.0.1:8443`, adding the bridge token server-side so it is not exposed to browser code. Set `FLARE_BRIDGE_URL` if the bridge is elsewhere. The proxy disables certificate verification only for this local development hop; production hosting must provide its own trusted TLS/reverse-proxy configuration and inject `Authorization` server-side.

Build and test crypto primitives with `make -C native/cpp-crypto test`. AES-GCM uses a 32-byte key, 12-byte nonce, and 16-byte tag. Keep signing keys and encryption keys secret; never reuse an AES-GCM nonce with the same key.

## Self-hosted sync

Run Django behind HTTPS (do not expose Django's development server to the network). Configure the same high-entropy bearer token on the receiving service and each daemon, and set a production Django secret and host allowlist:

```text
FLARE_DEBUG=false
FLARE_SECRET_KEY=<operator-managed Django secret>
FLARE_ALLOWED_HOSTS=sync.example.org
FLARE_SYNC_TOKEN=<operator-managed sync token>
```

On each daemon host, create `~/.config/flare/daemon.env` with mode `0600`:

```text
FLARE_SYNC_URL=https://sync.example.org
FLARE_SYNC_TOKEN=<same operator-managed sync token>
```

Then restart `flared`. The daemon sends bounded batches over HTTPS, requires explicit event acknowledgements, and retries failures with capped exponential backoff. If sync is not configured, items stay local. Event IDs make retries idempotent. Include a string `entity_id` in a payload when concurrent edits should be flagged; the server retains both versions as a conflict. The dashboard displays both versions and lets the user choose one; the server records the chosen version as a resolution event. An authorized operator can also inspect conflicts at `GET /api/sync/conflicts/` using the bearer token. This remains a simple shared-token, event-upload model—not per-device identity, key rotation, or bidirectional replication.

## Automatic local Git checkpoints

Automatic Git commits are off unless explicitly enabled. Local-only checkpoints can be enabled with:

```text
FLARE_GIT_AUTO_COMMIT=true
FLARE_GIT_REPOS=/absolute/path/to/repository
```

For automatic commit-and-push, also set:

```text
FLARE_GIT_AUTO_PUSH=true
```

Put these settings in `~/.config/flare/daemon.env` with mode `0600` and restart the daemon. The push workflow checks for a clean tracked working tree before it arms; if the repository has tracked changes when the daemon starts, it waits until it observes the tree clean so those existing changes are not swept into an automatic commit. After arming, it checks every 30 seconds, stages tracked changes only, commits with the local Git identity, and pushes only the current branch to its configured upstream. Untracked files are never added. Previously-created local commits are not pushed automatically. Configure Git credentials using your normal SSH agent or credential helper; do not put account passwords or access tokens in the daemon environment file. If commit or push fails, the daemon logs the error; a push failure can leave the new commit local.

Automatic pushing is an external side effect. Review the repository path, branch upstream, Git identity, and credential setup before enabling it.

## Backup and restore

`scripts/flare-backup.sh` uses SQLite's online backup API, runs an integrity check, and creates a mode-`0600` archive under `~/flare-backups` by default. Override paths with `FLARE_DATA_DIR` and `FLARE_BACKUP_DIR`.

Stop `flared` before restoring:

```sh
bash scripts/flare-restore.sh ~/flare-backups/flare_snapshot_<timestamp>.tar.gz
```

Restore validates archive members and database integrity, then creates a backup of the current data before replacement. Add `--yes` only for deliberate non-interactive restores.

Run `bash scripts/flare-doctor.sh` to check local tool availability, daemon/bridge health, data-directory permissions, SQLite integrity, and whether optional sync/Git settings are present.

## Security and known gaps

- Crypto primitives now use OpenSSL and have signature, tamper, and authenticated-decryption tests. They are not yet integrated into FLARE's data-at-rest path.
- The sync API uses one bearer secret and HTTPS; deploy it behind a trusted TLS endpoint, protect secret files, and restrict network access.
- The local bridge requires a separate bearer secret; keep it in a mode-`0600` environment file and do not put it in browser-visible frontend configuration.
- Licensing and the natural-language commit engine described in the SRS are not implemented. Git checkpoints are timestamped, deterministic local commits.
- Sync currently uploads events to one server; it does not replicate server-side events back to other devices. Conflict versions can be reviewed and one selected as a resolution, but this is not a full collaborative merge/editor workflow.
- There is no encrypted local database, per-device identity lifecycle, or production install/upgrade/uninstall flow yet.
- Run backend migrations before hosting the receiver. Keep SQLite and backup files on storage with appropriate access controls.

<table width="100%">
  <tr>
    <td align="left" bgcolor="#111827">
      <img src="./white.png" alt="BlueRhea Technologies" width="160">
    </td>
  </tr>
</table>
