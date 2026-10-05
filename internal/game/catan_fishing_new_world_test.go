package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishWorldGame(t *testing.T, n int) *State {
	t.Helper()
	layout, err := GenerateCatanNewWorldMap(n)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewCatanFishingNewWorld(n, CatanOptions{}, layout)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Catan.NewWorldMap(), layout) {
		t.Fatal("approved layout changed")
	}
	return s
}

func finishFishWorldLayout(t *testing.T, s *State) {
	t.Helper()
	for s.Phase == "catan_world_ports" || s.Phase == "catan_world_fish" {
		a, err := s.BotAction(s.Turn)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, s.Turn, a)
	}
}

func TestCatanFishingNewWorldSetupOrderRestoreAndPrivacy(t *testing.T) {
	for _, n := range []int{3, 4} {
		for sample := 0; sample < 8; sample++ {
			s := fishWorldGame(t, n)
			first := (sample + 1) % n
			s.Turn, s.Catan.StartPlayer = first, first
			layout := s.Catan.NewWorldMap()
			for index := 0; index < 16; index++ {
				g := s.Catan
				step, phase := index, "catan_world_ports"
				if index >= 10 {
					step, phase = index-10, "catan_world_fish"
				}
				if s.Phase != phase || s.Turn != (first+step)%n || s.CatanPendingActor() != s.Turn || g.SetupStep != 0 {
					t.Fatal("setup handoff/order", index, s.Phase, s.Turn)
				}
				for _, viewer := range []int{-1, (s.Turn + 1) % n, s.Turn} {
					view := s.View(viewer)["catan"].(map[string]any)
					fish := view["fishing"].(map[string]any)
					setup := fish["worldSetup"].(map[string]any)
					if _, leaked := setup["numbers"]; leaked {
						t.Fatal("hidden ground order leaked")
					}
					current, exposed := setup["current"]
					if exposed != (index >= 10) || exposed && current != g.Fishing.WorldSetup.Numbers[index-10] {
						t.Fatal("current ground exposure")
					}
					legal := view["legal"].(map[string][]int)
					if (len(legal["fishGrounds"]) > 0) != (index >= 10 && viewer == s.Turn) || len(legal["settlements"]) > 0 {
						t.Fatal("private legal hints or premature building")
					}
				}
				before, _ := json.Marshal(s.View(s.Turn))
				// Alter only unseen order. Current and already placed faces stay.
				at := g.Fishing.WorldSetup.Index
				if index >= 10 {
					at++
				}
				slices.Reverse(g.Fishing.WorldSetup.Numbers[at:])
				after, _ := json.Marshal(s.View(s.Turn))
				if string(before) != string(after) {
					t.Fatal("future ground order influences public view/legal choices")
				}
				a, err := s.BotAction(s.Turn)
				if err != nil {
					t.Fatal(err)
				}
				helperReject(t, s, (s.Turn+1)%n, a)
				helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
				helperReject(t, s, s.Turn, Action{Type: "catan_world_fish", Vertex: -1})
				if index%2 == 0 {
					s.AutoCatanPending()
				} else {
					helperApply(t, s, s.Turn, a)
				}
				g = s.Catan
				if g.SetupStep != 0 || g.Fishing.WorldSetup.Index != max(0, index-9) || g.newWorld().Index != min(10, index+1) {
					t.Fatal("timeout or move skipped a piece")
				}
				for _, seat := range g.Players {
					if sum(seat.Resources) != 0 || seat.Helper != nil {
						t.Fatal("layout granted starting resources or helper")
					}
				}
				for _, v := range g.Vertices {
					if v.Level != 0 {
						t.Fatal("layout placed a building")
					}
				}
				if !reflect.DeepEqual(g.NewWorldMap(), layout) || g.validateFishing() != nil {
					t.Fatal("layout changed or invalid fishing state")
				}
				restored := clone(*s)
				if !reflect.DeepEqual(restored, *s) {
					t.Fatal("roundtrip during layout")
				}
				s = &restored
			}
			if s.Phase != "catan_setup_settlement" || s.Turn != first || len(s.Catan.Fishing.Map.Grounds) != 6 {
				t.Fatal("did not start first settlement after six grounds")
			}
		}
	}
}

