package reminder

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"home-go/component"
	"home-go/dryrun"
	"home-go/entities"
	"home-go/events"
	"home-go/notifications"
	"home-go/storage/jsonstore"

	ga "saml.dev/gome-assistant"
)

var (
	configChunkIDs = []string{
		entities.InputText.HomeGoRemindersConfigChunk1,
		entities.InputText.HomeGoRemindersConfigChunk2,
		entities.InputText.HomeGoRemindersConfigChunk3,
		entities.InputText.HomeGoRemindersConfigChunk4,
		entities.InputText.HomeGoRemindersConfigChunk5,
		entities.InputText.HomeGoRemindersConfigChunk6,
		entities.InputText.HomeGoRemindersConfigChunk7,
		entities.InputText.HomeGoRemindersConfigChunk8,
	}
	runtimeChunkIDs = []string{
		entities.InputText.HomeGoRemindersRuntimeChunk1,
		entities.InputText.HomeGoRemindersRuntimeChunk2,
		entities.InputText.HomeGoRemindersRuntimeChunk3,
		entities.InputText.HomeGoRemindersRuntimeChunk4,
		entities.InputText.HomeGoRemindersRuntimeChunk5,
		entities.InputText.HomeGoRemindersRuntimeChunk6,
		entities.InputText.HomeGoRemindersRuntimeChunk7,
		entities.InputText.HomeGoRemindersRuntimeChunk8,
	}
	viewChunkIDs = []string{
		entities.InputText.HomeGoRemindersViewsChunk1,
		entities.InputText.HomeGoRemindersViewsChunk2,
		entities.InputText.HomeGoRemindersViewsChunk3,
		entities.InputText.HomeGoRemindersViewsChunk4,
		entities.InputText.HomeGoRemindersViewsChunk5,
		entities.InputText.HomeGoRemindersViewsChunk6,
		entities.InputText.HomeGoRemindersViewsChunk7,
		entities.InputText.HomeGoRemindersViewsChunk8,
	}
	uiDisplayEntities = map[string]string{
		"alexey": entities.InputText.HomeGoRemindersUiAlexey,
		"pok":    entities.InputText.HomeGoRemindersUiPok,
	}
)

// Component orchestrates reminder creation, scheduling, and notifications.
type Component struct {
	component.Base

	definitionStore *jsonstore.Store[definitionStoreData]
	runtimeStore    *jsonstore.Store[runtimeStoreData]
	viewStore       *jsonstore.Store[viewStoreData]

	definitions map[string]ReminderDefinition
	runtimes    map[string]ReminderRuntime

	notificationService *notifications.NotificationService
	mu                  sync.Mutex
}

// DefaultQuietHours is used unless overrides are provided.
var DefaultQuietHours = QuietHoursConfig{
	Enabled: true,
	Start:   "22:00",
	End:     "07:00",
}

// New creates a reminder component.
func New(base component.Base, state ga.State) *Component {
	base.State = state

	c := &Component{
		Base:                base,
		definitions:         make(map[string]ReminderDefinition),
		runtimes:            make(map[string]ReminderRuntime),
		definitionStore:     jsonstore.New[definitionStoreData](base.Service, state, configChunkIDs...),
		runtimeStore:        jsonstore.New[runtimeStoreData](base.Service, state, runtimeChunkIDs...),
		viewStore:           jsonstore.New[viewStoreData](base.Service, state, viewChunkIDs...),
		notificationService: notifications.NewNotificationService(base.Service),
	}

	c.restoreState()
	return c
}

func (c *Component) EventListeners() []ga.EventListener {
	createHandler := component.NewTypedEventHandler(entities.CustomEvents.ReminderCreate, c.handleCreateEvent)
	ackHandler := component.NewTypedEventHandler(entities.CustomEvents.ReminderAck, c.handleAckEvent)
	deleteHandler := component.NewTypedEventHandler(entities.CustomEvents.ReminderDelete, c.handleDeleteEvent)

	return []ga.EventListener{
		createHandler.Build(),
		ackHandler.Build(),
		deleteHandler.Build(),
	}
}

// Intervals runs the scheduler loop once per minute.
func (c *Component) Intervals() []ga.Interval {
	return []ga.Interval{
		ga.NewInterval().
			Call(c.runScheduler).
			Every("1m").
			Build(),
	}
}

