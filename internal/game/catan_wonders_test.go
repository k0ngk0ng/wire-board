package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func wondersGame(t *testing.T, n int, setup bool) *State {
	t.Helper()
	s, err := NewCatanWonders(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		t.Fatal(err)
	}
	if !setup {
		s.Catan.SetupStep = s.Catan.SetupLimit()
		s.Turn, s.Phase = 0, "catan_turn"
	}
	return s
}

func TestCatanWondersOfficialMaps(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := wondersGame(t, n, true)
			g, w := s.Catan, s.Catan.wonders()
			terrain, numbers := make([]int, 8), make([]int, 13)
			islands := map[int]int{}
			for _, tile := range g.Tiles {
				terrain[tile.Resource]++
				if tile.Number > 0 {
					numbers[tile.Number]++
				}
				if id := g.Seafarers.Islands[tile.ID]; id >= 0 {
					islands[id]++
				}
			}
			wantTerrain := []int{5, 5, 5, 5, 5, 3, 19, 2}
			wantNumbers := []int{0, 0, 2, 3, 3, 3, 3, 0, 3, 3, 3, 3, 1}
			wantIslands := []int{2, 3, 25}
			cards, markers, blocked, ports := 5, 7, 11, 9
			if n > 4 {
				wantTerrain = []int{7, 6, 7, 6, 6, 4, 24, 3}
				wantNumbers = []int{0, 0, 2, 3, 4, 4, 4, 0, 4, 4, 4, 4, 2}
				wantIslands = []int{1, 1, 2, 35}
				cards, markers, blocked, ports = 7, 9, 17, 11
			}
			if !reflect.DeepEqual(terrain, wantTerrain) || !reflect.DeepEqual(numbers, wantNumbers) {
				t.Fatal("printed component counts", terrain, numbers)
			}
			sizes := []int{}
			for _, size := range islands {
				sizes = append(sizes, size)
			}
			slices.Sort(sizes)
			if !reflect.DeepEqual(sizes, wantIslands) {
				t.Fatal("printed island geometry", sizes)
			}
			if len(w.Cards) != cards || len(w.Markers) != markers || len(w.SetupBlocked) != blocked || len(g.Ports) != ports {
				t.Fatal("scenario component counts", w, len(g.Ports))
			}
			if g.Tiles[g.Robber].Resource != CatanDesert || g.Seafarers.Pirate != -1 || g.pirateAllowed(0) || len(g.pirateBotChoices(0)) > 0 || g.Seafarers.IslandBonus != 1 {
				t.Fatal("robber, pirate or exploration rules")
			}
			seen := map[int]bool{}
			for _, id := range w.SetupBlocked {
				if seen[id] || g.canSettlement(0, id, true) || !g.landVertex(id) {
					t.Fatal("invalid/duplicate setup exclusion", id)
				}
				seen[id] = true
			}
			for _, v := range g.Vertices {
				want := g.landVertex(v.ID) && slices.Contains(g.Seafarers.StartIslands, g.islandAt(v.ID)) && !seen[v.ID]
				if g.canSettlement(0, v.ID, true) != want {
					t.Fatal("setup location", v.ID, want)
				}
			}
			portCounts := map[int]int{}
			occupied := map[int]bool{}
			for _, p := range g.Ports {
				e := g.Edges[p.Edge]
				if !g.edgeTerrain(p.Edge, true) || !g.edgeTerrain(p.Edge, false) || occupied[e.A] || occupied[e.B] {
					t.Fatal("port not coastal or shares an intersection", p)
				}
				occupied[e.A], occupied[e.B] = true, true
				portCounts[p.Resource]++
			}
			wantPorts := map[int]int{-1: 4, 0: 1, 1: 1, 2: 1, 3: 1, 4: 1}
			if n > 4 {
				wantPorts[-1], wantPorts[2] = 5, 2
			}
			if !reflect.DeepEqual(portCounts, wantPorts) {
				t.Fatal("port inventory", portCounts)
			}
			g.SetupStep = g.SetupLimit()
			for _, id := range w.SetupBlocked {
				if !g.canSettlement(0, id, true) {
					t.Fatal("setup exclusion survived into ordinary play", id)
				}
			}
		})
	}
}

