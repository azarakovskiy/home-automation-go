package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"home-go/internal/mocks/tech/mcp/pricing"

	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/mock/gomock"
)

func TestNewHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	h := NewHandler(svc)
	if h == nil {
		t.Fatal("NewHandler() = nil, want non-nil")
	}
	if h.pricingSrv != svc {
		t.Fatal("NewHandler() did not store pricingSrv")
	}
}

func TestCurrentPrice_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := pricing.NewMockPricingService(ctrl)
	svc.EXPECT().GetCurrentPrice().Return(0.1234, nil)

	h := NewHandler(svc)
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

	var got PriceResponse
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

	h := NewHandler(svc)
	_, err := h.CurrentPrice(context.Background(), mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: "pricing/current_price"},
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("CurrentPrice() error = %v, want %v", err, wantErr)
	}
}
