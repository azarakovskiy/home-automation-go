package reminders

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	domainreminders "home-go/internal/domain/reminders"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/entities"

	ga "saml.dev/gome-assistant"
)

const tickInterval = "1m"

type mqttTransport interface {
	Subscribe(ctx context.Context, topic string, handler func(context.Context, string, []byte)) error
}

type reminderManager interface {
	Create(ctx context.Context, cmd domainreminders.CreateCommand) (domainreminders.Reminder, error)
	Ack(ctx context.Context, reminderID domainreminders.ReminderID, targetUserID string) error
	Delete(ctx context.Context, reminderID domainreminders.ReminderID) error
	Tick(ctx context.Context, now time.Time) error
	List(ctx context.Context) ([]domainreminders.Reminder, error)
}

// SwitchHandle is implemented by *entities.SwitchHandle and test fakes.
type SwitchHandle interface {
	On(ctx context.Context) error
	Off(ctx context.Context) error
	OnCommand(fn func(context.Context, bool) error) error
}

type entityRuntime interface {
	Switch(ctx context.Context, spec entities.SwitchSpec) (SwitchHandle, error)
	Remove(ctx context.Context, key string) error
}

type switchEntry struct {
	handle SwitchHandle
	isOff  bool // true = pending ack (OFF)
}

// Handler subscribes to MQTT command topics, drives the reminder tick, and
// maintains one HA switch entity per active reminder.
type Handler struct {
	component.Base
	manager   reminderManager
	transport mqttTransport
	entityRT  entityRuntime
	prefix    string

	mu       sync.RWMutex
	switches map[domainreminders.ReminderID]switchEntry
}

// New constructs a Handler. entityRT should be runtimeEntities.ForDevice("Reminders").
func New(base component.Base, transport mqttTransport, manager reminderManager, entityRT entityRuntime, prefix string) *Handler {
	return &Handler{
		Base:      base,
		manager:   manager,
		transport: transport,
		entityRT:  entityRT,
		prefix:    prefix,
		switches:  make(map[domainreminders.ReminderID]switchEntry),
	}
}

// Start subscribes to the three MQTT command topics and syncs entities from DB.
func (h *Handler) Start(ctx context.Context) error {
	topics := []struct {
		suffix  string
		handler func(context.Context, string, []byte)
	}{
		{"create", h.handleCreate},
		{"ack", h.handleAck},
		{"delete", h.handleDelete},
	}

	for _, t := range topics {
		topic := fmt.Sprintf("%s/reminders/%s", h.prefix, t.suffix)
		if err := h.transport.Subscribe(ctx, topic, t.handler); err != nil {
			return fmt.Errorf("subscribe %s: %w", topic, err)
		}
	}

	rems, err := h.manager.List(ctx)
	if err != nil {
		log.Printf("ERROR: reminders: list for startup sync: %v", err)
		return nil
	}
	for _, rem := range rems {
		if err := h.declareEntity(ctx, rem); err != nil {
			log.Printf("ERROR: reminders: declare entity %s at startup: %v", rem.ID, err)
		}
	}
	return nil
}

// Intervals registers the 1-minute tick that drives due-reminder checking.
func (h *Handler) Intervals() []ga.Interval {
	tick := ga.NewInterval().
		Call(h.tick).
		Every(tickInterval).
		Build()
	return []ga.Interval{tick}
}

func (h *Handler) handleCreate(_ context.Context, _ string, payload []byte) {
	var cmd CreateCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		log.Printf("ERROR: reminders: parse create payload: %v", err)
		return
	}

	triggerAt, err := time.Parse(time.RFC3339, cmd.TriggerAt)
	if err != nil {
		log.Printf("ERROR: reminders: invalid trigger_at %q: %v", cmd.TriggerAt, err)
		return
	}

	schedule := domainreminders.Schedule{
		Kind:      domainreminders.ScheduleKindOnce,
		TriggerAt: triggerAt,
	}
	if cmd.RecurEvery != "" {
		d, err := time.ParseDuration(cmd.RecurEvery)
		if err != nil {
			log.Printf("ERROR: reminders: invalid recur_every %q: %v", cmd.RecurEvery, err)
			return
		}
		schedule.Kind = domainreminders.ScheduleKindRecurring
		schedule.RecurEvery = &d
	}
	if cmd.ValidUntil != "" {
		t, err := time.Parse(time.RFC3339, cmd.ValidUntil)
		if err != nil {
			log.Printf("ERROR: reminders: invalid valid_until %q: %v", cmd.ValidUntil, err)
			return
		}
		schedule.ValidUntil = &t
	}

	dc := domainreminders.CreateCommand{
		ID:       cmd.ID,
		Targets:  cmd.Targets,
		Schedule: schedule,
		Policy: domainreminders.DeliveryPolicy{
			RequiresAck: cmd.RequiresAck,
			Profile:     domainreminders.Profile(cmd.Profile),
		},
		Meta: domainreminders.Metadata{
			Source:  cmd.Source,
			Owner:   cmd.Owner,
			Message: cmd.Message,
		},
	}

	rem, err := h.manager.Create(context.Background(), dc)
	if err != nil {
		log.Printf("ERROR: reminders: create %s: %v", cmd.ID, err)
		return
	}
	if err := h.declareEntity(context.Background(), rem); err != nil {
		log.Printf("ERROR: reminders: declare entity after create %s: %v", rem.ID, err)
	}
}