func TestCatanWondersClaimOwnershipAndRepeatedBuilding(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for id := 0; id < 7; id++ {
			if n < 5 && id >= 5 {
				continue
			}
			t.Run(fmt.Sprintf("%d/%d", n, id), func(t *testing.T) {
				s := wondersGame(t, n, false)
				g := s.Catan
				// Establish the exact printed claim requirement, independent
				// of cost/ownership tests below.
				switch id {
				case 0:
					g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
					g.Players[0].Score = 6
				case 1:
					e := g.Edges[g.Ports[0].Edge]
					g.Vertices[e.A].Owner, g.Vertices[e.A].Level = 0, 2
					for side := 0; side < 5; side++ {
						tile := g.Tiles[0]
						for i, route := range g.Edges {
							if (route.A == tile.Vertices[side] && route.B == tile.Vertices[(side+1)%6]) || (route.B == tile.Vertices[side] && route.A == tile.Vertices[(side+1)%6]) {
								g.Edges[i].Owner = 0
							}
						}
					}
				case 2, 6:
					g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
					g.Vertices[6].Owner, g.Vertices[6].Level = 0, 2
				default:
					for _, m := range g.wonders().Markers {
						if m.Card == id {
							g.Vertices[m.Vertex].Owner, g.Vertices[m.Vertex].Level = 0, 1
							break
						}
					}
				}
				if !g.wonderClaimable(0, id) {
					t.Fatal("printed condition did not qualify")
				}
				helperReject(t, s, 1, Action{Type: "catan_wonder_claim", Card: id})
				helperApply(t, s, 0, Action{Type: "catan_wonder_claim", Card: id})
				if s.Catan.wonders().Cards[id].Level != 0 || sum(s.Catan.Players[0].Resources) != 0 {
					t.Fatal("claim should be free, without building a level")
				}
				helperReject(t, s, 0, Action{Type: "catan_wonder_claim", Card: (id + 1) % len(s.Catan.wonders().Cards)})
				s.Turn = 1
				helperReject(t, s, 1, Action{Type: "catan_wonder_claim", Card: id})
				helperReject(t, s, 1, Action{Type: "catan_wonder_build", Card: id})
				s.Turn = 0
				helperReject(t, s, 0, Action{Type: "catan_wonder_build", Card: id})
				// Claim persists even if the qualifying condition is lost.
				for i := range s.Catan.Vertices {
					s.Catan.Vertices[i].Owner, s.Catan.Vertices[i].Level = -1, 0
				}
				s.Catan.Players[0].Score = 2
				cost := catanWonderRules[id].Cost
				if sum(cost[:]) != 5 {
					t.Fatal("each level must cost exactly five resources")
				}
				for level := 1; level <= 4; level++ {
					helperGrant(s, 0, cost[:])
					helperApply(t, s, 0, Action{Type: "catan_wonder_build", Card: id})
					if s.Catan.wonders().Cards[id].Level != level || sum(s.Catan.Players[0].Resources) != 0 || s.Finished != (level == 4) {
						t.Fatal("level payment or immediate completion victory", level)
					}
					fleetSupply(t, s.Catan)
					copy := clone(*s)
					if !reflect.DeepEqual(*s, copy) {
						t.Fatal("wonder save roundtrip")
					}
					s = &copy
				}
				if !slices.Equal(s.Winners, []int{0}) {
					t.Fatal("wrong winner")
				}
				helperReject(t, s, 0, Action{Type: "catan_wonder_build", Card: id})
			})
		}
	}
}

func TestCatanWondersVictoryAndPrivateActions(t *testing.T) {
	s := wondersGame(t, 6, false)
	g := s.Catan
	g.Players[0].Score = 10
	s.catanVictory()
	if s.Finished {
		t.Fatal("ten points alone won")
	}
	w := g.wonders()
	w.Cards[0].Owner = 0
	s.catanVictory()
	if s.Finished {
		t.Fatal("unbuilt claimed wonder won")
	}
	w.Cards[0].Level = 1
	w.Cards[1].Owner, w.Cards[1].Level = 1, 1
	s.catanVictory()
	if s.Finished {
		t.Fatal("tied wonders won")
	}
	w.Cards[0].Level = 2
	s.Turn = 1
	s.catanVictory()
	if s.Finished {
		t.Fatal("off-turn victory")
	}
	s.Turn = 0
	g.Players[0].Score = 9
	s.catanVictory()
	if s.Finished {
		t.Fatal("leading wonder below ten points won")
	}
	cost := catanWonderRules[0].Cost
	helperGrant(s, 0, cost[:])
	for _, viewer := range []int{-1, 0, 1} {
		v := s.View(viewer)["catan"].(map[string]any)
		if (len(v["wonderBuilds"].([]int)) > 0) != (viewer == 0) || len(v["wonderClaims"].([]int)) != 0 {
			t.Fatal("wonder actions exposed to wrong viewer", viewer)
		}
		if viewer != 0 {
			p := v["players"].([]any)[0].(map[string]any)
			if _, ok := p["resources"]; ok {
				t.Fatal("private resources leaked")
			}
		}
	}
	for _, phase := range []string{"catan_roll", "catan_roads", "catan_robber"} {
		s.Phase = phase
		helperReject(t, s, 0, Action{Type: "catan_wonder_build", Card: 0})
		if len(s.View(0)["catan"].(map[string]any)["wonderBuilds"].([]int)) > 0 {
			t.Fatal("wonder action in wrong phase")
		}
	}
	s.Phase = "catan_robber"
	helperReject(t, s, 0, Action{Type: "catan_pirate", Tile: 1})
	if len(s.View(0)["catan"].(map[string]any)["legal"].(map[string][]int)["pirate"]) > 0 {
		t.Fatal("pirate in wonders")
	}
	s.Phase = "catan_turn"
	g = s.Catan
	g.Players[0].Score = 10
	g.Paired = &CatanPairedTurn{Primary: 3, Secondary: 0, Second: true}
	s.catanVictory()
	if !s.Finished || !slices.Equal(s.Winners, []int{0}) {
		t.Fatal("second paired player cannot win with leading wonder")
	}
}

