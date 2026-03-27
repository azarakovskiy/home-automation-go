package entities

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
)

const (
	defaultDiscoveryPrefix = "homeassistant"
	defaultAppPrefix       = "home-go"

	runtimeAvailabilityOnline  = "online"
	runtimeAvailabilityOffline = "offline"
)

type runtimeEntityKind string

const (
	runtimeKindSwitch       runtimeEntityKind = "switch"
	runtimeKindNumberSensor runtimeEntityKind = "sensor"
	runtimeKindBinarySensor runtimeEntityKind = "binary_sensor"
)

type RuntimeConfig struct {
	BrokerURL       string
	Username        string
	Password        string
	DiscoveryPrefix string
	AppPrefix       string
	ClientID        string
	RegistryPath    string
}

type CommonSpec struct {
	Key          string
	Name         string
	EntityIDHint string
	Icon         string
	DeviceClass  string
}

type SwitchSpec struct {
	CommonSpec
}

type NumberSensorSpec struct {
	CommonSpec
	UnitOfMeasurement string
}

type BinarySensorSpec struct {
	CommonSpec
}

type Runtime struct {
	mqtt runtimeTransport

	discoveryPrefix   string
	appPrefix         string
	availabilityTopic string
	haStatusTopic     string
	commandTopic      string

	registry *runtimeRegistry

	mu             sync.RWMutex
	entities       map[string]*runtimeEntity
	switchHandlers map[string]func(context.Context, bool) error
}

type runtimeEntity struct {
	key            string
	kind           runtimeEntityKind
	discoveryTopic string
	stateTopic     string
	commandTopic   string

	discoveryPayload []byte
	lastState        []byte
	hasState         bool
}

type SwitchHandle struct {
	runtime *Runtime
	key     string
}

type NumberSensorHandle struct {
	runtime *Runtime
	key     string
}

type BinarySensorHandle struct {
	runtime *Runtime
	key     string
}

func NewRuntime(cfg RuntimeConfig) (*Runtime, error) {
	cfg = cfg.withDefaults()
	if strings.TrimSpace(cfg.BrokerURL) == "" {
		return nil, fmt.Errorf("broker URL is required")
	}

	registry, err := newRuntimeRegistry(cfg.RegistryPath)
	if err != nil {
		return nil, fmt.Errorf("create runtime registry: %w", err)
	}

	rt := &Runtime{
		discoveryPrefix:   cfg.DiscoveryPrefix,
		appPrefix:         cfg.AppPrefix,
		availabilityTopic: fmt.Sprintf("%s/status", cfg.AppPrefix),
		haStatusTopic:     fmt.Sprintf("%s/status", cfg.DiscoveryPrefix),
		commandTopic:      fmt.Sprintf("%s/entities/+/set", cfg.AppPrefix),
		registry:          registry,
		entities:          make(map[string]*runtimeEntity),
		switchHandlers:    make(map[string]func(context.Context, bool) error),
	}

	transport, err := newPahoRuntimeTransport(cfg, rt.availabilityTopic)
	if err != nil {
		return nil, fmt.Errorf("create mqtt transport: %w", err)
	}
	rt.mqtt = transport
	rt.mqtt.SetOnConnect(rt.handleReconnect)

	if err := rt.mqtt.Connect(context.Background()); err != nil {
		return nil, fmt.Errorf("connect mqtt transport: %w", err)
	}

	if err := rt.mqtt.Subscribe(context.Background(), rt.haStatusTopic, rt.handleHomeAssistantStatus); err != nil {
		return nil, fmt.Errorf("subscribe to home assistant status: %w", err)
	}

	if err := rt.mqtt.Subscribe(context.Background(), rt.commandTopic, rt.handleCommand); err != nil {
		return nil, fmt.Errorf("subscribe to runtime commands: %w", err)
	}

	return rt, nil
}

func (r *Runtime) Close() error {
	return r.mqtt.Close()
}

func (r *Runtime) Switch(ctx context.Context, spec SwitchSpec) (*SwitchHandle, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return nil, fmt.Errorf("switch name is required")
	}
	entity, err := r.declare(ctx, runtimeKindSwitch, spec.CommonSpec, switchDiscoveryPayload(spec))
	if err != nil {
		return nil, err
	}
	return &SwitchHandle{runtime: r, key: entity.key}, nil
}

