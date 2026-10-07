package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func explorerRoadsEmptyHand(t *testing.T, s *State) {
	t.Helper()
	for r, n := range s.Catan.Players[s.Turn].Resources {
		s.Catan.Bank[r] += n
		s.Catan.Players[s.Turn].Resources[r] = 0
	}
}
func explorerRoadPlay(t *testing.T, s *State) {
	t.Helper()
	ckProgressGive(t, s, s.Turn, 7)
	explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 7})
}
func TestCatanExplorerCityFreeRoadsEmptyHandRestoreAndResume(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerCityProductionFixture(t, n)
			if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
				t.Fatal(err)
			}
			explorerRoadsEmptyHand(t, s)
			before := clone(*s.Catan)
			explorerRoadPlay(t, s)
			if s.Phase != "catan_roads" || s.Catan.FreeRoads != 2 || len(s.Catan.CitiesKnights.Players[0].Progress) != 0 {
				t.Fatal("card did not start free roads")
			}
			for _, kind := range []string{"catan_ship", "catan_explorer_ship", "catan_end", "catan_progress", "catan_skip_roads", "catan_knight_activate", "catan_trade_offer"} {
				explorerCityActionReject(t, s, 0, Action{Type: kind, Card: 4, Prompt: 1})
			}
			last := -1
			for step := 1; step <= 2; step++ {
				prior := clone(*s)
				sites := s.catanExplorerFreeRoadSites(0)
				if len(sites) == 0 || !reflect.DeepEqual(*s, prior) {
					t.Fatal("missing or mutating free-road preview")
				}
				last = sites[0]
				explorerCityActionReject(t, s, 1, Action{Type: "catan_road", Edge: last, Prompt: 1})
				explorerCityActionReject(t, s, 0, Action{Type: "catan_road", Edge: last, Prompt: 0})
				explorerCityActionReject(t, s, 0, Action{Type: "catan_road", Edge: -1, Prompt: 1})
				explorerKnightAct(t, s, Action{Type: "catan_road", Edge: last})
				if s.Catan.Edges[last].Owner != 0 || s.Catan.FreeRoads != 2-step || !slices.Equal(s.Catan.Bank, before.Bank) || sum(s.Catan.Players[0].Resources) != 0 || !reflect.DeepEqual(*s.Catan.Explorer.Fleet, *before.Explorer.Fleet) || !reflect.DeepEqual(*s.Catan.Explorer.Cargo, *before.Explorer.Cargo) || !reflect.DeepEqual(*s.Catan.Explorer.Economy, *before.Explorer.Economy) {
					t.Fatal("free road changed price, ship, cargo or production")
				}
			}
			if s.Phase != "catan_turn" || s.Catan.LongestOwner != -1 {
				t.Fatal("wrong action continuation or invented longest-road bonus")
			}
			explorerCityActionReject(t, s, 0, Action{Type: "catan_road", Edge: last, Prompt: 1})
			if sites := s.catanExplorerFreeRoadSites(0); len(sites) > 0 {
				explorerCityActionReject(t, s, 0, Action{Type: "catan_road", Edge: sites[0], Prompt: 1})
				explorerDevelopmentGrant(t, s, 0, 0, 1)
				explorerDevelopmentGrant(t, s, 0, 1, 1)
				explorerCityActionReject(t, s, 0, Action{Type: "catan_road", Edge: sites[0], Choice: "free", Prompt: 1})
				explorerKnightAct(t, s, Action{Type: "catan_road", Edge: sites[0]})
				if sum(s.Catan.Players[0].Resources) != 0 {
					t.Fatal("normal road did not resume paid cost")
				}
			}
		})
	}
}

