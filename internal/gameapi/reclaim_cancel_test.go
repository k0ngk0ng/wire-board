package gameapi

// Failure cases before the optional dispatch capability: opting in without
// callbacks is accepted; a legacy module is forced into the new behavior; the
// private capability leaks through the public module catalog.
import (
	"encoding/json"
	"testing"
)

func TestReclaimOnCancelRegistration(t *testing.T) {
	m := Module{ID: "reclaim", Name: "独立时限", MinPlayers: 2, MaxPlayers: 2,
		Configure: func(int, json.RawMessage) (json.RawMessage, error) { return nil, nil },
		Start:     func(Context, json.RawMessage) (Match, error) { return Match{}, nil },
		Apply:     func(Context, Match, json.RawMessage) (Match, error) { return Match{}, nil },
		View:      func(Match, int) (any, error) { return nil, nil },
	}
	if err := m.Validate(); err != nil {
		t.Fatal("legacy module rejected", err)
	}
	m.ReclaimOnCancel = true
	if m.Validate() == nil {
		t.Fatal("reclaim-on-cancel without callbacks accepted")
	}
	m.BotAction = func(Context, Match) (json.RawMessage, error) { return nil, nil }
	m.Reclaim = func(_ Context, v Match) (Match, error) { return v, nil }
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var public map[string]any
	if err = json.Unmarshal(raw, &public); err != nil {
		t.Fatal(err)
	}
	if len(public) != 4 {
		t.Fatal("private reclaim dispatch setting exposed", string(raw))
	}
}