func TestCatanWondersIslandPointsArePersonalAndOnce(t *testing.T) {
	s := wondersGame(t, 4, false)
	g := s.Catan
	home := g.Seafarers.StartIslands[0]
	for player := 0; player < 2; player++ {
		g.Seafarers.Seats[player].HomeIslands = []int{home}
		for _, tile := range g.Tiles {
			if tile.Resource != CatanSea {
				s.catanSettleIsland(player, tile.Vertices[4], false)
				s.catanSettleIsland(player, tile.Vertices[4], false)
			}
		}
		if g.Seafarers.Seats[player].IslandPoints != 2 {
			t.Fatal("each player earns one per small island, once", g.Seafarers.Seats)
		}
	}
}

func TestCatanWondersClaimRequirementsCannotBeBypassed(t *testing.T) {
	s := wondersGame(t, 6, false)
	for id := range catanWonderRules {
		helperReject(t, s, 0, Action{Type: "catan_wonder_claim", Card: id})
	}
	helperReject(t, s, 0, Action{Type: "catan_wonder_claim", Card: -1})
	helperReject(t, s, 0, Action{Type: "catan_wonder_claim", Card: 7})
	g := s.Catan
	g.Players[0].Score = 6
	if g.wonderClaimable(0, 0) {
		t.Fatal("castle without a city")
	}
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.Players[0].Score = 5
	if g.wonderClaimable(0, 0) || g.wonderClaimable(0, 2) || g.wonderClaimable(0, 6) {
		t.Fatal("castle below six points or theater/library with only one city")
	}
	g.Players[0].Score = 6
	if !g.wonderClaimable(0, 0) {
		t.Fatal("castle exact minimum")
	}
	port := g.Edges[g.Ports[0].Edge]
	g.Vertices[port.A].Owner, g.Vertices[port.A].Level = 0, 2
	if g.wonderClaimable(0, 1) {
		t.Fatal("monument without five-segment route")
	}
	// A continuous five-edge coastal ship line is a valid trade route;
	// replacing a middle ship with a road breaks continuity without a building.
	tile := g.Tiles[0]
	line := []int{}
	for side := 0; side < 5; side++ {
		a, b := tile.Vertices[side], tile.Vertices[(side+1)%6]
		for i, e := range g.Edges {
			if (e.A == a && e.B == b) || (e.A == b && e.B == a) {
				g.Edges[i].Owner, g.Edges[i].Ship = 0, true
				line = append(line, i)
			}
		}
	}
	if !g.wonderClaimable(0, 1) {
		t.Fatal("five ships do not qualify")
	}
	g.Edges[line[2]].Ship = false
	if g.wonderClaimable(0, 1) {
		t.Fatal("disconnected road/ship chain qualifies")
	}
	g.Edges[line[2]].Ship = true
	g.Vertices[tile.Vertices[2]].Owner, g.Vertices[tile.Vertices[2]].Level = 1, 1
	if g.wonderClaimable(0, 1) {
		t.Fatal("opponent-interrupted trade route qualifies")
	}
	for _, marker := range g.wonders().Markers {
		v := &g.Vertices[marker.Vertex]
		v.Owner, v.Level = 1, 1
		if g.wonderClaimable(0, marker.Card) {
			t.Fatal("opponent's marker building qualifies")
		}
		v.Owner = 0
		if !g.wonderClaimable(0, marker.Card) {
			t.Fatal("own marker building does not qualify")
		}
		v.Owner, v.Level = -1, 0
	}
}

