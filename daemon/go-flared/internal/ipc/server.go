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
	if err := os.Chmod(s.socketPath, 0666); err != nil {
		fmt.Printf("warn chmod: %v\n", err)
	}
	go s.acceptLoop()
	go sync.Worker()
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
				"engine": "ok",
				"socket": s.socketPath,
				"storage": "sqlite_wal",
			})
		case "enqueue":
			et := fmt.Sprintf("%v", req["entity_type"])
			payload, _ := json.Marshal(req["payload"])
			_ = enqueue(et, string(payload))
			_ = respond(conn, map[string]any{"ok": true})
		case "pull_queue":
			items, _ := pullQueue(10)
			_ = respond(conn, map[string]any{"items": items})
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
	rows, err := db.DB.Query(`SELECT queue_id, entity_type, payload_json, created_at FROM offline_sync_queue WHERE status=0 ORDER BY created_at LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []map[string]any
	for rows.Next() {
		var id, et, pj, ct string
		if err := rows.Scan(&id, &et, &pj, &ct); err != nil {
			continue
		}
		var p any
		_ = json.Unmarshal([]byte(pj), &p)
		res = append(res, map[string]any{"queue_id": id, "entity_type": et, "payload": p, "created_at": ct})
	}
	return res, nil
}
