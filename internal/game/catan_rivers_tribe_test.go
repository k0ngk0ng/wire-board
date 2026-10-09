package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func TestCatanRiversTribeMaps(t *testing.T) {
	layouts := map[string]bool{}
	for _, n := range []int{3, 4} {
		for i := 0; i < 25; i++ {
			s, err := newCatanRiversTribe(n)
			if err != nil {
				t.Fatal(n, i, err)
			}
			g := s.Catan
			if len(g.Rivers.Map.Bridges) != 7 || len(g.Rivers.Map.ExtraNumbers) != 2 {
				t.Fatal("components")
			}
			for _, id := range g.Rivers.Map.Swamps {
				if !g.tribeLand(id) || !g.robberLandAllowed(id) {
					t.Fatal("swamp not legal mainland")
				}
			}
			for _, tile := range g.Tiles {
				if g.Seafarers.Islands[tile.ID] != g.Seafarers.StartIslands[0] && g.tribeLand(tile.ID) {
					t.Fatal("outer island buildable")
				}
			}
			riversSeaRestore(t, s)
			raw, _ := json.Marshal(s.Catan.Tiles)
			layouts[string(raw)] = true
		}
	}
	if len(layouts) < 10 {
		t.Fatal("map not randomized")
	}
}
func TestCatanRiversTribeNaturalEngine(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := newCatanRiversTribe(n)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				for step := 0; step < 16000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(step, s.Phase, err)
					}
					if err = s.Apply(p, a); err != nil {
						t.Fatal(step, s.Phase, a, err)
					}
					if step%97 == 0 {
						riversSeaRestore(t, s)
						riverConserved(t, s)
						s.View(-1)
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round)
				}
				riversSeaRestore(t, s)
				t.Log("round", s.Round, "winners", s.Winners)
			})
		}
	}
}
func TestCatanRiversTribeInvalidSaves(t *testing.T) {
	s, err := newCatanRiversTribe(3)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Catan){
		"extra":         func(g *Catan) { g.Rivers.Map.ExtraNumbers[0].Tile = 0 },
		"number":        func(g *Catan) { g.Tiles[13].Number = 7 },
		"river":         func(g *Catan) { g.Rivers.Map.Channels[0].Tiles[0] = 0 },
		"ports":         func(g *Catan) { g.tribe().Ports = g.tribe().Ports[1:] },
		"points":        func(g *Catan) { g.tribe().Points[0]++ },
		"cards":         func(g *Catan) { g.DevDeck = g.DevDeck[1:] },
		"tile geometry": func(g *Catan) { g.Tiles[13].Vertices[0] = -1 },
		"edge geometry": func(g *Catan) { g.Edges[0].Tiles[0] = -1 },
		"tile id":       func(g *Catan) { g.Tiles[13].ID = 999 },
		"layout":        func(g *Catan) { g.Seafarers.Variable = false },
		"swamp":         func(g *Catan) { g.Tiles[g.Rivers.Map.Swamps[0]].Number = 3 },
	} {
		b := clone(*s)
		mutate(b.Catan)
		if b.Catan.validateRivers() == nil {
			t.Fatal("accepted", name)
		}
	}
	for _, e := range s.Catan.Rivers.Map.ExtraNumbers {
		if !s.Catan.tileProduces(s.Catan.Tiles[e.Tile], e.Number) {
			t.Fatal("extra production")
		}
	}
	for _, id := range s.Catan.Rivers.Map.Bridges {
		if s.Catan.edgeTerrain(id, true) {
			t.Fatal("bridge ship")
		}
	}
	if !slices.Equal(s.Catan.Seafarers.Islands, s.Catan.findIslands()) {
		t.Fatal("islands")
	}
}

func riverTribeRewardRoute(t *testing.T, s *State, edge int) {
	t.Helper()
	g := s.Catan
	// Walk backwards from the reward until reaching a legal mainland coast.
	previous, via := map[int]int{}, map[int]int{}
	queue := []int{g.Edges[edge].A, g.Edges[edge].B}
	for _, v := range queue {
		previous[v] = -1
	}
	root := -1
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for _, tile := range g.Tiles {
			if slices.Contains(tile.Vertices, v) && g.Seafarers.Islands[tile.ID] == g.Seafarers.StartIslands[0] {
				root = v
				break
			}
		}
		if root >= 0 {
			break
		}
		for _, e := range g.Edges {
			if e.ID == edge || !g.edgeTerrain(e.ID, true) {
				continue
			}
			next := -1
			if e.A == v {
				next = e.B
			} else if e.B == v {
				next = e.A
			}
			if next < 0 {
				continue
			}
			if _, ok := previous[next]; ok {
				continue
			}
			previous[next], via[next] = v, e.ID
			queue = append(queue, next)
		}
	}
	if root < 0 {
		t.Fatal("no mainland water path")
	}
	g.Vertices[root].Owner, g.Vertices[root].Level = 0, 1
	for v := root; previous[v] >= 0; v = previous[v] {
		e := via[v]
		g.Edges[e].Owner, g.Edges[e].Ship = 0, true
	}
	if !g.canShip(0, edge) {
		t.Fatal("reward ship not buildable")
	}
}

