package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishSeaGame(t *testing.T, n int, layout string) *State {
	t.Helper()
	s, err := NewCatanFishingSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "islands", Layout: layout}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func fishSeaActionGame(t *testing.T, phase string) (*State, int) {
	t.Helper()
	s := fishSeaGame(t, 3, "fixed")
	g := s.Catan
	s.Turn, s.Phase = 0, phase
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	ground := g.Fishing.Map.Grounds[0]
	g.Vertices[ground.Vertices[1]].Owner, g.Vertices[ground.Vertices[1]].Level = 0, 1
	g.Seafarers.Pirate = -1
	fishOwn(&g.Fishing.Tokens, 0, 0, 1, 11, 12, 21, 22, 23)
	s.catanScores()
	for _, e := range g.Edges {
		if g.canShip(0, e.ID) {
			return s, e.ID
		}
	}
	t.Fatal("fixture has no ship position")
	return nil, -1
}
func TestCatanFishingSeafarersSetupAndBoot(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"fixed", "variable"} {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				s := fishSeaGame(t, n, layout)
				g := s.Catan
				// Keep the boot in supply until its explicit target test below.
				fishTop(&g.Fishing.Tokens, 0, 1, 2, 3)
				if g.Robber < 0 || g.Tiles[g.Robber].Number != 12 || g.victoryTargetFor(0) != 13 || len(g.Fishing.Map.Lakes) != 0 {
					t.Fatal("scenario setup/target changed")
				}
				for step := 0; s.Phase != "catan_roll" && step < 50; step++ {
					actor := s.Turn
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					if err = s.Apply(actor, a); err != nil {
						t.Fatal(err)
					}
					restored := clone(*s)
					s = &restored
					if err := s.Catan.validateFishing(); err != nil {
						t.Fatal(err)
					}
				}
				if s.Phase != "catan_roll" {
					t.Fatal("setup did not finish")
				}
				for _, started := range s.Catan.Fishing.Started {
					if !started {
						t.Fatal("missing starting fish entitlement")
					}
				}
				g = s.Catan
				fishTop(&g.Fishing.Tokens, catanFishBoot)
				if err := g.Fishing.Tokens.beginDraw(0, append([]int{1}, make([]int, n-1)...)); err != nil {
					t.Fatal(err)
				}
				if g.victoryTargetFor(0) != 14 || g.victoryTargetFor(1) != 13 {
					t.Fatal("boot target")
				}
			})
		}
	}
	for _, n := range []int{2, 5, 6} {
		if _, err := NewCatanFishingSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "islands"}, nil); err == nil {
			t.Fatal("unverified player count accepted")
		}
	}
	for _, scenario := range []string{"fog", "shores", "pirate_islands"} {
		if _, err := NewCatanFishingSeafarers(3, CatanOptions{}, CatanSeafarersSetup{Scenario: scenario}, nil); err == nil {
			t.Fatal("unsupported recipe accepted")
		}
	}
	if _, err := NewCatanFishingSeafarers(3, CatanOptions{Helpers: true}, CatanSeafarersSetup{Scenario: "islands"}, nil); err == nil {
		t.Fatal("unverified Helpers accepted")
	}
}
func TestCatanFishingSeafarersShipAndPiratePayments(t *testing.T) {
	for _, phase := range []string{"catan_roll", "catan_turn"} {
		t.Run(phase, func(t *testing.T) {
			s, edge := fishSeaActionGame(t, phase)
			g := s.Catan
			// Pay two physical tokens worth five fish; ordinary resources stay untouched.
			before := clone(g.Bank)
			if err := s.Apply(0, Action{Type: "catan_fish_ship", Edge: edge, Tokens: []int{11, 21}}); err != nil {
				t.Fatal(err)
			}
			g = s.Catan
			if !g.Edges[edge].Ship || g.Edges[edge].Owner != 0 || !slices.Contains(g.Seafarers.BuiltShips, edge) || !slices.Equal(before, g.Bank) || sum(g.Players[0].Resources) != 0 || s.Phase != phase || g.FreeRoads != 0 {
				t.Fatal("ship effect/phase/payment")
			}
			if len(g.shipDestinations(0, edge)) > 0 {
				t.Fatal("new fish ship could move")
			}
			saved := clone(*s)
			s = &saved
			g = s.Catan
			tile := slices.IndexFunc(g.Tiles, func(t CatanTile) bool { return t.Resource == CatanSea })
			g.Seafarers.Pirate = tile
			robber := g.Robber
			if err := s.Apply(0, Action{Type: "catan_fish_pirate", Tokens: []int{22}}); err != nil {
				t.Fatal(err)
			}
			g = s.Catan
			if g.Seafarers.Pirate != -1 || g.Robber != robber || !slices.Equal(g.Fishing.Tokens.Discard, []int{11, 21, 22}) || s.Phase != phase || sum(g.Players[0].Resources) != 0 {
				t.Fatal("pirate removal stole resources/changed turn or paid token count")
			}
			fishGameReject(t, s, 0, Action{Type: "catan_fish_pirate", Tokens: []int{23}})
			fishGameReject(t, s, 0, Action{Type: "catan_fish_ship", Edge: edge, Tokens: []int{12, 23}})
			if err := g.validateFishing(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestCatanFishingSeafarersIllegalShipsAreAtomic(t *testing.T) {
	for _, change := range []func(*State, int) Action{
		func(s *State, e int) Action { return Action{Type: "catan_fish_ship", Edge: -1, Tokens: []int{11, 21}} },
		func(s *State, e int) Action { return Action{Type: "catan_fish_ship", Edge: e, Tokens: []int{21}} },
		func(s *State, e int) Action { return Action{Type: "catan_fish_ship", Edge: e, Tokens: []int{21, 21}} },
		func(s *State, e int) Action {
			s.Catan.Edges[e].Owner = 1
			return Action{Type: "catan_fish_ship", Edge: e, Tokens: []int{11, 21}}
		},
		func(s *State, e int) Action {
			for i := range s.Catan.Vertices {
				s.Catan.Vertices[i].Owner = -1
				s.Catan.Vertices[i].Level = 0
			}
			return Action{Type: "catan_fish_ship", Edge: e, Tokens: []int{11, 21}}
		},
		func(s *State, e int) Action {
			for i, count := 0, 0; i < len(s.Catan.Edges) && count < 15; i++ {
				if i != e {
					s.Catan.Edges[i].Owner = 0
					s.Catan.Edges[i].Ship = true
					count++
				}
			}
			return Action{Type: "catan_fish_ship", Edge: e, Tokens: []int{11, 21}}
		},
		func(s *State, e int) Action {
			s.Phase = "catan_roads"
			s.Catan.FreeRoads = 2
			return Action{Type: "catan_fish_ship", Edge: e, Tokens: []int{11, 21}}
		},
	} {
		s, e := fishSeaActionGame(t, "catan_turn")
		a := change(s, e)
		fishGameReject(t, s, 0, a)
	}
	s, _ := fishSeaActionGame(t, "catan_turn")
	g := s.Catan
	// Select a real ground with a sea hex, block it, then remove the pirate.
	found := false
	for _, ground := range g.Fishing.Map.Grounds {
		if ground.SeaTile == nil {
			continue
		}
		found = true
		for i := range g.Vertices {
			g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
		}
		g.Vertices[ground.Vertices[1]].Owner, g.Vertices[ground.Vertices[1]].Level = 0, 1
		g.Seafarers.Pirate = *ground.SeaTile
		fishGameReject(t, s, 0, Action{Type: "catan_fish_ship", Edge: ground.Edges[0], Tokens: []int{11, 21}})
		fishGameReject(t, s, 1, Action{Type: "catan_fish_pirate", Tokens: []int{11}})
		if !slices.Contains(s.catanFishLegal(0)["actions"].([]string), "catan_fish_pirate") {
			t.Fatal("missing legal pirate action")
		}
		due, err := g.Fishing.Map.production(g, ground.Number)
		if err != nil || sum(due) != 0 {
			t.Fatal("pirate did not block", err)
		}
		if err = s.Apply(0, Action{Type: "catan_fish_pirate", Tokens: []int{11}}); err != nil {
			t.Fatal(err)
		}
		g = s.Catan
		due, err = g.Fishing.Map.production(g, ground.Number)
		if err != nil || due[0] != 1 {
			t.Fatal("removal did not unblock fishing", err)
		}
		if !slices.Contains(s.catanFishLegal(0)["ships"].([]int), ground.Edges[0]) {
			t.Fatal("removal did not unblock ships")
		}
		break
	}
	if !found {
		t.Fatal("no sea-ground fixture")
	}
	base, p := fishActionFixture(t, 3, "catan_turn")
	fishGameReject(t, base, p, Action{Type: "catan_fish_ship", Edge: 0, Tokens: []int{11, 21}})
	fishGameReject(t, base, p, Action{Type: "catan_fish_pirate", Tokens: []int{11}})
}
func TestCatanFishingSeafarersBotAndPrivacy(t *testing.T) {
	s, _ := fishSeaActionGame(t, "catan_turn")
	g := s.Catan
	choices := s.catanFishBotChoices(0, g.seaBuildChoices(0), -1)
	ship := -1
	for i, c := range choices {
		if c.action.Type == "catan_fish_ship" {
			ship = i
			break
		}
	}
	if ship < 0 {
		t.Fatal("bot missing fish ship")
	}
	other := clone(*s)
	slices.Reverse(other.Catan.Fishing.Tokens.DrawPile)
	slices.Reverse(other.Catan.DevDeck)
	if !reflect.DeepEqual(choices, other.catanFishBotChoices(0, other.Catan.seaBuildChoices(0), -1)) {
		t.Fatal("bot reads hidden order")
	}
	if err := s.Apply(0, choices[ship].action); err != nil {
		t.Fatal(err)
	}
	for viewer := -1; viewer < 3; viewer++ {
		raw, _ := json.Marshal(s.View(viewer))
		var view map[string]any
		json.Unmarshal(raw, &view)
		f := view["catan"].(map[string]any)["fishing"].(map[string]any)
		tokens := f["tokens"].(map[string]any)
		if tokens["drawPile"] != nil {
			t.Fatal("hidden pile leaked")
		}
		legal := f["legal"].(map[string]any)
		if viewer != 0 && (len(legal["actions"].([]any)) != 0 || len(legal["ships"].([]any)) != 0) {
			t.Fatal("private affordability leaked")
		}
	}
	// Fish blockers should be removed before rolling even without any own ships.
	s, _ = fishSeaActionGame(t, "catan_roll")
	g = s.Catan
	for _, ground := range g.Fishing.Map.Grounds {
		if ground.SeaTile == nil {
			continue
		}
		for i := range g.Vertices {
			g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
		}
		g.Vertices[ground.Vertices[1]].Owner, g.Vertices[ground.Vertices[1]].Level = 0, 1
		g.Seafarers.Pirate = *ground.SeaTile
		g.Robber = -1
		a, err := s.BotAction(0)
		if err != nil || a.Type != "catan_fish_pirate" {
			t.Fatal("bot ignores fish blockade", a, err)
		}
		if err = s.Apply(0, a); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatal("no sea-ground fixture")
}
func TestCatanFishingSeafarersBotsFinishAndConserve(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"fixed", "variable"} {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				s := fishSeaGame(t, n, layout)
				paid, ships := 0, 0
				for step := 0; step < 8000 && !s.Finished; step++ {
					actor := s.Turn
					if p := s.CatanPendingActor(); p >= 0 {
						actor = p
					} else if s.Phase == "catan_discard" {
						for p, due := range s.Catan.DiscardDue {
							if due > 0 {
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
					if a.Type == "catan_fish_ship" {
						ships++
					}
					if err = s.Apply(actor, a); err != nil {
						t.Fatal(step, s.Phase, a, err)
					}
					g := s.Catan
					if err = g.validateFishing(); err != nil {
						t.Fatal(err)
					}
					fleetSupply(t, g)
					for p := range g.Players {
						roads, settlements, cities := g.pieces(p)
						if roads > 15 || settlements > 5 || cities > 4 || g.shipCount(p) > 15 {
							t.Fatal("piece inventory")
						}
					}
					if step%31 == 0 {
						restored := clone(*s)
						if !reflect.DeepEqual(*s, restored) {
							t.Fatal("restore")
						}
						s = &restored
					}
				}
				if !s.Finished || paid == 0 || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < s.Catan.victoryTargetFor(s.Winners[0]) {
					t.Fatal("incomplete combined game", s.Round, paid, ships)
				}
				t.Logf("round=%d fish-actions=%d fish-ships=%d", s.Round, paid, ships)
			})
		}
	}
}

func TestCatanFishingSeafarersProductionReplacementRestore(t *testing.T) {
	s, _ := fishSeaActionGame(t, "catan_roll")
	g := s.Catan
	for i := range g.Vertices {
		g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
	}
	for i := range g.Tiles {
		if g.Tiles[i].Resource < 5 {
			g.Tiles[i].Number = 2
		}
	}
	var ground *catanFishingGround
	for i := range g.Fishing.Map.Grounds {
		if g.Fishing.Map.Grounds[i].SeaTile != nil {
			ground = &g.Fishing.Map.Grounds[i]
			break
		}
	}
	if ground == nil {
		t.Fatal("missing sea fishing ground")
	}
	g.Vertices[ground.Vertices[1]].Owner, g.Vertices[ground.Vertices[1]].Level = 1, 2
	fishOwn(&g.Fishing.Tokens, 1, 2, 3, 4, 5, 6, 7, 8)
	g.Seafarers.Pirate = *ground.SeaTile
	g.RollID = 1
	if err := s.catanRollProduction(ground.Number); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_turn" || g.Fishing.Pending != nil || g.Fishing.LastRollID != 1 {
		t.Fatal("blocked production created response")
	}
	if err := s.Apply(0, Action{Type: "catan_fish_pirate", Tokens: []int{11}}); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	g.RollID = 2
	if err := s.catanRollProduction(ground.Number); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_fish_replace" || s.CatanPendingActor() != 1 || g.Fishing.LastRollID != 2 {
		t.Fatal("unblocked city missing replacement")
	}
	fishGameReject(t, s, 0, Action{Type: "catan_fish_keep"})
	saved := clone(*s)
	s = &saved
	if err := s.Apply(1, Action{Type: "catan_fish_keep"}); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	if s.Phase != "catan_turn" || s.Turn != 0 || s.CatanPendingActor() != -1 || len(g.Fishing.Tokens.Hands[1]) != 7 || g.Fishing.LastRollID != 2 {
		t.Fatal("replacement lost original turn")
	}
	for _, p := range g.Players {
		if sum(p.Resources) != 0 {
			t.Fatal("ordinary production repeated or fish treated as resources")
		}
	}
	fishGameReject(t, s, 1, Action{Type: "catan_fish_keep"})
}

func TestCatanFishingSeafarersShipLongestRouteVictory(t *testing.T) {
	for _, boot := range []bool{false, true} {
		t.Run(fmt.Sprint(boot), func(t *testing.T) {
			s, _ := fishSeaActionGame(t, "catan_roll")
			g := s.Catan
			start := -1
			for _, v := range g.Vertices {
				if v.Owner == 0 {
					start = v.ID
					break
				}
			}
			path := []int{}
			visited := map[int]bool{start: true}
			var extend func(int) bool
			extend = func(v int) bool {
				if len(path) == 5 {
					return true
				}
				for _, id := range g.touching(v) {
					e := g.Edges[id]
					next := e.A
					if next == v {
						next = e.B
					}
					if visited[next] || !g.edgeTerrain(id, true) {
						continue
					}
					path = append(path, id)
					visited[next] = true
					if extend(next) {
						return true
					}
					path = path[:len(path)-1]
					delete(visited, next)
				}
				return false
			}
			if !extend(start) {
				t.Fatal("no five-ship chain")
			}
			for _, id := range path[:4] {
				g.Edges[id].Owner, g.Edges[id].Ship = 0, true
			}
			// Eleven building points leave the final ship's two-point route award
			// decisive at the actual 13-point scenario target (14 with the boot).
			g.Vertices[start].Level = 2
			cities, settlements := 1, 0
			for i := range g.Vertices {
				if visited[i] {
					continue
				}
				if cities < 4 {
					g.Vertices[i].Owner, g.Vertices[i].Level = 0, 2
					cities++
				} else if settlements < 3 {
					g.Vertices[i].Owner, g.Vertices[i].Level = 0, 1
					settlements++
				}
			}
			if boot {
				fishTop(&g.Fishing.Tokens, catanFishBoot)
				if err := g.Fishing.Tokens.beginDraw(1, []int{0, 1, 0}); err != nil {
					t.Fatal(err)
				}
				g.Fishing.Tokens.BootOwner = 0
			}
			s.catanScores()
			if g.Players[0].Score != 11 || g.LongestOwner == 0 {
				t.Fatal("incorrect route fixture")
			}
			if err := s.Apply(0, Action{Type: "catan_fish_ship", Edge: path[4], Tokens: []int{11, 21}}); err != nil {
				t.Fatal(err)
			}
			g = s.Catan
			if g.LongestOwner != 0 || g.Players[0].Score != 13 || s.Finished == boot {
				t.Fatal("ship route did not score/check individual victory")
			}
		})
	}
}