func (c *Component) handleCreateEvent(service *ga.Service, state ga.State, payload events.ReminderCreateEvent) {
	now := time.Now()
	definition, err := c.buildDefinition(payload, now)
	if err != nil {
		log.Printf("WARNING: invalid reminder create payload: %v", err)
		return
	}

	runtime := c.bootstrapRuntime(definition, now)

	c.mu.Lock()
	c.definitions[definition.ID] = *definition
	c.runtimes[definition.ID] = runtime
	defSnapshot := cloneDefinitions(c.definitions)
	runtimeSnapshot := cloneRuntimes(c.runtimes)
	c.mu.Unlock()

	if err := c.definitionStore.Save(mapToDefinitionStoreData(defSnapshot)); err != nil {
		log.Printf("WARNING: failed to persist reminder definitions: %v", err)
	}
	if err := c.runtimeStore.Save(mapToRuntimeStoreData(runtimeSnapshot)); err != nil {
		log.Printf("WARNING: failed to persist reminder runtime: %v", err)
	}
	c.refreshViewsFromSnapshot(defSnapshot, runtimeSnapshot)
}

func (c *Component) handleAckEvent(service *ga.Service, state ga.State, payload events.ReminderAckEvent) {
	c.mu.Lock()
	runtime, ok := c.runtimes[payload.ID]
	if !ok {
		c.mu.Unlock()
		log.Printf("WARNING: reminder %s acked but not found", payload.ID)
		return
	}
	def, defExists := c.definitions[payload.ID]
	now := time.Now()
	if runtime.Completed && defExists && def.Mode != ModeSingle {
		c.mu.Unlock()
		return
	}

	runtime.Completed = true
	runtime.AcknowledgedBy = payload.User
	runtime.AcknowledgedAt = now
	runtime.NextTrigger = time.Time{}
	runtime.AwaitingAck = false

	isSingle := defExists && def.Mode == ModeSingle
	if isSingle {
		delete(c.definitions, payload.ID)
		delete(c.runtimes, payload.ID)
	} else {
		c.runtimes[payload.ID] = runtime
	}

	defSnapshot := cloneDefinitions(c.definitions)
	runtimeSnapshot := cloneRuntimes(c.runtimes)
	c.mu.Unlock()

	if isSingle {
		if err := c.definitionStore.Save(mapToDefinitionStoreData(defSnapshot)); err != nil {
			log.Printf("WARNING: failed to persist reminder definitions: %v", err)
		}
	}

	if err := c.runtimeStore.Save(mapToRuntimeStoreData(runtimeSnapshot)); err != nil {
		log.Printf("WARNING: failed to persist reminder runtime: %v", err)
	}
	c.refreshViewsFromSnapshot(defSnapshot, runtimeSnapshot)
}

func (c *Component) handleDeleteEvent(service *ga.Service, state ga.State, payload events.ReminderDeleteEvent) {
	c.mu.Lock()
	delete(c.definitions, payload.ID)
	delete(c.runtimes, payload.ID)
	defSnapshot := cloneDefinitions(c.definitions)
	runtimeSnapshot := cloneRuntimes(c.runtimes)
	c.mu.Unlock()

	if err := c.definitionStore.Save(mapToDefinitionStoreData(defSnapshot)); err != nil {
		log.Printf("WARNING: failed to persist reminder definitions: %v", err)
	}
	if err := c.runtimeStore.Save(mapToRuntimeStoreData(runtimeSnapshot)); err != nil {
		log.Printf("WARNING: failed to persist reminder runtime: %v", err)
	}
	c.refreshViewsFromSnapshot(defSnapshot, runtimeSnapshot)
}

func (c *Component) runScheduler(service *ga.Service, state ga.State) {
	now := time.Now()
	changed := false

	c.mu.Lock()
	for id, def := range c.definitions {
		runtime, ok := c.runtimes[id]
		if !ok {
			localDef := def
			runtime = c.bootstrapRuntime(&localDef, now)
		}

		updated, modified := c.evaluateReminder(def, runtime, now)
		if modified {
			c.runtimes[id] = updated
			changed = true
		}
	}

	var defSnapshot map[string]ReminderDefinition
	var runtimeSnapshot map[string]ReminderRuntime
	if changed {
		defSnapshot = cloneDefinitions(c.definitions)
		runtimeSnapshot = cloneRuntimes(c.runtimes)
	}
	c.mu.Unlock()

	if changed {
		if err := c.runtimeStore.Save(mapToRuntimeStoreData(runtimeSnapshot)); err != nil {
			log.Printf("WARNING: failed to persist reminder runtime: %v", err)
		}
		c.refreshViewsFromSnapshot(defSnapshot, runtimeSnapshot)
	}
}

