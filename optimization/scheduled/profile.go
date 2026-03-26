package scheduled

import domainscheduler "home-go/internal/domain/scheduler"

const (
	WeightIdle   = domainscheduler.WeightIdle
	WeightLow    = domainscheduler.WeightLow
	WeightMedium = domainscheduler.WeightMedium
	WeightHigh   = domainscheduler.WeightHigh
	WeightMax    = domainscheduler.WeightMax
)

type Profile = domainscheduler.Profile
