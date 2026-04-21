package scheduler

import (
	"fmt"
	"time"
)

// Plan is the minimal delayed-start state shared across devices.
type Plan struct {
	StartTime time.Time
}

// Store persists pending delayed-start state.
type Store interface {
	Save(Plan) error
	Restore() (*Plan, error)
	Clear() error
}

// Runner executes scheduled work and handles overdue restored schedules.
type Runner interface {
	StartNow() error
	HandleExpiredSchedule() error
}

// Scheduler owns the delayed-start lifecycle for a device.
type Scheduler struct {
	store   Store
	runner  Runner
	pending *Plan
}

func New(store Store, runner Runner) *Scheduler {
	return &Scheduler{
		store:  store,
		runner: runner,
	}
}

func (s *Scheduler) Schedule(plan Plan) error {
	if err := s.store.Save(plan); err != nil {
		return fmt.Errorf("save schedule: %w", err)
	}

	s.pending = &plan
	return nil
}

func (s *Scheduler) Restore(now time.Time) error {
	plan, err := s.store.Restore()
	if err != nil {
		return fmt.Errorf("restore schedule: %w", err)
	}
	if plan == nil {
		s.pending = nil
		return nil
	}

	if !plan.StartTime.After(now) {
		s.pending = nil
		if err := s.runner.HandleExpiredSchedule(); err != nil {
			return fmt.Errorf("handle expired schedule: %w", err)
		}
		if err := s.store.Clear(); err != nil {
			return fmt.Errorf("clear expired schedule: %w", err)
		}
		return nil
	}

	s.pending = plan
	return nil
}

func (s *Scheduler) Tick(now time.Time) error {
	if s.pending == nil || now.Before(s.pending.StartTime) {
		return nil
	}

	if err := s.runner.StartNow(); err != nil {
		return fmt.Errorf("start scheduled device: %w", err)
	}

	s.pending = nil
	if err := s.store.Clear(); err != nil {
		return fmt.Errorf("clear started schedule: %w", err)
	}

	return nil
}

func (s *Scheduler) Cancel() error {
	if err := s.store.Clear(); err != nil {
		return fmt.Errorf("clear schedule: %w", err)
	}

	s.pending = nil
	return nil
}

func (s *Scheduler) HasPending() bool {
	return s.pending != nil
}

func (s *Scheduler) Pending() *Plan {
	if s.pending == nil {
		return nil
	}

	plan := *s.pending
	return &plan
}
