package reminder

import (
	"time"
)

type definitionStoreData struct {
	Items []definitionDTO `json:"d,omitempty"`
}

type definitionDTO struct {
	ID           string            `json:"i"`
	Msg          string            `json:"m"`
	Title        string            `json:"t,omitempty"`
	Profile      string            `json:"p,omitempty"`
	Mode         string            `json:"o,omitempty"`
	StartUnix    int64             `json:"s,omitempty"`
	Initial      int               `json:"a,omitempty"`
	Min          int               `json:"n,omitempty"`
	Max          int               `json:"x,omitempty"`
	Speaker      string            `json:"sp,omitempty"`
	Phone        string            `json:"ph,omitempty"`
	Visible      []string          `json:"v,omitempty"`
	Night        bool              `json:"ng,omitempty"`
	SkipPresence bool              `json:"sr,omitempty"`
	QuietStart   string            `json:"qs,omitempty"`
	QuietEnd     string            `json:"qe,omitempty"`
	QuietOff     bool              `json:"qo,omitempty"`
	Metadata     map[string]string `json:"md,omitempty"`
	CreatedUnix  int64             `json:"c,omitempty"`
}

type runtimeStoreData struct {
	Items []runtimeDTO `json:"r,omitempty"`
}

type runtimeDTO struct {
	ID        string `json:"i"`
	NextUnix  int64  `json:"n,omitempty"`
	Repeat    int    `json:"r,omitempty"`
	Interval  int    `json:"l,omitempty"`
	Triggered int64  `json:"t,omitempty"`
	Completed bool   `json:"c,omitempty"`
	Cancelled bool   `json:"k,omitempty"`
	AckBy     string `json:"ab,omitempty"`
	AckAt     int64  `json:"aa,omitempty"`
	AwaitAck  bool   `json:"aw,omitempty"`
}

type viewStoreData struct {
	Users []viewUserDTO `json:"u,omitempty"`
}

type viewUserDTO struct {
	User  string    `json:"u"`
	Views []viewDTO `json:"v,omitempty"`
}

type viewDTO struct {
	ID        string `json:"i"`
	Title     string `json:"t,omitempty"`
	Msg       string `json:"m"`
	NextUnix  int64  `json:"n,omitempty"`
	Profile   string `json:"p,omitempty"`
	Mode      string `json:"o,omitempty"`
	Repeat    int    `json:"r,omitempty"`
	Completed bool   `json:"c,omitempty"`
	Cancelled bool   `json:"k,omitempty"`
	Speaker   string `json:"sp,omitempty"`
	Phone     string `json:"ph,omitempty"`
	AwaitAck  bool   `json:"aw,omitempty"`
}

func mapToDefinitionStoreData(defs map[string]ReminderDefinition) definitionStoreData {
	items := make([]definitionDTO, 0, len(defs))
	for _, def := range defs {
		dto := definitionDTO{
			ID:      def.ID,
			Msg:     def.Message,
			Title:   trimmedTitle(def.Title, def.Message),
			Profile: optionalProfile(def.Profile),
			Mode:    optionalMode(def.Mode),
		}

		if !def.StartTime.IsZero() {
			dto.StartUnix = def.StartTime.Unix()
		}
		if def.InitialRepeatMin != DefaultInitialRepeatMinutes {
			dto.Initial = def.InitialRepeatMin
		}
		if def.MinRepeatMin != DefaultMinRepeatMinutes {
			dto.Min = def.MinRepeatMin
		}
		if def.MaxRepeatMin != DefaultMaxRepeatMinutes {
			dto.Max = def.MaxRepeatMin
		}
		dto.Speaker = optionalTrim(def.SpeakerEntity)
		dto.Phone = optionalTrim(def.PhoneNotifier)
		dto.Visible = cloneStringSlice(def.VisibleTo)
		if def.NightModeAllowed {
			dto.Night = true
		}
		if !def.PresenceRequired {
			dto.SkipPresence = true
		}

		if def.QuietHours.Start != DefaultQuietHours.Start {
			dto.QuietStart = def.QuietHours.Start
		}
		if def.QuietHours.End != DefaultQuietHours.End {
			dto.QuietEnd = def.QuietHours.End
		}
		if !def.QuietHours.Enabled {
			dto.QuietOff = true
		}
		dto.Metadata = cloneStringMap(def.Metadata)
		if !def.CreatedAt.IsZero() {
			dto.CreatedUnix = def.CreatedAt.Unix()
		}

		items = append(items, dto)
	}

	return definitionStoreData{Items: items}
}

