package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	mcppkg "home-go/internal/mocks/tech/mcp/pricing"

	"go.uber.org/mock/gomock"
)

// jsonRPCResponse matches the fields we care about from the streamable HTTP reply.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post request: %v", err)
	}
	return resp
}

func postJSONWithSession(t *testing.T, url, sessionID string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post request: %v", err)
	}
	return resp
}

// initializeSession sends the MCP initialize handshake and returns the
// session ID the server issues. Subsequent JSON-RPC calls must include it.
func initializeSession(t *testing.T, url string) string {
	t.Helper()
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
		},
	}
	resp := postJSON(t, url, req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("initialize status = %d, want 200; body = %s", resp.StatusCode, body)
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize response missing Mcp-Session-Id header")
	}
	return sessionID
}

func TestNewServer_NotNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mcppkg.NewMockPricingService(ctrl)

	h := NewServer(svc)
	if h == nil {
		t.Fatal("NewServer() = nil, want non-nil handler")
	}
}

// TestNewServer_ServesCurrentPrice is the end-to-end check: if the resource
// isn't registered, the path is wrong, or the JSON-RPC transport is broken,
// this test fails.
func TestNewServer_ServesCurrentPrice(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mcppkg.NewMockPricingService(ctrl)
	svc.EXPECT().GetCurrentPrice().Return(0.42, nil)

	srv := httptest.NewServer(NewServer(svc))
	defer srv.Close()

	sessionID := initializeSession(t, srv.URL)

	readReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "resources/read",
		"params":  map[string]any{"uri": "pricing/current_price"},
	}
	resp := postJSONWithSession(t, srv.URL, sessionID, readReq)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var rpc jsonRPCResponse
	if err := json.Unmarshal(body, &rpc); err != nil {
		t.Fatalf("unmarshal jsonrpc response: %v; body = %s", err, body)
	}
	if rpc.Error != nil {
		t.Fatalf("jsonrpc error: code=%d message=%q", rpc.Error.Code, rpc.Error.Message)
	}
	if rpc.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want %q", rpc.JSONRPC, "2.0")
	}

	// Result shape: {"contents": [{"uri": "...", "mimeType": "...", "text": "..."}]}
	var result struct {
		Contents []struct {
			URI      string `json:"uri"`
			MIMEType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(rpc.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("contents len = %d, want 1", len(result.Contents))
	}
	c := result.Contents[0]
	if c.URI != "pricing/current_price" {
		t.Errorf("contents[0].uri = %q, want %q", c.URI, "pricing/current_price")
	}
	if c.MIMEType != "application/json" {
		t.Errorf("contents[0].mimeType = %q, want %q", c.MIMEType, "application/json")
	}

	var price map[string]float64
	if err := json.Unmarshal([]byte(c.Text), &price); err != nil {
		t.Fatalf("unmarshal price text %q: %v", c.Text, err)
	}
	if price["price"] != 0.42 {
		t.Errorf("price = %v, want 0.42", price["price"])
	}
}
