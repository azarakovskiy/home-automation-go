package dishwasher

import (
	"log"
	"time"

	"home-go/component"
	"home-go/entities"
	"home-go/optimization/optimizer"

	ga "saml.dev/gome-assistant"
)

// StateManager wraps the generic component.StateManager with dishwasher-specific logic
type StateManager struct {
	generic    *component.StateManager
	service    *ga.Service
	state      ga.State
	controller *Controller
}

func NewStateManager(service *ga.Service, state ga.State, controller *Controller) *StateManager {
	// Configure which HASS entities to use for dishwasher state persistence
	config := component.StateConfig{
		IsScheduledEntity:    entities.InputBoolean.KitchenDishwasherIsScheduled,
		ModeEntity:           entities.InputSelect.KitchenDishwasherScheduledMode,
		StartTimeEntity:      entities.InputDatetime.KitchenDishwasherScheduledStart,
		EstimatedCostEntity:  entities.InputNumber.KitchenDishwasherEstimatedCost,
		CurrentCostEntity:    entities.InputNumber.KitchenDishwasherCurrentCost,
		SavingsPercentEntity: entities.InputNumber.KitchenDishwasherSavingsPercent,

		ModeNoneValue: "none",
	}

	return &StateManager{
		generic:    component.NewStateManager(service, state, config),
		service:    service,
		state:      state,
		controller: controller,
	}
}

// SaveSchedule converts dishwasher schedule to generic state and saves it
func (sm *StateManager) SaveSchedule(schedule *PendingSchedule) error {
	scheduleState := component.ScheduleState{
		IsScheduled:    true,
		Mode:           string(schedule.Mode),
		StartTime:      schedule.StartTime,
		EstimatedCost:  schedule.Result.EstimatedCost,
		CurrentCost:    schedule.Result.CurrentCost,
		SavingsPercent: schedule.Result.SavingsPercent,
	}

	return sm.generic.SaveScheduleState(scheduleState)
}

// RestoreSchedule loads schedule state and converts back to dishwasher-specific format
func (sm *StateManager) RestoreSchedule() (*PendingSchedule, error) {
	scheduleState, err := sm.generic.RestoreScheduleState()
	if err != nil {
		return nil, err
	}

	if scheduleState == nil {
		return nil, nil // No pending schedule
	}

	// Check if start time has already passed
	if scheduleState.StartTime.Before(time.Now()) {
		log.Printf("Restored schedule has passed its start time, ensuring dishwasher is running")

		// Check if dishwasher socket is already on
		socketState, err := sm.state.Get(entities.Switch.KitchenDishwasherSocket)
		if err != nil {
			log.Printf("WARNING: Failed to check socket state: %v", err)
		} else if socketState.State != "on" {
			// Socket is off but schedule passed - turn it on to ensure dishwasher runs
			log.Printf("Socket is OFF for expired schedule, starting dishwasher now")
			if err := sm.controller.StartDishwasher(); err != nil {
				log.Printf("ERROR: Failed to start dishwasher for expired schedule: %v", err)
			}
		} else {
			log.Printf("Socket is already ON, dishwasher likely running")
		}

		// Clear stale schedule
		_ = sm.generic.ClearScheduleState()
		return nil, nil
	}

	// Reconstruct optimizer result
	result := &optimizer.OptimizationResult{
		StartTime:      scheduleState.StartTime,
		EstimatedCost:  scheduleState.EstimatedCost,
		CurrentCost:    scheduleState.CurrentCost,
		Savings:        scheduleState.CurrentCost - scheduleState.EstimatedCost,
		SavingsPercent: scheduleState.SavingsPercent,
	}

	return &PendingSchedule{
		Mode:      Mode(scheduleState.Mode),
		StartTime: scheduleState.StartTime,
		Result:    result,
	}, nil
}

// ClearSchedule clears the schedule state
func (sm *StateManager) ClearSchedule() error {
	return sm.generic.ClearScheduleState()
}

// IsScheduleCancelled checks if schedule was manually cancelled
func (sm *StateManager) IsScheduleCancelled() (bool, error) {
	return sm.generic.IsScheduleCancelled()
}
