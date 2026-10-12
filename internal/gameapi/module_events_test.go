package gameapi

// Failure scenarios: unknown/duplicate attention or eliminated seats, attention
// outside allowed actors, eliminated actors still enabled, negative deadlines,
// system callbacks absent, finished games retaining a timer. Old modules must
// retain the distinction between omitted attention and explicitly no prompt.
import (
	"encoding/json"
	"testing"
)

func TestEventEnvelopeValidation(t *testing.T) {
	base := Match{Data: json.RawMessage(`{}`), Actors: []int{0, 1}, Attention: []int{}, SystemDeadline: 100}
	if err := base.Validate(3); err != nil {
		t.Fatal(err)
	}
	if err := (Module{}).ValidateMatch(base, 3); err == nil {
		t.Fatal("system deadline without a callback accepted")
	}
	for name, change := range map[string]func(*Match){
		"negative_system_deadline":   func(m *Match) { m.SystemDeadline = -1 },
		"unknown_attention":          func(m *Match) { m.Attention = []int{3} },
		"duplicate_attention":        func(m *Match) { m.Attention = []int{0, 0} },
		"cannot_act_attention":       func(m *Match) { m.Attention = []int{2} },
		"eliminated_actor":           func(m *Match) { m.Eliminated = []int{0} },
		"unknown_eliminated":         func(m *Match) { m.Eliminated = []int{3} },
		"duplicate_eliminated":       func(m *Match) { m.Eliminated = []int{2, 2} },
		"finished_with_system_timer": func(m *Match) { m.Actors = nil; m.Result = &Result{Ranks: []int{1, 2, 3}, Winners: []int{0}} },
	} {
		t.Run(name, func(t *testing.T) {
			m := base
			change(&m)
			if m.Validate(3) == nil {
				t.Fatal("invalid envelope accepted")
			}
		})
	}
	for _, attention := range [][]int{nil, {}} {
		m := base
		m.Attention = attention
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		var restored Match
		if err = json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		if (restored.Attention == nil) != (attention == nil) {
			t.Fatal("lost explicit empty attention on persistence")
		}
	}
}
