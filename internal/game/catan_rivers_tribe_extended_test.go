package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func TestCatanRiversTribeExtendedNatural(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := newCatanRiversTribeExtended(n)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				for step := 0; step < 24000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(step, err)
					}
					if err = s.Apply(p, a); err != nil {
						t.Fatal(step, s.Phase, a, err)
					}
					if step%131 == 0 {
						riversSeaRestore(t, s)
						riverConserved(t, s)
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round)
				}
				riversSeaRestore(t, s)
				t.Log("round", s.Round)
			})
		}
	}
}

func TestCatanRiversTribeExtendedRewardContinuation(t *testing.T) {
	for _, reward := range []string{"point", "development", "port"} {
		t.Run(reward, func(t *testing.T) {
			s, err := newCatanRiversTribeExtended(6)
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

func TestCatanRiversTribeExtendedCorruption(t *testing.T) {
	s, err := newCatanRiversTribeExtended(6)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Catan.tribe().Tokens) != 10 || len(s.Catan.tribe().Ports) != 8 || len(s.Catan.tribe().Development) != 6 {
		t.Fatal("reward supply")
	}
	for name, mutate := range map[string]func(*Catan){
		"marker": func(g *Catan) { g.Rivers.SeaLayout = "unknown" }, "terrain": func(g *Catan) { g.Tiles[20].Resource = 0 },
		"number": func(g *Catan) { g.Rivers.Map.ExtraNumbers[0].Number = 7 }, "river": func(g *Catan) { g.Rivers.Map.Channels[0].Tiles[0] = 0 },
		"card": func(g *Catan) { g.DevDeck = g.DevDeck[1:] }, "token": func(g *Catan) { g.tribe().Tokens = g.tribe().Tokens[1:] },
		"port": func(g *Catan) { g.tribe().Ports = g.tribe().Ports[1:] }, "pairing": func(g *Catan) { g.Paired = nil },
	} {
		b := clone(*s)
		mutate(b.Catan)
		if b.Catan.validateRivers() == nil {
			t.Fatal("accepted", name)
		}
	}
}
