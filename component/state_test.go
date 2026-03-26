package component

import (
	"fmt"
	"testing"
	"time"

	"home-go/internal/mocks"

	"go.uber.org/mock/gomock"
	ga "saml.dev/gome-assistant"
)

func TestNewStateManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := &ga.Service{}
	mockState := mocks.NewMockStateInterface(ctrl)
	config := StateConfig{
		IsScheduledEntity: "input_boolean.test_scheduled",
		ModeEntity:        "input_select.test_mode",
	}

	sm := NewStateManager(mockService, mockState, config)

	if sm == nil {
		t.Fatal("NewStateManager returned nil")
	}

	if sm.service != mockService {
		t.Error("Service not set correctly")
	}

	if sm.state != mockState {
		t.Error("State not set correctly")
	}

	if sm.config.IsScheduledEntity != config.IsScheduledEntity {
		t.Error("Config not set correctly")
	}
}

// Note: SaveScheduleState is not tested here because it requires mocking ga.Service,
// which is a complex struct (not an interface) from an external library.
// These methods are better tested through integration tests with a real Home Assistant instance
// or by refactoring to use dependency injection with interfaces.

func TestStateManager_RestoreScheduleState(t *testing.T) {
	startTime := time.Date(2025, 10, 23, 15, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		config       StateConfig
		setupMock    func(*mocks.MockStateInterface)
		wantSchedule *ScheduleState
		wantErr      bool
	}{
		{
			name: "successfully restore schedule",
			config: StateConfig{
				IsScheduledEntity:    "input_boolean.test_scheduled",
				ModeEntity:           "input_select.test_mode",
				StartTimeEntity:      "input_datetime.test_start",
				EstimatedCostEntity:  "input_number.test_estimated",
				CurrentCostEntity:    "input_number.test_current",
				SavingsPercentEntity: "input_number.test_savings",
			},
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().Get("input_boolean.test_scheduled").Return(ga.EntityState{State: "on"}, nil)
				m.EXPECT().Get("input_select.test_mode").Return(ga.EntityState{State: "eco"}, nil)
				m.EXPECT().Get("input_datetime.test_start").Return(ga.EntityState{State: startTime.Format(time.RFC3339)}, nil)
				m.EXPECT().Get("input_number.test_estimated").Return(ga.EntityState{State: "0.50"}, nil)
				m.EXPECT().Get("input_number.test_current").Return(ga.EntityState{State: "0.60"}, nil)
				m.EXPECT().Get("input_number.test_savings").Return(ga.EntityState{State: "16.67"}, nil)
			},
			wantSchedule: &ScheduleState{
				IsScheduled:    true,
				Mode:           "eco",
				StartTime:      startTime,
				EstimatedCost:  0.50,
				CurrentCost:    0.60,
				SavingsPercent: 16.67,
			},
			wantErr: false,
		},
		{
			name: "no pending schedule (flag off)",
			config: StateConfig{
				IsScheduledEntity: "input_boolean.test_scheduled",
			},
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().Get("input_boolean.test_scheduled").Return(ga.EntityState{State: "off"}, nil)
			},
			wantSchedule: nil,
			wantErr:      false,
		},
		{
			name: "parse alternative datetime format",
			config: StateConfig{
				IsScheduledEntity:    "input_boolean.test_scheduled",
				ModeEntity:           "input_select.test_mode",
				StartTimeEntity:      "input_datetime.test_start",
				EstimatedCostEntity:  "input_number.test_estimated",
				CurrentCostEntity:    "input_number.test_current",
				SavingsPercentEntity: "input_number.test_savings",
			},
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().Get("input_boolean.test_scheduled").Return(ga.EntityState{State: "on"}, nil)
				m.EXPECT().Get("input_select.test_mode").Return(ga.EntityState{State: "auto"}, nil)
				m.EXPECT().Get("input_datetime.test_start").Return(ga.EntityState{State: "2025-10-23 15:00:00"}, nil)
				m.EXPECT().Get("input_number.test_estimated").Return(ga.EntityState{State: "0.40"}, nil)
				m.EXPECT().Get("input_number.test_current").Return(ga.EntityState{State: "0.50"}, nil)
				m.EXPECT().Get("input_number.test_savings").Return(ga.EntityState{State: "20.00"}, nil)
			},
			wantSchedule: &ScheduleState{
				IsScheduled:    true,
				Mode:           "auto",
				StartTime:      startTime,
				EstimatedCost:  0.40,
				CurrentCost:    0.50,
				SavingsPercent: 20.00,
			},
			wantErr: false,
		},
		{
			name: "failed to get scheduled flag",
			config: StateConfig{
				IsScheduledEntity: "input_boolean.test_scheduled",
			},
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().Get("input_boolean.test_scheduled").Return(ga.EntityState{}, fmt.Errorf("entity not found"))
			},
			wantErr: true,
		},
		{
			name: "failed to get mode",
			config: StateConfig{
				IsScheduledEntity: "input_boolean.test_scheduled",
				ModeEntity:        "input_select.test_mode",
			},
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().Get("input_boolean.test_scheduled").Return(ga.EntityState{State: "on"}, nil)
				m.EXPECT().Get("input_select.test_mode").Return(ga.EntityState{}, fmt.Errorf("entity error"))
			},
			wantErr: true,
		},
		{
			name: "invalid datetime format",
			config: StateConfig{
				IsScheduledEntity: "input_boolean.test_scheduled",
				ModeEntity:        "input_select.test_mode",
				StartTimeEntity:   "input_datetime.test_start",
			},
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().Get("input_boolean.test_scheduled").Return(ga.EntityState{State: "on"}, nil)
				m.EXPECT().Get("input_select.test_mode").Return(ga.EntityState{State: "eco"}, nil)
				m.EXPECT().Get("input_datetime.test_start").Return(ga.EntityState{State: "invalid-date"}, nil)
			},
			wantErr: true,
		},
		{
			name: "invalid estimated cost format",
			config: StateConfig{
				IsScheduledEntity:    "input_boolean.test_scheduled",
				ModeEntity:           "input_select.test_mode",
				StartTimeEntity:      "input_datetime.test_start",
				EstimatedCostEntity:  "input_number.test_estimated",
				CurrentCostEntity:    "input_number.test_current",
				SavingsPercentEntity: "input_number.test_savings",
			},
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().Get("input_boolean.test_scheduled").Return(ga.EntityState{State: "on"}, nil)
				m.EXPECT().Get("input_select.test_mode").Return(ga.EntityState{State: "eco"}, nil)
				m.EXPECT().Get("input_datetime.test_start").Return(ga.EntityState{State: startTime.Format(time.RFC3339)}, nil)
				m.EXPECT().Get("input_number.test_estimated").Return(ga.EntityState{State: "not-a-number"}, nil)
				m.EXPECT().Get("input_number.test_current").Return(ga.EntityState{State: "0.60"}, nil)
				m.EXPECT().Get("input_number.test_savings").Return(ga.EntityState{State: "16.67"}, nil)
				// Note: All Get() calls are made first, then parsing happens
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockState := mocks.NewMockStateInterface(ctrl)
			tt.setupMock(mockState)

			// Create minimal ga.Service for testing
			sm := NewStateManager(&ga.Service{}, mockState, tt.config)
			schedule, err := sm.RestoreScheduleState()

			if (err != nil) != tt.wantErr {
				t.Errorf("RestoreScheduleState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if (schedule == nil) != (tt.wantSchedule == nil) {
					t.Errorf("RestoreScheduleState() schedule = %v, want %v", schedule, tt.wantSchedule)
					return
				}

				if schedule != nil && tt.wantSchedule != nil {
					if schedule.Mode != tt.wantSchedule.Mode {
						t.Errorf("Mode = %s, want %s", schedule.Mode, tt.wantSchedule.Mode)
					}
					if !schedule.StartTime.Equal(tt.wantSchedule.StartTime) {
						t.Errorf("StartTime = %v, want %v", schedule.StartTime, tt.wantSchedule.StartTime)
					}
					const epsilon = 0.01
					if diff := schedule.EstimatedCost - tt.wantSchedule.EstimatedCost; diff < -epsilon || diff > epsilon {
						t.Errorf("EstimatedCost = %f, want %f", schedule.EstimatedCost, tt.wantSchedule.EstimatedCost)
					}
					if diff := schedule.CurrentCost - tt.wantSchedule.CurrentCost; diff < -epsilon || diff > epsilon {
						t.Errorf("CurrentCost = %f, want %f", schedule.CurrentCost, tt.wantSchedule.CurrentCost)
					}
					if diff := schedule.SavingsPercent - tt.wantSchedule.SavingsPercent; diff < -epsilon || diff > epsilon {
						t.Errorf("SavingsPercent = %f, want %f", schedule.SavingsPercent, tt.wantSchedule.SavingsPercent)
					}
				}
			}
		})
	}
}

