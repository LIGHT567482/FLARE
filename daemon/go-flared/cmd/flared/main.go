package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"flare/go-flared/internal/db"
	"flare/go-flared/internal/ipc"
)

func main() {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	dataDir := filepath.Join(home, ".flare-data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		panic(err)
	}

	if err := db.Init(filepath.Join(dataDir, "local.db")); err != nil {
		panic(err)
	}

	socketPath := "/tmp/flared.sock"
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		// ignore
	}

	srv := ipc.NewServer(socketPath, dataDir)
	if err := srv.Start(); err != nil {
		panic(err)
	}

	fmt.Printf("[flared] started, socket=%s, dataDir=%s\n", socketPath, dataDir)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	_ = srv.Stop()
}
