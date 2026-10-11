package game

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Failure scenarios fixed before the dead-end rule:
//   - the recorded frozen position (every city built, every road and ship
//     placed, no legal settlement left for either seat) must end on points
//     instead of rolling forever, and the log must carry the site rule;
//   - a fresh table, a table whose development deck still hides a victory-point
//     card, and a plain two-player seafarers table must all keep playing.
func catanTwoRiversFrozenState(t *testing.T) *State {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "catan-two-rivers-frozen.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s State
	if err = json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return &s
}

func TestCatanTwoRiversSeaDeadEnd(t *testing.T) {
	// Failure scenarios: held knights, including newly bought cards usable next
	// turn, can still claim the army bonus; conversely a tied army cannot take
	// that bonus from its current owner and must not keep a frozen table alive.
	t.Run("held knight can still claim the army", func(t *testing.T) {
		for _, newlyBought := range []bool{false, true} {
			s := catanTwoRiversFrozenState(t)
			g := s.Catan
			at := slices.Index(g.DevDiscard, 0)
			if at < 0 {
				t.Fatal("fixture has no discarded knight")
			}
			g.DevDiscard = slices.Delete(g.DevDiscard, at, at+1)
			g.Players[0].Knights = 2
			g.Players[0].Dev[0] = 1
			if newlyBought {
				g.Players[0].NewDev[0] = 1
			}
			if err := s.validateTwoRiversSea(); err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(s)
			if s.catanTwoRiversSeaDeadEnd() {
				t.Fatalf("held knight ignored (new=%t)", newlyBought)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("nonterminal check changed state")
			}
		}
	})
	t.Run("army tie cannot unseat the current owner", func(t *testing.T) {
		s := catanTwoRiversFrozenState(t)
		s.Catan.Players[0].Knights = 3
		s.Catan.Players[1].Knights = 3
		s.Catan.ArmyOwner = 0
		if err := s.validateTwoRiversSea(); err != nil {
			t.Fatal(err)
		}
		if !s.catanTwoRiversSeaDeadEnd() {
			t.Fatal("unchangeable army tie kept a frozen table alive")
		}
	})
	t.Run("frozen table ends on points", func(t *testing.T) {
		s := catanTwoRiversFrozenState(t)
		if err := s.validateTwoRiversSea(); err != nil {
			t.Fatal("fixture does not validate", err)
		}
		best, seat := -1, -1
		for i, p := range s.Catan.Players {
			if p.Score > best {
				best, seat = p.Score, i
			}
		}
		if best >= s.Catan.victoryTargetFor(seat) {
			t.Fatal("fixture already reached the target", best)
		}
		if !s.catanTwoRiversSeaDeadEnd() {
			t.Fatal("frozen table kept rolling")
		}
		if !s.Finished || s.Phase != "finished" || !slices.Equal(s.Winners, []int{seat}) {
			t.Fatal("frozen table did not finish on points", s.Finished, s.Phase, s.Winners, seat)
		}
		if last := s.Log[len(s.Log)-1]; !strings.Contains(last, CatanTwoRiversSeaDeadEndRules) {
			t.Fatal("dead-end site rule missing from the log", last)
		}
		if err := s.validateTwoRiversSea(); err != nil {
			t.Fatal("finished fixture does not validate", err)
		}
	})
	t.Run("frozen table ends through the normal turn end", func(t *testing.T) {
		s := catanTwoRiversFrozenState(t)
		if s.Phase != "catan_turn" {
			t.Fatal("fixture is not mid-turn", s.Phase)
		}
		for step := 0; step < 20 && !s.Finished; step++ {
			actor := twoFullActor(s)
			a, err := s.BotAction(actor)
			if err != nil {
				t.Fatal(step, s.Phase, err)
			}
			if err = s.Apply(actor, a); err != nil {
				t.Fatal(step, s.Phase, a, err)
			}
		}
		if !s.Finished {
			t.Fatalf("frozen table survived the turn end at round %d phase %s", s.Round, s.Phase)
		}
	})
	t.Run("fresh table keeps playing", func(t *testing.T) {
		s, err := newCatanTwoRiversSeafarers("shores", "")
		if err != nil {
			t.Fatal(err)
		}
		if s.catanTwoRiversSeaDeadEnd() {
			t.Fatal("fresh table ended before the target")
		}
	})
	t.Run("hidden victory card keeps playing", func(t *testing.T) {
		s := catanTwoRiversFrozenState(t)
		g := s.Catan
		g.Players[0].Dev[4]--
		g.DevDeck = append(g.DevDeck, 4)
		if s.catanTwoRiversSeaDeadEnd() {
			t.Fatal("a hidden victory-point card still decides the game")
		}
	})
	t.Run("plain two-player seafarers keeps playing", func(t *testing.T) {
		s := twoSeaGame(t, "shores", "", false, false)
		if s.catanTwoRiversSeaDeadEnd() {
			t.Fatal("dead-end rule applied outside the rivers combination")
		}
	})
}
