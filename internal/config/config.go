package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HAURL       string
	HAAuthToken string
	MQTT        MQTTConfig
	Database    DatabaseConfig
	HTTP        HTTPConfig
	Debug       bool
	DryRun      bool
}

type MQTTConfig struct {
	BrokerURL           string
	Username            string
	Password            string
	DiscoveryPrefix     string
	AppPrefix           string
	AppName             string
	DeviceNameSeparator string
}

type DatabaseConfig struct {
	Path string
}

type HTTPConfig struct {
	Host string
	Port int
}

func Load() (Config, error) {
	cfg := Config{
		HAURL:       os.Getenv("HA_URL"),
		HAAuthToken: os.Getenv("HA_AUTH_TOKEN"),
		MQTT: MQTTConfig{
			BrokerURL:           os.Getenv("HA_MQTT_BROKER_URL"),
			Username:            os.Getenv("HA_MQTT_USERNAME"),
			Password:            os.Getenv("HA_MQTT_PASSWORD"),
			DiscoveryPrefix:     "homeassistant",
			AppPrefix:           "home-go",
			AppName:             envOrDefault("MQTT_APP_NAME", "home-go"),
			DeviceNameSeparator: envOrDefault("MQTT_DEVICE_NAME_SEPARATOR", " / "),
		},
		Database: DatabaseConfig{
			Path: envOrDefault("SQLITE_PATH", "./home_go.db"),
		},
		HTTP: HTTPConfig{
			Host: envOrDefault("HTTP_HOST", "0.0.0.0"),
			Port: envIntOrDefault("HTTP_PORT", 8080),
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

func envOrDefault(name, defaultValue string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return defaultValue
}

func envIntOrDefault(name string, def int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
