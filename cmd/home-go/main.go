package main

import (
	"log"

	app "home-go/internal"
)

func main() {
	if err := app.RunFromEnv(); err != nil {
		log.Fatalf("Failed to start app: %v", err)
	}
}
