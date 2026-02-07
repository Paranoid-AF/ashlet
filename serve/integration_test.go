package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	ashlet "github.com/Paranoid-AF/ashlet"
)

func TestIntegrationRoundTrip(t *testing.T) {
	stub := &stubCompleter{
		resp: &ashlet.Response{
			Candidates: []ashlet.Candidate{
				{Completion: "status", Confidence: 0.95},
			},
		},
	}
	srv := newTestServer(t, stub)

	resp := sendRequest(t, srv.sockPath, &ashlet.Request{
		RequestID: 7,
		Input:     "git st",
		CursorPos: 6,
		Cwd:       "/tmp",
		SessionID: "test-session",
	})

	if resp.RequestID != 7 {
		t.Errorf("expected request_id 7, got %d", resp.RequestID)
	}
	if len(resp.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(resp.Candidates))
	}
	if resp.Candidates[0].Completion != "status" {
		t.Errorf("expected completion 'status', got %q", resp.Candidates[0].Completion)
	}
}

func TestIntegrationEmptyCandidates(t *testing.T) {
	stub := &stubCompleter{
		resp: &ashlet.Response{Candidates: []ashlet.Candidate{}},
	}
	srv := newTestServer(t, stub)

	conn, err := net.Dial("unix", srv.sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	req := &ashlet.Request{RequestID: 1, Input: "xyz"}
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

func TestIntegrationRequestIDSequence(t *testing.T) {
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

func TestIntegrationAPIError(t *testing.T) {
	stub := &stubCompleter{
		resp: &ashlet.Response{
			Candidates: []ashlet.Candidate{},
			Error: &ashlet.Error{
				Code:    "api_error",
				Message: "API connection failed",
			},
		},
	}
	srv := newTestServer(t, stub)

	conn, err := net.Dial("unix", srv.sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	req := &ashlet.Request{RequestID: 5, Input: "git"}
	data, _ := json.Marshal(req)
	conn.Write(append(data, '\n'))

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response")
	}

	raw := scanner.Text()
	if !strings.Contains(raw, `"candidates":[]`) {
		t.Errorf("expected candidates:[] even with error, got %s", raw)
	}
	if !strings.Contains(raw, `"api_error"`) {
		t.Errorf("expected api_error error, got %s", raw)
	}
}

func TestIntegrationMalformedRequest(t *testing.T) {
	stub := &stubCompleter{
		resp: &ashlet.Response{Candidates: []ashlet.Candidate{}},
	}
	srv := newTestServer(t, stub)

	// Send garbage
	conn, err := net.Dial("unix", srv.sockPath)
	if err != nil {
		t.Fatal(err)
	}
	conn.Write([]byte("not json\n"))
	conn.Close()

	// Server should survive — send a valid request after
	resp := sendRequest(t, srv.sockPath, &ashlet.Request{
		RequestID: 99,
		Input:     "test",
	})
	if resp.RequestID != 99 {
		t.Errorf("server should survive malformed request, expected id 99, got %d", resp.RequestID)
	}
}

func TestIntegrationConcurrent(t *testing.T) {
	stub := &stubCompleter{
		resp: &ashlet.Response{Candidates: []ashlet.Candidate{}},
	}
	srv := newTestServer(t, stub)

	const n = 10
	var wg sync.WaitGroup
	errs := make(chan string, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			resp := sendRequest(t, srv.sockPath, &ashlet.Request{
				RequestID: id,
				Input:     "concurrent",
			})
			if resp.RequestID != id {
				errs <- fmt.Sprintf("goroutine %d: expected id %d, got %d", id, id, resp.RequestID)
			}
		}(i + 1)
	}

	wg.Wait()
	close(errs)

	for e := range errs {
		t.Error(e)
	}
}
