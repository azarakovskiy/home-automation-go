package reminders

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	domainreminders "home-go/internal/domain/reminders"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/entities"

	ga "saml.dev/gome-assistant"
)

const tickInterval = "1m"

// switchHandle is the subset of entities.SwitchHandle used by the component.
// Defining it as an interface enables testing without a real MQTT broker.
type switchHandle interface {
	On(ctx context.Context) error
	OnCommand(fn func(context.Context, bool) error) error
}

// projector is the subset of entities.Runtime used by the component.
// Using an interface here decouples the component from the concrete runtime type
// and allows injection of a fake in unit tests.
type projector interface {
	Switch(ctx context.Context, spec entities.SwitchSpec) (switchHandle, error)
	Remove(ctx context.Context, key string) error
	Reconcile(ctx context.Context, keep []string) error
}

// runtimeProjector wraps *entities.Runtime to satisfy the projector interface.
// entities.Runtime.Switch returns a concrete *SwitchHandle; this adapter boxes
// it into the switchHandle interface.
type runtimeProjector struct {
	rt *entities.Runtime
}

func (r *runtimeProjector) Switch(ctx context.Context, spec entities.SwitchSpec) (switchHandle, error) {
	return r.rt.Switch(ctx, spec)
}

func (r *runtimeProjector) Remove(ctx context.Context, key string) error {
	return r.rt.Remove(ctx, key)
}

func (r *runtimeProjector) Reconcile(ctx context.Context, keep []string) error {
	return r.rt.Reconcile(ctx, keep)
}

// reminderManager is the subset of domainreminders.Manager used by the component.
// Using an interface here allows injection of a test double.
type reminderManager interface {
	Create(ctx context.Context, id domainreminders.ReminderID, targets []string, schedule domainreminders.Schedule, policy domainreminders.DeliveryPolicy, meta domainreminders.Metadata) (domainreminders.Action, error)
	Ack(ctx context.Context, reminderID domainreminders.ReminderID, userID string) (domainreminders.Action, error)
	Delete(ctx context.Context, reminderID domainreminders.ReminderID) (domainreminders.Action, error)
	Restore(ctx context.Context) ([]domainreminders.Reminder, error)
	Tick(ctx context.Context, now time.Time) ([]domainreminders.Action, error)
}

// Component is the Home Assistant adapter for the reminders feature.
// It bridges Home Assistant custom events and MQTT runtime entities to the
// domain reminders.Manager. It implements component.Component.
type Component struct {
	component.Base
	runtime projector
	manager reminderManager
}

// EventListeners registers typed custom event handlers for create, ack, and delete.
func (c *Component) EventListeners() []ga.EventListener {
	createHandler := component.NewTypedEventHandler(
		entities.CustomEvents.ReminderCreate,
		c.handleCreate,
	)
	ackHandler := component.NewTypedEventHandler(
		entities.CustomEvents.ReminderAck,
		c.handleAck,
	)
	deleteHandler := component.NewTypedEventHandler(
		entities.CustomEvents.ReminderDelete,
		c.handleDelete,
	)
	return []ga.EventListener{
		createHandler.Build(),
		ackHandler.Build(),
		deleteHandler.Build(),
	}
}

// Intervals registers the periodic tick that drives reminder due-checking.
func (c *Component) Intervals() []ga.Interval {
	tick := ga.NewInterval().
		Call(c.tick).
		Every(tickInterval).
		Build()
	return []ga.Interval{tick}
}

// Restore rebuilds MQTT projections for all active reminders and reconciles
// stale entities left over from a previous run. It must be called once during
// app startup after the component is constructed.
func (c *Component) Restore(ctx context.Context) error {
	rems, err := c.manager.Restore(ctx)
	if err != nil {
		return fmt.Errorf("restore reminders: %w", err)
	}

	var keys []string
	for _, rem := range rems {
		c.showProjection(ctx, rem)
		for _, userID := range rem.Targets {
			keys = append(keys, projectionKey(rem.ID, userID))
		}
	}

	if err := c.runtime.Reconcile(ctx, keys); err != nil {
		// Reconcile requires a persistent registry; log and continue if not available.
		log.Printf("WARNING: reminder projection reconcile failed: %v", err)
	}

	return nil
}

// --- internal: event handlers ---

