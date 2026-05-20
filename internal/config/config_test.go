package config

import "testing"

func TestLoad(t *testing.T) {
	tests := []struct {
		name         string
		env          map[string]string
		wantErr      bool
		wantDebug    bool
		wantDry      bool
		wantHTTPHost string
		wantHTTPPort int
	}{
		{
			name: "loads config and runtime flags",
			env: map[string]string{
				"HA_URL":             "http://home-assistant:8123",
				"HA_AUTH_TOKEN":      "token",
				"HA_MQTT_BROKER_URL": "tcp://mqtt:1883",
				"HA_MQTT_USERNAME":   "mqtt-user",
				"HA_MQTT_PASSWORD":   "mqtt-pass",
				"DEBUG":              "true",
				"DRY_RUN":            "true",
			},
			wantDebug: true,
			wantDry:   true,
		},
		{
			name: "requires ha url",
			env: map[string]string{
				"HA_AUTH_TOKEN":      "token",
				"HA_MQTT_BROKER_URL": "tcp://mqtt:1883",
			},
			wantErr: true,
		},
		{
			name: "requires auth token",
			env: map[string]string{
				"HA_URL":             "http://home-assistant:8123",
				"HA_MQTT_BROKER_URL": "tcp://mqtt:1883",
			},
			wantErr: true,
		},
		{
			name: "requires mqtt broker url",
			env: map[string]string{
				"HA_URL":        "http://home-assistant:8123",
				"HA_AUTH_TOKEN": "token",
			},
			wantErr: true,
		},
		{
			name: "uses HTTP defaults when env not set",
			env: map[string]string{
				"HA_URL":             "http://home-assistant:8123",
				"HA_AUTH_TOKEN":      "token",
				"HA_MQTT_BROKER_URL": "tcp://mqtt:1883",
			},
			wantHTTPHost: "0.0.0.0",
			wantHTTPPort: 8080,
		},
		{
			name: "reads HTTP_HOST and HTTP_PORT from env",
			env: map[string]string{
				"HA_URL":             "http://home-assistant:8123",
				"HA_AUTH_TOKEN":      "token",
				"HA_MQTT_BROKER_URL": "tcp://mqtt:1883",
				"HTTP_HOST":          "127.0.0.1",
				"HTTP_PORT":          "9090",
			},
			wantHTTPHost: "127.0.0.1",
			wantHTTPPort: 9090,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HA_URL", "")
			t.Setenv("HA_AUTH_TOKEN", "")
			t.Setenv("HA_MQTT_BROKER_URL", "")
			t.Setenv("HA_MQTT_USERNAME", "")
			t.Setenv("HA_MQTT_PASSWORD", "")
			t.Setenv("DEBUG", "")
			t.Setenv("DRY_RUN", "")
			t.Setenv("HTTP_HOST", "")
			t.Setenv("HTTP_PORT", "")
			t.Setenv("DATABASE_URL", "")

			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			assertConfig(t, cfg, tt.env, tt.wantDebug, tt.wantDry, tt.wantHTTPHost, tt.wantHTTPPort)
		})
	}
}

func assertConfig(t *testing.T, cfg Config, env map[string]string, wantDebug, wantDry bool, wantHTTPHost string, wantHTTPPort int) {
	t.Helper()
	if cfg.HAURL != env["HA_URL"] {
		t.Fatalf("HAURL = %q, want %q", cfg.HAURL, env["HA_URL"])
	}
	if cfg.HAAuthToken != env["HA_AUTH_TOKEN"] {
		t.Fatalf("HAAuthToken = %q, want %q", cfg.HAAuthToken, env["HA_AUTH_TOKEN"])
	}
	if cfg.MQTT.BrokerURL != env["HA_MQTT_BROKER_URL"] {
		t.Fatalf("MQTT.BrokerURL = %q, want %q", cfg.MQTT.BrokerURL, env["HA_MQTT_BROKER_URL"])
	}
	if cfg.MQTT.Username != env["HA_MQTT_USERNAME"] {
		t.Fatalf("MQTT.Username = %q, want %q", cfg.MQTT.Username, env["HA_MQTT_USERNAME"])
	}
	if cfg.MQTT.Password != env["HA_MQTT_PASSWORD"] {
		t.Fatalf("MQTT.Password = %q, want %q", cfg.MQTT.Password, env["HA_MQTT_PASSWORD"])
	}
	if cfg.Debug != wantDebug {
		t.Fatalf("Debug = %t, want %t", cfg.Debug, wantDebug)
	}
	if cfg.DryRun != wantDry {
		t.Fatalf("DryRun = %t, want %t", cfg.DryRun, wantDry)
	}
	if wantHTTPHost != "" {
		if cfg.HTTP.Host != wantHTTPHost {
			t.Fatalf("HTTP.Host = %q, want %q", cfg.HTTP.Host, wantHTTPHost)
		}
		if cfg.HTTP.Port != wantHTTPPort {
			t.Fatalf("HTTP.Port = %d, want %d", cfg.HTTP.Port, wantHTTPPort)
		}
	}
}
