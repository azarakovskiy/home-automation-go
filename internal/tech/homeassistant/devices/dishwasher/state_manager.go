package dishwasher

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	domaindishwasher "home-go/internal/domain/devices/dishwasher"
	"home-go/internal/domain/optimizer"
	"home-go/internal/tech/homeassistant/entities"

	ga "saml.dev/gome-assistant"
)

const (
	dishwasherScheduledKey         = "kitchen_dishwasher_is_scheduled"
	dishwasherScheduledModeKey     = "kitchen_dishwasher_scheduled_mode"
	dishwasherScheduledStartKey    = "kitchen_dishwasher_scheduled_start"
	dishwasherEstimatedCostKey     = "kitchen_dishwasher_estimated_cost"
	dishwasherCurrentCostKey       = "kitchen_dishwasher_current_cost"
	dishwasherSavingsPercentKey    = "kitchen_dishwasher_savings_percent"
	dishwasherScheduledModeCleared = "none"
)

// StateManager persists dishwasher schedule state through runtime MQTT entities.
type StateManager struct {
	state      ga.State
	controller *Controller

	scheduled      scheduleSwitch
	mode           textState
	startTime      textState
	estimatedCost  numberState
	currentCost    numberState
	savingsPercent numberState
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

type numberState interface {
	Set(context.Context, float64) error
	EntityID() string
}

func NewStateManager(runtime *entities.Runtime, state ga.State, controller *Controller) (*StateManager, error) {
	if runtime == nil {
		return nil, fmt.Errorf("runtime entities are required")
	}

	ctx := context.Background()

	scheduled, err := runtime.Switch(ctx, entities.SwitchSpec{
		CommonSpec: entities.CommonSpec{
			Key:          dishwasherScheduledKey,
			Name:         "Kitchen Dishwasher: Is Scheduled",
			EntityIDHint: "switch.kitchen_dishwasher_is_scheduled",
			Icon:         "mdi:dishwasher-alert",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("declare scheduled switch: %w", err)
	}

	mode, err := runtime.TextSensor(ctx, entities.TextSensorSpec{
		CommonSpec: entities.CommonSpec{
			Key:          dishwasherScheduledModeKey,
			Name:         "Kitchen Dishwasher: Scheduled Mode",
			EntityIDHint: "sensor.kitchen_dishwasher_scheduled_mode",
			Icon:         "mdi:dishwasher",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("declare mode sensor: %w", err)
	}

	startTime, err := runtime.TextSensor(ctx, entities.TextSensorSpec{
		CommonSpec: entities.CommonSpec{
			Key:          dishwasherScheduledStartKey,
			Name:         "Kitchen Dishwasher: Scheduled Start Time",
			EntityIDHint: "sensor.kitchen_dishwasher_scheduled_start",
			Icon:         "mdi:clock-start",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("declare start time sensor: %w", err)
	}

	estimatedCost, err := runtime.NumberSensor(ctx, entities.NumberSensorSpec{
		CommonSpec: entities.CommonSpec{
			Key:          dishwasherEstimatedCostKey,
			Name:         "Kitchen Dishwasher: Estimated Cost",
			EntityIDHint: "sensor.kitchen_dishwasher_estimated_cost",
			Icon:         "mdi:currency-eur",
		},
		UnitOfMeasurement: "EUR",
	})
	if err != nil {
		return nil, fmt.Errorf("declare estimated cost sensor: %w", err)
	}

	currentCost, err := runtime.NumberSensor(ctx, entities.NumberSensorSpec{
		CommonSpec: entities.CommonSpec{
			Key:          dishwasherCurrentCostKey,
			Name:         "Kitchen Dishwasher: Current Cost",
			EntityIDHint: "sensor.kitchen_dishwasher_current_cost",
			Icon:         "mdi:currency-eur",
		},
		UnitOfMeasurement: "EUR",
	})
	if err != nil {
		return nil, fmt.Errorf("declare current cost sensor: %w", err)
	}

	savingsPercent, err := runtime.NumberSensor(ctx, entities.NumberSensorSpec{
		CommonSpec: entities.CommonSpec{
			Key:          dishwasherSavingsPercentKey,
			Name:         "Kitchen Dishwasher: Savings Percent",
			EntityIDHint: "sensor.kitchen_dishwasher_savings_percent",
			Icon:         "mdi:percent",
		},
		UnitOfMeasurement: "%",
	})
	if err != nil {
		return nil, fmt.Errorf("declare savings sensor: %w", err)
	}

	return &StateManager{
		state:          state,
		controller:     controller,
		scheduled:      scheduled,
		mode:           mode,
		startTime:      startTime,
		estimatedCost:  estimatedCost,
		currentCost:    currentCost,
		savingsPercent: savingsPercent,
	}, nil
}

// SaveSchedule converts dishwasher schedule to runtime entity state and persists it.
func (sm *StateManager) SaveSchedule(schedule *domaindishwasher.PendingSchedule) error {
	ctx := context.Background()

	if err := sm.scheduled.On(ctx); err != nil {
		return fmt.Errorf("set scheduled flag: %w", err)
	}
	if err := sm.mode.Set(ctx, string(schedule.Mode)); err != nil {
		log.Printf("WARNING: Failed to save mode: %v", err)
	}
	if err := sm.startTime.Set(ctx, schedule.StartTime.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("set start time: %w", err)
	}
	if err := sm.estimatedCost.Set(ctx, schedule.Result.EstimatedCost); err != nil {
		return fmt.Errorf("set estimated cost: %w", err)
	}
	if err := sm.currentCost.Set(ctx, schedule.Result.CurrentCost); err != nil {
		return fmt.Errorf("set current cost: %w", err)
	}
	if err := sm.savingsPercent.Set(ctx, schedule.Result.SavingsPercent); err != nil {
		log.Printf("WARNING: Failed to save savings percent: %v", err)
	}

	return nil
}

// RestoreSchedule loads schedule state from runtime entities.
func (sm *StateManager) RestoreSchedule() (*domaindishwasher.PendingSchedule, error) {
	isScheduledState, err := sm.getState(sm.scheduled.EntityID())
	if err != nil {
		if isMissingEntityError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get scheduled flag: %w", err)
	}
	if isScheduledState.State != "on" {
		return nil, nil
	}

	modeState, err := sm.getState(sm.mode.EntityID())
	if err != nil {
		return nil, fmt.Errorf("get mode: %w", err)
	}

	startState, err := sm.getState(sm.startTime.EntityID())
	if err != nil {
		return nil, fmt.Errorf("get start time: %w", err)
	}

	startTime, err := parseScheduleTime(startState.State)
	if err != nil {
		return nil, fmt.Errorf("parse start time: %w", err)
	}

	estimatedCostState, err := sm.getState(sm.estimatedCost.EntityID())
	if err != nil {
		return nil, fmt.Errorf("get estimated cost: %w", err)
	}

	currentCostState, err := sm.getState(sm.currentCost.EntityID())
	if err != nil {
		return nil, fmt.Errorf("get current cost: %w", err)
	}

	savingsPercentState, err := sm.getState(sm.savingsPercent.EntityID())
	if err != nil {
		return nil, fmt.Errorf("get savings percent: %w", err)
	}

	estimatedCost, err := parseFloat(estimatedCostState.State)
	if err != nil {
		return nil, fmt.Errorf("parse estimated cost: %w", err)
	}

	currentCost, err := parseFloat(currentCostState.State)
	if err != nil {
		return nil, fmt.Errorf("parse current cost: %w", err)
	}

	savingsPercent, err := parseFloat(savingsPercentState.State)
	if err != nil {
		return nil, fmt.Errorf("parse savings percent: %w", err)
	}

	if startTime.Before(time.Now()) {
		log.Printf("Restored schedule has passed its start time, ensuring dishwasher is running")

		socketState, err := sm.state.Get(entities.Switch.KitchenDishwasherSocket)
		if err != nil {
			log.Printf("WARNING: Failed to check socket state: %v", err)
		} else if socketState.State != "on" {
			log.Printf("Socket is OFF for expired schedule, starting dishwasher now")
			if err := sm.controller.StartDishwasher(); err != nil {
				log.Printf("ERROR: Failed to start dishwasher for expired schedule: %v", err)
			}
		} else {
			log.Printf("Socket is already ON, dishwasher likely running")
		}

		if err := sm.ClearSchedule(); err != nil {
			return nil, fmt.Errorf("clear expired schedule: %w", err)
		}
		return nil, nil
	}

	result := &optimizer.OptimizationResult{
		StartTime:      startTime,
		EstimatedCost:  estimatedCost,
		CurrentCost:    currentCost,
		Savings:        currentCost - estimatedCost,
		SavingsPercent: savingsPercent,
	}

	return &domaindishwasher.PendingSchedule{
		Mode:      domaindishwasher.Mode(modeState.State),
		StartTime: startTime,
		Result:    result,
	}, nil
}

// ClearSchedule clears persisted dishwasher schedule state.
func (sm *StateManager) ClearSchedule() error {
	ctx := context.Background()

	if err := sm.scheduled.Off(ctx); err != nil {
		return fmt.Errorf("clear scheduled flag: %w", err)
	}
	if err := sm.mode.Set(ctx, dishwasherScheduledModeCleared); err != nil {
		log.Printf("WARNING: Failed to clear mode: %v", err)
	}
	if err := sm.startTime.Set(ctx, ""); err != nil {
		log.Printf("WARNING: Failed to clear start time: %v", err)
	}
	if err := sm.estimatedCost.Set(ctx, 0); err != nil {
		log.Printf("WARNING: Failed to clear estimated cost: %v", err)
	}
	if err := sm.currentCost.Set(ctx, 0); err != nil {
		log.Printf("WARNING: Failed to clear current cost: %v", err)
	}
	if err := sm.savingsPercent.Set(ctx, 0); err != nil {
		log.Printf("WARNING: Failed to clear savings percent: %v", err)
	}

	return nil
}

// IsScheduleCancelled checks whether the persisted schedule switch is off.
func (sm *StateManager) IsScheduleCancelled() (bool, error) {
	isScheduledState, err := sm.getState(sm.scheduled.EntityID())
	if err != nil {
		if isMissingEntityError(err) {
			return false, nil
		}
		return false, fmt.Errorf("get scheduled flag: %w", err)
	}

	return isScheduledState.State != "on", nil
}

func (sm *StateManager) OnScheduledCommand(fn func(context.Context, bool) error) error {
	return sm.scheduled.OnCommand(fn)
}

func (sm *StateManager) SetScheduledFlag(ctx context.Context, on bool) error {
	if on {
		return sm.scheduled.On(ctx)
	}
	return sm.scheduled.Off(ctx)
}

func (sm *StateManager) getState(entityID string) (ga.EntityState, error) {
	return sm.state.Get(entityID)
}

func parseScheduleTime(value string) (time.Time, error) {
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

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func isMissingEntityError(err error) bool {
	if err == nil {
		return false
	}

	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "not found")
}
