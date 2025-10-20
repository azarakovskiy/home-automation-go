package main

import (
	"encoding/json"
	"fmt"
	"home-go/entities"
	"log"
	"os"

	ga "saml.dev/gome-assistant"
)

func main() {
	// Get Home Assistant URL from environment or use default
	haURL := os.Getenv("HA_URL")
	if haURL == "" {
		log.Fatalf("HA_URL is not set")
	}
	authToken := os.Getenv("HA_AUTH_TOKEN")
	if authToken == "" {
		log.Fatalf("HA_AUTH_TOKEN is not set")
	}

	app, err := ga.NewApp(ga.NewAppRequest{
		URL:         haURL,
		HAAuthToken: authToken,
	})
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}
	defer app.Cleanup()

	dishwasherListener := ga.NewEventListener().
		EventTypes(entities.CustomEvents.ScheduleDishwasher).
		Call(consumeEvent).
		Build()

	lightListener := ga.NewEntityListener().
		EntityIds(entities.InputBoolean.HeadHealed, entities.InputBoolean.VitaminsTaken).
		Call(blinkLights).
		Build()

	app.RegisterEntityListeners(lightListener)
	app.RegisterEventListeners(dishwasherListener)

	app.Start()
}

type EventData struct {
	Source      string `json:"source"`
	User        string `json:"user"`
	Timestamp   string `json:"timestamp"`
	Temperature string `json:"temperature"`
	// Add more fields if needed
}

type HAEvent struct {
	EventType string    `json:"event_type"`
	Data      EventData `json:"data"`
	Origin    string    `json:"origin"`
	TimeFired string    `json:"time_fired"`
	Context   any       `json:"context"`
}

func consumeEvent(service *ga.Service, state ga.State, event ga.EventData) {
	e := HAEvent{}
	if err := json.Unmarshal(event.RawEventJSON, &e); err != nil {
		log.Fatalf("Failed to parse json")
	}

	fmt.Printf("Event Type: %s\n", e.EventType)
	fmt.Printf("User: %s\n", e.Data.User)
	fmt.Printf("Temperature: %s\n", e.Data.Temperature)
}

func blinkLights(service *ga.Service, state ga.State, sensor ga.EntityData) {
	light := entities.Light.LivingRoomBlackLamp
	switch sensor.ToState {
	case "on":
		err := service.HomeAssistant.TurnOn(light)
		if err != nil {
			log.Printf("Failed to turn on switch: %v", err)
		}
	case "off":
		err := service.HomeAssistant.TurnOff(light)
		if err != nil {
			log.Printf("Failed to turn off switch: %v", err)
		}
	}
}
