package reminders

import "errors"

var (
	ErrNotFound        = errors.New("reminder not found")
	ErrNoTargets       = errors.New("reminder must have at least one target")
	ErrInvalidSchedule = errors.New("invalid reminder schedule")
	ErrInvalidPolicy   = errors.New("invalid completion policy")
	ErrNotTarget       = errors.New("user is not a target of this reminder")
	ErrNotActive       = errors.New("reminder is not active")
)
