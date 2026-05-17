package http

import (
	"context"
	"fmt"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
)

// Server wraps a Gin engine and an http.Server for graceful shutdown.
type Server struct {
	srv *nethttp.Server
}

// NewServer creates a Gin server bound to host:port.
// Routes registered: GET /health and GET /noise/:type.
// gin.Logger is intentionally omitted to suppress health-check log spam.
func NewServer(host string, port int, noiseHandler gin.HandlerFunc, healthHandler gin.HandlerFunc) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.GET("/health", healthHandler)
	engine.GET("/noise/:type", noiseHandler)

	return &Server{
		srv: &nethttp.Server{
			Addr:    fmt.Sprintf("%s:%d", host, port),
			Handler: engine,
		},
	}
}

// Start listens and serves until ctx is cancelled, then shuts down gracefully.
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != nethttp.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return s.srv.Shutdown(context.Background())
	}
}