func (r *Runtime) NumberSensor(ctx context.Context, spec NumberSensorSpec) (*NumberSensorHandle, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return nil, fmt.Errorf("number sensor name is required")
	}
	entity, err := r.declare(ctx, runtimeKindNumberSensor, spec.CommonSpec, numberSensorDiscoveryPayload(spec))
	if err != nil {
		return nil, err
	}
	return &NumberSensorHandle{runtime: r, key: entity.key}, nil
}

func (r *Runtime) BinarySensor(ctx context.Context, spec BinarySensorSpec) (*BinarySensorHandle, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return nil, fmt.Errorf("binary sensor name is required")
	}
	entity, err := r.declare(ctx, runtimeKindBinarySensor, spec.CommonSpec, binarySensorDiscoveryPayload(spec))
	if err != nil {
		return nil, err
	}
	return &BinarySensorHandle{runtime: r, key: entity.key}, nil
}

func (r *Runtime) Remove(ctx context.Context, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key is required")
	}

	r.mu.Lock()
	entity, ok := r.entities[key]
	if ok {
		delete(r.entities, key)
		delete(r.switchHandlers, key)
	}
	r.mu.Unlock()

	if !ok {
		kind, exists := r.registry.Kind(key)
		if !exists {
			return nil
		}
		entity = r.buildEntity(key, kind)
	}

	if err := r.publish(ctx, entity.discoveryTopic, true, []byte("")); err != nil {
		return fmt.Errorf("remove discovery topic for %s: %w", key, err)
	}
	if err := r.publish(ctx, entity.stateTopic, true, []byte("")); err != nil {
		return fmt.Errorf("remove state topic for %s: %w", key, err)
	}
	if err := r.registry.Remove(key); err != nil {
		return fmt.Errorf("remove %s from registry: %w", key, err)
	}

	return nil
}

func (r *Runtime) Reconcile(ctx context.Context, keep []string) error {
	if !r.registry.Persistent() {
		return fmt.Errorf("reconcile requires registry path")
	}

	keepSet := make(map[string]struct{}, len(keep))
	for _, key := range keep {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		keepSet[key] = struct{}{}
	}

	for key := range r.registry.Snapshot() {
		if _, ok := keepSet[key]; ok {
			continue
		}
		if err := r.Remove(ctx, key); err != nil {
			return fmt.Errorf("remove stale entity %s: %w", key, err)
		}
	}

	return nil
}

func (h *SwitchHandle) On(ctx context.Context) error {
	return h.Set(ctx, true)
}

func (h *SwitchHandle) Off(ctx context.Context) error {
	return h.Set(ctx, false)
}

func (h *SwitchHandle) Set(ctx context.Context, on bool) error {
	payload := []byte("OFF")
	if on {
		payload = []byte("ON")
	}
	return h.runtime.setState(ctx, h.key, payload)
}

func (h *SwitchHandle) OnCommand(fn func(context.Context, bool) error) error {
	if fn == nil {
		return fmt.Errorf("command handler is required")
	}

	h.runtime.mu.Lock()
	defer h.runtime.mu.Unlock()
	entity, ok := h.runtime.entities[h.key]
	if !ok {
		return fmt.Errorf("switch %s is not declared", h.key)
	}
	if entity.kind != runtimeKindSwitch {
		return fmt.Errorf("entity %s is not a switch", h.key)
	}

	h.runtime.switchHandlers[h.key] = fn
	return nil
}

func (h *SwitchHandle) Remove(ctx context.Context) error {
	return h.runtime.Remove(ctx, h.key)
}

func (h *NumberSensorHandle) Set(ctx context.Context, value float64) error {
	payload := strconv.FormatFloat(value, 'f', -1, 64)
	return h.runtime.setState(ctx, h.key, []byte(payload))
}

func (h *NumberSensorHandle) Remove(ctx context.Context) error {
	return h.runtime.Remove(ctx, h.key)
}

func (h *BinarySensorHandle) On(ctx context.Context) error {
	return h.Set(ctx, true)
}

