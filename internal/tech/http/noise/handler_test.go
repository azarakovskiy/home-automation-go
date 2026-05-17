package noise

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestContext(ctx context.Context, noiseType string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := nethttp.NewRequestWithContext(ctx, "GET", "/noise/"+noiseType, nil)
	c.Request = req
	c.Params = gin.Params{{Key: "type", Value: noiseType}}
	return c, w
}

func TestHandler_ServeNoise_white(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	c, w := newTestContext(ctx, "white")
	h := &Handler{}
	h.ServeNoise(c)

	if w.Code != nethttp.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "audio/wav" {
		t.Fatalf("Content-Type = %q, want audio/wav", ct)
	}
	body := w.Body.Bytes()
	if len(body) < 44 {
		t.Fatalf("body length = %d, want at least 44 (WAV header)", len(body))
	}
	if string(body[0:4]) != "RIFF" {
		t.Fatalf("bytes[0:4] = %q, want RIFF", body[0:4])
	}
	if string(body[8:12]) != "WAVE" {
		t.Fatalf("bytes[8:12] = %q, want WAVE", body[8:12])
	}
}

func TestHandler_ServeNoise_pink(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	c, w := newTestContext(ctx, "pink")
	h := &Handler{}
	h.ServeNoise(c)

	if w.Code != nethttp.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "audio/wav" {
		t.Fatalf("Content-Type = %q, want audio/wav", ct)
	}
}

func TestHandler_ServeNoise_unknownType(t *testing.T) {
	ctx := context.Background()
	c, w := newTestContext(ctx, "ogg")
	h := &Handler{}
	h.ServeNoise(c)

	if w.Code != nethttp.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}
