package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"flare/go-flared/internal/db"
	gitengine "flare/go-flared/internal/git"
	"flare/go-flared/internal/ipc"
	"flare/go-flared/internal/sync"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("resolve user home directory: %v", err)
	}
	if home == "" {
		log.Fatal("user home directory is empty")
	}
	dataDir := filepath.Join(home, ".flare-data")
	if configuredDir := os.Getenv("FLARE_DATA_DIR"); configuredDir != "" {
		dataDir = configuredDir
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("create data directory: %v", err)
	}
	if err := os.Chmod(dataDir, 0700); err != nil {
		log.Fatalf("secure data directory: %v", err)
	}

	if err := db.Init(filepath.Join(dataDir, "local.db")); err != nil {
		log.Fatalf("initialize local database: %v", err)
	}

	socketPath := filepath.Join(dataDir, "flared.sock")
	if info, err := os.Lstat(socketPath); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			log.Fatalf("refusing to replace non-socket path %s", socketPath)
		}
		conn, dialErr := net.DialTimeout("unix", socketPath, time.Second)
		if dialErr == nil {
			_ = conn.Close()
			log.Fatalf("another flared instance is listening on %s", socketPath)
		}
		if err := os.Remove(socketPath); err != nil {
			log.Fatalf("remove stale socket: %v", err)
		}
	} else if !os.IsNotExist(err) {
		log.Fatalf("inspect socket path: %v", err)
	}

	srv := ipc.NewServer(socketPath, dataDir)
	if err := srv.Start(); err != nil {
		log.Fatalf("start IPC server: %v", err)
	}
	go sync.Worker()
	go gitengine.Worker()

	fmt.Printf("[flared] started, socket=%s, dataDir=%s\n", socketPath, dataDir)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	_ = srv.Stop()
}
