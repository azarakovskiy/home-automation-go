package health

import (
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHandler_ServeHealth(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := nethttp.NewRequest("GET", "/health", nil)
	c.Request = req

	startTime := time.Now().Add(-3 * time.Hour)
	h := New(startTime)
	h.ServeHealth(c)

	if w.Code != nethttp.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf(`body["status"] = %q, want "ok"`, body["status"])
	}
	if body["uptime"] == "" {
		t.Fatal(`body["uptime"] is empty`)
	}
}
