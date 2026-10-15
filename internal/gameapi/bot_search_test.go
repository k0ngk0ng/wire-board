package gameapi

// Failure scenarios before implementation: half-configured search, no safe
// fallback or reclaim callback, negative/unbounded budget, and accidental JSON
// publication of a private executable contract.
import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestBotSearchRegistration(t *testing.T) {
	base := Module{ID: "search", Name: "搜索", MinPlayers: 2, MaxPlayers: 2,
		Configure: func(int, json.RawMessage) (json.RawMessage, error) { return nil, nil },
		Start:     func(Context, json.RawMessage) (Match, error) { return Match{}, nil },
		Apply:     func(Context, Match, json.RawMessage) (Match, error) { return Match{}, nil },
		View:      func(Match, int) (any, error) { return nil, nil },
		BotAction: func(Context, Match) (json.RawMessage, error) { return nil, nil },
		Reclaim:   func(Context, Match) (Match, error) { return Match{}, nil },
		BotSearch: &BotSearch{Input: func(Context, Match) (json.RawMessage, error) { return nil, nil }, Think: func(context.Context, json.RawMessage) (json.RawMessage, error) { return nil, nil }},
	}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"input", "think", "fallback", "reclaim", "negative", "unbounded"} {
		t.Run(kind, func(t *testing.T) {
			m := base
			cfg := *base.BotSearch
			m.BotSearch = &cfg
			switch kind {
			case "input":
				cfg.Input = nil
			case "think":
				cfg.Think = nil
			case "fallback":
				m.BotAction = nil
			case "reclaim":
				m.Reclaim = nil
			case "negative":
				cfg.Budget = -1
			case "unbounded":
				cfg.Budget = 11 * time.Second
			}
			if m.Validate() == nil {
				t.Fatal("invalid search accepted")
			}
		})
	}
	raw, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	if len(fields) != 4 {
		t.Fatal("private search contract exposed", string(raw))
	}
}
