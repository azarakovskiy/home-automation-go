package pricing

import (
	"fmt"
	"testing"
	"time"

	"home-go/entities"
	"home-go/mocks"

	"go.uber.org/mock/gomock"
	ga "saml.dev/gome-assistant"
)

func TestNewService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockState := mocks.NewMockStateInterface(ctrl)
	service := NewService(mockState)

	if service == nil {
		t.Fatal("NewService returned nil")
	}

	if service.state != mockState {
		t.Error("Service state not set correctly")
	}
}

func TestService_GetPriceSlots(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*mocks.MockStateInterface)
		wantSlotCount  int
		wantFirstPrice float64
		wantErr        bool
	}{
		{
			name: "successfully parse price slots",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{
						Attributes: map[string]any{
							"prices": []any{
								map[string]any{
									"from":  "2025-10-23T10:00:00Z",
									"till":  "2025-10-23T11:00:00Z",
									"price": 0.15,
								},
								map[string]any{
									"from":  "2025-10-23T11:00:00Z",
									"till":  "2025-10-23T12:00:00Z",
									"price": 0.20,
								},
								map[string]any{
									"from":  "2025-10-23T12:00:00Z",
									"till":  "2025-10-23T13:00:00Z",
									"price": 0.25,
								},
							},
						},
					}, nil)
			},
			wantSlotCount:  3,
			wantFirstPrice: 0.15,
			wantErr:        false,
		},
		{
			name: "state.Get returns error",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{}, fmt.Errorf("sensor not found"))
			},
			wantErr: true,
		},
		{
			name: "prices attribute not found",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{
						Attributes: map[string]any{},
					}, nil)
			},
			wantErr: true,
		},
		{
			name: "prices attribute is not a list",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{
						Attributes: map[string]any{
							"prices": "not a list",
						},
					}, nil)
			},
			wantErr: true,
		},
		{
			name: "skip invalid items in list",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{
						Attributes: map[string]any{
							"prices": []any{
								map[string]any{
									"from":  "2025-10-23T10:00:00Z",
									"till":  "2025-10-23T11:00:00Z",
									"price": 0.15,
								},
								"invalid item", // Should be skipped
								map[string]any{
									"from":  "invalid-date",
									"till":  "2025-10-23T12:00:00Z",
									"price": 0.20,
								}, // Should be skipped (invalid from)
								map[string]any{
									"from":  "2025-10-23T12:00:00Z",
									"till":  "2025-10-23T13:00:00Z",
									"price": 0.25,
								},
							},
						},
					}, nil)
			},
			wantSlotCount:  2,
			wantFirstPrice: 0.15,
			wantErr:        false,
		},
		{
			name: "empty prices list",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{
						Attributes: map[string]any{
							"prices": []any{},
						},
					}, nil)
			},
			wantSlotCount: 0,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockState := mocks.NewMockStateInterface(ctrl)
			tt.setupMock(mockState)

			service := NewService(mockState)
			slots, err := service.GetPriceSlots()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetPriceSlots() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(slots) != tt.wantSlotCount {
					t.Errorf("GetPriceSlots() got %d slots, want %d", len(slots), tt.wantSlotCount)
				}

				if tt.wantSlotCount > 0 && slots[0].Price != tt.wantFirstPrice {
					t.Errorf("GetPriceSlots() first price = %f, want %f", slots[0].Price, tt.wantFirstPrice)
				}
			}
		})
	}
}

