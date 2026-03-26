package dishwasher

import (
	domaindishwasher "home-go/internal/domain/devices/dishwasher"
	domainscheduler "home-go/internal/domain/scheduler"
)

const (
	ModeAuto      = domaindishwasher.ModeAuto
	ModeAutoQuick = domaindishwasher.ModeAutoQuick
	ModeCancel    = domaindishwasher.ModeCancel
)

type Mode = domaindishwasher.Mode

func GetProfileForMode(mode Mode) (domainscheduler.Profile, error) {
	return domaindishwasher.GetProfileForMode(mode)
}
