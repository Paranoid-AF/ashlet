package ashlet

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResponseCandidatesEmptyNotNull(t *testing.T) {
	resp := Response{Candidates: []Candidate{}}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"candidates":[]`) {
		t.Errorf("expected candidates:[], got %s", data)
	}
}

func TestResponseCandidatesNilMarshalIsNull(t *testing.T) {
	resp := Response{}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"candidates":null`) {
		t.Errorf("expected candidates:null for nil slice, got %s", data)
	}
}

func TestRequestIDJSONRoundTrip(t *testing.T) {
	req := Request{RequestID: 42, Input: "git st", CursorPos: 6}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	// Verify raw JSON uses "request_id" key
	if !strings.Contains(string(data), `"request_id"`) {
		t.Errorf("expected request_id key in JSON, got %s", data)
	}

	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.RequestID != 42 {
		t.Errorf("expected RequestID 42, got %d", decoded.RequestID)
	}

	// Response round-trip
	resp := Response{RequestID: 42, Candidates: []Candidate{}}
	data, err = json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"request_id"`) {
		t.Errorf("expected request_id key in response JSON, got %s", data)
	}

	var decodedResp Response
	if err := json.Unmarshal(data, &decodedResp); err != nil {
		t.Fatal(err)
	}
	if decodedResp.RequestID != 42 {
		t.Errorf("expected response RequestID 42, got %d", decodedResp.RequestID)
	}
}

func TestResponseErrorOmittedWhenNil(t *testing.T) {
	resp := Response{Candidates: []Candidate{}}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"error"`) {
		t.Errorf("expected no error key, got %s", data)
	}
}

func TestCandidateCursorPosOmittedWhenNil(t *testing.T) {
	c := Candidate{Completion: "git status", Confidence: 0.95}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"cursor_pos"`) {
		t.Errorf("expected cursor_pos to be omitted when nil, got %s", data)
	}
}

func TestCandidateCursorPosIncludedWhenSet(t *testing.T) {
	pos := 15
	c := Candidate{Completion: "git commit -m \"\"", CursorPos: &pos, Confidence: 0.95}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"cursor_pos":15`) {
		t.Errorf("expected cursor_pos:15, got %s", data)
	}
}

func TestCandidateCursorPosZeroIncluded(t *testing.T) {
	pos := 0
	c := Candidate{Completion: "git status", CursorPos: &pos, Confidence: 0.95}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"cursor_pos":0`) {
		t.Errorf("expected cursor_pos:0 to be included, got %s", data)
	}
}

func TestResponseErrorIncluded(t *testing.T) {
	resp := Response{
		Candidates: []Candidate{},
		Error: &Error{
			Code:    "api_error",
			Message: "something went wrong",
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"error"`) {
		t.Error("expected error key in JSON")
	}
	if !strings.Contains(s, `"api_error"`) {
		t.Error("expected api_error code")
	}
}
