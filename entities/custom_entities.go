package entities

type CustomEventsDomain struct {
	ScheduledStart string
}

var CustomEvents = CustomEventsDomain{
	ScheduledStart: "event.custom_scheduled_start",
}
