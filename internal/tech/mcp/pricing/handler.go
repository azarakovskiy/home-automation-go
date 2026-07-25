package pricing

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

//go:generate mockgen -destination=../../../mocks/tech/mcp/pricing/pricing_service.go -package=pricing home-go/internal/tech/mcp/pricing PricingService
type PricingService interface {
	GetCurrentPrice() (float64, error)
	GetNextPrice(windowMinutes int) (NextPriceInfo, error)
	GetPriceSummary() (PriceSummary, error)
	FindCheapestWindow(durationMinutes, deadlineMinutes int) (CheapestWindow, error)
}

type Handler struct {
	pricingSrv PricingService
}

func NewHandler(pricingSrv PricingService) *Handler {
	return &Handler{
		pricingSrv: pricingSrv,
	}
}

func (h *Handler) CurrentPrice(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	price, err := h.pricingSrv.GetCurrentPrice()
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(PriceResponse{Price: price})
	if err != nil {
		return nil, fmt.Errorf("marshal price response: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func (h *Handler) NextPrice(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	windowMinutes, err := request.RequireInt("window_minutes")
	if err != nil {
		return nil, fmt.Errorf("invalid window_minutes: %w", err)
	}

	info, err := h.pricingSrv.GetNextPrice(windowMinutes)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("get next price failed", err), nil
	}

	return mcp.NewToolResultJSON(info)
}

func (h *Handler) DailySummary(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	summary, err := h.pricingSrv.GetPriceSummary()
	if err != nil {
		return mcp.NewToolResultErrorFromErr("get price summary failed", err), nil
	}

	return mcp.NewToolResultJSON(summary)
}

func (h *Handler) CheapestWindow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	durationMinutes, err := request.RequireInt("duration_minutes")
	if err != nil {
		return nil, fmt.Errorf("invalid duration_minutes: %w", err)
	}
	deadlineMinutes, err := request.RequireInt("deadline_minutes")
	if err != nil {
		return nil, fmt.Errorf("invalid deadline_minutes: %w", err)
	}
	if durationMinutes > deadlineMinutes {
		return mcp.NewToolResultError(fmt.Sprintf(
			"deadline_minutes (%d) must be at least duration_minutes (%d)",
			deadlineMinutes, durationMinutes,
		)), nil
	}

	window, err := h.pricingSrv.FindCheapestWindow(durationMinutes, deadlineMinutes)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("find cheapest window failed", err), nil
	}

	return mcp.NewToolResultJSON(window)
}
