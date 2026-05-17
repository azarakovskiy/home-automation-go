package health

import (
	"context"
	"strings"
	"testing"
	"time"

	"home-go/internal/tech/homeassistant/component"

	ga "saml.dev/gome-assistant"
)

type mockHandle struct {
	lastValue string
	err       error
}

func (m *mockHandle) Set(_ context.Context, value string) error {
	m.lastValue = value
	return m.err
}

func TestComponent_Intervals_returnsOne(t *testing.T) {
	c := &Component{
		Base:      component.Base{},
		handle:    &mockHandle{},
		startTime: time.Now(),
	}
	if got := len(c.Intervals()); got != 1 {
		t.Fatalf("Intervals() length = %d, want 1", got)
	}
}

func TestComponent_publish_setsNonEmptyUptime(t *testing.T) {
	mock := &mockHandle{}
	c := &Component{
		Base:      component.Base{},
		handle:    mock,
		startTime: time.Now().Add(-2 * time.Hour),
	}

	c.publish((*ga.Service)(nil), ga.State(nil))

	if mock.lastValue == "" {
		t.Fatal("publish did not set any uptime value")
	}
	for _, sub := range []string{"ns", "µs", "ms"} {
		if strings.Contains(mock.lastValue, sub) {
			t.Fatalf("publish uptime %q contains sub-second unit %q", mock.lastValue, sub)
		}
	}
}
