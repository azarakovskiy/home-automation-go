package entities

type CustomEventsDomain struct {
	ScheduleDishwasher string
}

var CustomEvents = CustomEventsDomain{
	ScheduleDishwasher: "event.custom_schedule_dishwasher",
}
