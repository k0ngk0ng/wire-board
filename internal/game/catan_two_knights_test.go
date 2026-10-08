package game

import (
	"fmt"
	"slices"
	"testing"
)

func twoKnightsFixture(t *testing.T) *State {
	t.Helper()
	s, e := newCatanTwoCitiesKnights()
	if e != nil {
		t.Fatal(e)
	}
	for s.Catan.setup() {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		helperApply(t, s, p, a)
	}
	for p := range 2 {
		r, v, c := s.Catan.pieces(p)
		if r != 2 || v != 1 || c != 1 || s.Catan.Players[p].Score != 3 || sum(s.Catan.Players[p].Resources[5:]) != 0 {
			t.Fatal("wrong two-player city opening")
		}
	}
	s.Phase = "catan_turn"
	s.Catan.Two.Rolls = []int{2, 3}
	s.Catan.RollID = 2
	s.Catan.Dice = []int{1, 2}
	s.Catan.CitiesKnights.EventDie = 0
	return s
}

func twoKnightsHand(s *State, p int, hand []int) {
	for c, n := range hand {
		s.Catan.Bank[c] += s.Catan.Players[p].Resources[c] - n
		s.Catan.Players[p].Resources[c] = n
	}
}

func TestCatanTwoKnightsRecruitPromoteAndSmithing(t *testing.T) {
	s := twoKnightsFixture(t)
	p := s.Turn
	twoKnightsHand(s, p, []int{4, 4, 6, 4, 6, 0, 0, 0})
	for step := range 2 {
		vertex := -1
		for _, v := range s.Catan.Vertices {
			if s.Catan.knightRecruitable(p, v.ID) {
				vertex = v.ID
				break
			}
		}
		if vertex < 0 {
			t.Fatal("no own recruit site")
		}
		helperApply(t, s, p, Action{Type: "catan_knight_recruit", Vertex: vertex})
		if s.Phase != "catan_two_build" || s.Catan.Two.Pending.Kind != "knight" {
			t.Fatal("missing neutral recruit")
		}
		choices := s.Catan.twoNeutralChoices("knight")
		if step == 0 && choices[0].Edge < 0 {
			t.Fatal("neutrals without roads must fall back to road")
		}
		if step == 1 && choices[0].Vertex < 0 {
			t.Fatal("neutral with legal knight site fell back")
		}
		twoCoreRestore(t, s)
		if step == 0 {
			// Select a road whose free endpoint can actually host a knight.
			// Another legal road may end at a blocking player settlement.
			found := false
			for _, choice := range choices {
				candidate := clone(*s)
				if candidate.Catan.placeTwoNeutral("road", choice) == nil && len(candidate.Catan.twoKnightChoices("knight")) > 0 {
					helperApply(t, s, p, Action{Type: "catan_two_build", Target: -choice.Owner - 2, Edge: choice.Edge, Vertex: choice.Vertex})
					found = true
					break
				}
			}
			if !found {
				t.Fatal("no fixture road with a recruitable endpoint")
			}
		} else {
			s.AutoCatanPending()
		}
		if s.Phase != "catan_turn" {
			t.Fatal("neutral recruit response stalled")
		}
	}
	// One neutral knight exists after the fallback road and second recruit.
	// Prepare another legal neutral road/knight for the two-upgrade card fixture.
	g := s.Catan
	for attempt := 0; attempt < 8 && len(g.twoKnightChoices("knight")) == 0; attempt++ {
		choices := g.twoNeutralChoices("road")
		if len(choices) == 0 {
			t.Fatal("no neutral road")
		}
		if e := g.placeTwoNeutral("road", choices[0]); e != nil {
			t.Fatal(e)
		}
	}
	choices := g.twoKnightChoices("knight")
	if len(choices) == 0 {
		t.Fatal("no second neutral knight")
	}
	if e := g.placeTwoNeutral("knight", choices[0]); e != nil {
		t.Fatal(e)
	}
	own := []int{}
	for _, n := range g.CitiesKnights.Knights {
		if n.Owner == p {
			own = append(own, n.Vertex)
		}
	}
	ckProgressGive(t, s, p, 8)
	helperApply(t, s, p, Action{Type: "catan_progress", Card: 8, Targets: own})
	if s.Catan.Two.Pending == nil || len(s.Catan.Two.Pending.Remaining) != 1 {
		t.Fatal("missing two smithing responses")
	}
	for i := range 2 {
		twoCoreRestore(t, s)
		s.AutoCatanPending()
		if i == 0 && s.Phase != "catan_two_build" {
			t.Fatal("second neutral promotion lost")
		}
	}
	if s.Phase != "catan_turn" {
		t.Fatal("smithing did not finish")
	}
	for _, n := range s.Catan.CitiesKnights.Knights {
		if n.Strength != 2 || n.Active {
			t.Fatal("wrong smithing result", n)
		}
		if n.Owner < 0 && s.Catan.knightCanPromote(&n) {
			t.Fatal("neutral can upgrade to mighty")
		}
	}
	ckSupply(t, s.Catan)
	ckProgressStock(t, s.Catan)
}

