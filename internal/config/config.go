package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	HAURL       string
	HAAuthToken string
	MQTT        MQTTConfig
	Debug       bool
	DryRun      bool
}

type MQTTConfig struct {
	BrokerURL string
	Username  string
	Password  string
}

func Load() (Config, error) {
	cfg := Config{
		HAURL:       os.Getenv("HA_URL"),
		HAAuthToken: os.Getenv("HA_AUTH_TOKEN"),
		MQTT: MQTTConfig{
			BrokerURL: os.Getenv("HA_MQTT_BROKER_URL"),
			Username:  os.Getenv("HA_MQTT_USERNAME"),
			Password:  os.Getenv("HA_MQTT_PASSWORD"),
		},
		Debug:  isEnabled("DEBUG"),
		DryRun: isEnabled("DRY_RUN"),
	}

	if strings.TrimSpace(cfg.HAURL) == "" {
		return Config{}, fmt.Errorf("HA_URL is not set")
	}
	if strings.TrimSpace(cfg.HAAuthToken) == "" {
		return Config{}, fmt.Errorf("HA_AUTH_TOKEN is not set")
	}
	if strings.TrimSpace(cfg.MQTT.BrokerURL) == "" {
		return Config{}, fmt.Errorf("HA_MQTT_BROKER_URL is not set")
	}

	return cfg, nil
}

func isEnabled(name string) bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(name)), "true")
}
