package entities

type CustomEventsDomain struct {
	ScheduledStart  string
	Notify          string
	ReminderCreate  string
	ReminderAck     string
	ReminderDelete  string
	GetPriceSummary string
}

var CustomEvents = CustomEventsDomain{
	ScheduledStart:  "event.custom_scheduled_start",
	Notify:          "event.custom_notify",
	ReminderCreate:  "event.custom_reminder_create",
	ReminderAck:     "event.custom_reminder_ack",
	ReminderDelete:  "event.custom_reminder_delete",
	GetPriceSummary: "event.custom_get_price_summary",
}

// CustomSensorsDomain contains sensors that are not auto-generated
// These come from external sources like HA companion apps
type CustomSensorsDomain struct {
	OfficeLaptopWorkInternalBatteryLevel string
	OfficeLaptopWorkInternalBatteryState string
}

var CustomSensors = CustomSensorsDomain{
	OfficeLaptopWorkInternalBatteryLevel: "sensor.office_laptop_work_internal_battery_level",
	OfficeLaptopWorkInternalBatteryState: "sensor.office_laptop_work_internal_battery_state",
}
