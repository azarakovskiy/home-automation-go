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
