package dryrun

import (
	"fmt"
	"testing"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name  string
		input bool
		want  bool
	}{
		{name: "enabled", input: true, want: true},
		{name: "disabled", input: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled = false
			Init(tt.input)
			if IsEnabled() != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", IsEnabled(), tt.want)
			}
		})
	}
}

func TestCall(t *testing.T) {
	tests := []struct {
		name        string
		dryRunMode  bool
		shouldCall  bool
		fnReturns   error
		expectError bool
	}{
		{
			name:        "dry-run enabled, function not called",
			dryRunMode:  true,
			shouldCall:  false,
			fnReturns:   nil,
			expectError: false,
		},
		{
			name:        "dry-run disabled, function called",
			dryRunMode:  false,
			shouldCall:  true,
			fnReturns:   nil,
			expectError: false,
		},
		{
			name:        "dry-run disabled, function returns error",
			dryRunMode:  false,
			shouldCall:  true,
			fnReturns:   fmt.Errorf("test error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled = tt.dryRunMode
			called := false

			err := Call("TestAction", "test.entity", func() error {
				called = true
				return tt.fnReturns
			})

			if called != tt.shouldCall {
				t.Errorf("Function called = %v, want %v", called, tt.shouldCall)
			}

			if (err != nil) != tt.expectError {
				t.Errorf("Error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestCallWithData(t *testing.T) {
	tests := []struct {
		name        string
		dryRunMode  bool
		data        interface{}
		shouldCall  bool
		fnReturns   error
		expectError bool
	}{
		{
			name:        "dry-run enabled with string data",
			dryRunMode:  true,
			data:        "test value",
			shouldCall:  false,
			fnReturns:   nil,
			expectError: false,
		},
		{
			name:        "dry-run enabled with numeric data",
			dryRunMode:  true,
			data:        42,
			shouldCall:  false,
			fnReturns:   nil,
			expectError: false,
		},
		{
			name:        "dry-run disabled, function called",
			dryRunMode:  false,
			data:        "test value",
			shouldCall:  true,
			fnReturns:   nil,
			expectError: false,
		},
		{
			name:        "dry-run disabled, function returns error",
			dryRunMode:  false,
			data:        123.45,
			shouldCall:  true,
			fnReturns:   fmt.Errorf("test error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled = tt.dryRunMode
			called := false

			err := CallWithData("TestAction", "test.entity", tt.data, func() error {
				called = true
				return tt.fnReturns
			})

			if called != tt.shouldCall {
				t.Errorf("Function called = %v, want %v", called, tt.shouldCall)
			}

			if (err != nil) != tt.expectError {
				t.Errorf("Error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{
			name:    "enabled",
			enabled: true,
			want:    true,
		},
		{
			name:    "disabled",
			enabled: false,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled = tt.enabled
			if got := IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
