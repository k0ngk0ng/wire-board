package server

import (
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestSanguoshaThreeV3RoundObservationTracksCompletedTurns(t *testing.T) {
	for _, test := range []struct {
		name         string
		unmarked     int
		active       int
		flipped      bool
		stillFlipped bool
		dead         bool
		want         int
	}{
		{"another seat responds for the last active turn", 1, 1, false, false, false, -1},
		{"a living untouched seat must still act", 2, 1, false, false, false, 2},
		{"a consumed flipped turn counts", 2, 1, true, false, false, -1},
		{"an unconsumed flip does not count", 2, 1, true, true, false, 2},
		{"a dead seat does not block the next round", 2, 1, false, false, true, -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			after, err := game.NewSanguosha(6, game.SGOptions{Mode: "3v3"})
			if err != nil {
				t.Fatal(err)
			}
			acted := []bool{true, true, true, true, true, true}
			acted[test.unmarked] = false
			flipped := make([]bool, 6)
			flipped[test.unmarked] = test.flipped
			after.Sanguosha.Players[test.unmarked].Flipped = test.stillFlipped
			after.Sanguosha.Players[test.unmarked].Dead = test.dead
			if got := sanguoshaThreeMissingRoundSeat(acted, flipped, test.active, after); got != test.want {
				t.Fatalf("missing seat = %d, want %d", got, test.want)
			}
		})
	}
}
