// Package ipc implements the Unix domain socket server for ashlet.
package ipc

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"os"

	"github.com/Paranoid-AF/ashlet/internal/completion"
	"github.com/Paranoid-AF/ashlet/pkg/protocol"
)

// Server listens on a Unix domain socket for completion requests.
type Server struct {
	listener net.Listener
	sockPath string
	engine   *completion.Engine
}

// NewServer creates a new IPC server bound to the given socket path.
func NewServer(sockPath string) (*Server, error) {
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
		engine:   completion.NewEngine(),
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

// Close shuts down the server and removes the socket file.
func (s *Server) Close() {
	s.listener.Close()
	os.Remove(s.sockPath)
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}

	var req protocol.Request
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		log.Printf("invalid request: %v", err)
		return
	}

	resp := s.engine.Complete(&req)

	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("failed to marshal response: %v", err)
		return
	}

	conn.Write(append(data, '\n'))
}
