package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	HAURL       string
	HAAuthToken string
	Debug       bool
	DryRun      bool
}

func Load() (Config, error) {
	cfg := Config{
		HAURL:       os.Getenv("HA_URL"),
		HAAuthToken: os.Getenv("HA_AUTH_TOKEN"),
		Debug:       isEnabled("DEBUG"),
		DryRun:      isEnabled("DRY_RUN"),
	}

	if strings.TrimSpace(cfg.HAURL) == "" {
		return Config{}, fmt.Errorf("HA_URL is not set")
	}
	if strings.TrimSpace(cfg.HAAuthToken) == "" {
		return Config{}, fmt.Errorf("HA_AUTH_TOKEN is not set")
	}

	return cfg, nil
}

func isEnabled(name string) bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(name)), "true")
}
