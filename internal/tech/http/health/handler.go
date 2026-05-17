package health

import (
	nethttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler serves the /health endpoint.
type Handler struct {
	startTime time.Time
}

// New creates a Handler that measures uptime from startTime.
func New(startTime time.Time) *Handler {
	return &Handler{startTime: startTime}
}

// ServeHealth handles GET /health.
func (h *Handler) ServeHealth(c *gin.Context) {
	c.JSON(nethttp.StatusOK, gin.H{
		"status": "ok",
		"uptime": time.Since(h.startTime).Round(time.Second).String(),
	})
}
