package game

import (
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerFishingBootPassVictoryAndDeparture(t *testing.T) {
	s := explorerFishingGame(t, 3, "land-ho", false, true, false)
	p, other := s.Turn, (s.Turn+1)%3
	fishTop(&s.Catan.Fishing.Tokens, catanFishBoot)
	due := make([]int, 3)
	due[p] = 1
	if err := s.Catan.Fishing.Tokens.beginDraw(p, due); err != nil {
		t.Fatal(err)
	}
	explorerVictoryBuildings(t, s, p, 8)
	explorerVictoryBuildings(t, s, other, 8)
	s.catanExplorerVictory()
	if s.Finished {
		t.Fatal("boot holder won below additional point target")
	}
	explorerFishingAct(t, s, p, Action{Type: "catan_fish_boot", Target: other})
	if !s.Finished || !slices.Equal(s.Winners, []int{p}) || s.Catan.Fishing.Tokens.BootOwner != other {
		t.Fatal("boot pass did not award current player's victory")
	}

	for _, cities := range []bool{false, true} {
		s = explorerFishingGame(t, 3, "explorers-and-pirates", cities, true, true)
		p = s.Turn
		explorerFishingHand(t, s, p, 0, 11, 21)
		fishTop(&s.Catan.Fishing.Tokens, catanFishBoot)
		due = make([]int, 3)
		due[p] = 1
		if err := s.Catan.Fishing.Tokens.beginDraw(p, due); err != nil {
			t.Fatal(err)
		}
		if err := s.EliminateCatan(p); err != nil {
			t.Fatal(err)
		}
		if len(s.Catan.Fishing.Tokens.Hands[p]) != 0 || s.Catan.Fishing.Tokens.BootOwner != -1 {
			t.Fatal("departure retained private fish or boot")
		}
		explorerFishingRestore(t, s)
		explorerEventReady(t, s)
	}
}

func TestCatanExplorerFishingPairedSecondaryDoesNotProduce(t *testing.T) {
	for _, cities := range []bool{false, true} {
		s := explorerFishingGame(t, 6, "explorers-and-pirates", cities, true, true)
		explorerEventReady(t, s)
		before := clone(s.Catan.Fishing)
		roll := s.Catan.RollID
		explorerEventEnd(t, s)
		if !s.Catan.Paired.Second || s.Catan.RollID != roll || !reflect.DeepEqual(before, s.Catan.Fishing) {
			t.Fatal("secondary turn duplicated fish production")
		}
		explorerFishingRestore(t, s)
	}
}

func TestCatanExplorerFishingAlchemyAndCorruptContinuations(t *testing.T) {
	s := explorerFishingLakeFixture(t, true, false)
	p := s.Turn
	ckProgressGive(t, s, p, 0)
	if err := s.catanExplorerCityRollAction(p, Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 1}, Prompt: int(s.Catan.TurnSerial)}, func(int) int { return 3 }); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_fish_replace" || s.Catan.Fishing.LastRollID != 1 {
		t.Fatal("alchemy skipped fish production")
	}
	explorerFishingRestore(t, s)
	for name, damage := range map[string]func(*State){
		"version":              func(s *State) { s.Catan.Fishing.Explorer = "other" },
		"starting":             func(s *State) { s.Catan.Fishing.Started[0] = false },
		"roll":                 func(s *State) { s.Catan.Fishing.LastRollID-- },
		"resume":               func(s *State) { s.Catan.Fishing.Pending.Resume = "explorer_ready" },
		"resources":            func(s *State) { s.Catan.Fishing.Pending.Received[0] = -1 },
		"foreign-event":        func(s *State) { s.Catan.CardEvent = &CatanCardEvent{Kind: "earthquake"} },
		"phase":                func(s *State) { s.Phase = "catan_turn" },
		"eliminated-recipient": func(s *State) { s.Catan.Players[s.Catan.Fishing.Tokens.Pending[0].Player].Eliminated = true },
	} {
		t.Run(name, func(t *testing.T) {
			bad := clone(*s)
			damage(&bad)
			if bad.validateCatanExplorer() == nil {
				t.Fatal("accepted corrupt fish continuation")
			}
		})
	}
}