func TestService_GetCurrentPrice(t *testing.T) {
	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)
	hourFromNow := now.Add(1 * time.Hour)
	twoHoursFromNow := now.Add(2 * time.Hour)

	tests := []struct {
		name      string
		setupMock func(*mocks.MockStateInterface)
		wantPrice float64
		wantErr   bool
	}{
		{
			name: "finds current price slot",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{
						Attributes: map[string]any{
							"prices": []any{
								map[string]any{
									"from":  hourAgo.Format(time.RFC3339),
									"till":  hourFromNow.Format(time.RFC3339),
									"price": 0.15,
								},
								map[string]any{
									"from":  hourFromNow.Format(time.RFC3339),
									"till":  twoHoursFromNow.Format(time.RFC3339),
									"price": 0.20,
								},
							},
						},
					}, nil)
			},
			wantPrice: 0.15,
			wantErr:   false,
		},
		{
			name: "no current price slot found",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{
						Attributes: map[string]any{
							"prices": []any{
								map[string]any{
									"from":  hourFromNow.Format(time.RFC3339),
									"till":  twoHoursFromNow.Format(time.RFC3339),
									"price": 0.15,
								},
							},
						},
					}, nil)
			},
			wantErr: true,
		},
		{
			name: "GetPriceSlots fails",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{}, fmt.Errorf("sensor error"))
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

			service := NewService(mockState)
			price, err := service.GetCurrentPrice()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetCurrentPrice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && price != tt.wantPrice {
				t.Errorf("GetCurrentPrice() = %f, want %f", price, tt.wantPrice)
			}
		})
	}
}

func TestService_GetPriceSlotsInWindow(t *testing.T) {
	baseTime := time.Date(2025, 10, 23, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		setupMock     func(*mocks.MockStateInterface)
		from          time.Time
		until         time.Time
		wantSlotCount int
		wantErr       bool
	}{
		{
			name: "filter slots within window",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{
						Attributes: map[string]any{
							"prices": []any{
								map[string]any{
									"from":  baseTime.Format(time.RFC3339),
									"till":  baseTime.Add(1 * time.Hour).Format(time.RFC3339),
									"price": 0.15,
								},
								map[string]any{
									"from":  baseTime.Add(1 * time.Hour).Format(time.RFC3339),
									"till":  baseTime.Add(2 * time.Hour).Format(time.RFC3339),
									"price": 0.20,
								},
								map[string]any{
									"from":  baseTime.Add(2 * time.Hour).Format(time.RFC3339),
									"till":  baseTime.Add(3 * time.Hour).Format(time.RFC3339),
									"price": 0.25,
								},
								map[string]any{
									"from":  baseTime.Add(3 * time.Hour).Format(time.RFC3339),
									"till":  baseTime.Add(4 * time.Hour).Format(time.RFC3339),
									"price": 0.30,
								},
							},
						},
					}, nil)
			},
			from:          baseTime.Add(30 * time.Minute),
			until:         baseTime.Add(150 * time.Minute),
			wantSlotCount: 2, // Both 1-hour and 2-hour slots start within window (after baseTime+30m)
			wantErr:       false,
		},
		{
			name: "no slots in window",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{
						Attributes: map[string]any{
							"prices": []any{
								map[string]any{
									"from":  baseTime.Format(time.RFC3339),
									"till":  baseTime.Add(1 * time.Hour).Format(time.RFC3339),
									"price": 0.15,
								},
							},
						},
					}, nil)
			},
			from:          baseTime.Add(2 * time.Hour),
			until:         baseTime.Add(3 * time.Hour),
			wantSlotCount: 0,
			wantErr:       false,
		},
		{
			name: "GetPriceSlots fails",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{}, fmt.Errorf("sensor error"))
			},
			from:    baseTime,
			until:   baseTime.Add(1 * time.Hour),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockState := mocks.NewMockStateInterface(ctrl)
			tt.setupMock(mockState)

			service := NewService(mockState)
			slots, err := service.GetPriceSlotsInWindow(tt.from, tt.until)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetPriceSlotsInWindow() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(slots) != tt.wantSlotCount {
				t.Errorf("GetPriceSlotsInWindow() got %d slots, want %d", len(slots), tt.wantSlotCount)
			}
		})
	}
}

