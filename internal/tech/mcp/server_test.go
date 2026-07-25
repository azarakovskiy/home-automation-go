package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	mcppkg "home-go/internal/mocks/tech/mcp/pricing"
	mcppkgpricing "home-go/internal/tech/mcp/pricing"

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

// TestNewServer_ServesNextPrice is the end-to-end check for the
// pricing_get_next_price tool.
func TestNewServer_ServesNextPrice(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mcppkg.NewMockPricingService(ctrl)
	want := mcppkgpricing.NextPriceInfo{
		Price: 0.18,
		From:  "2025-01-01T14:00:00Z",
		Till:  "2025-01-01T15:00:00Z",
		Level: "expensive",
	}
	svc.EXPECT().GetNextPrice(120).Return(want, nil)

	srv := httptest.NewServer(NewServer(svc))
	defer srv.Close()
	sessionID := initializeSession(t, srv.URL)

	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "pricing_get_next_price",
			"arguments": map[string]any{"window_minutes": float64(120)},
		},
	}
	resp := postJSONWithSession(t, srv.URL, sessionID, callReq)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var rpc jsonRPCResponse
	if err := json.Unmarshal(body, &rpc); err != nil {
		t.Fatalf("unmarshal jsonrpc: %v; body = %s", err, body)
	}
	if rpc.Error != nil {
		t.Fatalf("jsonrpc error: code=%d message=%q", rpc.Error.Code, rpc.Error.Message)
	}

	// Result shape: {"content": [{"type": "text", "text": "<json>"}], "isError": false}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(rpc.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.IsError {
		t.Fatalf("result.IsError = true, want false")
	}
	if len(result.Content) != 1 {
		t.Fatalf("content len = %d, want 1", len(result.Content))
	}

	var got mcppkgpricing.NextPriceInfo
	if err := json.Unmarshal([]byte(result.Content[0].Text), &got); err != nil {
		t.Fatalf("unmarshal tool text %q: %v", result.Content[0].Text, err)
	}
	if got != want {
		t.Errorf("NextPriceInfo = %+v, want %+v", got, want)
	}
}

// TestNewServer_ServesDailySummary is the end-to-end check for the
// pricing_get_daily_summary tool.
func TestNewServer_ServesDailySummary(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mcppkg.NewMockPricingService(ctrl)
	want := mcppkgpricing.PriceSummary{
		CurrentPrice:       0.12,
		CurrentLevel:       "average",
		MedianPrice:        0.11,
		CheapThreshold:     0.08,
		ExpensiveThreshold: 0.15,
		MinPrice:           -0.02,
		MaxPrice:           0.35,
		AveragePrice:       0.12,
		CheapWindows: []mcppkgpricing.PriceWindow{
			{From: "2025-01-01T02:00:00Z", Till: "2025-01-01T05:00:00Z", AvgPrice: 0.05},
		},
	}
	svc.EXPECT().GetPriceSummary().Return(want, nil)

	srv := httptest.NewServer(NewServer(svc))
	defer srv.Close()
	sessionID := initializeSession(t, srv.URL)

	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params":  map[string]any{"name": "pricing_get_daily_summary"},
	}
	resp := postJSONWithSession(t, srv.URL, sessionID, callReq)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var rpc jsonRPCResponse
	if err := json.Unmarshal(body, &rpc); err != nil {
		t.Fatalf("unmarshal jsonrpc: %v; body = %s", err, body)
	}
	if rpc.Error != nil {
		t.Fatalf("jsonrpc error: code=%d message=%q", rpc.Error.Code, rpc.Error.Message)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(rpc.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content len = %d, want 1", len(result.Content))
	}

	var got mcppkgpricing.PriceSummary
	if err := json.Unmarshal([]byte(result.Content[0].Text), &got); err != nil {
		t.Fatalf("unmarshal tool text: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PriceSummary = %+v, want %+v", got, want)
	}
}

// TestNewServer_ServesCheapestWindow is the end-to-end check for the
// pricing_find_cheapest_window tool.
func TestNewServer_ServesCheapestWindow(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mcppkg.NewMockPricingService(ctrl)
	want := mcppkgpricing.CheapestWindow{
		From:     "2025-01-01T03:00:00Z",
		Till:     "2025-01-01T05:00:00Z",
		AvgPrice: 0.06,
		Level:    "cheap",
	}
	svc.EXPECT().FindCheapestWindow(120, 720).Return(want, nil)

	srv := httptest.NewServer(NewServer(svc))
	defer srv.Close()
	sessionID := initializeSession(t, srv.URL)

	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "pricing_find_cheapest_window",
			"arguments": map[string]any{"duration_minutes": float64(120), "deadline_minutes": float64(720)},
		},
	}
	resp := postJSONWithSession(t, srv.URL, sessionID, callReq)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var rpc jsonRPCResponse
	if err := json.Unmarshal(body, &rpc); err != nil {
		t.Fatalf("unmarshal jsonrpc: %v; body = %s", err, body)
	}
	if rpc.Error != nil {
		t.Fatalf("jsonrpc error: code=%d message=%q", rpc.Error.Code, rpc.Error.Message)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(rpc.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content len = %d, want 1", len(result.Content))
	}

	var got mcppkgpricing.CheapestWindow
	if err := json.Unmarshal([]byte(result.Content[0].Text), &got); err != nil {
		t.Fatalf("unmarshal tool text: %v", err)
	}
	if got != want {
		t.Errorf("CheapestWindow = %+v, want %+v", got, want)
	}
}
