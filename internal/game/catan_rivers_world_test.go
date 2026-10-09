package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func TestCatanRiversWorldSetupAndMaps(t *testing.T) {
	layouts := map[string]bool{}
	for _, n := range []int{3, 4} {
		for repeat := 0; repeat < 25; repeat++ {
			s, err := newCatanRiversWorld(n)
			if err != nil {
				t.Fatal(n, repeat, err)
			}
			g := s.Catan
			first := s.Turn
			raw, _ := json.Marshal(g.Tiles)
			layouts[string(raw)] = true
			if g.Robber != -1 || s.Phase != "catan_world_ports" {
				t.Fatal("start")
			}
			for _, c := range g.Rivers.Map.Channels {
				if !slices.Contains(g.worldPortEdges(), c.Outlet) || g.edgeTerrain(c.Outlet, true) || g.edgeTerrain(c.Outlet, false) {
					t.Fatal("mouth port/route")
				}
			}
			for i := 0; i < 10; i++ {
				if s.Turn != (first+i)%n {
					t.Fatal("port actor")
				}
				riverReject(t, s, (s.Turn+1)%n, Action{Type: "catan_world_port", Edge: g.worldPortEdges()[0]})
				s.AutoCatanPending()
				riversSeaRestore(t, s)
				g = s.Catan
				if g.newWorld().Index != i+1 || g.SetupStep != 0 {
					t.Fatal("autoplay port step")
				}
			}
			if s.Phase != "catan_setup_settlement" || s.Turn != first || g.Robber != -1 {
				t.Fatal("setup continuation")
			}
			for i := 0; i < 4*n; i++ {
				a, err := s.BotAction(s.Turn)
				if err != nil {
					t.Fatal(err)
				}
				helperApply(t, s, s.Turn, a)
				riversSeaRestore(t, s)
			}
			if s.Phase != "catan_roll" {
				t.Fatal("setup end", s.Phase)
			}
			riverConserved(t, s)
		}
	}
	if len(layouts) < 40 {
		t.Fatal("insufficient diversity")
	}
}
func TestCatanRiversWorldNaturalEngine(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := newCatanRiversWorld(n)
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
func TestCatanRiversWorldPrivacyAndInvalidSaves(t *testing.T) {
	s, err := newCatanRiversWorld(3)
	if err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []int{-1, 0, 1, 2} {
		view := s.View(viewer)["catan"].(map[string]any)["seafarers"].(map[string]any)["newWorld"].(map[string]any)
		if view["ports"] != nil || view["current"] != s.Catan.newWorld().Ports[0] {
			t.Fatal("hidden port deck")
		}
	}
	before, _ := json.Marshal(s.View(s.Turn))
	slices.Reverse(s.Catan.newWorld().Ports[1:])
	after, _ := json.Marshal(s.View(s.Turn))
	if string(before) != string(after) {
		t.Fatal("future port order leaked")
	}
	for name, mutate := range map[string]func(*Catan){
		"tile id":  func(g *Catan) { g.Tiles[0].ID = 999 },
		"geometry": func(g *Catan) { g.Edges[0].Tiles[0] = -1 },
		"terrain":  func(g *Catan) { g.Tiles[0].Resource = 99 },
		"number":   func(g *Catan) { g.Tiles[0].Number = 7 },
		"river":    func(g *Catan) { g.Rivers.Map.Channels[0].Tiles[0] = -1 },
		"extra":    func(g *Catan) { g.Rivers.Map.ExtraNumbers = []catanFishingExtraNumber{{Tile: 0, Number: 2}} },
		"ports":    func(g *Catan) { g.newWorld().Ports[0] = 99 },
		"index":    func(g *Catan) { g.newWorld().Index++ },
		"layout":   func(g *Catan) { g.Seafarers.Layout = "fixed" },
		"pirate":   func(g *Catan) { g.Seafarers.Pirate = g.Rivers.Map.Swamps[0] },
	} {
		b := clone(*s)
		mutate(b.Catan)
		if b.Catan.validateRivers() == nil {
			t.Fatal("accepted", name)
		}
	}
}

func TestCatanRiversWorldShipsBridgesAndElimination(t *testing.T) {
	for _, kind := range []string{"ship", "bridge"} {
		t.Run(kind, func(t *testing.T) {
			s, err := newCatanRiversWorld(3)
			if err != nil {
				t.Fatal(err)
			}
			for s.Phase == "catan_world_ports" {
				s.AutoCatanPending()
			}
			g := s.Catan
			g.SetupStep = g.SetupLimit()
			s.Phase = "catan_turn"
			s.Turn = 0
			target := -1
			for _, e := range g.Edges {
				if kind == "ship" && (!g.riverEdge(e.ID) || !g.edgeTerrain(e.ID, true)) {
					continue
				}
				if kind == "bridge" && !slices.Contains(g.Rivers.Map.Bridges, e.ID) {
					continue
				}
				g.Vertices[e.A].Owner, g.Vertices[e.A].Level = 0, 1
				if kind == "bridge" || g.canShip(0, e.ID) {
					target = e.ID
					break
				}
				g.Vertices[e.A].Owner, g.Vertices[e.A].Level = -1, 0
			}
			if target < 0 {
				t.Fatal("no river route")
			}
			helperGrant(s, 0, []int{2, 3, 2, 0, 0})
			want := 1
			a := Action{Type: "catan_ship", Edge: target}
			if kind == "bridge" {
				a.Type = "catan_bridge"
				want = 3
			}
			helperApply(t, s, 0, a)
			if s.Catan.Rivers.Gold[0] != want {
				t.Fatal("river reward", kind, s.Catan.Rivers.Gold)
			}
			riversSeaRestore(t, s)
			riverConserved(t, s)
			g = s.Catan
			if kind == "ship" {
				for _, e := range g.shipDestinations(0, target) {
					if slices.Contains(g.Rivers.Map.Bridges, e) {
						t.Fatal("ship moving onto bridge")
					}
				}
			}
			if err = s.EliminateCatan(0); err != nil {
				t.Fatal(err)
			}
			riversSeaRestore(t, s)
			riverConserved(t, s)
			if s.Catan.Rivers.Gold[0] != 0 {
				t.Fatal("eliminated gold retained")
			}
		})
	}
}

func TestCatanRiversWorldSettlementAndUpgrade(t *testing.T) {
	s, err := newCatanRiversWorld(4)
	if err != nil {
		t.Fatal(err)
	}
	for s.Phase == "catan_world_ports" {
		s.AutoCatanPending()
	}
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	s.Turn = 0
	s.Phase = "catan_turn"
	target := -1
	for _, v := range g.Vertices {
		if !g.riverVertex(v.ID) {
			continue
		}
		for _, edge := range g.touching(v.ID) {
			if !g.edgeTerrain(edge, false) {
				continue
			}
			g.Edges[edge].Owner = 0
			if g.canSettlement(0, v.ID, false) {
				target = v.ID
				break
			}
			g.Edges[edge].Owner = -1
		}
		if target >= 0 {
			break
		}
	}
	if target < 0 {
		t.Fatal("missing river settlement")
	}
	// A newly reached island yields one point as in ordinary New World.
	helperGrant(s, 0, catanPrices["catan_settlement"])
	helperApply(t, s, 0, Action{Type: "catan_settlement", Vertex: target})
	if s.Catan.Rivers.Gold[0] != 1 || s.Catan.Seafarers.Seats[0].IslandPoints != 1 {
		t.Fatal("settlement rewards")
	}
	helperGrant(s, 0, catanPrices["catan_city"])
	helperApply(t, s, 0, Action{Type: "catan_city", Vertex: target})
	if s.Catan.Rivers.Gold[0] != 1 || s.Catan.Seafarers.Seats[0].IslandPoints != 1 {
		t.Fatal("upgrade duplicate rewards")
	}
	riversSeaRestore(t, s)
	riverConserved(t, s)
}