func TestService_GetAveragePrice(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(*mocks.MockStateInterface)
		wantAverage  float64
		wantErr      bool
		errorMessage string
	}{
		{
			name: "calculates average from multiple slots",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{
						Attributes: map[string]any{
							"prices": []any{
								map[string]any{
									"from":  "2025-10-23T10:00:00Z",
									"till":  "2025-10-23T11:00:00Z",
									"price": 0.10,
								},
								map[string]any{
									"from":  "2025-10-23T11:00:00Z",
									"till":  "2025-10-23T12:00:00Z",
									"price": 0.20,
								},
								map[string]any{
									"from":  "2025-10-23T12:00:00Z",
									"till":  "2025-10-23T13:00:00Z",
									"price": 0.30,
								},
							},
						},
					}, nil)
			},
			wantAverage: 0.20, // (0.10 + 0.20 + 0.30) / 3
			wantErr:     false,
		},
		{
			name: "no price slots available",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{
						Attributes: map[string]any{
							"prices": []any{},
						},
					}, nil)
			},
			wantErr:      true,
			errorMessage: "no price slots available",
		},
		{
			name: "GetPriceSlots fails",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{}, fmt.Errorf("sensor error"))
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

			service := NewService(mockState)
			avg, err := service.GetAveragePrice()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetAveragePrice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				const epsilon = 0.0001
				if diff := avg - tt.wantAverage; diff < -epsilon || diff > epsilon {
					t.Errorf("GetAveragePrice() = %f, want %f (diff: %f)", avg, tt.wantAverage, diff)
				}
			} else if tt.errorMessage != "" && err.Error() != tt.errorMessage {
				t.Errorf("GetAveragePrice() error message = %q, want %q", err.Error(), tt.errorMessage)
			}
		})
	}
}

func TestService_IsCurrentlyExpensive(t *testing.T) {
	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)
	hourFromNow := now.Add(1 * time.Hour)
	twoHoursFromNow := now.Add(2 * time.Hour)

	tests := []struct {
		name        string
		setupMock   func(*mocks.MockStateInterface)
		wantExpensive bool
		wantErr     bool
	}{
		{
			name: "current price above average",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{
						Attributes: map[string]any{
							"prices": []any{
								map[string]any{
									"from":  hourAgo.Format(time.RFC3339),
									"till":  hourFromNow.Format(time.RFC3339),
									"price": 0.30, // Current: 0.30
								},
								map[string]any{
									"from":  hourFromNow.Format(time.RFC3339),
									"till":  twoHoursFromNow.Format(time.RFC3339),
									"price": 0.10, // Future: 0.10
								},
							},
						},
					}, nil).
					Times(2) // Called twice: once by GetCurrentPrice, once by GetAveragePrice
			},
			wantExpensive: true, // 0.30 > 0.20 (average)
			wantErr:     false,
		},
		{
			name: "current price below average",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{
						Attributes: map[string]any{
							"prices": []any{
								map[string]any{
									"from":  hourAgo.Format(time.RFC3339),
									"till":  hourFromNow.Format(time.RFC3339),
									"price": 0.10, // Current: 0.10
								},
								map[string]any{
									"from":  hourFromNow.Format(time.RFC3339),
									"till":  twoHoursFromNow.Format(time.RFC3339),
									"price": 0.30, // Future: 0.30
								},
							},
						},
					}, nil).
					Times(2)
			},
			wantExpensive: false, // 0.10 < 0.20 (average)
			wantErr:     false,
		},
		{
			name: "GetCurrentPrice fails",
			setupMock: func(m *mocks.MockStateInterface) {
				m.EXPECT().
					Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
					Return(ga.EntityState{}, fmt.Errorf("sensor error"))
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

			service := NewService(mockState)
			expensive, err := service.IsCurrentlyExpensive()

			if (err != nil) != tt.wantErr {
				t.Errorf("IsCurrentlyExpensive() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && expensive != tt.wantExpensive {
				t.Errorf("IsCurrentlyExpensive() = %v, want %v", expensive, tt.wantExpensive)
			}
		})
	}
}