func (c *Component) handleCreate(_ *ga.Service, _ ga.State, event CreateReminderEvent) {
	ctx := context.Background()

	triggerAt, err := time.Parse(time.RFC3339, event.TriggerAt)
	if err != nil {
		log.Printf("ERROR: reminders: invalid trigger_at %q: %v", event.TriggerAt, err)
		return
	}

	schedule := domainreminders.Schedule{
		Kind:      domainreminders.ScheduleKindOnce,
		TriggerAt: triggerAt,
	}
	if event.RecurEvery != "" {
		d, err := time.ParseDuration(event.RecurEvery)
		if err != nil {
			log.Printf("ERROR: reminders: invalid recur_every %q: %v", event.RecurEvery, err)
			return
		}
		schedule.Kind = domainreminders.ScheduleKindRecurring
		schedule.RecurEvery = &d
	}
	if event.ValidUntil != "" {
		t, err := time.Parse(time.RFC3339, event.ValidUntil)
		if err != nil {
			log.Printf("ERROR: reminders: invalid valid_until %q: %v", event.ValidUntil, err)
			return
		}
		schedule.ValidUntil = &t
	}

	policy := domainreminders.DeliveryPolicy{
		RequiresAck:      event.RequiresAck,
		CompletionPolicy: domainreminders.CompletionPolicy(event.CompletionPolicy),
		Profile:          domainreminders.Profile(event.Profile),
	}

	meta := domainreminders.Metadata{
		Source:  event.Source,
		Owner:   event.Owner,
		Message: event.Message,
	}

	action, err := c.manager.Create(ctx, event.ID, event.Targets, schedule, policy, meta)
	if err != nil {
		log.Printf("ERROR: reminders: create %s: %v", event.ID, err)
		return
	}

	c.applyAction(ctx, action)
}

func (c *Component) handleAck(_ *ga.Service, _ ga.State, event AckReminderEvent) {
	ctx := context.Background()

	action, err := c.manager.Ack(ctx, event.ID, event.UserID)
	if err != nil {
		log.Printf("ERROR: reminders: ack %s for %s: %v", event.ID, event.UserID, err)
		return
	}

	c.applyAction(ctx, action)
}

func (c *Component) handleDelete(_ *ga.Service, _ ga.State, event DeleteReminderEvent) {
	ctx := context.Background()

	action, err := c.manager.Delete(ctx, event.ID)
	if err != nil {
		log.Printf("ERROR: reminders: delete %s: %v", event.ID, err)
		return
	}

	c.applyAction(ctx, action)
}

func (c *Component) tick(_ *ga.Service, _ ga.State) {
	ctx := context.Background()

	actions, err := c.manager.Tick(ctx, time.Now().UTC())
	if err != nil {
		log.Printf("ERROR: reminders: tick: %v", err)
		return
	}

	for _, action := range actions {
		c.applyAction(ctx, action)
	}
}

// --- internal: projection helpers ---

// applyAction dispatches a manager Action to the appropriate projection operation.
func (c *Component) applyAction(ctx context.Context, action domainreminders.Action) {
	switch action.Kind {
	case domainreminders.ActionShowProjection:
		c.showProjection(ctx, action.Reminder)
	case domainreminders.ActionRemoveProjection:
		c.removeProjection(ctx, action.Reminder)
	case domainreminders.ActionNoop:
		// nothing to do
	}
}

// showProjection creates or refreshes a per-user Switch entity for the reminder.
// The switch is ON while the reminder is pending. When the user sends an OFF
// command, it is treated as a per-user acknowledgement.
func (c *Component) showProjection(ctx context.Context, rem domainreminders.Reminder) {
	for _, userID := range rem.Targets {
		key := projectionKey(rem.ID, userID)

		sw, err := c.runtime.Switch(ctx, entities.SwitchSpec{
			CommonSpec: entities.CommonSpec{
				Key:  key,
				Name: fmt.Sprintf("Reminder %s: %s", rem.ID, userID),
				Icon: "mdi:bell",
			},
		})
		if err != nil {
			log.Printf("ERROR: reminders: declare switch %s: %v", key, err)
			continue
		}

		if err := sw.On(ctx); err != nil {
			log.Printf("ERROR: reminders: set switch ON %s: %v", key, err)
		}

		// Capture loop variables for the command closure.
		reminderID := rem.ID
		uid := userID

		if err := sw.OnCommand(func(ctx context.Context, on bool) error {
			if on {
				// Ignore: reminder is already active; re-set to ON to prevent confusion.
				return sw.On(ctx)
			}
			// OFF = user acknowledgement.
			action, err := c.manager.Ack(ctx, reminderID, uid)
			if err != nil {
				return fmt.Errorf("ack reminder %s for %s: %w", reminderID, uid, err)
			}
			c.applyAction(ctx, action)
			return nil
		}); err != nil {
			log.Printf("ERROR: reminders: register ack handler %s: %v", key, err)
		}
	}
}

// removeProjection removes the per-user Switch entities for a reminder.
func (c *Component) removeProjection(ctx context.Context, rem domainreminders.Reminder) {
	for _, userID := range rem.Targets {
		key := projectionKey(rem.ID, userID)
		if err := c.runtime.Remove(ctx, key); err != nil {
			log.Printf("ERROR: reminders: remove switch %s: %v", key, err)
		}
	}
}

// projectionKey returns a stable MQTT entity key for a reminder+user pair.
// Keys must not contain "/" per the runtime's key validation rules.
func projectionKey(reminderID, userID string) string {
	id := sanitizeKeyPart(reminderID)
	uid := sanitizeKeyPart(userID)
	return fmt.Sprintf("reminder_%s_%s", id, uid)
}

// sanitizeKeyPart replaces characters that are invalid in runtime entity keys.
func sanitizeKeyPart(s string) string {
	s = strings.TrimSpace(s)
	return strings.ReplaceAll(s, "/", "_")
}
