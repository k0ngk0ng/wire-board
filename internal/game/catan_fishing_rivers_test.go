package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishingRiverRestore(t *testing.T, s *State) {
	t.Helper()
	raw, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var restored State
	if e = json.Unmarshal(raw, &restored); e != nil {
		t.Fatal(e)
	}
	if e = restored.Catan.validateRivers(); e != nil {
		t.Fatal(e)
	}
	if e = restored.Catan.validateFishing(); e != nil {
		t.Fatal(e)
	}
	if e = restored.validateCatanTwo(); e != nil {
		t.Fatal(e)
	}
	if e = restored.validateCatanEventSession(); e != nil {
		t.Fatal(e)
	}
	*s = restored
}
func TestCatanFishingRiversNaturalEngine(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/knights%t/events%t", n, knights, events), func(t *testing.T) {
					s, e := newCatanFishingRivers(n, knights)
					if e != nil {
						t.Fatal(e)
					}
					if events {
						if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
							t.Fatal(e)
						}
					}
					phases := map[string]int{}
					for step := 0; step < 16000 && !s.Finished; step++ {
						p := twoFullActor(s)
						a, e := s.BotAction(p)
						if e != nil {
							t.Fatal(step, s.Phase, e)
						}
						if e = s.Apply(p, a); e != nil {
							t.Fatal(step, s.Phase, a, e)
						}
						phases[a.Type]++
						if step%83 == 0 {
							fishingRiverRestore(t, s)
							for viewer := -1; viewer < n; viewer++ {
								s.View(viewer)
							}
						}
					}
					if !s.Finished {
						t.Fatal("unfinished", s.Round, s.Phase)
					}
					t.Log("rounds", s.Round, "fish bridges", phases["catan_fish_bridge"])
				})
			}
		}
	}
}

