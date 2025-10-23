package component

import (
	"fmt"
	"log"
	"time"

	ga "saml.dev/gome-assistant"
)

// ScheduleState represents the persisted state of a pending schedule
// This is a generic structure that can be used by any device
type ScheduleState struct {
	IsScheduled    bool
	Mode           string
	StartTime      time.Time
	EstimatedCost  float64
	CurrentCost    float64
	SavingsPercent float64

	// Display fields (optional, for UI)
	DelayMinutes   int
	OptimalTimeStr string
	SavingsStr     string
}

// StateConfig defines which HASS entities to use for persistence
// Each device component provides its own entity IDs
type StateConfig struct {
	IsScheduledEntity    string // input_boolean
	ModeEntity           string // input_select
	StartTimeEntity      string // input_datetime
	EstimatedCostEntity  string // input_number
	CurrentCostEntity    string // input_number
	SavingsPercentEntity string // input_number

	// Optional display entities (can be empty strings)
	DelayEntity       string // input_number
	OptimalTimeEntity string // input_text
	SavingsEntity     string // input_text

	// For input_select, what value means "no schedule"
	ModeNoneValue string // e.g., "none"
}

// StateManager handles generic state persistence for device schedules
// This can be used by any component that needs to persist scheduling state
type StateManager struct {
	service *ga.Service
	state   ga.State
	config  StateConfig
}

// NewStateManager creates a new generic state manager
func NewStateManager(service *ga.Service, state ga.State, config StateConfig) *StateManager {
	return &StateManager{
		service: service,
		state:   state,
		config:  config,
	}
}

// SaveScheduleState persists schedule state to HASS entities
func (sm *StateManager) SaveScheduleState(scheduleState ScheduleState) error {
	// Set scheduled flag to true
	if err := sm.service.InputBoolean.TurnOn(sm.config.IsScheduledEntity); err != nil {
		return fmt.Errorf("failed to set scheduled flag: %w", err)
	}

	// Save mode using HomeAssistant.TurnOn with option parameter
	// This is a workaround since input_select.select_option isn't directly exposed
	if err := sm.service.HomeAssistant.TurnOn(
		sm.config.ModeEntity,
		map[string]interface{}{
			"option": scheduleState.Mode,
		},
	); err != nil {
		log.Printf("WARNING: Failed to save mode: %v", err)
	}

	// Save start time
	if err := sm.service.InputDatetime.Set(
		sm.config.StartTimeEntity,
		scheduleState.StartTime,
	); err != nil {
		return fmt.Errorf("failed to save start time: %w", err)
	}

	// Save costs
	if err := sm.service.InputNumber.Set(sm.config.EstimatedCostEntity, float32(scheduleState.EstimatedCost)); err != nil {
		return fmt.Errorf("failed to save estimated cost: %w", err)
	}

	if err := sm.service.InputNumber.Set(sm.config.CurrentCostEntity, float32(scheduleState.CurrentCost)); err != nil {
		return fmt.Errorf("failed to save current cost: %w", err)
	}

	if err := sm.service.InputNumber.Set(sm.config.SavingsPercentEntity, float32(scheduleState.SavingsPercent)); err != nil {
		log.Printf("WARNING: Failed to save savings percent: %v", err)
	}

	// Save optional display fields
	if sm.config.DelayEntity != "" {
		if err := sm.service.InputNumber.Set(sm.config.DelayEntity, float32(scheduleState.DelayMinutes)); err != nil {
			log.Printf("WARNING: Failed to save delay: %v", err)
		}
	}

	if sm.config.OptimalTimeEntity != "" {
		if err := sm.service.InputText.Set(sm.config.OptimalTimeEntity, scheduleState.OptimalTimeStr); err != nil {
			log.Printf("WARNING: Failed to save optimal time: %v", err)
		}
	}

	if sm.config.SavingsEntity != "" {
		if err := sm.service.InputText.Set(sm.config.SavingsEntity, scheduleState.SavingsStr); err != nil {
			log.Printf("WARNING: Failed to save savings display: %v", err)
		}
	}

	log.Printf("Schedule state saved to HASS entities")
	return nil
}

