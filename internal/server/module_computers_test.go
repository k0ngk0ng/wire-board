package server

// Failures: fixed computer participants do not occupy host seats, so checking
// Seat.Bot alone wrongly rates a match; restore/history must also enforce this
// metadata without mislabeling a competitive match as cooperative.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

func TestModuleFixedComputersDoNotRate(t *testing.T) {
	for _, computers := range []bool{false, true} {
		m := gameapi.Module{ID: "fixture", MinPlayers: 2, MaxPlayers: 2,
			Configure: func(_ int, raw json.RawMessage) (json.RawMessage, error) { return json.RawMessage(`{}`), nil },
			Start: func(gameapi.Context, json.RawMessage) (gameapi.Match, error) {
				return gameapi.Match{Data: json.RawMessage(`{}`), HasComputerPlayers: computers, Result: &gameapi.Result{Ranks: []int{1, 2}, Winners: []int{0}, Team: true}}, nil
			},
		}
		s := Server{modules: map[string]gameapi.Module{"fixture": m}}
		r := Room{Kind: "fixture", Seats: []Seat{{}, {}}}
		if err := s.startModule(&r, 0, time.Now()); err != nil {
			t.Fatal(err)
		}
		if r.Rated == computers {
			t.Fatal("wrong initial rating eligibility", computers, r.Rated)
		}
		// Persisted old room may still have Rated=true: result boundary must defend it.
		r.Rated = true
		record := MatchRecord{Players: make([]MatchPlayer, 2)}
		rateMatch(&r, &record)
		if record.Rated == computers {
			t.Fatal("wrong final rating eligibility", computers, record)
		}
	}
}