func TestCatanWondersBotsTradeForTheirWonder(t *testing.T) {
	for _, helper := range []bool{false, true} {
		s := wondersGame(t, 3, false)
		g := s.Catan
		g.wonders().Cards[3].Owner = 0
		g.wonders().Cards[3].Level = 3
		resources := []int{3, 0, 1, 0, 4}
		if helper {
			g.Options.Helpers, g.Options.AllHelpers = true, true
			g.TurnSerial = 2
			g.Players[0].Helper = &CatanHelperSeat{ID: 9}
			g.HelperDisplay = []int{1, 2, 3}
			resources[4] = 2
		}
		helperGrant(s, 0, resources)
		a, err := s.BotAction(0)
		want := "catan_bank"
		if helper {
			want = "catan_helper"
		}
		if err != nil || a.Type != want || len(a.Take) != 5 || a.Take[3] != 1 {
			t.Fatal("bot did not trade for missing wonder resource", helper, a, err)
		}
		// Alter only hidden opponent information and deck order.
		g.Players[1].Resources[1], g.Players[1].Resources[4] = 2, 5
		slices.Reverse(g.DevDeck)
		other, err := s.BotAction(0)
		if err != nil || !reflect.DeepEqual(a, other) {
			t.Fatal("wonder strategy depends on hidden information")
		}
		helperApply(t, s, 0, a)
		if helper {
			helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
		}
		a, err = s.BotAction(0)
		if err != nil || a.Type != "catan_wonder_build" || a.Card != 3 {
			t.Fatal("bot did not finish affordable level four", a, err)
		}
		helperApply(t, s, 0, a)
		if !s.Finished {
			t.Fatal("bot completion did not win")
		}
	}
}

func TestCatanWondersInitialRobberChoice(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, mode := range []string{"manual", "timeout", "bot"} {
			for _, variable := range []bool{false, true} {
				if variable && n > 4 {
					continue
				}
				t.Run(fmt.Sprintf("%d/%s/variable=%v", n, mode, variable), func(t *testing.T) {
					s := wondersGame(t, n, true)
					s.Turn, s.Catan.StartPlayer = n-1, n-1
					if variable {
						if err := s.randomizeCatanSeafarersMap(); err != nil {
							t.Fatal(err)
						}
					}
					candidates := s.Catan.wonderStartTiles()
					want := 3
					if n > 4 {
						want = 4
					}
					if len(candidates) != want || s.Phase != "catan_wonders_start" || s.CatanPendingActor() != n-1 {
						t.Fatal("missing desert choice", candidates, s.Phase)
					}
					for viewer := -1; viewer < n; viewer++ {
						legal := s.View(viewer)["catan"].(map[string]any)["legal"].(map[string][]int)
						if (len(legal["robber"]) == want) != (viewer == n-1) || len(legal["settlements"]) > 0 || len(legal["pirate"]) > 0 {
							t.Fatal("wrong initial hints", viewer, legal)
						}
					}
					helperReject(t, s, 0, Action{Type: "catan_wonders_start", Tile: candidates[0]})
					helperReject(t, s, n-1, Action{Type: "catan_wonders_start", Tile: -1})
					helperReject(t, s, n-1, Action{Type: "catan_wonders_start", Tile: 0}) // gold, not desert
					helperReject(t, s, n-1, Action{Type: "catan_settlement", Vertex: 0})
					if variable {
						before := clone(*s)
						if s.randomizeCatanSeafarersMap() == nil || !reflect.DeepEqual(*s, before) {
							t.Fatal("repeated shuffle accepted")
						}
					}
					restored := clone(*s)
					s = &restored
					chosen := candidates[0]
					if chosen == s.Catan.Robber {
						chosen = candidates[1]
					}
					switch mode {
					case "manual":
						helperApply(t, s, n-1, Action{Type: "catan_wonders_start", Tile: chosen})
					case "timeout":
						chosen = s.Catan.Robber
						s.AutoCatanPending()
					case "bot":
						chosen = s.Catan.Robber
						a, err := s.BotAction(n - 1)
						if err != nil {
							t.Fatal(err)
						}
						helperApply(t, s, n-1, a)
					}
					if s.Catan.Robber != chosen || s.Phase != "catan_setup_settlement" || s.Catan.SetupStep != 0 || s.Turn != n-1 || s.CatanPendingActor() != -1 {
						t.Fatal("choice skipped setup")
					}
					for _, v := range s.Catan.Vertices {
						if v.Level > 0 {
							t.Fatal("choice also placed building")
						}
					}
					helperReject(t, s, n-1, Action{Type: "catan_wonders_start", Tile: chosen})
					before := clone(*s)
					if s.randomizeCatanSeafarersMap() == nil || !reflect.DeepEqual(*s, before) {
						t.Fatal("reshuffled after confirmation")
					}
					a, err := s.BotAction(n - 1)
					if err != nil {
						t.Fatal(err)
					}
					helperApply(t, s, n-1, a)
					if s.Phase != "catan_setup_road" || s.Catan.SetupVertex < 0 {
						t.Fatal("first settlement unavailable")
					}
				})
			}
		}
	}
}