func TestCatanExplorerCityFreeRoadsLastPieceAndNoPieces(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	reserved := s.catanExplorerFreeRoadSites(0)[0]
	g := s.Catan
	for i := range g.Edges {
		g.Edges[i].Owner = -1
	}
	count := 0
	for i := range g.Edges {
		if i != reserved && catanExplorerLandEdge(g, i) {
			g.Edges[i].Owner = 0
			count++
			if count == 14 {
				break
			}
		}
	}
	if count != 14 {
		t.Fatal("not enough controlled road components")
	}
	explorerRoadsEmptyHand(t, s)
	explorerCityRestore(t, s)
	explorerRoadPlay(t, s)
	explorerKnightAct(t, s, Action{Type: "catan_road", Edge: reserved})
	if s.Phase != "catan_turn" || s.Catan.FreeRoads != 0 {
		t.Fatal("last road piece left a stalled free phase")
	}
	before := clone(*s.Catan)
	explorerRoadPlay(t, s)
	if s.Phase != "catan_turn" || s.Catan.FreeRoads != 0 || !slices.Equal(s.Catan.Bank, before.Bank) || !reflect.DeepEqual(s.Catan.Edges, before.Edges) {
		t.Fatal("no-piece card did not finish harmlessly")
	}
	if s.Catan.knightCount(0, 1) != 0 {
		t.Fatal("roads changed knight inventory")
	}
}

func TestCatanExplorerCityFreeRoadsBlockersAndCorruptSave(t *testing.T) {
	s, v, e := explorerKnightFixture(t, 3)
	g := s.Catan
	g.Edges[e[1]].Owner = -1
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 1, Vertex: v[1], Strength: 1}}
	explorerRoadPlay(t, s)
	explorerCityActionReject(t, s, 0, Action{Type: "catan_road", Edge: e[1], Prompt: 1})
	for _, edge := range s.Catan.Edges {
		if !catanExplorerLandEdge(s.Catan, edge.ID) {
			explorerCityActionReject(t, s, 0, Action{Type: "catan_road", Edge: edge.ID, Prompt: 1})
			break
		}
	}
	for _, reason := range []string{"count", "phase", "resume", "cargo", "trade"} {
		t.Run(reason, func(t *testing.T) {
			bad := clone(*s)
			switch reason {
			case "count":
				bad.Catan.FreeRoads = 3
			case "phase":
				bad.Phase = "catan_turn"
			case "resume":
				bad.Catan.ResumePhase = "catan_roll"
			case "cargo":
				bad.Catan.Explorer.Cargo.Turn.Phase = "ended"
			case "trade":
				bad.Catan.TradeID = 1
				bad.Catan.Trade = &CatanTrade{ID: 1, From: 0, Give: []int{1, 0, 0, 0, 0, 0, 0, 0}, Take: []int{0, 1, 0, 0, 0, 0, 0, 0}, Responses: make([]int, 3)}
			}
			if err := bad.validateExplorerCityProduction(); err == nil {
				t.Fatal("corrupt free-road state restored")
			}
			explorerCityActionReject(t, &bad, 0, Action{Type: "catan_road", Edge: e[1], Prompt: 1})
		})
	}
}

