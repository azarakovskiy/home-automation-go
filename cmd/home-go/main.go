package main

import (
	"log"

	"home-go/internal/tech/bootstrap"
)

func main() {
	if err := bootstrap.RunFromEnv(); err != nil {
		log.Fatalf("Failed to start app: %v", err)
	}
}
