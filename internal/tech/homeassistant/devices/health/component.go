package health

import (
	"context"
	"fmt"
	"log"
	"time"

	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/entities"

	ga "saml.dev/gome-assistant"
)

// sensorHandle is the subset of *entities.TextSensorHandle used by the component.
type sensorHandle interface {
	Set(ctx context.Context, value string) error
}

// runtimeAdapter wraps *entities.Runtime so it satisfies a testable interface.
type runtimeAdapter struct {
	rt *entities.Runtime
}

func (r *runtimeAdapter) textSensor(ctx context.Context, spec entities.TextSensorSpec) (sensorHandle, error) {
	return r.rt.TextSensor(ctx, spec)
}

// Component publishes the service uptime as an MQTT text sensor every minute.
type Component struct {
	component.Base
	handle    sensorHandle
	startTime time.Time
}

// New declares the MQTT health uptime sensor and returns a ready Component.
func New(ctx context.Context, base component.Base, runtime *entities.Runtime, startTime time.Time) (*Component, error) {
	adapter := &runtimeAdapter{rt: runtime}
	handle, err := adapter.textSensor(ctx, entities.TextSensorSpec{
		CommonSpec: entities.CommonSpec{
			Key:  "health_uptime",
			Name: "Home Go Uptime",
			Icon: "mdi:clock-outline",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("declare health uptime sensor: %w", err)
	}
	return &Component{
		Base:      base,
		handle:    handle,
		startTime: startTime,
	}, nil
}

// Intervals returns the single 1-minute publish tick.
func (c *Component) Intervals() []ga.Interval {
	tick := ga.NewInterval().
		Call(c.publish).
		Every("1m").
		Build()
	return []ga.Interval{tick}
}

func (c *Component) publish(_ *ga.Service, _ ga.State) {
	ctx := context.Background()
	uptime := time.Since(c.startTime).Round(time.Second).String()
	if err := c.handle.Set(ctx, uptime); err != nil {
		log.Printf("ERROR: health: publish uptime: %v", err)
	}
}
