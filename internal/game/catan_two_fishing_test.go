package game

import (
	"fmt"
	"slices"
	"testing"
)

func twoFishingGame(t *testing.T) *State {
	t.Helper()
	s, err := NewCatanTwoFishing(2, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCatanTwoFishingSetupAndCompleteGames(t *testing.T) {
	for _, events := range []bool{false, true} {
		for sample := range 2 {
			t.Run(fmt.Sprintf("events=%v/%d", events, sample), func(t *testing.T) {
				s := twoFishingGame(t)
				if events {
					if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				for _, hand := range s.Catan.Fishing.Tokens.Hands {
					values := []int{}
					for _, id := range hand {
						values = append(values, catanFishValue(id))
					}
					slices.Sort(values)
					if !slices.Equal(values, []int{1, 1, 2, 2, 3}) {
						t.Fatal("starting fish", values)
					}
				}
				steps := 0
				for ; steps < 5000 && !s.Finished; steps++ {
					setup := s.Catan.setup()
					p := twoFullActor(s)
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(steps, s.Phase, err)
					}
					if err = s.Apply(p, a); err != nil {
						t.Fatal(steps, s.Phase, a, err)
					}
					assertTwoFullState(t, s)
					if err := s.Catan.validateFishing(); err != nil {
						t.Fatal(err)
					}
					if setup {
						for _, h := range s.Catan.Fishing.Tokens.Hands {
							if len(h) != 5 {
								t.Fatal("setup awarded extra fish")
							}
						}
					}
					if steps%31 == 0 {
						twoCoreRestore(t, s)
					}
				}
				if !s.Finished {
					t.Fatal("not finished", steps, s.Phase)
				}
				if s.Catan.Players[s.Turn].Score < s.Catan.victoryTargetFor(s.Turn) {
					t.Fatal("ignored boot")
				}
				twoCoreRestore(t, s)
				t.Log("natural complete game", steps, "steps", s.Round, "rounds")
			})
		}
	}
}

func TestCatanTwoFishingPricesAndNeutralRoad(t *testing.T) {
	s := twoFishingGame(t)
	for s.Catan.setup() {
		p := twoFullActor(s)
		a, err := s.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Apply(p, a); err != nil {
			t.Fatal(err)
		}
	}
	g, p := s.Catan, s.Turn
	for _, phase := range []string{"catan_roll", "catan_turn"} {
		t.Run(phase, func(t *testing.T) {
			next := clone(*s)
			next.Phase = phase
			if phase == "catan_turn" {
				next.Catan.Two.Rolls = []int{4, 5}
			}
			g := next.Catan
			road := -1
			for _, e := range g.Edges {
				if g.canRoad(p, e.ID) {
					road = e.ID
					break
				}
			}
			if road < 0 {
				t.Fatal("no road")
			}
			a := Action{Type: "catan_fish_road", Edge: road, Tokens: g.fishPayment(p, 5)}
			if err := next.Apply(p, a); err != nil {
				t.Fatal(err)
			}
			if next.Phase != "catan_two_build" || next.Catan.Two.Pending.Resume != phase {
				t.Fatal("missing neutral construction", next.Phase)
			}
			twoCoreRestore(t, &next)
			next.AutoCatanPending()
			if next.Phase != phase {
				t.Fatal("wrong continuation", next.Phase)
			}
		})
	}
	for _, a := range []string{"catan_two_trade", "catan_two_robber", "catan_two_knight"} {
		helperReject(t, s, p, Action{Type: a})
	}
	// Explicit price fixture: public points alone determine the discount.
	g.Players[1-p].Score++
	for kind, full := range catanFishCosts {
		if g.fishActionCost(p, kind) != full-1 || g.fishActionCost(1-p, kind) != full {
			t.Fatal("discount", kind)
		}
	}
	g.Players[p].Dev[4]++
	g.Players[p].Score++
	if g.fishActionCost(p, "catan_fish_resource") != 3 {
		t.Fatal("hidden point leaked through cost")
	}
	legal := s.catanFishLegal(p)
	if legal["costs"].(map[string]int)["catan_fish_resource"] != 3 {
		t.Fatal("public price")
	}
	if s.catanTwoTokenWindow(p) || s.catanTwoCanExchangeKnight(p) {
		t.Fatal("trade tokens still enabled")
	}
}

func TestCatanTwoFishingDiscountPaymentAndVersionIsolation(t *testing.T) {
	s := twoFishingGame(t)
	for s.Catan.setup() {
		p := twoFullActor(s)
		a, err := s.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, p, a)
	}
	g, p := s.Catan, s.Turn
	// A legal opponent city makes its public score one higher. Resource costs
	// are irrelevant to this explicit price fixture; no hidden cards are used.
	for i, v := range g.Vertices {
		if v.Owner == 1-p && v.Level == 1 {
			g.Vertices[i].Level = 2
			break
		}
	}
	s.catanScores()
	ids := g.fishPayment(p, 3)
	before := g.Players[p].Resources[0]
	helperApply(t, s, p, Action{Type: "catan_fish_resource", Color: 0, Tokens: ids})
	if s.Catan.Players[p].Resources[0] != before+1 || len(s.Catan.Fishing.Tokens.Discard) != len(ids) {
		t.Fatal("discount not used by payment")
	}
	for _, change := range []func(*State){
		func(s *State) { s.Catan.Fishing.Two = "" },
		func(s *State) { s.Catan.Fishing.Two = "unverified" },
		func(s *State) { s.Catan.Two.Bank = 20 },
		func(s *State) { s.Catan.Two.Tokens[0] = 1 },
		func(s *State) { s.Catan.Two.KnightExchanged = true },
		func(s *State) { s.Catan.Two = nil },
	} {
		next := clone(*s)
		change(&next)
		helperReject(t, &next, p, Action{Type: "catan_roll"})
	}
	ordinary, err := NewCatanFishing(3, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for kind, cost := range catanFishCosts {
		if ordinary.Catan.fishActionCost(0, kind) != cost {
			t.Fatal("ordinary price changed")
		}
	}
	ordinary.Catan.Fishing.Two = CatanTwoFishingRules
	helperReject(t, ordinary, ordinary.Turn, Action{Type: "catan_settlement", Vertex: 0})
	if _, err = NewCatanFishing(2, CatanOptions{}); err == nil {
		t.Fatal("ordinary constructor bypassed two-player rules")
	}
	for _, options := range []CatanOptions{{Helpers: true}, {FiveSix: true}, {AllHelpers: true}} {
		if _, err = NewCatanTwoFishing(2, options); err == nil {
			t.Fatal("unverified combination accepted")
		}
	}
}