func TestCatanFishingNewWorldRejectsCorruptStateAndUnsupportedOptions(t *testing.T) {
	layout, err := GenerateCatanNewWorldMap(3)
	if err != nil {
		t.Fatal(err)
	}
	for _, options := range []CatanOptions{{Helpers: true}, {AllHelpers: true}, {FiveSix: true}} {
		if _, err := NewCatanFishingNewWorld(3, options, layout); err == nil {
			t.Fatal("unsupported combination accepted", options)
		}
	}
	for _, n := range []int{2, 5, 6} {
		if _, err := NewCatanFishingNewWorld(n, CatanOptions{FiveSix: n > 4}, layout); err == nil {
			t.Fatal("unverified player count accepted")
		}
	}
	if _, err := NewCatanFishingNewWorld(3, CatanOptions{}, nil); err == nil {
		t.Fatal("missing approved map silently generated")
	}
	for _, change := range []func(*Catan){
		func(g *Catan) { g.Fishing.WorldSetup.Index = -1 },
		func(g *Catan) { g.Fishing.WorldSetup.Numbers[0] = 7 },
		func(g *Catan) { g.Fishing.WorldSetup = nil },
		func(g *Catan) { g.Fishing.Map.Lakes = []catanFishingLake{{Tile: 0}} },
		func(g *Catan) { g.Fishing.Map.Grounds[0].Vertices[1] = -1 },
		func(g *Catan) { g.Fishing.Map.Grounds[0].Number = 7 },
		func(g *Catan) { g.Fishing.Map.Grounds[1] = g.Fishing.Map.Grounds[0] },
		func(g *Catan) { g.Ports[0].Edge = g.Fishing.Map.Grounds[0].Edges[0] },
		func(g *Catan) { g.newWorld().Index-- },
	} {
		s := fishWorldGame(t, 3)
		finishFishWorldLayout(t, s)
		change(s.Catan)
		if s.Catan.validateFishing() == nil {
			t.Fatal("corrupt fishing layout accepted")
		}
		helperReject(t, s, s.Turn, Action{Type: "catan_settlement", Vertex: 0})
	}
}

func TestCatanFishingNewWorldStartingEntitlementAndPirate(t *testing.T) {
	s, err := NewCatanFishingNewWorld(4, CatanOptions{}, fishWorldCompactLayout())
	if err != nil {
		t.Fatal(err)
	}
	for s.Phase == "catan_world_ports" {
		helperApply(t, s, s.Turn, Action{Type: "catan_world_port", Edge: s.Catan.worldPortEdges()[0]})
	}
	for s.Phase == "catan_world_fish" {
		coasts := s.Catan.worldFishCoasts()
		chosen := coasts[0]
		for _, c := range coasts {
			if c.SeaTile >= 0 {
				chosen = c
				break
			}
		}
		helperApply(t, s, s.Turn, Action{Type: "catan_world_fish", Vertex: chosen.Vertices[1]})
	}
	startingDraws := 0
	for s.Catan.setup() {
		g, actor := s.Catan, s.Turn
		var a Action
		var err error
		// Prefer an available ground on the second settlement to exercise fish.
		a, err = s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		if s.Phase == "catan_setup_settlement" && g.SetupStep >= 4 {
			for _, ground := range g.Fishing.Map.Grounds {
				for _, v := range ground.Vertices {
					if g.canSettlement(actor, v, true) {
						a.Vertex = v
						break
					}
				}
			}
		}
		before := len(g.Fishing.Tokens.Hands[actor])
		pileBefore, wantFish := len(g.Fishing.Tokens.DrawPile), 0
		if a.Type == "catan_settlement" && g.SetupStep >= 4 {
			for _, ground := range g.Fishing.Map.Grounds {
				if slices.Contains(ground.Vertices[:], a.Vertex) {
					wantFish = 1
				}
			}
		}
		helperApply(t, s, actor, a)
		g = s.Catan
		if pileBefore-len(g.Fishing.Tokens.DrawPile) != wantFish {
			t.Fatal("second-settlement fish entitlement missing or duplicated")
		}
		startingDraws += wantFish
		if g.SetupStep < 4 && len(g.Fishing.Tokens.Hands[actor]) > 0 {
			t.Fatal("first settlement awarded fish")
		}
		if len(g.Fishing.Tokens.Hands[actor])-before > 1 {
			t.Fatal("starting settlement got multiple fish")
		}
	}
	if startingDraws == 0 {
		t.Fatal("fixture failed to exercise starting fish draw")
	}
	g := s.Catan
	for _, started := range g.Fishing.Started {
		if !started {
			t.Fatal("second-settlement entitlement not recorded")
		}
	}
	// A ground on a sea hex is blocked only by the pirate on that hex.
	for _, ground := range g.Fishing.Map.Grounds {
		if ground.SeaTile == nil {
			continue
		}
		for i := range g.Vertices {
			g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
		}
		g.Vertices[ground.Vertices[0]].Owner, g.Vertices[ground.Vertices[0]].Level = 0, 1
		g.Seafarers.Pirate = -1
		due, err := g.Fishing.Map.production(g, ground.Number)
		if err != nil || due[0] != 1 {
			t.Fatal("unblocked fishing production", due, err)
		}
		g.Seafarers.Pirate = *ground.SeaTile
		due, err = g.Fishing.Map.production(g, ground.Number)
		if err != nil || due[0] != 0 {
			t.Fatal("pirate failed to block ground", due, err)
		}
		return
	}
	t.Fatal("fixture has no inner-sea fishing ground")
}