func (h *BinarySensorHandle) Off(ctx context.Context) error {
	return h.Set(ctx, false)
}

func (h *BinarySensorHandle) Set(ctx context.Context, on bool) error {
	payload := []byte("OFF")
	if on {
		payload = []byte("ON")
	}
	return h.runtime.setState(ctx, h.key, payload)
}

func (h *BinarySensorHandle) Remove(ctx context.Context) error {
	return h.runtime.Remove(ctx, h.key)
}

func (r *Runtime) declare(ctx context.Context, kind runtimeEntityKind, spec CommonSpec, payload map[string]any) (*runtimeEntity, error) {
	key, err := validateRuntimeKey(spec.Key)
	if err != nil {
		return nil, err
	}

	entity := r.buildEntity(key, kind)
	discoveryPayload, err := json.Marshal(mergeDiscoveryPayload(payload, r.baseDiscoveryPayload(spec, entity)))
	if err != nil {
		return nil, fmt.Errorf("marshal discovery payload for %s: %w", key, err)
	}
	entity.discoveryPayload = discoveryPayload

	r.mu.RLock()
	if existing, ok := r.entities[key]; ok {
		if existing.kind != kind {
			r.mu.RUnlock()
			return nil, fmt.Errorf("entity %s already exists as %s", key, existing.kind)
		}
		entity.lastState = existing.lastState
		entity.hasState = existing.hasState
	}
	r.mu.RUnlock()

	if err := r.publish(ctx, entity.discoveryTopic, true, entity.discoveryPayload); err != nil {
		return nil, fmt.Errorf("publish discovery for %s: %w", key, err)
	}
	if err := r.registry.Upsert(key, kind); err != nil {
		return nil, fmt.Errorf("store entity %s in registry: %w", key, err)
	}

	r.mu.Lock()
	r.entities[key] = entity
	r.mu.Unlock()
	return entity, nil
}

func (r *Runtime) setState(ctx context.Context, key string, payload []byte) error {
	r.mu.Lock()
	entity, ok := r.entities[key]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("entity %s is not declared", key)
	}
	entity.lastState = append([]byte(nil), payload...)
	entity.hasState = true
	stateTopic := entity.stateTopic
	r.mu.Unlock()

	if err := r.publish(ctx, stateTopic, true, payload); err != nil {
		return fmt.Errorf("publish state for %s: %w", key, err)
	}
	return nil
}

func (r *Runtime) publish(ctx context.Context, topic string, retained bool, payload []byte) error {
	if err := r.mqtt.Publish(ctx, topic, retained, payload); err != nil {
		return err
	}
	return nil
}

func (r *Runtime) handleReconnect(ctx context.Context) error {
	if err := r.publish(ctx, r.availabilityTopic, true, []byte(runtimeAvailabilityOnline)); err != nil {
		return fmt.Errorf("publish availability: %w", err)
	}

	r.mu.RLock()
	entities := make([]*runtimeEntity, 0, len(r.entities))
	for _, entity := range r.entities {
		entities = append(entities, entity)
	}
	r.mu.RUnlock()

	for _, entity := range entities {
		if err := r.publish(ctx, entity.discoveryTopic, true, entity.discoveryPayload); err != nil {
			return fmt.Errorf("republish discovery for %s: %w", entity.key, err)
		}
		if entity.hasState {
			if err := r.publish(ctx, entity.stateTopic, true, entity.lastState); err != nil {
				return fmt.Errorf("republish state for %s: %w", entity.key, err)
			}
		}
	}

	return nil
}

func (r *Runtime) handleHomeAssistantStatus(ctx context.Context, _ string, payload []byte) {
	if string(payload) != runtimeAvailabilityOnline {
		return
	}

	if err := r.handleReconnect(ctx); err != nil {
		log.Printf("WARNING: failed to republish runtime entities after HA birth: %v", err)
	}
}

