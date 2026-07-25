package pricing

// PriceResponse is the payload of the pricing/current_price resource.
type PriceResponse struct {
	Price float64 `json:"price"`
}

// NextPriceInfo describes the next price change after the current time.
type NextPriceInfo struct {
	Price float64 `json:"price"`
	From  string  `json:"from"`
	Till  string  `json:"till"`
	Level string  `json:"level"`
}

// PriceWindow is a time range of slots used in summary responses.
type PriceWindow struct {
	From     string  `json:"from"`
	Till     string  `json:"till"`
	AvgPrice float64 `json:"avg_price"`
}

// PriceSummary describes the available pricing horizon from the current
// moment until the last known slot. The thresholds let the agent reason
// about what "cheap" or "expensive" means numerically for the day.
type PriceSummary struct {
	CurrentPrice       float64       `json:"current_price"`
	CurrentLevel       string        `json:"current_level"`
	MedianPrice        float64       `json:"median_price"`
	CheapThreshold     float64       `json:"cheap_threshold"`
	ExpensiveThreshold float64       `json:"expensive_threshold"`
	MinPrice           float64       `json:"min_price"`
	MaxPrice           float64       `json:"max_price"`
	AveragePrice       float64       `json:"average_price"`
	CheapWindows       []PriceWindow `json:"cheap_windows"`
	ExpensiveWindows   []PriceWindow `json:"expensive_windows"`
	NegativeWindows    []PriceWindow `json:"negative_windows,omitempty"`
}

// CheapestWindow is the optimal consecutive block of slots for an appliance
// of the requested duration within the requested deadline.
type CheapestWindow struct {
	From     string  `json:"from"`
	Till     string  `json:"till"`
	AvgPrice float64 `json:"avg_price"`
	Level    string  `json:"level"`
}
