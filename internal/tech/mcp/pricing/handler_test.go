package pricing_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"home-go/internal/mocks/tech/mcp/pricing"
	mcppkg "home-go/internal/tech/mcp/pricing"

	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/mock/gomock"
)

func TestNewHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	h := mcppkg.NewHandler(svc)
	if h == nil {
		t.Fatal("NewHandler() = nil, want non-nil")
	}
}

func TestCurrentPrice_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	svc.EXPECT().GetCurrentPrice().Return(0.1234, nil)

	h := mcppkg.NewHandler(svc)
	contents, err := h.CurrentPrice(context.Background(), mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: "pricing/current_price"},
	})
	if err != nil {
		t.Fatalf("CurrentPrice() error = %v", err)
	}

	if len(contents) != 1 {
		t.Fatalf("CurrentPrice() returned %d contents, want 1", len(contents))
	}

	text, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("CurrentPrice() contents[0] type = %T, want TextResourceContents", contents[0])
	}
	if text.URI != "pricing/current_price" {
		t.Errorf("TextResourceContents.URI = %q, want %q", text.URI, "pricing/current_price")
	}
	if text.MIMEType != "application/json" {
		t.Errorf("TextResourceContents.MIMEType = %q, want %q", text.MIMEType, "application/json")
	}

	var got mcppkg.PriceResponse
	if err := json.Unmarshal([]byte(text.Text), &got); err != nil {
		t.Fatalf("unmarshal Text: %v", err)
	}
	if got.Price != 0.1234 {
		t.Errorf("PriceResponse.Price = %v, want 0.1234", got.Price)
	}
}

func TestCurrentPrice_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	wantErr := errors.New("pricing unavailable")
	svc.EXPECT().GetCurrentPrice().Return(0.0, wantErr)

	h := mcppkg.NewHandler(svc)
	_, err := h.CurrentPrice(context.Background(), mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: "pricing/current_price"},
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("CurrentPrice() error = %v, want %v", err, wantErr)
	}
}

func TestNextPrice_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	want := mcppkg.NextPriceInfo{
		Price: 0.18,
		From:  "2025-01-01T14:00:00Z",
		Till:  "2025-01-01T15:00:00Z",
		Level: "expensive",
	}
	svc.EXPECT().GetNextPrice(60).Return(want, nil)

	h := mcppkg.NewHandler(svc)
	result, err := h.NextPrice(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "pricing_get_next_price",
			Arguments: map[string]any{"window_minutes": float64(60)},
		},
	})
	if err != nil {
		t.Fatalf("NextPrice() error = %v", err)
	}
	if result == nil {
		t.Fatal("NextPrice() result = nil")
	}

	text := firstText(t, result)
	var got mcppkg.NextPriceInfo
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != want {
		t.Errorf("NextPriceInfo = %+v, want %+v", got, want)
	}
}

func TestNextPrice_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	wantErr := errors.New("no upcoming price")
	svc.EXPECT().GetNextPrice(60).Return(mcppkg.NextPriceInfo{}, wantErr)

	h := mcppkg.NewHandler(svc)
	result, err := h.NextPrice(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "pricing_get_next_price",
			Arguments: map[string]any{"window_minutes": float64(60)},
		},
	})
	if err != nil {
		t.Fatalf("NextPrice() error = %v", err)
	}
	if !result.IsError {
		t.Errorf("NextPrice() result.IsError = false, want true")
	}
}

func TestNextPrice_MissingParam(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	h := mcppkg.NewHandler(svc)
	_, err := h.NextPrice(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "pricing_get_next_price", Arguments: map[string]any{}},
	})
	if err == nil {
		t.Fatal("NextPrice() with missing param error = nil, want non-nil")
	}
}