func TestCatanRiversTribeRewardContinuation(t *testing.T) {
	for _, reward := range []string{"point", "development", "port"} {
		t.Run(reward, func(t *testing.T) {
			s, err := newCatanRiversTribe(3)
			if err != nil {
				t.Fatal(err)
			}
			g := s.Catan
			s.Turn, s.Phase = 0, "catan_turn"
			g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
			g.Robber, g.Seafarers.Pirate = g.Rivers.Map.Swamps[0], -1
			edge := g.tribe().Tokens[0]
			card := -1
			if reward == "development" {
				edge, card = g.tribe().Development[0].Edge, g.tribe().Development[0].Card
			}
			if reward == "port" {
				edge = g.tribe().Ports[0].Edge
			}
			riverTribeRewardRoute(t, s, edge)
			// A river mouth accepts a port, but remains forbidden to roads and ships.
			mouth := g.Rivers.Map.Channels[0].Outlet
			g.Vertices[g.Edges[mouth].A].Owner, g.Vertices[g.Edges[mouth].A].Level = 0, 1
			if !slices.Contains(g.tribePortEdges(0), mouth) || g.edgeTerrain(mouth, true) || g.edgeTerrain(mouth, false) {
				t.Fatal("port/route coast semantics")
			}
			helperGrant(s, 0, []int{1, 0, 1, 0, 0})
			riversSeaRestore(t, s)
			helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
			g = s.Catan
			if reward == "point" && g.tribe().Points[0] != 1 {
				t.Fatal("point reward")
			}
			if card >= 0 {
				if g.Players[0].Dev[card] != 1 || g.Players[0].NewDev[card] != 1 {
					t.Fatal("new development")
				}
				if card != 4 {
					helperReject(t, s, 0, Action{Type: "catan_dev", Card: card})
				}
			}
			if reward == "port" {
				if s.Phase != "catan_port" || g.tribe().Pending.AfterRoute == nil {
					t.Fatal("port continuation missing")
				}
				riversSeaRestore(t, s)
				helperReject(t, s, 1, Action{Type: "catan_port", Edge: mouth})
				helperApply(t, s, 0, Action{Type: "catan_port", Edge: mouth})
				g = s.Catan
				if len(g.tribe().HeldPorts[0]) != 0 || !slices.ContainsFunc(g.Ports, func(p CatanPort) bool { return p.Edge == mouth }) {
					t.Fatal("port placement")
				}
			}
			if s.Phase != "catan_turn" {
				t.Fatal("did not resume turn", s.Phase)
			}
			before, _ := json.Marshal(g.tribe())
			if err = s.catanCollectTribe(0, edge); err != nil {
				t.Fatal(err)
			}
			after, _ := json.Marshal(g.tribe())
			if string(before) != string(after) {
				t.Fatal("reward collected twice")
			}
			riversSeaRestore(t, s)
			if err = s.EliminateCatan(0); err != nil {
				t.Fatal(err)
			}
			riversSeaRestore(t, s)
			riverConserved(t, s)
		})
	}
}

func TestCatanRiversTribeExtraProductionAndPrivacy(t *testing.T) {
	s, err := newCatanRiversTribe(4)
	if err != nil {
		t.Fatal(err)
	}
	for _, extra := range s.Catan.Rivers.Map.ExtraNumbers {
		for _, blocked := range []bool{false, true} {
			b := clone(*s)
			g := b.Catan
			g.SetupStep = g.SetupLimit()
			b.Phase = "catan_turn"
			g.Robber = g.Rivers.Map.Swamps[0]
			if blocked {
				g.Robber = extra.Tile
			}
			tile := g.Tiles[extra.Tile]
			g.Vertices[tile.Vertices[0]].Owner, g.Vertices[tile.Vertices[0]].Level = 0, 2
			if err = b.catanRollProduction(extra.Number); err != nil {
				t.Fatal(err)
			}
			want := 2
			if blocked {
				want = 0
			}
			if g.Players[0].Resources[tile.Resource] != want {
				t.Fatal("production / robber", extra, blocked)
			}
			riversSeaRestore(t, &b)
		}
	}
	for _, viewer := range []int{-1, 0, 1, 2, 3} {
		raw, _ := json.Marshal(s.View(viewer))
		var view map[string]any
		if err = json.Unmarshal(raw, &view); err != nil {
			t.Fatal(err)
		}
		g := view["catan"].(map[string]any)
		if g["devDeck"] != nil {
			t.Fatal("deck leaked")
		}
		tr := g["seafarers"].(map[string]any)["tribe"].(map[string]any)
		for _, card := range tr["development"].([]any) {
			if card.(map[string]any)["card"] != nil {
				t.Fatal("reward identity leaked")
			}
		}
	}
}
