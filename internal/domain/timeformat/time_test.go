package timeformat

import (
	"testing"
	"time"
)

func TestForSpeech(t *testing.T) {
	tests := []struct {
		name     string
		hour     int
		minute   int
		expected string
	}{
		{"midnight", 0, 0, "midnight"},
		{"noon", 12, 0, "noon"},
		{"3 AM on the hour", 3, 0, "3 AM"},
		{"3:30 AM with minutes", 3, 30, "3:30 AM"},
		{"12:15 PM after noon", 12, 15, "12:15 PM"},
		{"3 PM on the hour", 15, 0, "3 PM"},
		{"3:30 PM with minutes", 15, 30, "3:30 PM"},
		{"11:59 PM before midnight", 23, 59, "11:59 PM"},
		{"6 AM morning", 6, 0, "6 AM"},
		{"6:15 AM with minutes", 6, 15, "6:15 AM"},
		{"9 PM evening", 21, 0, "9 PM"},
		{"9:45 PM with minutes", 21, 45, "9:45 PM"},
		{"1 AM after midnight", 1, 0, "1 AM"},
		{"1:05 AM with leading zero", 1, 5, "1:05 AM"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime := time.Date(2025, 10, 23, tt.hour, tt.minute, 0, 0, time.UTC)
			result := ForSpeech(testTime)
			if result != tt.expected {
				t.Errorf("ForSpeech(%d:%02d) = %q, want %q", tt.hour, tt.minute, result, tt.expected)
			}
		})
	}
}