func (c *Component) evaluateReminder(def ReminderDefinition, runtime ReminderRuntime, now time.Time) (ReminderRuntime, bool) {
	if runtime.Completed || runtime.Cancelled {
		return runtime, false
	}

	if def.Mode == ModeSingle && runtime.AwaitingAck {
		return runtime, false
	}

	if runtime.NextTrigger.IsZero() {
		runtime.NextTrigger = def.StartTime
		if runtime.NextTrigger.IsZero() || runtime.NextTrigger.Before(now) {
			runtime.NextTrigger = now
		}
		return runtime, true
	}

	if runtime.NextTrigger.After(now) {
		return runtime, false
	}

	if def.PresenceRequired {
		away, err := c.IsAway()
		if err != nil {
			log.Printf("WARNING: failed to check presence for reminder %s: %v", def.ID, err)
		} else if away {
			runtime.NextTrigger = now.Add(15 * time.Minute)
			return runtime, true
		}
	}

	if !def.NightModeAllowed {
		isNight, err := c.IsNightMode()
		if err != nil {
			log.Printf("WARNING: failed to check night mode for reminder %s: %v", def.ID, err)
		} else if isNight {
			runtime.NextTrigger = now.Add(10 * time.Minute)
			return runtime, true
		}
	}

	if def.QuietHours.Enabled && def.QuietHours.isQuiet(now) {
		runtime.NextTrigger = def.QuietHours.nextWindowEnd(now)
		return runtime, true
	}

	return c.triggerReminder(def, runtime, now), true
}

func (c *Component) triggerReminder(def ReminderDefinition, runtime ReminderRuntime, now time.Time) ReminderRuntime {
	notifyData := map[string]any{
		"reminder_id":  def.ID,
		"title":        def.Title,
		"repeat_count": runtime.RepeatCount,
		"mode":         def.Mode,
	}
	if def.SpeakerEntity != "" {
		notifyData["speaker_entity"] = def.SpeakerEntity
	}
	if def.PhoneNotifier != "" {
		notifyData["phone_notifier"] = def.PhoneNotifier
	}

	event := notifications.NotificationEvent{
		Device:  "reminder",
		Type:    "due",
		Message: def.Message,
		Data:    notifyData,
	}
	if err := c.notificationService.Notify(event); err != nil {
		log.Printf("ERROR: failed to send reminder %s notification: %v", def.ID, err)
	}

	runtime.RepeatCount++
	runtime.LastTriggered = now
	if runtime.LastIntervalMin == 0 {
		runtime.LastIntervalMin = def.InitialRepeatMin
	}

	if def.Mode == ModeSingle {
		runtime.AwaitingAck = true
		runtime.NextTrigger = time.Time{}
		return runtime
	}

	interval, cancel := nextIntervalDuration(def.Profile, def, runtime.LastIntervalMin)
	if cancel {
		runtime.Cancelled = true
		runtime.NextTrigger = time.Time{}
		return runtime
	}
	runtime.LastIntervalMin = int(interval.Minutes())
	runtime.NextTrigger = now.Add(interval)
	return runtime
}

func (c *Component) bootstrapRuntime(def *ReminderDefinition, now time.Time) ReminderRuntime {
	next := def.StartTime
	if next.IsZero() || next.Before(now) {
		next = now
	}
	return ReminderRuntime{
		ID:              def.ID,
		NextTrigger:     next,
		RepeatCount:     0,
		LastIntervalMin: def.InitialRepeatMin,
		AwaitingAck:     false,
	}
}

