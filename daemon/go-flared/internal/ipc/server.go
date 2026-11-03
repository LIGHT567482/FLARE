package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"flare/go-flared/internal/db"
	"flare/go-flared/internal/sync"
)

type Server struct {
	lis        net.Listener
	socketPath string
	dataDir    string
}

func NewServer(socketPath, dataDir string) *Server {
	return &Server{socketPath: socketPath, dataDir: dataDir}
}

func (s *Server) Start() error {
	var err error
	s.lis, err = net.Listen("unix", s.socketPath)
	if err != nil {
		return err
	}
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		_ = s.lis.Close()
		return fmt.Errorf("secure IPC socket: %w", err)
	}
	go s.acceptLoop()
	return nil
}

func (s *Server) Stop() error {
	if s.lis != nil {
		return s.lis.Close()
	}
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.lis.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewScanner(conn)
	r.Buffer(make([]byte, 1024*1024), 1024*1024)
	r.Split(bufio.ScanLines)
	for r.Scan() {
		line := strings.TrimSpace(r.Text())
		if line == "" {
			continue
		}
		var req map[string]any
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			_ = respond(conn, map[string]any{"error": "bad_json"})
			continue
		}
		switch req["cmd"] {
		case "ping":
			_ = respond(conn, map[string]any{"pong": time.Now().Unix()})
		case "status":
			_ = respond(conn, map[string]any{
				"engine":  "ok",
				"socket":  s.socketPath,
				"storage": "sqlite_wal",
			})
		case "enqueue":
			et, ok := req["entity_type"].(string)
			if !ok || len(et) == 0 || len(et) > 64 {
				_ = respond(conn, map[string]any{"error": "entity_type_required"})
				continue
			}
			value, exists := req["payload"]
			if !exists || value == nil {
				value = map[string]any{}
			}
			if _, ok := value.(map[string]any); !ok {
				_ = respond(conn, map[string]any{"error": "payload_must_be_object"})
				continue
			}
			payload, err := json.Marshal(value)
			if err != nil {
				_ = respond(conn, map[string]any{"error": "payload_encode_failed"})
				continue
			}
			if err := enqueue(et, string(payload)); err != nil {
				_ = respond(conn, map[string]any{"error": "enqueue_failed", "detail": err.Error()})
				continue
			}
			_ = respond(conn, map[string]any{"ok": true})
		case "pull_queue":
			limit := 10
			if requested, ok := req["limit"].(float64); ok {
				if requested < 1 || requested > 100 {
					_ = respond(conn, map[string]any{"error": "queue_limit_out_of_range"})
					continue
				}
				limit = int(requested)
			}
			items, err := pullQueue(limit)
			if err != nil {
				_ = respond(conn, map[string]any{"error": "queue_read_failed", "detail": err.Error()})
				continue
			}
			_ = respond(conn, map[string]any{"items": items})
		case "sync_conflicts":
			conflicts, err := sync.ListConflicts()
			if err != nil {
				_ = respond(conn, map[string]any{"error": "conflict_list_failed", "detail": err.Error()})
				continue
			}
			_ = respond(conn, map[string]any{"conflicts": conflicts})
		case "resolve_conflict":
			conflictID := fmt.Sprintf("%v", req["conflict_id"])
			eventID := fmt.Sprintf("%v", req["event_id"])
			if err := sync.ResolveConflict(conflictID, eventID); err != nil {
				_ = respond(conn, map[string]any{"error": "conflict_resolution_failed", "detail": err.Error()})
				continue
			}
			_ = respond(conn, map[string]any{"ok": true})
		case "health":
			_ = respond(conn, map[string]any{"status": "ok"})
		default:
			_ = respond(conn, map[string]any{"error": "unknown_cmd"})
		}
	}
}

func respond(conn net.Conn, v any) error {
	b, _ := json.Marshal(v)
	_, err := conn.Write(append(b, '\n'))
	return err
}

func enqueue(entityType, payload string) error {
	_, err := db.DB.Exec(`INSERT INTO offline_sync_queue (queue_id, entity_type, payload_json, status, attempts, created_at)
VALUES (lower(hex(randomblob(16))), ?, ?, 0, 0, CURRENT_TIMESTAMP)`, entityType, payload)
	return err
}

func pullQueue(limit int) ([]map[string]any, error) {
	rows, err := db.DB.Query(`SELECT queue_id, entity_type, payload_json, created_at, status, attempts
FROM offline_sync_queue
WHERE status IN (0, 3)
ORDER BY created_at
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make([]map[string]any, 0)
	for rows.Next() {
		var id, et, pj, ct string
		var status, attempts int
		if err := rows.Scan(&id, &et, &pj, &ct, &status, &attempts); err != nil {
			return nil, err
		}
		var p any
		if err := json.Unmarshal([]byte(pj), &p); err != nil {
			return nil, fmt.Errorf("decode queued payload %s: %w", id, err)
		}
		state := "pending"
		if status == 3 {
			state = "failed"
		}
		res = append(res, map[string]any{
			"queue_id": id, "entity_type": et, "payload": p,
			"created_at": ct, "status": state, "attempts": attempts,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}
