package dishwasher

import (
	domaindishwasher "home-go/internal/domain/devices/dishwasher"
	domainscheduler "home-go/internal/domain/scheduler"
)

func GetProfileForMode(mode Mode) (domainscheduler.Profile, error) {
	return domaindishwasher.GetProfileForMode(mode)
}