func fishingRiverBridgeFixture(t *testing.T, n int, phase string, knights bool) (*State, int, int) {
	t.Helper()
	s, e := newCatanFishingRivers(n, knights)
	if e != nil {
		t.Fatal(e)
	}
	g := s.Catan
	p := g.StartPlayer
	s.Turn = p
	s.Phase = phase
	g.SetupStep = g.SetupLimit()
	g.TurnSerial = 1
	if g.Paired != nil {
		g.Paired.Primary = p
		g.Paired.Secondary = (p + 3) % n
		g.Paired.Second = false
	}
	if knights {
		g.CitiesKnights.RobberStart = g.Rivers.Map.Swamps[0]
	}
	if g.Two != nil && phase == "catan_turn" {
		g.Two.Rolls = []int{5, 8}
	}
	edge := -1
	for _, id := range g.Rivers.Map.Bridges {
		v := g.Edges[id].A
		if g.Vertices[v].Owner == -1 {
			edge = id
			g.Vertices[v].Owner = p
			g.Vertices[v].Level = 1
			break
		}
	}
	if edge < 0 {
		t.Fatal("no bridge fixture")
	}
	tokens, _ := newCatanFishingTokens(n)
	g.Fishing.Tokens = *tokens
	fishOwn(&g.Fishing.Tokens, p, 21, 22)
	s.catanScores()
	fishingRiverRestore(t, s)
	return s, p, edge
}
func TestCatanFishingRiversBridgePaymentAndNeutral(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		for _, phase := range []string{"catan_roll", "catan_turn"} {
			for _, knights := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/%s/%t", n, phase, knights), func(t *testing.T) {
					s, p, edge := fishingRiverBridgeFixture(t, n, phase, knights)
					g := s.Catan
					hand := slices.Clone(g.Players[p].Resources)
					gold := g.Rivers.Gold[p]
					cost := g.fishActionCost(p, "catan_fish_bridge")
					if cost != 6 {
						t.Fatal("unexpected full price", cost)
					}
					if !slices.Contains(s.catanFishLegal(p)["bridges"].([]int), edge) {
						t.Fatal("legal bridge absent")
					}
					before := clone(*s)
					if s.Apply(p, Action{Type: "catan_fish_bridge", Edge: edge, Tokens: []int{21}}) == nil || !reflect.DeepEqual(before, *s) {
						t.Fatal("underpayment mutated")
					}
					a := Action{Type: "catan_fish_bridge", Edge: edge, Tokens: []int{21, 22}}
					if err := s.Apply(p, a); err != nil {
						t.Fatal(err)
					}
					g = s.Catan
					if !g.Edges[edge].Bridge || g.Edges[edge].Owner != p || g.Rivers.Gold[p] != gold+3 || !slices.Equal(hand, g.Players[p].Resources) || len(g.Fishing.Tokens.Hands[p]) != 0 {
						t.Fatal("bridge reward/payment")
					}
					if n == 2 {
						if g.Two.Pending == nil || g.Two.Pending.Kind != "bridge" || g.Two.Pending.Resume != phase {
							t.Fatal("missing neutral bridge continuation", s.Phase)
						}
						if g.Two.Bank != 0 || sum(g.Two.Tokens) != 0 {
							t.Fatal("trade token leak")
						}
						a, e := s.BotAction(p)
						if e != nil {
							t.Fatal(e)
						}
						if e = s.Apply(p, a); e != nil {
							t.Fatal(e)
						}
						if s.Phase != phase {
							t.Fatal("did not resume", s.Phase)
						}
					} else if s.Phase != phase {
						t.Fatal("changed turn phase")
					}
					fishingRiverRestore(t, s)
				})
			}
		}
	}
}
func TestCatanFishingRiversFrameAndBaseIsolation(t *testing.T) {
	for n := 2; n <= 6; n++ {
		s, e := newCatanFishingRivers(n, false)
		if e != nil {
			t.Fatal(e)
		}
		g := s.Catan
		if len(g.Fishing.Map.Lakes) != 0 || g.Rivers.Map == nil || g.victoryTarget() != 10 {
			t.Fatal("wrong recipe")
		}
		g.Fishing.Tokens.DrawPile = slices.DeleteFunc(g.Fishing.Tokens.DrawPile, func(id int) bool { return id == catanFishBoot })
		g.Fishing.Tokens.BootOwner = 0
		if g.victoryTargetFor(0) != 11 {
			t.Fatal("boot target")
		}
		for _, mutate := range []func(*State){
			func(s *State) { s.Catan.Fishing.Rivers = "" },
			func(s *State) { s.Catan.Fishing.Map.Lakes = []catanFishingLake{{Tile: 0, Numbers: []int{2}}} },
			func(s *State) { s.Catan.Fishing.Map.Grounds[0].Edges[0] = s.Catan.Ports[0].Edge },
			func(s *State) { s.Catan.Fishing.Map.Grounds[0].Number = 12 },
		} {
			bad := clone(*s)
			mutate(&bad)
			if bad.Catan.validateFishing() == nil {
				t.Fatal("accepted invalid combined map")
			}
		}
	}
	s := fishingGame(t, 3)
	if s.Catan.fishingRivers() || len(s.Catan.Fishing.Map.Lakes) != 1 {
		t.Fatal("base fishing changed")
	}
	r, e := NewCatanRivers(3, CatanOptions{})
	if e != nil || r.Catan.Fishing != nil {
		t.Fatal("base river changed", e)
	}
}

