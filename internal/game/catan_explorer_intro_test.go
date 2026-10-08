package game

import (
	"fmt"
	"testing"
)

func explorerIntroActor(s *State) int {
	p := s.CatanPendingActor()
	if p >= 0 {
		return p
	}
	if s.Phase == "catan_discard" {
		for p, due := range s.Catan.DiscardDue {
			if due > 0 {
				return p
			}
		}
	}
	return s.Turn
}

func TestCatanExplorerIntroRecipesAndTurns(t *testing.T) {
	for n := 3; n <= 6; n++ {
		for _, cities := range []bool{false, true} {
			for _, fishing := range []bool{false, true} {
				for _, lakes := range []bool{false, true} {
					if lakes && !fishing {
						continue
					}
					t.Run(fmt.Sprintf("%d/city%t/fish%t/lake%t", n, cities, fishing, lakes), func(t *testing.T) {
						var s *State
						var err error
						if fishing {
							s, err = NewCatanExplorerFishing(n, "land-ho", cities, lakes)
						} else if cities {
							s, err = NewCatanExplorerCitiesKnights(n, "land-ho")
						} else {
							s, err = NewCatanExplorerLandHo(n)
						}
						if err != nil {
							t.Fatal(err)
						}
						b := s.Catan.Explorer.Board
						if b.Target != 8+btoi(cities)*5 || s.Catan.Explorer.Pirate != nil || s.Catan.Explorer.Lairs != nil {
							t.Fatal("intro target or components")
						}
						if n > 4 || cities {
							if b.IntroRules != CatanExplorerIntroRules || b.Layout != "variable" || len(b.Opening) != 0 || s.Catan.Explorer.Setup == nil {
								t.Fatal("adapted opening")
							}
						} else if b.IntroRules != "" || b.Layout != "fixed" || len(b.Opening) != 4 || s.Catan.Explorer.Setup != nil {
							t.Fatal("printed opening changed")
						}
						if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
							t.Fatal(err)
						}
						for step := 0; step < 400 && s.Catan.TurnSerial < 5; step++ {
							if s.CatanExplorerSetupBlocked() {
								if err = s.ResetCatanExplorerSetup(); err != nil {
									t.Fatal(err)
								}
							}
							p := explorerIntroActor(s)
							a, err := s.BotAction(p)
							if err != nil {
								t.Fatal(step, s.Phase, err)
							}
							if err = s.Apply(p, a); err != nil {
								t.Fatal(step, s.Phase, a, err)
							}
							explorerFishingRestore(t, s)
						}
						if s.Catan.TurnSerial < 5 {
							t.Fatal("opening/turns did not progress")
						}
					})
				}
			}
		}
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestCatanExplorerIntroNaturalMatches(t *testing.T) {
	for _, tc := range []struct {
		n                  int
		city, fish, events bool
	}{
		{5, false, false, false}, {6, false, true, true}, {3, true, false, false}, {6, true, true, true},
	} {
		t.Run(fmt.Sprintf("%d/city%t/fish%t/events%t", tc.n, tc.city, tc.fish, tc.events), func(t *testing.T) {
			var s *State
			var err error
			if tc.fish {
				s, err = NewCatanExplorerFishing(tc.n, "land-ho", tc.city, true)
			} else if tc.city {
				s, err = NewCatanExplorerCitiesKnights(tc.n, "land-ho")
			} else {
				s, err = NewCatanExplorerLandHo(tc.n)
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.events {
				if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
					t.Fatal(err)
				}
			}
			seen := map[string]int{}
			for step := 0; step < 10000 && !s.Finished; step++ {
				if s.CatanExplorerSetupBlocked() {
					if err = s.ResetCatanExplorerSetup(); err != nil {
						t.Fatal(err)
					}
				}
				p := explorerIntroActor(s)
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(step, s.Phase, err)
				}
				if err = s.Apply(p, a); err != nil {
					t.Fatal(step, s.Phase, a, err)
				}
				seen[a.Type]++
				if step%71 == 0 {
					explorerFishingRestore(t, s)
				}
			}
			if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < s.Catan.victoryTargetFor(s.Winners[0]) {
				t.Fatal("natural game did not finish", s.Round, s.Phase)
			}
			t.Log("round", s.Round, "actions", seen)
			explorerFishingRestore(t, s)
		})
	}
}

