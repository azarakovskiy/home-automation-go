package mcp

import (
	"net/http"

	"home-go/internal/config"
	"home-go/internal/tech/mcp/pricing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const version = "1.0.0"

// NewServer wires the MCP server with all resources and returns it wrapped in
// a streamable HTTP transport, ready to mount at /mcp.
func NewServer(pricingSrv pricing.PricingService) http.Handler {
	srv := server.NewMCPServer(
		config.AppName, version,
		server.WithResourceCapabilities(false, false),
		server.WithRecovery(),
	)

	pricingHandler := pricing.NewHandler(pricingSrv)
	srv.AddResource(mcp.NewResource(
		"pricing/current_price",
		"Current Electricity Price",
		mcp.WithResourceDescription("Current Electricity Price"),
		mcp.WithMIMEType("application/json"),
	), pricingHandler.CurrentPrice)

	return server.NewStreamableHTTPServer(srv)
}
