package gameapi

// Failure inventory before implementation: ordinary paused snapshots acquire a
// system clock; a non-realtime module opts into paused timers; opt-in weakens
// the existing no-player-deadline/timeout/attention/result contract; persisted
// positive paused deadlines become invalid during restore.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func pausedClockModule() Module {
	m := Module{ID: "paused_clock", Name: "暂停呈现", MinPlayers: 1, MaxPlayers: 1, Realtime: true,
		Configure: func(int, json.RawMessage) (json.RawMessage, error) { return json.RawMessage(`{}`), nil },
		Start:     func(Context, json.RawMessage) (Match, error) { return Match{}, nil },
		Apply:     func(_ Context, m Match, _ json.RawMessage) (Match, error) { return m, nil },
		View:      func(Match, int) (any, error) { return nil, nil },
		OnTimer:   func(_ Context, m Match) (Match, error) { return m, nil }}
	// Keep the pre-implementation red test compilable while testing the API.
	field := reflect.ValueOf(&m).Elem().FieldByName("OnPausedTimer")
	if field.IsValid() {
		field.Set(reflect.ValueOf(func(_ Context, m Match) (Match, error) { return m, nil }))
	}
	return m
}

func TestPausedTimerCapabilityValidation(t *testing.T) {
	m := pausedClockModule()
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	match := Match{Data: json.RawMessage(`{}`), Paused: true, Actors: []int{0}, Attention: []int{}, SystemDeadline: 123}
	if err := m.ValidateMatch(match, 1); err != nil {
		t.Fatal("opt-in paused clock rejected", err)
	}
	if err := match.Validate(1); err == nil {
		t.Fatal("unscoped validation lost existing pause contract")
	}
	plain := m
	field := reflect.ValueOf(&plain).Elem().FieldByName("OnPausedTimer")
	if field.IsValid() {
		field.SetZero()
	}
	if err := plain.ValidateMatch(match, 1); err == nil {
		t.Fatal("ordinary paused module gained timer")
	}
	m.Realtime = false
	if err := m.Validate(); err == nil {
		t.Fatal("non-realtime module registered paused timer")
	}
	if err := m.ValidateMatch(match, 1); err == nil {
		t.Fatal("non-realtime module accepted paused clock")
	}
	m.Realtime = true
	for name, change := range map[string]func(*Match){
		"deadline":           func(v *Match) { v.Deadline = 100; v.TimeoutActors = []int{0} },
		"timeout":            func(v *Match) { v.TimeoutActors = []int{0} },
		"attention":          func(v *Match) { v.Attention = []int{0} },
		"implicit-attention": func(v *Match) { v.Attention = nil },
		"result": func(v *Match) {
			v.Actors = nil
			v.SystemDeadline = 0
			v.Result = &Result{Ranks: []int{1}, Winners: []int{0}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			v := match
			change(&v)
			if err := m.ValidateMatch(v, 1); err == nil {
				t.Fatal("invalid paused control admitted")
			}
		})
	}
}
