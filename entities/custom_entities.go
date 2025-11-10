package entities

type CustomEventsDomain struct {
	ScheduledStart string
	Notify         string
	ReminderCreate string
	ReminderAck    string
	ReminderDelete string
}

var CustomEvents = CustomEventsDomain{
	ScheduledStart: "event.custom_scheduled_start",
	Notify:         "event.custom_notify",
	ReminderCreate: "event.home_go_reminder_create",
	ReminderAck:    "event.home_go_reminder_ack",
	ReminderDelete: "event.home_go_reminder_delete",
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