func TestCatanFishingRiversDiscountGoldLedgerAndPrivacy(t *testing.T) {
	s, p, edge := fishingRiverBridgeFixture(t, 2, "catan_turn", false)
	g := s.Catan
	// A public extra city raises the opponent above us; hidden VP never prices fish.
	other := 1 - p
	for i := range g.Vertices {
		if g.Vertices[i].Owner == -1 {
			g.Vertices[i].Owner = other
			g.Vertices[i].Level = 2
			break
		}
	}
	s.catanScores()
	if g.fishActionCost(p, "catan_fish_bridge") != 5 {
		t.Fatal("missing trailing discount")
	}
	tokens, _ := newCatanFishingTokens(2)
	g.Fishing.Tokens = *tokens
	fishOwn(&g.Fishing.Tokens, p, 11, 21)
	gold := g.Rivers.Bank
	g.Rivers.Bank = 0
	g.Rivers.Gold[other] += gold
	if err := s.Apply(p, Action{Type: "catan_fish_bridge", Edge: edge, Tokens: []int{11, 21}}); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	if g.Rivers.GoldIssued != 3 || g.Rivers.Gold[p] != 3 || g.Rivers.Bank != 0 {
		t.Fatal("gold ledger bridge")
	}
	for _, viewer := range []int{-1, p, other} {
		v := s.View(viewer)["catan"].(map[string]any)["fishing"].(map[string]any)["tokens"].(catanFishTokensView)
		for i, hand := range v.Players {
			if i != viewer && hand.Tokens != nil {
				t.Fatal("fish faces leaked")
			}
		}
	}
	fishingRiverRestore(t, s)
}

func TestCatanFishingRiversBridgeSupplyAndImmediateVictory(t *testing.T) {
	s, p, edge := fishingRiverBridgeFixture(t, 3, "catan_turn", false)
	g := s.Catan
	count := 0
	for _, id := range g.Rivers.Map.Bridges {
		if id != edge && count < 3 {
			g.Edges[id].Owner = p
			g.Edges[id].Bridge = true
			count++
		}
	}
	if count != 3 {
		t.Fatal("bridge inventory fixture")
	}
	before := clone(*s)
	if s.Apply(p, Action{Type: "catan_fish_bridge", Edge: edge, Tokens: []int{21, 22}}) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("built fourth bridge")
	}
	s, p, edge = fishingRiverBridgeFixture(t, 3, "catan_turn", false)
	g = s.Catan
	villages := 0
	for i := range g.Vertices {
		if g.Vertices[i].Owner >= 0 {
			g.Vertices[i].Owner = -1
			g.Vertices[i].Level = 0
		}
	}
	for i := range g.Vertices {
		if villages < 4 {
			g.Vertices[i].Owner = p
			g.Vertices[i].Level = 2
			villages++
		}
	}
	at := g.Edges[edge].A
	if g.Vertices[at].Owner != p {
		g.Vertices[at].Owner = p
		g.Vertices[at].Level = 1
	} else {
		for i := range g.Vertices {
			if g.Vertices[i].Owner == -1 {
				g.Vertices[i].Owner = p
				g.Vertices[i].Level = 1
				break
			}
		}
	}
	// Nine building points minus poverty; the bridge's three gold makes us sole richest.
	s.catanScores()
	if g.Players[p].Score >= 10 {
		t.Fatal("premature win fixture")
	}
	if err := s.Apply(p, Action{Type: "catan_fish_bridge", Edge: edge, Tokens: []int{21, 22}}); err != nil {
		t.Fatal(err)
	}
	if !s.Finished || s.Catan.Players[p].Score != 10 {
		t.Fatal("bridge wealth did not win", s.Catan.Players[p].Score)
	}
}

func TestCatanFishingRiversFishProductionAndSwamp(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		s, e := newCatanFishingRivers(n, false)
		if e != nil {
			t.Fatal(e)
		}
		g := s.Catan
		ground := g.Fishing.Map.Grounds[0]
		v := ground.Vertices[1]
		g.Vertices[v].Owner = 0
		g.Vertices[v].Level = 2
		due, e := g.Fishing.Map.production(g, ground.Number)
		if e != nil {
			t.Fatal(e)
		}
		if due[0] != 2 {
			t.Fatal("city fish production", due)
		}
		g.Robber = g.Rivers.Map.Swamps[0]
		again, e := g.Fishing.Map.production(g, ground.Number)
		if e != nil || !slices.Equal(due, again) {
			t.Fatal("swamp robber blocked coastal fish", e)
		}
		if n == 2 {
			g.Vertices[v].Owner = -2
			due, e = g.Fishing.Map.production(g, ground.Number)
			if e != nil || sum(due) != 0 {
				t.Fatal("neutral received fish")
			}
		}
	}
}