func (c *Component) restoreState() {
	defData, err := c.definitionStore.Load()
	if err != nil {
		log.Printf("WARNING: failed to restore reminder definitions: %v", err)
	}
	runData, rErr := c.runtimeStore.Load()
	if rErr != nil {
		log.Printf("WARNING: failed to restore reminder runtime: %v", rErr)
	}

	c.mu.Lock()
	if mapData := defData.toMap(); len(mapData) > 0 {
		c.definitions = mapData
	} else if c.definitions == nil {
		c.definitions = make(map[string]ReminderDefinition)
	}

	if runtimeMap := runData.toMap(); len(runtimeMap) > 0 {
		c.runtimes = runtimeMap
	} else if c.runtimes == nil {
		c.runtimes = make(map[string]ReminderRuntime)
	}

	now := time.Now()
	for id, def := range c.definitions {
		if _, ok := c.runtimes[id]; !ok {
			localDef := def
			c.runtimes[id] = c.bootstrapRuntime(&localDef, now)
		}
	}

	defSnapshot := cloneDefinitions(c.definitions)
	runtimeSnapshot := cloneRuntimes(c.runtimes)
	c.mu.Unlock()

	c.refreshViewsFromSnapshot(defSnapshot, runtimeSnapshot)
}

func normalizeProfile(profile string) ReminderProfile {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case string(ProfileAnnoying):
		return ProfileAnnoying
	case string(ProfileQuiet):
		return ProfileQuiet
	default:
		return ProfileNormal
	}
}

func normalizeMode(mode string, legacyOneTime *bool) ReminderMode {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case string(ModeSingle), "one_time", "one-time":
		return ModeSingle
	case string(ModeRepeating), "repeat":
		return ModeRepeating
	}
	if legacyOneTime != nil && *legacyOneTime {
		return ModeSingle
	}
	return ModeRepeating
}

func (c *Component) buildDefinition(payload events.ReminderCreateEvent, now time.Time) (*ReminderDefinition, error) {
	id := strings.TrimSpace(payload.ID)
	if id == "" {
		return nil, fmt.Errorf("id cannot be empty")
	}

	message := strings.TrimSpace(payload.Message)
	if message == "" {
		return nil, fmt.Errorf("message cannot be empty")
	}

	profile := normalizeProfile(payload.Profile)
	mode := normalizeMode(payload.Mode, payload.OneTime)

	initial := intOrDefault(payload.InitialRepeatMin.Ptr(), DefaultInitialRepeatMinutes)
	minRepeat := intOrDefault(payload.MinRepeatMin.Ptr(), DefaultMinRepeatMinutes)
	maxRepeat := intOrDefault(payload.MaxRepeatMin.Ptr(), DefaultMaxRepeatMinutes)

	if minRepeat <= 0 {
		minRepeat = DefaultMinRepeatMinutes
	}
	if initial < minRepeat {
		initial = minRepeat
	}
	if maxRepeat < initial {
		maxRepeat = initial
	}

	quiet := DefaultQuietHours
	if payload.QuietHours != nil {
		quiet.Enabled = payload.QuietHours.Enabled
		if strings.TrimSpace(payload.QuietHours.Start) != "" {
			quiet.Start = payload.QuietHours.Start
		}
		if strings.TrimSpace(payload.QuietHours.End) != "" {
			quiet.End = payload.QuietHours.End
		}
	}

	startTime, err := resolveStartTime(payload, now)
	if err != nil {
		return nil, err
	}

	definition := &ReminderDefinition{
		ID:               id,
		Title:            firstNonEmpty(payload.Title, message),
		Message:          message,
		Profile:          profile,
		Mode:             mode,
		StartTime:        startTime,
		InitialRepeatMin: initial,
		MinRepeatMin:     minRepeat,
		MaxRepeatMin:     maxRepeat,
		SpeakerEntity:    strings.TrimSpace(payload.SpeakerEntity),
		PhoneNotifier:    strings.TrimSpace(payload.PhoneNotifier),
		VisibleTo:        uniqueStrings(payload.VisibleTo),
		NightModeAllowed: boolOrDefault(payload.NightModeAllowed, false),
		PresenceRequired: boolOrDefault(payload.PresenceRequired, true),
		QuietHours:       quiet,
		Metadata:         payload.Metadata,
		CreatedAt:        now,
	}

	return definition, nil
}

func intOrDefault(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	if *value <= 0 {
		return fallback
	}
	return *value
}

func boolOrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func resolveStartTime(payload events.ReminderCreateEvent, now time.Time) (time.Time, error) {
	if payload.InitialDelayMinutes != nil {
		return now.Add(time.Duration(*payload.InitialDelayMinutes) * time.Minute), nil
	}

	startStr := strings.TrimSpace(payload.StartTime)
	if startStr == "" {
		return now, nil
	}

	if parsed, err := parseAbsoluteTime(startStr, now.Location()); err == nil {
		return parsed, nil
	}
	if parsed, err := parseClockTime(startStr, now); err == nil {
		return parsed, nil
	}

	return now, fmt.Errorf("invalid start_time: %s", startStr)
}

