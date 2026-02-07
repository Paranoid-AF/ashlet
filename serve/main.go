// Command ashletd is the ashlet daemon.
// It listens on a Unix domain socket for completion requests from shell clients,
// gathers context, and returns AI-powered completions.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	verbose := flag.Bool("verbose", false, "log every request and response to stdout")
	flag.Parse()

	if *showVersion {
		fmt.Println("ashletd", Version)
		os.Exit(0)
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	socketPath := resolveSocketPath()

	slog.Info("starting", "socket", socketPath)

	srv, err := NewServer(socketPath)
	if err != nil {
		slog.Error("failed to start server", "error", err)
		os.Exit(1)
	}
	defer srv.Close()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("shutting down")
		srv.Close()
		os.Exit(0)
	}()

	slog.Info("ready")
	if err := srv.Serve(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
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