func TestDailySummary_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	want := mcppkg.PriceSummary{
		CurrentPrice:       0.12,
		CurrentLevel:       "average",
		MedianPrice:        0.11,
		CheapThreshold:     0.08,
		ExpensiveThreshold: 0.15,
		MinPrice:           -0.02,
		MaxPrice:           0.35,
		AveragePrice:       0.12,
		CheapWindows: []mcppkg.PriceWindow{
			{From: "2025-01-01T02:00:00Z", Till: "2025-01-01T05:00:00Z", AvgPrice: 0.05},
		},
		ExpensiveWindows: []mcppkg.PriceWindow{
			{From: "2025-01-01T18:00:00Z", Till: "2025-01-01T21:00:00Z", AvgPrice: 0.32},
		},
	}
	svc.EXPECT().GetPriceSummary().Return(want, nil)

	h := mcppkg.NewHandler(svc)
	result, err := h.DailySummary(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "pricing_get_daily_summary"},
	})
	if err != nil {
		t.Fatalf("DailySummary() error = %v", err)
	}

	text := firstText(t, result)
	var got mcppkg.PriceSummary
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PriceSummary = %+v, want %+v", got, want)
	}
}

func TestDailySummary_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	svc.EXPECT().GetPriceSummary().Return(mcppkg.PriceSummary{}, errors.New("no slots"))

	h := mcppkg.NewHandler(svc)
	result, err := h.DailySummary(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "pricing_get_daily_summary"},
	})
	if err != nil {
		t.Fatalf("DailySummary() error = %v", err)
	}
	if !result.IsError {
		t.Errorf("DailySummary() result.IsError = false, want true")
	}
}

func TestCheapestWindow_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	want := mcppkg.CheapestWindow{
		From:     "2025-01-01T03:00:00Z",
		Till:     "2025-01-01T05:00:00Z",
		AvgPrice: 0.06,
		Level:    "cheap",
	}
	svc.EXPECT().FindCheapestWindow(120, 720).Return(want, nil)

	h := mcppkg.NewHandler(svc)
	result, err := h.CheapestWindow(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "pricing_find_cheapest_window",
			Arguments: map[string]any{"duration_minutes": float64(120), "deadline_minutes": float64(720)},
		},
	})
	if err != nil {
		t.Fatalf("CheapestWindow() error = %v", err)
	}

	text := firstText(t, result)
	var got mcppkg.CheapestWindow
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != want {
		t.Errorf("CheapestWindow = %+v, want %+v", got, want)
	}
}

func TestCheapestWindow_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	svc.EXPECT().FindCheapestWindow(120, 720).Return(mcppkg.CheapestWindow{}, errors.New("no window"))

	h := mcppkg.NewHandler(svc)
	result, err := h.CheapestWindow(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "pricing_find_cheapest_window",
			Arguments: map[string]any{"duration_minutes": float64(120), "deadline_minutes": float64(720)},
		},
	})
	if err != nil {
		t.Fatalf("CheapestWindow() error = %v", err)
	}
	if !result.IsError {
		t.Errorf("CheapestWindow() result.IsError = false, want true")
	}
}

func TestCheapestWindow_MissingParam(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	h := mcppkg.NewHandler(svc)
	_, err := h.CheapestWindow(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "pricing_find_cheapest_window",
			Arguments: map[string]any{"duration_minutes": float64(120)},
		},
	})
	if err == nil {
		t.Fatal("CheapestWindow() with missing deadline error = nil, want non-nil")
	}
}

func TestCheapestWindow_DeadlineLessThanDuration(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	// Service must not be called: the handler validates before delegating.
	h := mcppkg.NewHandler(svc)
	result, err := h.CheapestWindow(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "pricing_find_cheapest_window",
			Arguments: map[string]any{"duration_minutes": float64(120), "deadline_minutes": float64(60)},
		},
	})
	if err != nil {
		t.Fatalf("CheapestWindow() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("CheapestWindow() result.IsError = false, want true")
	}
}

// firstText extracts the first text content from a tool result. Tool results
// in this codebase always carry JSON; tests decode the text and compare structs.
func firstText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] type = %T, want TextContent", result.Content[0])
	}
	return tc.Text
}