func (h *Handler) handleAck(_ context.Context, _ string, payload []byte) {
	var cmd AckCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		log.Printf("ERROR: reminders: parse ack payload: %v", err)
		return
	}
	if err := h.manager.Ack(context.Background(), cmd.ID, cmd.UserID); err != nil {
		log.Printf("ERROR: reminders: ack %s for %s: %v", cmd.ID, cmd.UserID, err)
		return
	}
	h.syncEntities(context.Background())
}

func (h *Handler) handleDelete(_ context.Context, _ string, payload []byte) {
	var cmd DeleteCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		log.Printf("ERROR: reminders: parse delete payload: %v", err)
		return
	}
	if err := h.manager.Delete(context.Background(), cmd.ID); err != nil {
		log.Printf("ERROR: reminders: delete %s: %v", cmd.ID, err)
		return
	}
	h.syncEntities(context.Background())
}

func (h *Handler) tick(_ *ga.Service, _ ga.State) {
	if err := h.manager.Tick(context.Background(), time.Now().UTC()); err != nil {
		log.Printf("ERROR: reminders: tick: %v", err)
	}
	h.syncEntities(context.Background())
}

// syncEntities reconciles the in-memory switch map against manager.List().
func (h *Handler) syncEntities(ctx context.Context) {
	rems, err := h.manager.List(ctx)
	if err != nil {
		log.Printf("ERROR: reminders: list for sync: %v", err)
		return
	}

	inDB := make(map[domainreminders.ReminderID]domainreminders.Reminder, len(rems))
	for _, rem := range rems {
		inDB[rem.ID] = rem
	}

	h.mu.RLock()
	inMap := make(map[domainreminders.ReminderID]struct{}, len(h.switches))
	for id := range h.switches {
		inMap[id] = struct{}{}
	}
	h.mu.RUnlock()

	// Remove entities no longer in DB.
	for id := range inMap {
		if _, ok := inDB[id]; ok {
			continue
		}
		if err := h.entityRT.Remove(ctx, entityKey(id)); err != nil {
			log.Printf("ERROR: reminders: remove entity %s: %v", id, err)
		}
		h.mu.Lock()
		delete(h.switches, id)
		h.mu.Unlock()
	}

	// Declare new or update existing.
	for _, rem := range rems {
		h.mu.RLock()
		entry, exists := h.switches[rem.ID]
		h.mu.RUnlock()

		wantOff := entityIsOff(rem)
		if !exists {
			if err := h.declareEntity(ctx, rem); err != nil {
				log.Printf("ERROR: reminders: declare entity %s during sync: %v", rem.ID, err)
			}
			continue
		}
		if entry.isOff == wantOff {
			continue
		}
		if wantOff {
			if err := entry.handle.Off(ctx); err != nil {
				log.Printf("ERROR: reminders: set entity OFF %s: %v", rem.ID, err)
			}
		} else {
			if err := entry.handle.On(ctx); err != nil {
				log.Printf("ERROR: reminders: set entity ON %s: %v", rem.ID, err)
			}
		}
		h.mu.Lock()
		h.switches[rem.ID] = switchEntry{handle: entry.handle, isOff: wantOff}
		h.mu.Unlock()
	}
}

// declareEntity declares one HA switch for rem and registers its command callback.
func (h *Handler) declareEntity(ctx context.Context, rem domainreminders.Reminder) error {
	name := rem.Meta.Message
	if len(name) > 64 {
		name = name[:64]
	}
	handle, err := h.entityRT.Switch(ctx, entities.SwitchSpec{
		CommonSpec: entities.CommonSpec{
			Key:  entityKey(rem.ID),
			Name: name,
			Icon: "mdi:bell",
		},
	})
	if err != nil {
		return fmt.Errorf("declare switch: %w", err)
	}

	isOff := entityIsOff(rem)
	if isOff {
		if err := handle.Off(ctx); err != nil {
			log.Printf("ERROR: reminders: set initial OFF for %s: %v", rem.ID, err)
		}
	} else {
		if err := handle.On(ctx); err != nil {
			log.Printf("ERROR: reminders: set initial ON for %s: %v", rem.ID, err)
		}
	}

	remID := rem.ID
	if err := handle.OnCommand(func(ctx context.Context, on bool) error {
		if !on {
			return nil
		}
		if err := h.manager.Ack(ctx, remID, rem.Meta.Owner); err != nil {
			return fmt.Errorf("ack from switch: %w", err)
		}
		h.syncEntities(ctx)
		return nil
	}); err != nil {
		return fmt.Errorf("register command callback: %w", err)
	}

	h.mu.Lock()
	h.switches[rem.ID] = switchEntry{handle: handle, isOff: isOff}
	h.mu.Unlock()
	return nil
}

// entityIsOff returns true when the reminder is awaiting acknowledgment (fired but not acked).
func entityIsOff(rem domainreminders.Reminder) bool {
	return rem.Policy.RequiresAck && rem.State.FireCount > 0
}

// entityKey builds the stable HA entity key for a reminder ID.
func entityKey(id domainreminders.ReminderID) string {
	slug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(string(id)), " ", "_"))
	return "reminder_" + slug
}
