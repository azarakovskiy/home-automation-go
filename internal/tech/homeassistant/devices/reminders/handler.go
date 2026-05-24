package reminders

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	domainreminders "home-go/internal/domain/reminders"
	"home-go/internal/tech/homeassistant/component"

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
}

// Handler subscribes to MQTT command topics and drives the reminder tick.
type Handler struct {
	component.Base
	manager   reminderManager
	transport mqttTransport
	prefix    string
}

// New constructs a Handler.
func New(base component.Base, transport mqttTransport, manager reminderManager, prefix string) *Handler {
	return &Handler{
		Base:      base,
		manager:   manager,
		transport: transport,
		prefix:    prefix,
	}
}

// Start subscribes to the three MQTT command topics. Call once during app startup.
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
		ID:      cmd.ID,
		Targets: cmd.Targets,
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

	if _, err := h.manager.Create(context.Background(), dc); err != nil {
		log.Printf("ERROR: reminders: create %s: %v", cmd.ID, err)
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
	}
}

func (h *Handler) handleDelete(_ context.Context, _ string, payload []byte) {
	var cmd DeleteCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		log.Printf("ERROR: reminders: parse delete payload: %v", err)
		return
	}
	if err := h.manager.Delete(context.Background(), cmd.ID); err != nil {
		log.Printf("ERROR: reminders: delete %s: %v", cmd.ID, err)
	}
}

func (h *Handler) tick(_ *ga.Service, _ ga.State) {
	if err := h.manager.Tick(context.Background(), time.Now().UTC()); err != nil {
		log.Printf("ERROR: reminders: tick: %v", err)
	}
}