func TestCatanFishingNewWorldBotsComplete(t *testing.T) {
	for _, n := range []int{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := fishWorldGame(t, n)
			paid := 0
			for step := 0; step < 10000 && !s.Finished; step++ {
				actor := s.Turn
				if p := s.CatanPendingActor(); p >= 0 {
					actor = p
				} else if s.Phase == "catan_discard" {
					for p, count := range s.Catan.DiscardDue {
						if count > 0 {
							actor = p
							break
						}
					}
				}
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatal(step, s.Phase, err)
				}
				if _, ok := catanFishCosts[a.Type]; ok {
					paid++
				}
				helperApply(t, s, actor, a)
				g := s.Catan
				if err := g.validateFishing(); err != nil {
					t.Fatal(err)
				}
				fleetSupply(t, g)
				cards := len(g.DevDeck) + len(g.DevDiscard)
				for p, seat := range g.Players {
					cards += sum(seat.Dev)
					roads, villages, cities := g.pieces(p)
					if roads > 15 || villages > 5 || cities > 4 || g.shipCount(p) > 15 {
						t.Fatal("piece supply")
					}
				}
				if cards != 25 {
					t.Fatal("development supply")
				}
				if step%31 == 0 {
					copy := clone(*s)
					s = &copy
				}
			}
			if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < s.Catan.victoryTargetFor(s.Winners[0]) {
				t.Fatal("new world fishing did not finish")
			}
			t.Logf("rounds=%d fish-actions=%d score=%d", s.Round, paid, s.Catan.Players[s.Winners[0]].Score)
		})
	}
}

// Deliberately compact approved geometry makes later port choices compete with
// fish corners. Keep this deterministic: hidden component order is irrelevant.
func fishWorldCompactLayout() *CatanNewWorldMap {
	layout := &CatanNewWorldMap{Hexes: make([]CatanNewWorldHex, 42)}
	terrain := []int{}
	for resource, count := range []int{5, 4, 5, 5, 4} {
		for range count {
			terrain = append(terrain, resource)
		}
	}
	numbers := []int{6, 2, 2, 3, 3, 3, 4, 4, 4, 5, 5, 5, 9, 9, 9, 10, 10, 10, 11, 11, 11, 12, 12}
	for i := range layout.Hexes {
		layout.Hexes[i].Resource = CatanSea
		if i < 23 {
			layout.Hexes[i] = CatanNewWorldHex{terrain[i], numbers[i]}
		}
	}
	return layout
}

func TestCatanFishingNewWorldPortCannotStrandRemainingGrounds(t *testing.T) {
	layout := fishWorldCompactLayout()
	s, err := NewCatanFishingNewWorld(3, CatanOptions{}, layout)
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range []int{64, 2, 4, 9, 14, 19, 25, 28} {
		helperApply(t, s, s.Turn, Action{Type: "catan_world_port", Edge: edge})
	}
	raw := *s.Catan
	raw.Fishing = nil
	if !slices.Contains(raw.worldPortEdges(), 46) || slices.Contains(s.Catan.worldPortEdges(), 46) {
		t.Fatal("capacity-losing port was not distinguished from ordinary port legality")
	}
	helperReject(t, s, s.Turn, Action{Type: "catan_world_port", Edge: 46})
	finishFishWorldLayout(t, s)
	if !reflect.DeepEqual(s.Catan.NewWorldMap(), layout) || s.Catan.validateFishing() != nil {
		t.Fatal("map changed or safe alternatives failed to complete setup")
	}
}
