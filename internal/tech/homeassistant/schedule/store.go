package schedule

import (
	"context"
	"fmt"
	"strings"
	"time"

	domainscheduler "home-go/internal/domain/scheduler"
	"home-go/internal/tech/homeassistant/entities"

	ga "saml.dev/gome-assistant"
)

type Config struct {
	Scheduled entities.SwitchSpec
	StartTime entities.TextSensorSpec
}

type Store struct {
	state     ga.State
	scheduled scheduleSwitch
	startTime textState
}

type scheduleSwitch interface {
	On(context.Context) error
	Off(context.Context) error
	OnCommand(func(context.Context, bool) error) error
	EntityID() string
}

type textState interface {
	Set(context.Context, string) error
	EntityID() string
}

func NewStore(runtime *entities.Runtime, state ga.State, config Config) (*Store, error) {
	if runtime == nil {
		return nil, fmt.Errorf("runtime entities are required")
	}

	ctx := context.Background()

	scheduled, err := runtime.Switch(ctx, config.Scheduled)
	if err != nil {
		return nil, fmt.Errorf("declare scheduled switch: %w", err)
	}

	startTime, err := runtime.TextSensor(ctx, config.StartTime)
	if err != nil {
		return nil, fmt.Errorf("declare scheduled time sensor: %w", err)
	}

	return newStoreFromHandles(state, scheduled, startTime), nil
}

func newStoreFromHandles(state ga.State, scheduled scheduleSwitch, startTime textState) *Store {
	return &Store{
		state:     state,
		scheduled: scheduled,
		startTime: startTime,
	}
}

func (s *Store) Save(plan domainscheduler.Plan) error {
	ctx := context.Background()

	if err := s.scheduled.On(ctx); err != nil {
		return fmt.Errorf("set scheduled flag: %w", err)
	}
	if err := s.startTime.Set(ctx, plan.StartTime.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("set scheduled time: %w", err)
	}

	return nil
}

func (s *Store) Restore() (*domainscheduler.Plan, error) {
	isScheduled, err := s.loadScheduledFlag()
	if err != nil {
		return nil, err
	}
	if !isScheduled {
		return nil, nil
	}

	state, err := s.getRequiredState(s.startTime.EntityID(), "scheduled time")
	if err != nil {
		return nil, err
	}

	startTime, err := ParseScheduledTime(state.State)
	if err != nil {
		return nil, fmt.Errorf("parse scheduled time: %w", err)
	}

	return &domainscheduler.Plan{StartTime: startTime}, nil
}

func (s *Store) Clear() error {
	ctx := context.Background()

	if err := s.scheduled.Off(ctx); err != nil {
		return fmt.Errorf("clear scheduled flag: %w", err)
	}
	if err := s.startTime.Set(ctx, ""); err != nil {
		return fmt.Errorf("clear scheduled time: %w", err)
	}

	return nil
}

func (s *Store) OnCommand(fn func(context.Context, bool) error) error {
	return s.scheduled.OnCommand(fn)
}

func (s *Store) SetScheduledFlag(ctx context.Context, on bool) error {
	if on {
		return s.scheduled.On(ctx)
	}
	return s.scheduled.Off(ctx)
}

func (s *Store) loadScheduledFlag() (bool, error) {
	state, err := s.state.Get(s.scheduled.EntityID())
	if err != nil {
		if IsMissingEntityError(err) {
			return false, nil
		}
		return false, fmt.Errorf("get scheduled flag: %w", err)
	}

	return state.State == "on", nil
}

func (s *Store) getRequiredState(entityID string, label string) (ga.EntityState, error) {
	state, err := s.state.Get(entityID)
	if err != nil {
		return ga.EntityState{}, fmt.Errorf("get %s: %w", label, err)
	}
	return state, nil
}

func ParseScheduledTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed, nil
	}

	parsed, fallbackErr := time.Parse("2006-01-02 15:04:05", value)
	if fallbackErr == nil {
		return parsed, nil
	}

	return time.Time{}, err
}

func IsMissingEntityError(err error) bool {
	if err == nil {
		return false
	}

	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "not found")
}