func (data definitionStoreData) toMap() map[string]ReminderDefinition {
	result := make(map[string]ReminderDefinition, len(data.Items))
	for _, dto := range data.Items {
		if dto.ID == "" {
			continue
		}
		def := ReminderDefinition{
			ID:               dto.ID,
			Title:            dto.Title,
			Message:          dto.Msg,
			Profile:          ProfileNormal,
			Mode:             ModeRepeating,
			StartTime:        unixToTime(dto.StartUnix),
			InitialRepeatMin: DefaultInitialRepeatMinutes,
			MinRepeatMin:     DefaultMinRepeatMinutes,
			MaxRepeatMin:     DefaultMaxRepeatMinutes,
			SpeakerEntity:    dto.Speaker,
			PhoneNotifier:    dto.Phone,
			VisibleTo:        cloneStringSlice(dto.Visible),
			NightModeAllowed: dto.Night,
			PresenceRequired: !dto.SkipPresence,
			QuietHours: QuietHoursConfig{
				Enabled: !dto.QuietOff,
				Start:   DefaultQuietHours.Start,
				End:     DefaultQuietHours.End,
			},
			Metadata:  cloneStringMap(dto.Metadata),
			CreatedAt: unixToTime(dto.CreatedUnix),
		}

		if def.Title == "" {
			def.Title = def.Message
		}
		if dto.Profile != "" {
			def.Profile = ReminderProfile(dto.Profile)
		}
		if dto.Mode != "" {
			def.Mode = ReminderMode(dto.Mode)
		}
		if dto.Initial != 0 {
			def.InitialRepeatMin = dto.Initial
		}
		if dto.Min != 0 {
			def.MinRepeatMin = dto.Min
		}
		if dto.Max != 0 {
			def.MaxRepeatMin = dto.Max
		}
		if dto.QuietStart != "" {
			def.QuietHours.Start = dto.QuietStart
		}
		if dto.QuietEnd != "" {
			def.QuietHours.End = dto.QuietEnd
		}

		result[def.ID] = def
	}
	return result
}

func mapToRuntimeStoreData(runtimes map[string]ReminderRuntime) runtimeStoreData {
	items := make([]runtimeDTO, 0, len(runtimes))
	for _, rt := range runtimes {
		dto := runtimeDTO{
			ID:        rt.ID,
			NextUnix:  timeToUnix(rt.NextTrigger),
			Repeat:    rt.RepeatCount,
			Interval:  rt.LastIntervalMin,
			Triggered: timeToUnix(rt.LastTriggered),
			Completed: rt.Completed,
			Cancelled: rt.Cancelled,
			AckBy:     rt.AcknowledgedBy,
			AckAt:     timeToUnix(rt.AcknowledgedAt),
			AwaitAck:  rt.AwaitingAck,
		}
		items = append(items, dto)
	}
	return runtimeStoreData{Items: items}
}

func (data runtimeStoreData) toMap() map[string]ReminderRuntime {
	result := make(map[string]ReminderRuntime, len(data.Items))
	for _, dto := range data.Items {
		if dto.ID == "" {
			continue
		}
		rt := ReminderRuntime{
			ID:              dto.ID,
			NextTrigger:     unixToTime(dto.NextUnix),
			RepeatCount:     dto.Repeat,
			LastIntervalMin: dto.Interval,
			LastTriggered:   unixToTime(dto.Triggered),
			Completed:       dto.Completed,
			Cancelled:       dto.Cancelled,
			AcknowledgedBy:  dto.AckBy,
			AcknowledgedAt:  unixToTime(dto.AckAt),
			AwaitingAck:     dto.AwaitAck,
		}
		result[rt.ID] = rt
	}
	return result
}

func mapToViewStoreData(views map[string][]ReminderView) viewStoreData {
	users := make([]viewUserDTO, 0, len(views))
	for user, entries := range views {
		dto := viewUserDTO{
			User:  user,
			Views: make([]viewDTO, 0, len(entries)),
		}
		for _, entry := range entries {
			item := viewDTO{
				ID:        entry.ID,
				Title:     entry.Title,
				Msg:       entry.Message,
				NextUnix:  timeToUnix(entry.NextTrigger),
				Profile:   optionalProfile(entry.Profile),
				Mode:      optionalMode(entry.Mode),
				Repeat:    entry.RepeatCount,
				Completed: entry.Completed,
				Cancelled: entry.Cancelled,
				Speaker:   entry.Speaker,
				Phone:     entry.Phone,
				AwaitAck:  entry.AwaitingAck,
			}
			dto.Views = append(dto.Views, item)
		}
		users = append(users, dto)
	}
	return viewStoreData{Users: users}
}

func (data viewStoreData) toMap() map[string][]ReminderView {
	result := make(map[string][]ReminderView, len(data.Users))
	for _, user := range data.Users {
		items := make([]ReminderView, 0, len(user.Views))
		for _, dto := range user.Views {
			view := ReminderView{
				ID:          dto.ID,
				Title:       dto.Title,
				Message:     dto.Msg,
				NextTrigger: unixToTime(dto.NextUnix),
				Profile:     ProfileNormal,
				Mode:        ModeRepeating,
				RepeatCount: dto.Repeat,
				Completed:   dto.Completed,
				Cancelled:   dto.Cancelled,
				Speaker:     dto.Speaker,
				Phone:       dto.Phone,
				AwaitingAck: dto.AwaitAck,
			}
			if view.Title == "" {
				view.Title = view.Message
			}
			if dto.Profile != "" {
				view.Profile = ReminderProfile(dto.Profile)
			}
			if dto.Mode != "" {
				view.Mode = ReminderMode(dto.Mode)
			}
			items = append(items, view)
		}
		result[user.User] = items
	}
	return result
}

func optionalProfile(profile ReminderProfile) string {
	if profile == ProfileNormal || profile == "" {
		return ""
	}
	return string(profile)
}

func optionalMode(mode ReminderMode) string {
	if mode == "" || mode == ModeRepeating {
		return ""
	}
	return string(mode)
}

func trimmedTitle(title, message string) string {
	if title == "" || title == message {
		return ""
	}
	return title
}

func optionalTrim(value string) string {
	if value == "" {
		return ""
	}
	return value
}

func timeToUnix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func unixToTime(ts int64) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0)
}

func cloneStringSlice(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