func parseAbsoluteTime(value string, loc *time.Location) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if loc == nil {
		loc = time.Local
	}
	layouts := []string{"2006-01-02 15:04", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid absolute time: %s", value)
}

func parseClockTime(value string, now time.Time) (time.Time, error) {
	hourMinute := strings.TrimSpace(value)
	parts := strings.Split(hourMinute, ":")
	if len(parts) < 2 {
		return time.Time{}, errors.New("invalid clock format")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, err
	}
	if hour < 0 || hour > 23 {
		return time.Time{}, fmt.Errorf("invalid hour: %d", hour)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, err
	}
	if minute < 0 || minute > 59 {
		return time.Time{}, fmt.Errorf("invalid minute: %d", minute)
	}
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !target.After(now) {
		target = target.Add(24 * time.Hour)
	}
	return target, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func cloneDefinitions(src map[string]ReminderDefinition) map[string]ReminderDefinition {
	dst := make(map[string]ReminderDefinition, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneRuntimes(src map[string]ReminderRuntime) map[string]ReminderRuntime {
	dst := make(map[string]ReminderRuntime, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (c *Component) refreshViews() {
	c.mu.Lock()
	defSnapshot := cloneDefinitions(c.definitions)
	runtimeSnapshot := cloneRuntimes(c.runtimes)
	c.mu.Unlock()

	c.refreshViewsFromSnapshot(defSnapshot, runtimeSnapshot)
}

func (c *Component) refreshViewsFromSnapshot(defs map[string]ReminderDefinition, runtimes map[string]ReminderRuntime) {
	views := make(map[string][]ReminderView)
	for id, def := range defs {
		runtime, ok := runtimes[id]
		if !ok {
			runtime = ReminderRuntime{ID: id}
		}

		view := ReminderView{
			ID:          id,
			Title:       def.Title,
			Message:     def.Message,
			NextTrigger: runtime.NextTrigger,
			Profile:     def.Profile,
			Mode:        def.Mode,
			VisibleTo:   def.VisibleTo,
			RepeatCount: runtime.RepeatCount,
			Completed:   runtime.Completed,
			Cancelled:   runtime.Cancelled,
			Speaker:     def.SpeakerEntity,
			Phone:       def.PhoneNotifier,
			AwaitingAck: runtime.AwaitingAck,
		}

		if len(def.VisibleTo) == 0 {
			views["_all"] = append(views["_all"], view)
		} else {
			for _, user := range def.VisibleTo {
				views[user] = append(views[user], view)
			}
		}
	}

	for key := range views {
		list := views[key]
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].NextTrigger.Before(list[j].NextTrigger)
		})
		views[key] = list
	}

	if err := c.viewStore.Save(mapToViewStoreData(views)); err != nil {
		log.Printf("WARNING: failed to persist reminder views: %v", err)
	}

	c.updateUiEntities(views)
}

func (c *Component) updateUiEntities(views map[string][]ReminderView) {
	for user, entityID := range uiDisplayEntities {
		payload := buildUiPayload(views[user])
		if err := dryrun.CallWithData("InputText.Set", entityID, payload, func() error {
			return c.Service.InputText.Set(entityID, payload)
		}); err != nil {
			log.Printf("WARNING: failed to update reminder UI entity %s: %v", entityID, err)
		}
	}
}

const maxUiLabelLen = 40

func buildUiPayload(entries []ReminderView) string {
	var builder strings.Builder
	for _, view := range entries {
		if view.ID == "" || view.Completed || view.Cancelled {
			continue
		}
		label := strings.TrimSpace(view.Title)
		if label == "" {
			label = strings.TrimSpace(view.Message)
		}
		if label == "" {
			continue
		}
		if len(label) > maxUiLabelLen {
			label = label[:maxUiLabelLen-1] + "…"
		}

		chip := fmt.Sprintf("%s|%s", view.ID, label)
		nextLen := builder.Len()
		if nextLen > 0 {
			nextLen += 1 // semicolon
		}
		nextLen += len(chip)
		if nextLen > 255 {
			break
		}
		if builder.Len() > 0 {
			builder.WriteRune(';')
		}
		builder.WriteString(chip)
	}
	return builder.String()
}
