package main

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"

	ashlet "github.com/Paranoid-AF/ashlet"
	defaults "github.com/Paranoid-AF/ashlet/default"
	"github.com/Paranoid-AF/ashlet/generate"
)

// Completer processes a completion request and returns a response.
type Completer interface {
	Complete(ctx context.Context, req *ashlet.Request) *ashlet.Response
	WarmContext(ctx context.Context, cwd string)
	Close()
}

// sessionEntry tracks a cancellable in-flight request for a session.
type sessionEntry struct {
	requestID int
	cancel    context.CancelFunc
}

// Server listens on a Unix domain socket for completion requests.
type Server struct {
	listener net.Listener
	sockPath string
	engine   Completer

	mu       sync.Mutex
	sessions map[string]sessionEntry
}

// NewServer creates a new IPC server bound to the given socket path.
func NewServer(sockPath string) (*Server, error) {
	engine := generate.NewEngine()
	return NewServerWithCompleter(sockPath, engine)
}

// NewServerWithCompleter creates a new IPC server with a custom Completer.
func NewServerWithCompleter(sockPath string, completer Completer) (*Server, error) {
	// Remove stale socket file if it exists
	if err := os.Remove(sockPath); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, err
	}

	return &Server{
		listener: listener,
		sockPath: sockPath,
		engine:   completer,
		sessions: make(map[string]sessionEntry),
	}, nil
}

// Serve accepts connections and handles requests.
func (s *Server) Serve() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

// Close shuts down the server, inference engine, and removes the socket file.
func (s *Server) Close() {
	s.engine.Close()
	s.listener.Close()
	os.Remove(s.sockPath)
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}

	raw := scanner.Bytes()
	slog.Debug("request", "data", string(raw))

	// Check if this is a context warm-up request (has "type":"context" field)
	var ctxReq ashlet.ContextRequest
	if err := json.Unmarshal(raw, &ctxReq); err == nil && ctxReq.Type == "context" {
		s.handleContextRequest(conn, &ctxReq)
		return
	}

	// Check if this is a config request (has "action" field)
	var cfgReq ashlet.ConfigRequest
	if err := json.Unmarshal(raw, &cfgReq); err == nil && cfgReq.Action != "" {
		s.handleConfigRequest(conn, &cfgReq)
		return
	}

	var req ashlet.Request
	if err := json.Unmarshal(raw, &req); err != nil {
		slog.Warn("invalid request", "error", err)
		return
	}

	// Cancel any in-flight request for this session and create a new context.
	ctx, cancel := context.WithCancel(context.Background())
	sid := req.SessionID
	reqID := req.RequestID
	if sid != "" {
		s.mu.Lock()
		if prev, ok := s.sessions[sid]; ok {
			prev.cancel()
		}
		s.sessions[sid] = sessionEntry{requestID: reqID, cancel: cancel}
		s.mu.Unlock()
	}
	defer func() {
		cancel()
		if sid != "" {
			s.mu.Lock()
			if cur, ok := s.sessions[sid]; ok && cur.requestID == reqID {
				delete(s.sessions, sid)
			}
			s.mu.Unlock()
		}
	}()

	resp := s.engine.Complete(ctx, &req)

	// If cancelled, skip writing — the client has already moved on.
	if ctx.Err() != nil {
		return
	}

	resp.RequestID = req.RequestID

	data, err := json.Marshal(resp)
	if err != nil {
		slog.Error("failed to marshal response", "error", err)
		return
	}

	slog.Debug("response", "data", string(data))

	conn.Write(append(data, '\n'))
}

func (s *Server) handleContextRequest(conn net.Conn, req *ashlet.ContextRequest) {
	resp := ashlet.ContextResponse{OK: true}

	cwd := strings.TrimRight(req.Cwd, "\n")
	if cwd == "" {
		resp.OK = false
		resp.Error = &ashlet.Error{Code: "invalid_request", Message: "cwd is required"}
	} else {
		// Gather in background — respond immediately
		go s.engine.WarmContext(context.Background(), cwd)
	}

	data, err := json.Marshal(resp)
	if err != nil {
		slog.Error("failed to marshal context response", "error", err)
		return
	}

	slog.Debug("response", "data", string(data))

	conn.Write(append(data, '\n'))
}

func (s *Server) handleConfigRequest(conn net.Conn, req *ashlet.ConfigRequest) {
	var resp ashlet.ConfigResponse

	switch req.Action {
	case "get":
		cfg, err := ashlet.LoadConfig()
		if err != nil {
			resp.Error = &ashlet.Error{
				Code:    "config_error",
				Message: err.Error(),
			}
		} else {
			resp.Config = cfg
		}

	case "reload":
		// Respond immediately; reload engine in the background.
		// Engine reload can block for tens of seconds waiting for
		// llama-server health, so we must not block the client.
		go s.reloadEngine()
		cfg, _ := ashlet.LoadConfig()
		resp.Config = cfg

	case "defaults":
		resp.Config = ashlet.DefaultConfig()

	case "default_prompt":
		resp.Prompt = defaults.DefaultPrompt

	case "validate":
		cfg, err := ashlet.LoadConfig()
		if err != nil {
			resp.Error = &ashlet.Error{
				Code:    "config_error",
				Message: err.Error(),
			}
		} else {
			resp.Warnings = ashlet.ValidateConfig(cfg)
		}

	default:
		resp.Error = &ashlet.Error{
			Code:    "unknown_action",
			Message: "unknown config action: " + req.Action,
		}
	}

	data, err := json.Marshal(resp)
	if err != nil {
		slog.Error("failed to marshal config response", "error", err)
		return
	}

	slog.Debug("response", "data", string(data))

	conn.Write(append(data, '\n'))
}

func (s *Server) reloadEngine() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close old engine
	if s.engine != nil {
		s.engine.Close()
	}

	// Create new engine with updated config
	s.engine = generate.NewEngine()
	slog.Info("engine reloaded")
}
