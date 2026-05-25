package reminders

import (
	"context"

	"home-go/internal/tech/homeassistant/entities"
)

// DeviceRuntimeAdapter wraps *entities.DeviceRuntime to satisfy entityRuntime.
type DeviceRuntimeAdapter struct {
	dr *entities.DeviceRuntime
}

// AdaptDeviceRuntime wraps a *entities.DeviceRuntime for use with New.
func AdaptDeviceRuntime(dr *entities.DeviceRuntime) *DeviceRuntimeAdapter {
	return &DeviceRuntimeAdapter{dr: dr}
}

func (a *DeviceRuntimeAdapter) Switch(ctx context.Context, spec entities.SwitchSpec) (SwitchHandle, error) {
	return a.dr.Switch(ctx, spec)
}

func (a *DeviceRuntimeAdapter) Remove(ctx context.Context, key string) error {
	return a.dr.Remove(ctx, key)
}
