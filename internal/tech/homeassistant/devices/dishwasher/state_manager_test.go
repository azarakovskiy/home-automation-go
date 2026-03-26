package dishwasher

import (
	"fmt"
	"testing"
	"time"

	"home-go/internal/domain/optimizer"
	"home-go/internal/mocks"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/entities"

	"go.uber.org/mock/gomock"
	ga "saml.dev/gome-assistant"
)

func TestNewStateManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := &ga.Service{}
	mockState := mocks.NewMockState(ctrl)
	mockController := &Controller{service: mockService}

	sm := NewStateManager(mockService, mockState, mockController)

	if sm == nil {
		t.Fatal("NewStateManager returned nil")
	}

	if sm.service != mockService {
		t.Error("Service not set correctly")
	}

	if sm.state != mockState {
		t.Error("State not set correctly")
	}

	if sm.controller != mockController {
		t.Error("Controller not set correctly")
	}

	if sm.generic == nil {
		t.Error("Generic state manager not initialized")
	}
}

// Note: SaveSchedule is not fully testable here because it calls SaveScheduleState
// which requires mocking ga.Service struct. Integration tests are more appropriate.
func TestStateManager_SaveSchedule(t *testing.T) {
	t.Skip("SaveSchedule requires ga.Service struct mocking - use integration tests")
}

func TestStateManager_RestoreSchedule(t *testing.T) {
	futureTime := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name         string
		setupMocks   func(*mocks.MockState, *component.StateManager)
		wantSchedule *PendingSchedule
		wantErr      bool
	}{
		{
			name: "successfully restore future schedule",
			setupMocks: func(mockState *mocks.MockState, genericMgr *component.StateManager) {
				// Mock the generic RestoreScheduleState to return a valid schedule
				mockState.EXPECT().Get(entities.InputBoolean.KitchenDishwasherIsScheduled).Return(ga.EntityState{State: "on"}, nil)
				mockState.EXPECT().Get(entities.InputSelect.KitchenDishwasherScheduledMode).Return(ga.EntityState{State: "auto"}, nil)
				mockState.EXPECT().Get(entities.InputDatetime.KitchenDishwasherScheduledStart).Return(ga.EntityState{State: futureTime.Format(time.RFC3339)}, nil)
				mockState.EXPECT().Get(entities.InputNumber.KitchenDishwasherEstimatedCost).Return(ga.EntityState{State: "0.50"}, nil)
				mockState.EXPECT().Get(entities.InputNumber.KitchenDishwasherCurrentCost).Return(ga.EntityState{State: "0.60"}, nil)
				mockState.EXPECT().Get(entities.InputNumber.KitchenDishwasherSavingsPercent).Return(ga.EntityState{State: "16.67"}, nil)
			},
			wantSchedule: &PendingSchedule{
				Mode:      ModeAuto,
				StartTime: futureTime,
				Result: &optimizer.OptimizationResult{
					StartTime:      futureTime,
					EstimatedCost:  0.50,
					CurrentCost:    0.60,
					Savings:        0.10,
					SavingsPercent: 16.67,
				},
			},
			wantErr: false,
		},
		{
			name: "no pending schedule",
			setupMocks: func(mockState *mocks.MockState, genericMgr *component.StateManager) {
				mockState.EXPECT().Get(entities.InputBoolean.KitchenDishwasherIsScheduled).Return(ga.EntityState{State: "off"}, nil)
			},
			wantSchedule: nil,
			wantErr:      false,
		},
		{
			name: "error getting schedule state",
			setupMocks: func(mockState *mocks.MockState, genericMgr *component.StateManager) {
				mockState.EXPECT().Get(entities.InputBoolean.KitchenDishwasherIsScheduled).Return(ga.EntityState{}, fmt.Errorf("entity error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockService := &ga.Service{}
			mockState := mocks.NewMockState(ctrl)
			mockController := &Controller{service: mockService}

			sm := NewStateManager(mockService, mockState, mockController)
			tt.setupMocks(mockState, sm.generic)

			schedule, err := sm.RestoreSchedule()

			if (err != nil) != tt.wantErr {
				t.Errorf("RestoreSchedule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if (schedule == nil) != (tt.wantSchedule == nil) {
					t.Errorf("RestoreSchedule() schedule = %v, want %v", schedule, tt.wantSchedule)
					return
				}

				if schedule != nil && tt.wantSchedule != nil {
					if schedule.Mode != tt.wantSchedule.Mode {
						t.Errorf("Mode = %s, want %s", schedule.Mode, tt.wantSchedule.Mode)
					}

					const epsilon = 0.01
					if diff := schedule.Result.EstimatedCost - tt.wantSchedule.Result.EstimatedCost; diff < -epsilon || diff > epsilon {
						t.Errorf("EstimatedCost = %f, want %f", schedule.Result.EstimatedCost, tt.wantSchedule.Result.EstimatedCost)
					}
				}
			}
		})
	}
}

// Note: ClearSchedule is not fully testable here because it calls ClearScheduleState
// which requires mocking ga.Service struct. Integration tests are more appropriate.
func TestStateManager_ClearSchedule(t *testing.T) {
	t.Skip("ClearSchedule requires ga.Service struct mocking - use integration tests")
}

func TestStateManager_IsScheduleCancelled(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*mocks.MockState)
		wantCancelled bool
		wantErr       bool
	}{
		{
			name: "schedule is active",
			setupMock: func(m *mocks.MockState) {
				m.EXPECT().Get(entities.InputBoolean.KitchenDishwasherIsScheduled).Return(ga.EntityState{State: "on"}, nil)
			},
			wantCancelled: false,
			wantErr:       false,
		},
		{
			name: "schedule is cancelled",
			setupMock: func(m *mocks.MockState) {
				m.EXPECT().Get(entities.InputBoolean.KitchenDishwasherIsScheduled).Return(ga.EntityState{State: "off"}, nil)
			},
			wantCancelled: true,
			wantErr:       false,
		},
		{
			name: "error checking schedule state",
			setupMock: func(m *mocks.MockState) {
				m.EXPECT().Get(entities.InputBoolean.KitchenDishwasherIsScheduled).Return(ga.EntityState{}, fmt.Errorf("entity error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockService := &ga.Service{}
			mockState := mocks.NewMockState(ctrl)
			mockController := &Controller{service: mockService}

			sm := NewStateManager(mockService, mockState, mockController)
			tt.setupMock(mockState)

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
