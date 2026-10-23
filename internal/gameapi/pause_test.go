package gameapi

// Failure inventory before the pause contract: paused snapshots retain a player
// or system deadline, automatic responders, action attention, or a final result;
// save/restore silently loses the explicit paused state. Actors are still legal
// because games use ordinary authenticated actions for manual resume votes.
import (
	"encoding/json"
	"testing"
)

func TestPausedEnvelopeValidation(t *testing.T) {
	base := Match{Data: json.RawMessage(`{}`), Paused: true, Actors: []int{0, 1}, Attention: []int{}}
	if err := base.Validate(2); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Match){
		"player deadline":    func(m *Match) { m.Deadline = 100; m.TimeoutActors = []int{0} },
		"system deadline":    func(m *Match) { m.SystemDeadline = 100 },
		"timeout actor":      func(m *Match) { m.TimeoutActors = []int{0} },
		"attention":          func(m *Match) { m.Attention = []int{0} },
		"implicit attention": func(m *Match) { m.Attention = nil },
		"finished":           func(m *Match) { m.Actors = nil; m.Result = &Result{Ranks: []int{1, 2}, Winners: []int{0}} },
	} {
		t.Run(name, func(t *testing.T) {
			m := base
			change(&m)
			if m.Validate(2) == nil {
				t.Fatal("invalid paused state accepted")
			}
		})
	}
	raw, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	var restored Match
	if err = json.Unmarshal(raw, &restored); err != nil || !restored.Paused || restored.Attention == nil {
		t.Fatal("pause contract lost on persistence", err)
	}
	var legacy Match
	if err = json.Unmarshal([]byte(`{"data":{},"actors":[0]}`), &legacy); err != nil || legacy.Paused || legacy.Validate(2) != nil {
		t.Fatal("legacy match changed", err)
	}
}
