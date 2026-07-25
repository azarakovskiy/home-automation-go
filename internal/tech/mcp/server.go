package mcp

import (
	"net/http"

	"home-go/internal/tech/mcp/pricing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ServerName follows the MCP best practice: {service}-mcp-server.
// Distinct from the app name so the MCP endpoint is identifiable in the
// client UI.
const ServerName = "home-go-mcp-server"

const version = "1.0.0"

// NewServer wires the MCP server with all resources and tools and returns it
// wrapped in a streamable HTTP transport, ready to mount at /mcp.
func NewServer(pricingSrv pricing.PricingService) http.Handler {
	srv := server.NewMCPServer(
		ServerName, version,
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

	srv.AddTool(mcp.NewTool("pricing_get_next_price",
		mcp.WithDescription("Return the next price change after now. Requires: window_minutes — how far ahead to look, in minutes."),
		mcp.WithNumber("window_minutes",
			mcp.Description("How far ahead to look for the next price change, in minutes."),
			mcp.Required(),
			mcp.Min(1),
		),
		mcp.WithTitleAnnotation("Next price change"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
	), pricingHandler.NextPrice)

	srv.AddTool(mcp.NewTool("pricing_get_daily_summary",
		mcp.WithDescription("Return a structured overview of prices from now until the last available slot: current price/level, median, thresholds, min/max/average, and cheap/expensive/negative windows."),
		mcp.WithTitleAnnotation("Daily price summary"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
	), pricingHandler.DailySummary)

	srv.AddTool(mcp.NewTool("pricing_find_cheapest_window",
		mcp.WithDescription("Find the cheapest consecutive block of slots for an appliance. Requires: duration_minutes (how long the appliance runs) and deadline_minutes (how far ahead to search, in minutes from now)."),
		mcp.WithNumber("duration_minutes",
			mcp.Description("How long the appliance runs, in minutes."),
			mcp.Required(),
			mcp.Min(1),
		),
		mcp.WithNumber("deadline_minutes",
			mcp.Description("How far ahead to search, in minutes from now."),
			mcp.Required(),
			mcp.Min(1),
		),
		mcp.WithTitleAnnotation("Cheapest run window"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
	), pricingHandler.CheapestWindow)

	return server.NewStreamableHTTPServer(srv)
}
