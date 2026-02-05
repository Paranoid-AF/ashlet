// Command ashletd is the ashlet daemon.
// It listens on a Unix domain socket for completion requests from shell clients,
// gathers context, and returns AI-powered completions.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Paranoid-AF/ashlet/internal/ipc"
)

func main() {
	socketPath := resolveSocketPath()

	log.Printf("ashletd starting, socket: %s", socketPath)

	srv, err := ipc.NewServer(socketPath)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
	defer srv.Close()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutting down...")
		srv.Close()
		os.Exit(0)
	}()

	log.Println("ashletd ready")
	if err := srv.Serve(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func resolveSocketPath() string {
	if path := os.Getenv("ASHLET_SOCKET"); path != "" {
		return path
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir + "/ashlet.sock"
	}
	return fmt.Sprintf("/tmp/ashlet-%d.sock", os.Getuid())
}
