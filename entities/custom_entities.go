package entities

type CustomEventsDomain struct {
	ScheduledStart string
	Notify         string
}

var CustomEvents = CustomEventsDomain{
	ScheduledStart: "event.custom_scheduled_start",
	Notify:         "event.custom_notify",
}
