package ipc

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	"flare/go-flared/internal/db"
)

func TestServerUsesPrivateSocketAndQueuesEvents(t *testing.T) {
	tempDir := t.TempDir()
	previousDB := db.DB
	if err := db.Init(filepath.Join(tempDir, "local.db")); err != nil {
		t.Fatal(err)
	}
	currentDB := db.DB
	t.Cleanup(func() {
		_ = currentDB.Close()
		db.DB = previousDB
	})

	socketPath := filepath.Join(tempDir, "flared.sock")
	server := NewServer(socketPath, tempDir)
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Errorf("stop IPC server: %v", err)
		}
	})
	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("socket permissions = %04o; want 0600", info.Mode().Perm())
	}

	status := callIPC(t, socketPath, map[string]any{"cmd": "status"})
	if status["engine"] != "ok" {
		t.Fatalf("unexpected daemon status: %#v", status)
	}

	queued := callIPC(t, socketPath, map[string]any{
		"cmd": "enqueue", "entity_type": "note",
		"payload": map[string]any{"entity_id": "note-1"},
	})
	if queued["ok"] != true {
		t.Fatalf("enqueue failed: %#v", queued)
	}
	pulled := callIPC(t, socketPath, map[string]any{"cmd": "pull_queue", "limit": 100})
	items, ok := pulled["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected pulled items: %#v", pulled)
	}
	item, ok := items[0].(map[string]any)
	if !ok || item["entity_type"] != "note" || item["status"] != "pending" {
		t.Fatalf("unexpected queued item: %#v", items[0])
	}

	invalid := callIPC(t, socketPath, map[string]any{
		"cmd": "enqueue", "entity_type": "note", "payload": []any{"not", "an", "object"},
	})
	if invalid["error"] != "payload_must_be_object" {
		t.Fatalf("expected invalid payload rejection, got %#v", invalid)
	}
}

func callIPC(t *testing.T, socketPath string, request map[string]any) map[string]any {
	t.Helper()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		t.Fatal(err)
	}
	return response
}