func TestCatanTwoKnightsForcedTradeTypesAndFiniteSupply(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprint(mixed), func(t *testing.T) {
			s := twoKnightsFixture(t)
			p := s.Turn
			twoKnightsHand(s, p, []int{1, 0, 0, 0, 0, 1, 0, 0})
			twoKnightsHand(s, 1-p, []int{0, 1, 0, 0, 0, 0, 4, 0})
			choice := "resources"
			cost := 1
			if mixed {
				choice = "mixed"
				cost = 2
			}
			before := s.Catan.Two.Tokens[p]
			helperApply(t, s, p, Action{Type: "catan_two_trade", Choice: choice})
			g := s.Catan
			if g.Two.Tokens[p] != before-cost || g.Two.Trade.Mixed != mixed {
				t.Fatal("wrong price or type")
			}
			if !mixed && (sum(g.Two.Trade.Drawn[5:]) != 0 || g.Players[1-p].Resources[6] != 4) {
				t.Fatal("resource-only trade stole commodities")
			}
			if !mixed {
				helperReject(t, s, p, Action{Type: "catan_two_return", Give: []int{1, 0, 0, 0, 0, 1, 0, 0}})
			}
			for _, viewer := range []int{-1, 1 - p} {
				v := s.View(viewer)["catan"].(map[string]any)["two"].(map[string]any)
				if v["trade"].(map[string]any)["drawn"] != nil {
					t.Fatal("forced draw leaked")
				}
			}
			twoCoreRestore(t, s)
			a, e := s.catanTwoReturnBot(p)
			if e != nil {
				t.Fatal(e)
			}
			helperApply(t, s, p, a)
			if s.Phase != "catan_turn" || s.Catan.Two.Trade != nil {
				t.Fatal("trade not completed")
			}
			ckSupply(t, s.Catan)
		})
	}
	s := twoKnightsFixture(t)
	p := s.Turn
	g := s.Catan
	g.Two.Bank = 1
	g.Two.Tokens = []int{9, 10}
	if e := s.catanTwoEarn(p, 3); e != nil {
		t.Fatal(e)
	}
	if g.Two.Bank != 0 || g.Two.TokensIssued != 0 || sum(g.Two.Tokens) != 20 {
		t.Fatal("finite supply issued tokens")
	}
	if e := s.validateCatanTwo(); e != nil {
		t.Fatal(e)
	}
	if e := s.catanTwoEarn(p, 2); e != nil {
		t.Fatal(e)
	}
	if sum(g.Two.Tokens) != 20 {
		t.Fatal("empty supply yielded tokens")
	}
}

func TestCatanTwoKnightsTwoEventsAndAlchemy(t *testing.T) {
	s := twoKnightsFixture(t)
	p := s.Turn
	g := s.Catan
	g.Two.Rolls = []int{}
	g.RollID = 0
	g.Dice = nil
	g.CitiesKnights.EventDie = -1
	s.Phase = "catan_roll"
	ckProgressGive(t, s, p, 0, 0)
	before := clone(*s)
	if e := s.catanPlayProgressRandom(p, Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 1}}, func(int) int { return 3 }); e != nil {
		t.Fatal(e)
	}
	if e := s.catanTwoAfterAction(&before, Action{Type: "catan_progress", Card: 0}); e != nil {
		t.Fatal(e)
	}
	if s.Phase != "catan_roll" || !slices.Equal(s.Catan.Two.Rolls, []int{2}) || s.Catan.CitiesKnights.BarbarianPosition != 1 {
		t.Fatal("first city production not completed")
	}
	helperReject(t, s, p, Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 2}})
	bad := clone(*s)
	if e := bad.catanCityRoll(1, 1, 3); e == nil {
		t.Fatal("same second dice accepted")
	}
	before = clone(*s)
	if e := s.catanCityRoll(1, 2, 3); e != nil {
		t.Fatal(e)
	}
	if e := s.catanTwoAfterAction(&before, Action{Type: "catan_roll"}); e != nil {
		t.Fatal(e)
	}
	if s.Phase != "catan_turn" || s.Catan.CitiesKnights.BarbarianPosition != 2 || s.Catan.RollID != 2 {
		t.Fatal("second city event not resolved")
	}
	if e := s.validateCatanTwo(); e != nil {
		t.Fatal(e)
	}
}

func TestCatanTwoKnightsCompleteEngineGames(t *testing.T) {
	for _, events := range []bool{false, true} {
		for sample := range 2 {
			t.Run(fmt.Sprintf("events=%v/%d", events, sample), func(t *testing.T) {
				s, err := newCatanTwoCitiesKnights()
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				steps := 0
				used := map[string]int{}
				for ; steps < 8000 && !s.Finished; steps++ {
					p := twoFullActor(s)
					a, e := s.BotAction(p)
					if e != nil {
						t.Fatal(steps, s.Phase, e)
					}
					used[a.Type]++
					if e = s.Apply(p, a); e != nil {
						t.Fatal(steps, s.Phase, a, e)
					}
					if e = s.validateCatanTwo(); e != nil {
						t.Fatal(e)
					}
					ckProgressStock(t, s.Catan)
					if steps%31 == 0 {
						twoCoreRestore(t, s)
					}
				}
				if !s.Finished {
					t.Fatal("not finished", steps, s.Phase, used)
				}
				if s.Catan.Players[s.Turn].Score < 13 {
					t.Fatal("wrong victory threshold")
				}
				t.Log(steps, "steps", s.Round, "rounds", used)
			})
		}
	}
}
