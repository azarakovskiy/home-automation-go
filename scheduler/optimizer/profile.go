package optimizer

// Profile is a generic device profile implementation
// Devices can use this directly or create their own implementation of DeviceProfile
type Profile struct {
	Mode              string
	DurationHours     int
	StageWeights      []float64
	PowerKW           float64
	MinSavingsPercent float64 // Minimum savings percentage to delay start
}

// Verify Profile implements the interface at compile time
var _ DeviceProfile = (*Profile)(nil)

func (p Profile) GetDurationHours() int {
	return p.DurationHours
}

func (p Profile) GetStageWeights() []float64 {
	return p.StageWeights
}

func (p Profile) GetPowerKW() float64 {
	return p.PowerKW
}

func (p Profile) GetMode() string {
	return p.Mode
}

func (p Profile) GetMinSavingsPercent() float64 {
	return p.MinSavingsPercent
}
