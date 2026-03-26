package config

import "testing"

func TestLoad(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		wantErr   bool
		wantDebug bool
		wantDry   bool
	}{
		{
			name: "loads config and runtime flags",
			env: map[string]string{
				"HA_URL":        "http://home-assistant:8123",
				"HA_AUTH_TOKEN": "token",
				"DEBUG":         "true",
				"DRY_RUN":       "true",
			},
			wantDebug: true,
			wantDry:   true,
		},
		{
			name: "requires ha url",
			env: map[string]string{
				"HA_AUTH_TOKEN": "token",
			},
			wantErr: true,
		},
		{
			name: "requires auth token",
			env: map[string]string{
				"HA_URL": "http://home-assistant:8123",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HA_URL", "")
			t.Setenv("HA_AUTH_TOKEN", "")
			t.Setenv("DEBUG", "")
			t.Setenv("DRY_RUN", "")

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

			if cfg.HAURL != tt.env["HA_URL"] {
				t.Fatalf("HAURL = %q, want %q", cfg.HAURL, tt.env["HA_URL"])
			}
			if cfg.HAAuthToken != tt.env["HA_AUTH_TOKEN"] {
				t.Fatalf("HAAuthToken = %q, want %q", cfg.HAAuthToken, tt.env["HA_AUTH_TOKEN"])
			}
			if cfg.Debug != tt.wantDebug {
				t.Fatalf("Debug = %t, want %t", cfg.Debug, tt.wantDebug)
			}
			if cfg.DryRun != tt.wantDry {
				t.Fatalf("DryRun = %t, want %t", cfg.DryRun, tt.wantDry)
			}
		})
	}
}
