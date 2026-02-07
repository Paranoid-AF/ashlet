package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ashlet "github.com/Paranoid-AF/ashlet"
)

// stubCompleter returns a fixed response for testing.
type stubCompleter struct {
	resp *ashlet.Response
}

func (s *stubCompleter) Complete(_ context.Context, _ *ashlet.Request) *ashlet.Response {
	// Return a copy to avoid race conditions when server sets RequestID
	return &ashlet.Response{
		Candidates: s.resp.Candidates,
		Error:      s.resp.Error,
	}
}

func (s *stubCompleter) WarmContext(_ context.Context, _ string) {}

func (s *stubCompleter) Close() {}

var testSocketCounter atomic.Int64

func newTestServer(t *testing.T, completer Completer) *Server {
	t.Helper()
	// Use /tmp directly to avoid macOS 104-char Unix socket path limit
	n := testSocketCounter.Add(1)
	sockPath := fmt.Sprintf("/tmp/ashlet-t%d.sock", n)
	srv, err := NewServerWithCompleter(sockPath, completer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	go srv.Serve()
	return srv
}

func sendRequest(t *testing.T, sockPath string, req *ashlet.Request) *ashlet.Response {
	t.Helper()
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	conn.Write(append(data, '\n'))

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response from server")
	}

	var resp ashlet.Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return &resp
}

func TestHandleConnEchoesRequestID(t *testing.T) {
	stub := &stubCompleter{
		resp: &ashlet.Response{Candidates: []ashlet.Candidate{}},
	}
	srv := newTestServer(t, stub)

	resp := sendRequest(t, srv.sockPath, &ashlet.Request{
		RequestID: 17,
		Input:     "git st",
		CursorPos: 6,
	})

	if resp.RequestID != 17 {
		t.Errorf("expected request_id 17, got %d", resp.RequestID)
	}
}

func TestHandleConnCandidatesNotNull(t *testing.T) {
	stub := &stubCompleter{
		resp: &ashlet.Response{Candidates: []ashlet.Candidate{}},
	}
	srv := newTestServer(t, stub)

	conn, err := net.Dial("unix", srv.sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	req := &ashlet.Request{RequestID: 1, Input: "ls"}
	data, _ := json.Marshal(req)
	conn.Write(append(data, '\n'))

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response")
	}

	raw := scanner.Text()
	if !strings.Contains(raw, `"candidates":[]`) {
		t.Errorf("expected candidates:[] in raw JSON, got %s", raw)
	}
}

func TestHandleConnSequentialIDs(t *testing.T) {
	stub := &stubCompleter{
		resp: &ashlet.Response{Candidates: []ashlet.Candidate{}},
	}
	srv := newTestServer(t, stub)

	for _, id := range []int{1, 2, 3} {
		resp := sendRequest(t, srv.sockPath, &ashlet.Request{
			RequestID: id,
			Input:     "test",
		})
		if resp.RequestID != id {
			t.Errorf("expected request_id %d, got %d", id, resp.RequestID)
		}
	}
}

// slowCompleter blocks until its context is cancelled.
type slowCompleter struct {
	mu        sync.Mutex
	cancelled []int // request IDs whose contexts were cancelled
}

func (s *slowCompleter) Complete(ctx context.Context, req *ashlet.Request) *ashlet.Response {
	<-ctx.Done()
	s.mu.Lock()
	s.cancelled = append(s.cancelled, req.RequestID)
	s.mu.Unlock()
	return &ashlet.Response{Candidates: []ashlet.Candidate{}}
}

func (s *slowCompleter) WarmContext(_ context.Context, _ string) {}

func (s *slowCompleter) Close() {}

func sendConfigRequest(t *testing.T, sockPath string, req *ashlet.ConfigRequest) *ashlet.ConfigResponse {
	t.Helper()
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	conn.Write(append(data, '\n'))

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response from server")
	}

	var resp ashlet.ConfigResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return &resp
}

func TestConfigDefaultsAction(t *testing.T) {
	stub := &stubCompleter{
		resp: &ashlet.Response{Candidates: []ashlet.Candidate{}},
	}
	srv := newTestServer(t, stub)

	resp := sendConfigRequest(t, srv.sockPath, &ashlet.ConfigRequest{Action: "defaults"})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	if resp.Config == nil {
		t.Fatal("expected non-nil config")
	}
	if resp.Config.Generation.Model == "" {
		t.Error("expected non-empty generation model")
	}
	if resp.Config.Embedding.Model == "" {
		t.Error("expected non-empty embedding model")
	}
}

func TestHandleConnCancelsOldSession(t *testing.T) {
	slow := &slowCompleter{}
	srv := newTestServer(t, slow)

	// Send first request (will block in Complete until cancelled).
	conn1, err := net.Dial("unix", srv.sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn1.Close()

	req1, _ := json.Marshal(&ashlet.Request{
		RequestID: 1,
		Input:     "git st",
		SessionID: "sess1",
	})
	conn1.Write(append(req1, '\n'))

	// Give the server time to start processing req1.
	time.Sleep(50 * time.Millisecond)

	// Send second request for the same session — should cancel req1.
	conn2, err := net.Dial("unix", srv.sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()

	req2, _ := json.Marshal(&ashlet.Request{
		RequestID: 2,
		Input:     "git status",
		SessionID: "sess1",
	})
	conn2.Write(append(req2, '\n'))

	// Give the server time to cancel req1 and start processing req2.
	time.Sleep(50 * time.Millisecond)

	slow.mu.Lock()
	found := false
	for _, id := range slow.cancelled {
		if id == 1 {
			found = true
			break
		}
	}
	slow.mu.Unlock()

	if !found {
		t.Error("expected request 1 to be cancelled when request 2 arrived for the same session")
	}
}

func sendContextRequest(t *testing.T, sockPath string, req *ashlet.ContextRequest) *ashlet.ContextResponse {
	t.Helper()
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	conn.Write(append(data, '\n'))

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response from server")
	}

	var resp ashlet.ContextResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return &resp
}

func TestHandleConnContextRequest(t *testing.T) {
	stub := &stubCompleter{
		resp: &ashlet.Response{Candidates: []ashlet.Candidate{}},
	}
	srv := newTestServer(t, stub)

	resp := sendContextRequest(t, srv.sockPath, &ashlet.ContextRequest{
		Type: "context",
		Cwd:  "/tmp",
	})

	if !resp.OK {
		t.Errorf("expected OK=true, got false")
	}
	if resp.Error != nil {
		t.Errorf("expected no error, got %+v", resp.Error)
	}
}

func TestHandleConnContextRequestNoCwd(t *testing.T) {
	stub := &stubCompleter{
		resp: &ashlet.Response{Candidates: []ashlet.Candidate{}},
	}
	srv := newTestServer(t, stub)

	resp := sendContextRequest(t, srv.sockPath, &ashlet.ContextRequest{
		Type: "context",
		Cwd:  "",
	})

	if resp.OK {
		t.Errorf("expected OK=false for empty cwd")
	}
	if resp.Error == nil {
		t.Errorf("expected error for empty cwd")
	}
}