func (r *Runtime) handleCommand(ctx context.Context, topic string, payload []byte) {
	key, err := parseRuntimeCommandTopic(r.appPrefix, topic)
	if err != nil {
		log.Printf("WARNING: ignoring invalid command topic %s: %v", topic, err)
		return
	}

	desiredState, err := parseRuntimeBoolPayload(payload)
	if err != nil {
		log.Printf("WARNING: ignoring invalid command payload for %s: %v", key, err)
		return
	}

	r.mu.RLock()
	handler := r.switchHandlers[key]
	r.mu.RUnlock()
	if handler == nil {
		return
	}

	go func() {
		if err := handler(ctx, desiredState); err != nil {
			log.Printf("WARNING: switch handler failed for %s: %v", key, err)
		}
	}()
}

func (r *Runtime) buildEntity(key string, kind runtimeEntityKind) *runtimeEntity {
	return &runtimeEntity{
		key:            key,
		kind:           kind,
		discoveryTopic: fmt.Sprintf("%s/%s/%s/config", r.discoveryPrefix, kind, key),
		stateTopic:     fmt.Sprintf("%s/entities/%s/state", r.appPrefix, key),
		commandTopic:   fmt.Sprintf("%s/entities/%s/set", r.appPrefix, key),
	}
}

func (r *Runtime) baseDiscoveryPayload(spec CommonSpec, entity *runtimeEntity) map[string]any {
	payload := map[string]any{
		"name":                  spec.Name,
		"unique_id":             fmt.Sprintf("%s_%s", r.appPrefix, entity.key),
		"availability_topic":    r.availabilityTopic,
		"payload_available":     runtimeAvailabilityOnline,
		"payload_not_available": runtimeAvailabilityOffline,
		"state_topic":           entity.stateTopic,
	}
	if entity.kind == runtimeKindSwitch {
		payload["command_topic"] = entity.commandTopic
	}

	if spec.EntityIDHint != "" {
		payload["default_entity_id"] = spec.EntityIDHint
	}
	if spec.Icon != "" {
		payload["icon"] = spec.Icon
	}
	if spec.DeviceClass != "" {
		payload["device_class"] = spec.DeviceClass
	}
	return payload
}

func (c RuntimeConfig) withDefaults() RuntimeConfig {
	if c.DiscoveryPrefix == "" {
		c.DiscoveryPrefix = defaultDiscoveryPrefix
	}
	if c.AppPrefix == "" {
		c.AppPrefix = defaultAppPrefix
	}
	return c
}

func switchDiscoveryPayload(spec SwitchSpec) map[string]any {
	return map[string]any{
		"payload_on":  "ON",
		"payload_off": "OFF",
		"state_on":    "ON",
		"state_off":   "OFF",
	}
}

func numberSensorDiscoveryPayload(spec NumberSensorSpec) map[string]any {
	payload := map[string]any{}
	if spec.UnitOfMeasurement != "" {
		payload["unit_of_measurement"] = spec.UnitOfMeasurement
	}
	return payload
}

func binarySensorDiscoveryPayload(spec BinarySensorSpec) map[string]any {
	return map[string]any{
		"payload_on":  "ON",
		"payload_off": "OFF",
	}
}

func mergeDiscoveryPayload(parts ...map[string]any) map[string]any {
	merged := make(map[string]any)
	for _, part := range parts {
		for key, value := range part {
			merged[key] = value
		}
	}
	return merged
}

func validateRuntimeKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}
	if strings.Contains(key, "/") {
		return "", fmt.Errorf("key %q must not contain /", key)
	}
	return key, nil
}

func parseRuntimeCommandTopic(appPrefix, topic string) (string, error) {
	prefix := fmt.Sprintf("%s/entities/", appPrefix)
	suffix := "/set"
	if !strings.HasPrefix(topic, prefix) || !strings.HasSuffix(topic, suffix) {
		return "", fmt.Errorf("topic does not match runtime command pattern")
	}

	key := strings.TrimSuffix(strings.TrimPrefix(topic, prefix), suffix)
	if key == "" || strings.Contains(key, "/") {
		return "", fmt.Errorf("invalid runtime key in topic")
	}
	return key, nil
}

func parseRuntimeBoolPayload(payload []byte) (bool, error) {
	switch strings.TrimSpace(strings.ToUpper(string(payload))) {
	case "ON":
		return true, nil
	case "OFF":
		return false, nil
	default:
		return false, fmt.Errorf("expected ON or OFF, got %q", string(payload))
	}
}
