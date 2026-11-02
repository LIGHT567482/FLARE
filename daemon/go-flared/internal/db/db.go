package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func Init(path string) error {
	var err error
	DB, err = sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
PRAGMA encoding = 'UTF-8';
PRAGMA auto_vacuum = INCREMENTAL;
PRAGMA page_size = 4096;`)
	if err != nil {
		return fmt.Errorf("pragmas: %w", err)
	}
	_, err = DB.Exec(`
CREATE TABLE IF NOT EXISTS system_node_config (
  node_id TEXT PRIMARY KEY,
  hardware_hash TEXT NOT NULL UNIQUE,
  license_key TEXT NOT NULL,
  activation_status INTEGER DEFAULT 0,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS local_audit_logs (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  hash_chain TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS offline_sync_queue (
  queue_id TEXT PRIMARY KEY,
  entity_type TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  status INTEGER DEFAULT 0,
  attempts INTEGER DEFAULT 0,
  last_attempt_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`)
	return err
}