func TestCatanExplorerIntroCitySevenTaxationAndVersion(t *testing.T) {
	s := explorerCityStateStarted(t, 3, "land-ho")
	for _, damage := range []func(*State){
		func(s *State) { s.Catan.Explorer.Board.IntroRules = "" },
		func(s *State) { s.Catan.Explorer.Board.IntroRules = "unknown" },
		func(s *State) { s.Catan.Explorer.Board.Target = 8 },
		func(s *State) { s.Catan.Explorer.Pirate = newCatanExplorerPirate() },
	} {
		bad := clone(*s)
		damage(&bad)
		if bad.validateCatanExplorer() == nil {
			t.Fatal("corrupt intro accepted")
		}
	}
	g := s.Catan
	// Explicit legal response fixture: eight cards and an aqueduct.
	for p := range g.Players {
		for r, n := range g.Players[p].Resources {
			g.Bank[r] += n
			g.Players[p].Resources[r] = 0
		}
		g.Players[p].Resources[p] = 8
		g.Bank[p] -= 8
		g.CitiesKnights.Players[p].Improvements[CatanScience] = 3
	}
	if err := s.catanExplorerCityRoll(3, 4, 3); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_discard" {
		t.Fatal("seven did not discard")
	}
	for s.Phase == "catan_discard" {
		p := explorerIntroActor(s)
		cards := make([]int, 8)
		cards[p] = 4
		explorerCityStateAct(t, s, p, Action{Type: "catan_discard", Tokens: cards})
	}
	if s.Phase != "catan_turn" || s.Catan.CitiesKnights.Pending != nil || s.Catan.Explorer.Pirate != nil {
		t.Fatal("seven inserted pirate/aqueduct")
	}
	for _, gold := range s.Catan.Explorer.Economy.Gold {
		if gold != 2 {
			t.Fatal("seven granted compensation")
		}
	}
	p := s.Turn
	ckProgressGive(t, s, p, 21)
	explorerCityStateReject(t, s, p, Action{Type: "catan_progress", Card: 21, Prompt: int(s.Catan.TurnSerial)})
	s.Catan.CitiesKnights.Invasions = 1
	explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 21})
	if s.Phase != "catan_turn" || s.Catan.Explorer.Pirate != nil {
		t.Fatal("taxation created pirate")
	}
}

func TestCatanExplorerIntroDeparture(t *testing.T) {
	for _, tc := range []struct {
		n    int
		city bool
	}{{5, false}, {3, true}, {6, true}} {
		t.Run(fmt.Sprintf("%d/city%t", tc.n, tc.city), func(t *testing.T) {
			s := explorerFishingGame(t, tc.n, "land-ho", tc.city, true, true)
			p := s.Turn
			if err := s.EliminateCatan(p); err != nil {
				t.Fatal(err)
			}
			explorerFishingRestore(t, s)
			if !s.Catan.Players[p].Eliminated || sum(s.Catan.Players[p].Resources) != 0 || len(s.Catan.Fishing.Tokens.Hands[p]) != 0 {
				t.Fatal("departure retained holdings")
			}
			serial := s.Catan.TurnSerial
			for i := 0; i < 250 && s.Catan.TurnSerial < serial+4 && !s.Finished; i++ {
				actor := explorerIntroActor(s)
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatal(err)
				}
				if err = s.Apply(actor, a); err != nil {
					t.Fatal(err)
				}
			}
			if !s.Finished && s.Catan.TurnSerial < serial+4 {
				t.Fatal("post-departure game stalled")
			}
			explorerFishingRestore(t, s)
		})
	}
}