func TestStateManager_IsScheduleCancelled(t *testing.T) {
	tests := []struct {
		name          string
		config        StateConfig
		setupMock     func(*mocks.MockStateInterface)
		wantCancelled bool
		wantErr       bool
	}{
		{
			name: "schedule is active",
			config: StateConfig{
				IsScheduledEntity: "input_boolean.test_scheduled",
			},
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().Get("input_boolean.test_scheduled").Return(ga.EntityState{State: "on"}, nil)
			},
			wantCancelled: false,
			wantErr:       false,
		},
		{
			name: "schedule is cancelled",
			config: StateConfig{
				IsScheduledEntity: "input_boolean.test_scheduled",
			},
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().Get("input_boolean.test_scheduled").Return(ga.EntityState{State: "off"}, nil)
			},
			wantCancelled: true,
			wantErr:       false,
		},
		{
			name: "failed to get scheduled flag",
			config: StateConfig{
				IsScheduledEntity: "input_boolean.test_scheduled",
			},
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().Get("input_boolean.test_scheduled").Return(ga.EntityState{}, fmt.Errorf("entity error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockState := mocks.NewMockStateInterface(ctrl)
			tt.setupMock(mockState)

			sm := NewStateManager(&ga.Service{}, mockState, tt.config)
			cancelled, err := sm.IsScheduleCancelled()

			if (err != nil) != tt.wantErr {
				t.Errorf("IsScheduleCancelled() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && cancelled != tt.wantCancelled {
				t.Errorf("IsScheduleCancelled() = %v, want %v", cancelled, tt.wantCancelled)
			}
		})
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{
			name:    "parse integer",
			input:   "42",
			want:    42.0,
			wantErr: false,
		},
		{
			name:    "parse decimal",
			input:   "3.14159",
			want:    3.14159,
			wantErr: false,
		},
		{
			name:    "parse negative",
			input:   "-10.5",
			want:    -10.5,
			wantErr: false,
		},
		{
			name:    "parse zero",
			input:   "0",
			want:    0.0,
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "not-a-number",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFloat(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseFloat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				const epsilon = 0.00001
				if diff := got - tt.want; diff < -epsilon || diff > epsilon {
					t.Errorf("parseFloat() = %f, want %f", got, tt.want)
				}
			}
		})
	}
}
