package main

import (
	"log"

	"home-go/internal/tech/bootstrap"
)

func main() {
	if err := bootstrap.RunFromEnv(); err != nil {
		log.Fatal(err)
	}
}