// RestoreScheduleState loads schedule state from HASS entities
func (sm *StateManager) RestoreScheduleState() (*ScheduleState, error) {
	// Check if there's a pending schedule
	isScheduledState, err := sm.state.Get(sm.config.IsScheduledEntity)
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled flag: %w", err)
	}

	if isScheduledState.State != "on" {
		return nil, nil // No pending schedule
	}

	// Load mode
	modeState, err := sm.state.Get(sm.config.ModeEntity)
	if err != nil {
		return nil, fmt.Errorf("failed to get mode: %w", err)
	}

	// Load start time
	startTimeState, err := sm.state.Get(sm.config.StartTimeEntity)
	if err != nil {
		return nil, fmt.Errorf("failed to get start time: %w", err)
	}

	startTime, err := time.Parse(time.RFC3339, startTimeState.State)
	if err != nil {
		// Try alternative format
		startTime, err = time.Parse("2006-01-02 15:04:05", startTimeState.State)
		if err != nil {
			return nil, fmt.Errorf("failed to parse start time: %w", err)
		}
	}

	// Load costs
	estimatedCostState, err := sm.state.Get(sm.config.EstimatedCostEntity)
	if err != nil {
		return nil, fmt.Errorf("failed to get estimated cost: %w", err)
	}

	currentCostState, err := sm.state.Get(sm.config.CurrentCostEntity)
	if err != nil {
		return nil, fmt.Errorf("failed to get current cost: %w", err)
	}

	savingsPercentState, err := sm.state.Get(sm.config.SavingsPercentEntity)
	if err != nil {
		return nil, fmt.Errorf("failed to get savings percent: %w", err)
	}

	// Parse numeric values
	estimatedCost, err := parseFloat(estimatedCostState.State)
	if err != nil {
		return nil, fmt.Errorf("failed to parse estimated cost: %w", err)
	}

	currentCost, err := parseFloat(currentCostState.State)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current cost: %w", err)
	}

	savingsPercent, err := parseFloat(savingsPercentState.State)
	if err != nil {
		return nil, fmt.Errorf("failed to parse savings percent: %w", err)
	}

	scheduleState := &ScheduleState{
		IsScheduled:    true,
		Mode:           modeState.State,
		StartTime:      startTime,
		EstimatedCost:  estimatedCost,
		CurrentCost:    currentCost,
		SavingsPercent: savingsPercent,
	}

	log.Printf("Schedule state restored from HASS entities")
	return scheduleState, nil
}

// ClearScheduleState clears all schedule state from HASS entities
func (sm *StateManager) ClearScheduleState() error {
	// Set scheduled flag to false
	if err := sm.service.InputBoolean.TurnOff(sm.config.IsScheduledEntity); err != nil {
		return fmt.Errorf("failed to clear scheduled flag: %w", err)
	}

	// Reset mode to "none" value
	if err := sm.service.HomeAssistant.TurnOn(
		sm.config.ModeEntity,
		map[string]interface{}{
			"option": sm.config.ModeNoneValue,
		},
	); err != nil {
		log.Printf("WARNING: Failed to reset mode: %v", err)
	}

	// Reset datetime to zero
	if err := sm.service.InputDatetime.Set(sm.config.StartTimeEntity, time.Time{}); err != nil {
		log.Printf("WARNING: Failed to reset start time: %v", err)
	}

	// Reset costs
	if err := sm.service.InputNumber.Set(sm.config.EstimatedCostEntity, 0); err != nil {
		log.Printf("WARNING: Failed to reset estimated cost: %v", err)
	}

	if err := sm.service.InputNumber.Set(sm.config.CurrentCostEntity, 0); err != nil {
		log.Printf("WARNING: Failed to reset current cost: %v", err)
	}

	if err := sm.service.InputNumber.Set(sm.config.SavingsPercentEntity, 0); err != nil {
		log.Printf("WARNING: Failed to reset savings percent: %v", err)
	}

	// Clear optional display fields
	if sm.config.DelayEntity != "" {
		if err := sm.service.InputNumber.Set(sm.config.DelayEntity, 0); err != nil {
			log.Printf("WARNING: Failed to clear delay: %v", err)
		}
	}

	if sm.config.OptimalTimeEntity != "" {
		if err := sm.service.InputText.Set(sm.config.OptimalTimeEntity, "N/A"); err != nil {
			log.Printf("WARNING: Failed to clear optimal time: %v", err)
		}
	}

	if sm.config.SavingsEntity != "" {
		if err := sm.service.InputText.Set(sm.config.SavingsEntity, "€0.00"); err != nil {
			log.Printf("WARNING: Failed to clear savings: %v", err)
		}
	}

	log.Printf("Schedule state cleared successfully")
	return nil
}

// IsScheduleCancelled checks if the schedule was manually cancelled by the user
func (sm *StateManager) IsScheduleCancelled() (bool, error) {
	isScheduledState, err := sm.state.Get(sm.config.IsScheduledEntity)
	if err != nil {
		return false, fmt.Errorf("failed to get scheduled flag: %w", err)
	}

	// If the flag is off, the schedule was cancelled
	return isScheduledState.State != "on", nil
}

// parseFloat is a helper to parse float from string state
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