func TestCatanExplorerCityResourceProgressIncludesHarborsAndShortage(t *testing.T) {
	for _, card := range []int{4, 6} {
		for _, remaining := range []int{0, 1, 19} {
			t.Run(fmt.Sprintf("card%d/bank%d", card, remaining), func(t *testing.T) {
				s := explorerCityProductionFixture(t, 3)
				if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
					t.Fatal(err)
				}
				g, x := s.Catan, s.Catan.Explorer
				resource := 3
				if card == 6 {
					resource = 4
				}
				// Controlled terrain shuffle preserving printed pasture and recipe.
				harbor := -1
				for _, v := range g.Vertices {
					if v.Owner == 0 && v.Harbor {
						harbor = v.ID
						break
					}
				}
				target := -1
				for _, id := range x.Board.Starting {
					if id != x.Board.FramePasture && slices.Contains(g.Tiles[id].Vertices, harbor) {
						target = id
						break
					}
				}
				if target < 0 {
					t.Fatal("harbor lacks movable starting terrain")
				}
				for _, id := range x.Board.Starting {
					if id != x.Board.FramePasture && g.Tiles[id].Resource == resource {
						g.Tiles[id].Resource, g.Tiles[target].Resource = g.Tiles[target].Resource, g.Tiles[id].Resource
						break
					}
				}
				if g.Tiles[target].Resource != resource {
					t.Fatal("fixture terrain missing")
				}
				for p := range g.Players {
					g.Bank[resource] += g.Players[p].Resources[resource]
					g.Players[p].Resources[resource] = 0
				}
				g.Players[1].Resources[resource] = g.Bank[resource] - remaining
				g.Bank[resource] = remaining
				ckProgressGive(t, s, 0, card)
				explorerCityRestore(t, s)
				before := clone(*s.Catan)
				explorerKnightAct(t, s, Action{Type: "catan_progress", Card: card})
				gain := s.Catan.Players[0].Resources[resource] - before.Players[0].Resources[resource]
				if remaining < 2 && gain != remaining || remaining == 19 && (gain < 2 || gain%2 != 0) || s.Catan.Bank[resource] != remaining-gain {
					t.Fatal("wrong resource gain or supply limit")
				}
				for r := range before.Bank {
					if r != resource && (s.Catan.Bank[r] != before.Bank[r] || s.Catan.Players[0].Resources[r] != before.Players[0].Resources[r]) {
						t.Fatal("wrong card type paid")
					}
				}
				if !slices.Equal(s.Catan.Explorer.Economy.Gold, before.Explorer.Economy.Gold) || s.Phase != "catan_turn" {
					t.Fatal("progress payout started production or gold bonus")
				}
			})
		}
	}
}

func TestCatanExplorerCityPaidSettlementThenUpgrade(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	g, x := s.Catan, s.Catan.Explorer
	vertex := -1
	// A controlled pre-existing connecting road; the settlement and upgrade
	// below both go through the actual combined atomic action dispatcher.
	for _, v := range g.Vertices {
		if !catanExplorerBotSite(g, 0, v.ID, false) {
			continue
		}
		for _, e := range g.Edges {
			if (e.A == v.ID || e.B == v.ID) && (e.Owner == -1 || e.Owner == 0) && x.Cargo.landEdge(g, 0, e.ID) {
				g.Edges[e.ID].Owner = 0
				vertex = v.ID
				break
			}
		}
		if vertex >= 0 {
			break
		}
	}
	if vertex < 0 {
		t.Fatal("no controlled settlement site")
	}
	for r, want := range []int{1, 1, 1, 3, 3, 0, 0, 0} {
		g.Bank[r] += g.Players[0].Resources[r] - want
		g.Players[0].Resources[r] = want
	}
	explorerCityRestore(t, s)
	before := clone(*s.Catan)
	explorerCityActionReject(t, s, 0, Action{Type: "catan_settlement", Vertex: vertex, Choice: "free", Prompt: 1})
	explorerKnightAct(t, s, Action{Type: "catan_settlement", Vertex: vertex})
	if s.Catan.Vertices[vertex].Level != 1 || s.Catan.Players[0].Score != before.Players[0].Score+1 || !slices.Equal(s.Catan.Players[0].Resources, []int{0, 0, 0, 2, 3, 0, 0, 0}) {
		t.Fatal("settlement price or score")
	}
	explorerKnightAct(t, s, Action{Type: "catan_city", Vertex: vertex})
	if !s.Catan.cityAt(vertex) || s.Catan.Players[0].Score != before.Players[0].Score+2 || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("new settlement failed actual paid city upgrade")
	}
	for r := range before.Bank {
		if s.Catan.Bank[r] != before.Bank[r]+before.Players[0].Resources[r] {
			t.Fatal("construction lost cards", r)
		}
	}
}
