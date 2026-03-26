package dishwasher

import (
	domaindishwasher "home-go/internal/domain/devices/dishwasher"
	"home-go/optimization/scheduled"
)

func GetProfileForMode(mode Mode) (scheduled.Profile, error) {
	return domaindishwasher.GetProfileForMode(mode)
}
