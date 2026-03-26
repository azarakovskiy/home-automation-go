package bootstrap

import (
	"os"
	"testing"

	"home-go/internal/config"
)

func TestSyncRuntimeEnv(t *testing.T) {
	tests := []struct {
		name  string
		cfg   config.Config
		want  map[string]string
		unset []string
	}{
		{
			name: "enables runtime flags",
			cfg: config.Config{
				Debug:  true,
				DryRun: true,
			},
			want: map[string]string{
				"DEBUG":   "true",
				"DRY_RUN": "true",
			},
		},
		{
			name: "disables runtime flags",
			cfg: config.Config{
				Debug:  false,
				DryRun: false,
			},
			unset: []string{
				"DEBUG",
				"DRY_RUN",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DEBUG", "preset")
			t.Setenv("DRY_RUN", "preset")

			syncRuntimeEnv(tt.cfg)

			for key, want := range tt.want {
				if got := os.Getenv(key); got != want {
					t.Fatalf("%s = %q, want %q", key, got, want)
				}
			}

			for _, key := range tt.unset {
				if got := os.Getenv(key); got != "" {
					t.Fatalf("%s = %q, want empty", key, got)
				}
			}
		})
	}
}
