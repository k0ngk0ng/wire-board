package server

import (
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// A module may declare a practice/casual result. It is archived and displayed,
// but must not write competitive rating points. Normal two-player results keep
// the existing winner/loser deltas.
func TestRateMatchUnratedPractice(t *testing.T) {
	room := &Room{
		ID: "practice", MatchID: "practice-1", Kind: "tetris", Status: "finished", Rated: true,
		Seats:  []Seat{{User: User{ID: "a"}}, {User: User{ID: "b"}}},
		Game:   &game.State{Finished: true, Winners: []int{0}},
		Module: &gameapi.Match{Result: &gameapi.Result{Unrated: true, Ranks: []int{1, 2}, Winners: []int{0}, Scores: []int{120, 80}}},
	}
	record := MatchRecord{Players: []MatchPlayer{{User: User{ID: "a"}}, {User: User{ID: "b"}}}}
	rateMatch(room, &record)
	if record.Rated || !room.Module.Result.Unrated {
		t.Fatalf("practice record rated=%v result=%+v", record.Rated, room.Module.Result)
	}
	for i := range record.Players {
		if record.Players[i].RatingDelta != 0 {
			t.Fatalf("practice player %d got rating %+v", i, record.Players[i])
		}
	}

	room.Module.Result.Unrated = false
	record = MatchRecord{Players: []MatchPlayer{{User: User{ID: "a"}}, {User: User{ID: "b"}}}}
	rateMatch(room, &record)
	if !record.Rated {
		t.Fatal("normal match was not rated")
	}
	if record.Players[0].RatingDelta != 20 || record.Players[1].RatingDelta != -5 {
		t.Fatalf("deltas=%d,%d", record.Players[0].RatingDelta, record.Players[1].RatingDelta)
	}
}
