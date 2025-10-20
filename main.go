package main

import (
	"home-go/entities"
	"log"
	"os"

	ga "saml.dev/gome-assistant"
)

func main() {
	// Get Home Assistant URL from environment or use default
	haURL := os.Getenv("HA_URL")
	if haURL == "" {
		haURL = "http://192.168.1.43:8123" // Default fallback
	}

	// Replace with your Home Assistant URL and auth token
	app, err := ga.NewApp(ga.NewAppRequest{
		URL:              haURL,
		HAAuthToken:      os.Getenv("HA_AUTH_TOKEN"),
		HomeZoneEntityId: "zone.home",
	})
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}
	defer app.Cleanup()

	lightListener := ga.NewEntityListener().
		EntityIds(entities.InputBoolean.HeadHealed, entities.InputBoolean.VitaminsTaken).
		Call(blinkLights).
		Build()

	app.RegisterEntityListeners(lightListener)

	app.Start()
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
