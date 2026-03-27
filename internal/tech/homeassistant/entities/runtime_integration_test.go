package entities

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const runtimeIntegrationEntityID = "sensor.runtime_integration_probe"

type runtimeIntegrationEnv struct {
	haURL         string
	haAuthToken   string
	mqttBrokerURL string
	mqttUsername  string
	mqttPassword  string
}

type runtimeHAState struct {
	EntityID string `json:"entity_id"`
	State    string `json:"state"`
}

func setEnv(t *testing.T) {
	t.Setenv("HA_URL", "http://192.168.1.43:8123")
	t.Setenv("HA_AUTH_TOKEN", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiI4ZDQ2ZThhNzEwM2Q0NjE2OTMwODY5MjJkOGU0MGNkYiIsImlhdCI6MTc2MDk4OTA5NCwiZXhwIjoyMDc2MzQ5MDk0fQ.qNpTY-ShJYvPPuHW9NyN59QTj8EZ8iu6kE8n1K_aSew")
	t.Setenv("HA_MQTT_BROKER_URL", "mqtt://192.168.1.43:1883")
	t.Setenv("HA_MQTT_USERNAME", "homego")
	t.Setenv("HA_MQTT_PASSWORD", "homego-service")
}

func TestRuntimeIntegrationCreateEntity(t *testing.T) {
	setEnv(t)

	env := loadRuntimeIntegrationEnv(t)
	rt := newIntegrationRuntime(t, env, "create")

	sensor, err := rt.NumberSensor(context.Background(), NumberSensorSpec{
		CommonSpec: CommonSpec{
			Key:          strings.TrimPrefix(runtimeIntegrationEntityID, "sensor."),
			Name:         "Runtime Integration Create",
			EntityIDHint: runtimeIntegrationEntityID,
			Icon:         "mdi:test-tube",
		},
		UnitOfMeasurement: "EUR",
	})
	if err != nil {
		t.Fatalf("NumberSensor() error = %v", err)
	}

	t.Logf("created entity %s", runtimeIntegrationEntityID)
	waitForHAEntity(t, env, runtimeIntegrationEntityID, 20*time.Second, func(_ runtimeHAState, status int) bool {
		return status == http.StatusOK
	})

	_ = sensor
}

func TestRuntimeIntegrationUpdateEntity(t *testing.T) {
	setEnv(t)

	env := loadRuntimeIntegrationEnv(t)
	rt := newIntegrationRuntime(t, env, "update")

	sensor, err := rt.NumberSensor(context.Background(), NumberSensorSpec{
		CommonSpec: CommonSpec{
			Key:          strings.TrimPrefix(runtimeIntegrationEntityID, "sensor."),
			Name:         "Runtime Integration Update",
			EntityIDHint: runtimeIntegrationEntityID,
			Icon:         "mdi:test-tube",
		},
		UnitOfMeasurement: "EUR",
	})
	if err != nil {
		t.Fatalf("NumberSensor() error = %v", err)
	}

	if err := sensor.Set(context.Background(), 12.34); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	t.Logf("updated entity %s to 12.34", runtimeIntegrationEntityID)
	waitForHAEntity(t, env, runtimeIntegrationEntityID, 20*time.Second, func(state runtimeHAState, status int) bool {
		return status == http.StatusOK && state.State == "12.34"
	})
}

func TestRuntimeIntegrationRemoveEntity(t *testing.T) {
	setEnv(t)

	env := loadRuntimeIntegrationEnv(t)
	rt := newIntegrationRuntime(t, env, "remove")

	sensor, err := rt.NumberSensor(context.Background(), NumberSensorSpec{
		CommonSpec: CommonSpec{
			Key:          strings.TrimPrefix(runtimeIntegrationEntityID, "sensor."),
			Name:         "Runtime Integration Remove",
			EntityIDHint: runtimeIntegrationEntityID,
			Icon:         "mdi:test-tube",
		},
		UnitOfMeasurement: "EUR",
	})
	if err != nil {
		t.Fatalf("NumberSensor() error = %v", err)
	}

	if err := sensor.Remove(context.Background()); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	t.Logf("removed entity %s", runtimeIntegrationEntityID)
	waitForHAEntity(t, env, runtimeIntegrationEntityID, 20*time.Second, func(_ runtimeHAState, status int) bool {
		return status == http.StatusNotFound
	})
}

func loadRuntimeIntegrationEnv(t *testing.T) runtimeIntegrationEnv {
	t.Helper()

	env := runtimeIntegrationEnv{
		haURL:         strings.TrimRight(strings.TrimSpace(os.Getenv("HA_URL")), "/"),
		haAuthToken:   strings.TrimSpace(os.Getenv("HA_AUTH_TOKEN")),
		mqttBrokerURL: strings.TrimSpace(os.Getenv("HA_MQTT_BROKER_URL")),
		mqttUsername:  strings.TrimSpace(os.Getenv("HA_MQTT_USERNAME")),
		mqttPassword:  strings.TrimSpace(os.Getenv("HA_MQTT_PASSWORD")),
	}

	var missing []string
	if env.haURL == "" {
		missing = append(missing, "HA_URL")
	}
	if env.haAuthToken == "" {
		missing = append(missing, "HA_AUTH_TOKEN")
	}
	if env.mqttBrokerURL == "" {
		missing = append(missing, "HA_MQTT_BROKER_URL")
	}
	if len(missing) > 0 {
		t.Skipf("runtime integration test requires %s", strings.Join(missing, ", "))
	}

	return env
}

func newIntegrationRuntime(t *testing.T, env runtimeIntegrationEnv, suffix string) *Runtime {
	t.Helper()

	rt, err := NewRuntime(RuntimeConfig{
		BrokerURL:    env.mqttBrokerURL,
		Username:     env.mqttUsername,
		Password:     env.mqttPassword,
		AppPrefix:    "home-go-test",
		ClientID:     fmt.Sprintf("home-go-test-%s-%d", suffix, time.Now().UnixNano()),
		RegistryPath: "",
	})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}

	t.Cleanup(func() {
		if err := rt.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	return rt
}

func waitForHAEntity(t *testing.T, env runtimeIntegrationEnv, entityID string, timeout time.Duration, match func(runtimeHAState, int) bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, status, err := getHAState(t.Context(), env, entityID)
		if err == nil && match(state, status) {
			t.Logf("entity %s reached expected state with http status %d and state %q", entityID, status, state.State)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	state, status, err := getHAState(t.Context(), env, entityID)
	t.Fatalf("entity %s did not reach expected state before timeout; last status=%d state=%q err=%v", entityID, status, state.State, err)
}

func getHAState(ctx context.Context, env runtimeIntegrationEnv, entityID string) (runtimeHAState, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, env.haURL+"/api/states/"+entityID, nil)
	if err != nil {
		return runtimeHAState{}, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+env.haAuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return runtimeHAState{}, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return runtimeHAState{}, resp.StatusCode, nil
	}

	var state runtimeHAState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return runtimeHAState{}, resp.StatusCode, err
	}
	return state, resp.StatusCode, nil
}
