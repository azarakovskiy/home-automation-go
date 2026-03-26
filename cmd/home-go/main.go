package main

import (
	"log"

	"home-go/internal/app"
)

func main() {
	if err := app.RunFromEnv(); err != nil {
		log.Fatalf("Failed to start app: %v", err)
	}
}
