package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func explorerFishingHand(t *testing.T, s *State, player int, ids ...int) {
	t.Helper()
	f, err := newCatanFishingTokens(len(s.Catan.Players))
	if err != nil {
		t.Fatal(err)
	}
	s.Catan.Fishing.Tokens = *f
	fishOwn(&s.Catan.Fishing.Tokens, player, ids...)
}
func explorerFishingAct(t *testing.T, s *State, player int, a Action) {
	t.Helper()
	a.Prompt = int(s.Catan.TurnSerial)
	if err := s.Apply(player, a); err != nil {
		t.Fatal(s.Phase, a, err)
	}
	explorerFishingRestore(t, s)
}
func explorerFishingReject(t *testing.T, s *State, player int, a Action) {
	t.Helper()
	before, _ := json.Marshal(s)
	if err := s.Apply(player, a); err == nil {
		t.Fatal("accepted fish action", a)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("rejected fish action mutated state")
	}
}

func TestCatanExplorerFishingPaymentsActionsAndDiscount(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		for _, cities := range []bool{false, true} {
			if cities && n == 2 {
				continue
			}
			t.Run(fmt.Sprintf("%d/cities%t", n, cities), func(t *testing.T) {
				s := explorerFishingGame(t, n, "explorers-and-pirates", cities, true, false)
				p := s.Turn
				explorerFishingHand(t, s, p, 0, 11, 21, 22, 23)
				before := s.Catan.Players[p].Resources[0]
				explorerFishingAct(t, s, p, Action{Type: "catan_fish_resource", Color: 0, Tokens: []int{0, 21}})
				if s.Catan.Players[p].Resources[0] != before+1 {
					t.Fatal("resource before production missing")
				}
				for _, a := range []Action{
					{Type: "catan_fish_resource", Color: 5, Tokens: []int{21, 22}, Prompt: int(s.Catan.TurnSerial)},
					{Type: "catan_fish_resource", Color: 0, Tokens: []int{22, 22}, Prompt: int(s.Catan.TurnSerial)},
					{Type: "catan_fish_resource", Color: 0, Tokens: []int{0}, Prompt: int(s.Catan.TurnSerial)},
					{Type: "catan_fish_ship", Tokens: []int{22, 23}, Prompt: int(s.Catan.TurnSerial)},
					{Type: "catan_fish_dev", Tokens: []int{22, 23, 11}, Prompt: int(s.Catan.TurnSerial)},
					{Type: "catan_fish_progress", Tokens: []int{22, 23, 11}, Prompt: int(s.Catan.TurnSerial)},
					{Type: "catan_fish_resource", Color: 0, Tokens: []int{22, 23}},
				} {
					explorerFishingReject(t, s, p, a)
				}
				if n == 2 {
					other := 1 - p
					found := false
					for _, v := range s.Catan.Vertices {
						if catanExplorerBotSite(s.Catan, other, v.ID, false) {
							s.Catan.Vertices[v.ID].Owner, s.Catan.Vertices[v.ID].Level = other, 1
							found = true
							break
						}
					}
					if !found {
						t.Fatal("no legal opponent fixture building")
					}
					s.catanScores()
					if s.Catan.explorerFishCost(p, "catan_fish_resource") != 3 {
						t.Fatal("missing lower-score discount")
					}
					explorerFishingAct(t, s, p, Action{Type: "catan_fish_resource", Color: 1, Tokens: []int{22}})
				}
				explorerEventReady(t, s)
				explorerFishingHand(t, s, p, 21, 22, 23)
				for r, amount := range s.Catan.Players[p].Resources {
					s.Catan.Bank[r] += amount
					s.Catan.Players[p].Resources[r] = 0
				}
				choices := s.catanExplorerFishingChoices(p)
				for _, kind := range []string{"catan_fish_road", "catan_fish_ship"} {
					index := slices.IndexFunc(choices, func(a Action) bool { return a.Type == kind })
					if index < 0 {
						t.Fatal("missing free construction choice", kind)
					}
					copy := clone(*s)
					a := choices[index]
					cargo := clone(copy.Catan.Explorer.Cargo)
					explorerFishingAct(t, &copy, p, a)
					if sum(copy.Catan.Players[p].Resources) != 0 {
						t.Fatal("fish construction fabricated ordinary resources")
					}
					if kind == "catan_fish_road" && copy.Catan.Edges[a.Edge].Owner != p {
						t.Fatal("road not built")
					}
					if kind == "catan_fish_ship" && (copy.Catan.Explorer.Fleet.Positions[a.Slot] != a.Edge || copy.Catan.Explorer.Motion == nil || copy.Catan.Explorer.Motion.Ship != a.Slot) {
						t.Fatal("ship or its public animation missing")
					}
					if kind == "catan_fish_road" && !reflect.DeepEqual(cargo, copy.Catan.Explorer.Cargo) {
						t.Fatal("road changed cargo")
					}
				}
				victim := (p + 1) % n
				color := 0
				if cities {
					color = 5
				}
				for r, amount := range s.Catan.Players[victim].Resources {
					s.Catan.Bank[r] += amount
					s.Catan.Players[victim].Resources[r] = 0
				}
				s.Catan.Bank[color]--
				s.Catan.Players[victim].Resources[color] = 1
				explorerFishingAct(t, s, p, Action{Type: "catan_fish_steal", Target: victim, Tokens: []int{21}})
				if s.Catan.Players[p].Resources[color] != 1 || sum(s.Catan.Players[victim].Resources) != 0 {
					t.Fatal("fish theft failed resource/commodity")
				}
			})
		}
	}
}

func explorerFishingClearStep(t *testing.T, s *State, ship int) int {
	t.Helper()
	g, x := s.Catan, s.Catan.Explorer
	from := x.Fleet.Positions[ship]
	for _, edge := range g.Edges {
		if edge.ID == from {
			continue
		}
		owner, tile := -1, -1
		if x.Pirate != nil {
			owner, tile = x.Pirate.Owner, x.Pirate.Tile
		}
		q, err := x.Fleet.quote(g, s.Turn, g.TurnSerial, ship, []int{edge.ID}, owner, tile)
		if err == nil && len(q.Exploring) == 0 {
			return edge.ID
		}
	}
	t.Fatal("no non-discovering one-step path")
	return -1
}

func TestCatanExplorerFishingSecondVoyageWoolAndTurnReset(t *testing.T) {
	for _, wool := range []bool{false, true} {
		s := explorerFishingGame(t, 3, "land-ho", false, true, false)
		explorerEventReady(t, s)
		p := s.Turn
		ship := p * 3
		explorerFishingHand(t, s, p, 21, 22, 23, 24, 25, 26)
		s.Catan.Bank[2]--
		s.Catan.Players[p].Resources[2]++
		explorerFishingAct(t, s, p, Action{Type: "catan_explorer_begin_move"})
		if wool {
			explorerFishingAct(t, s, p, Action{Type: "catan_explorer_wool", Slot: ship})
		}
		from := s.Catan.Explorer.Fleet.Positions[ship]
		to := explorerFishingClearStep(t, s, ship)
		path := []int{to, from, to, from}
		spent := 4
		if wool {
			path = append(path, to, from)
			spent = 6
		}
		explorerFishingAct(t, s, p, Action{Type: "catan_explorer_sail", Slot: ship, Targets: path})
		explorerFishingAct(t, s, p, Action{Type: "catan_fish_voyage", Slot: ship, Tokens: []int{21, 22, 23}})
		move := s.Catan.Explorer.Fleet.Turn.Ships[ship]
		if move.Remaining != 4 || move.Spent != spent || move.Second == nil || move.Second.Spent != spent || move.Second.Wool != wool {
			t.Fatal("extra voyage allowance or provenance", move)
		}
		if wool {
			explorerFishingReject(t, s, p, Action{Type: "catan_explorer_wool", Slot: ship, Prompt: int(s.Catan.TurnSerial)})
		} else {
			explorerFishingAct(t, s, p, Action{Type: "catan_explorer_wool", Slot: ship})
			if s.Catan.Explorer.Fleet.Turn.Ships[ship].Remaining != 6 {
				t.Fatal("first wool during second voyage did not add two points")
			}
		}
		secondPath := []int{to, from, to, from}
		if !wool {
			secondPath = append(secondPath, to, from)
		}
		explorerFishingAct(t, s, p, Action{Type: "catan_explorer_sail", Slot: ship, Targets: secondPath})
		explorerFishingReject(t, s, p, Action{Type: "catan_fish_voyage", Slot: ship, Tokens: []int{24, 25, 26}, Prompt: int(s.Catan.TurnSerial)})
		for _, damage := range []func(*State){
			func(s *State) { s.Catan.Explorer.Fleet.Turn.Ships[ship].Second.Spent = 99 },
			func(s *State) { s.Catan.Explorer.Fleet.Turn.Ships[ship].Second.Stopped = true },
			func(s *State) { s.Catan.Explorer.Fleet.Turn.Ships[ship].Remaining = 2 },
		} {
			bad := clone(*s)
			damage(&bad)
			if bad.validateCatanExplorer() == nil {
				t.Fatal("accepted forged extra voyage")
			}
		}
		explorerFishingAct(t, s, p, Action{Type: "catan_end"})
		explorerEventReady(t, s)
		explorerFishingAct(t, s, s.Turn, Action{Type: "catan_explorer_begin_move"})
		for _, ship := range s.Catan.Explorer.Fleet.Turn.Ships {
			if ship.Second != nil {
				t.Fatal("second voyage leaked into next movement")
			}
		}
	}
}

func TestCatanExplorerFishingFreeShipDiscoveryAndRestart(t *testing.T) {
	s := explorerFishingGame(t, 3, "land-ho", false, true, false)
	explorerEventReady(t, s)
	g, x, p := s.Catan, s.Catan.Explorer, s.Turn
	buildEdge := -1
	// Explicit midgame harbor at a revealed coast with fog touching the far
	// endpoint. This isolates construction discovery, not natural progression.
search:
	for _, hidden := range slices.Clone(x.Board.Hidden) {
		if hidden.Resource >= 5 {
			continue
		}
		if _, err := x.Board.reveal(g, hidden.Tile); err != nil {
			t.Fatal(err)
		}
		for _, v := range g.Vertices {
			if v.Level != 0 || !catanExplorerLandVertex(g, v.ID) {
				continue
			}
			near := slices.ContainsFunc(g.Edges, func(e CatanEdge) bool {
				return e.A == v.ID && g.Vertices[e.B].Level > 0 || e.B == v.ID && g.Vertices[e.A].Level > 0
			})
			if near {
				continue
			}
			for _, edge := range g.Edges {
				if edge.A != v.ID && edge.B != v.ID || !catanExplorerSeaEdge(g, edge.ID) || slices.Contains(x.Fleet.Positions, edge.ID) {
					continue
				}
				fogEdge := slices.ContainsFunc(edge.Tiles, func(id int) bool { return g.Tiles[id].Resource == CatanFog })
				fogTip := slices.ContainsFunc(g.Tiles, func(h CatanTile) bool { return h.Resource == CatanFog && catanExplorerTouches(g, edge.ID, h.ID) })
				if fogEdge || !fogTip {
					continue
				}
				g.Vertices[v.ID].Owner, g.Vertices[v.ID].Level, g.Vertices[v.ID].Harbor = p, 2, true
				buildEdge = edge.ID
				break search
			}
		}
	}
	if buildEdge < 0 {
		t.Fatal("no discovery harbor fixture")
	}
	s.catanScores()
	explorerFishingHand(t, s, p, 21, 22, 23, 24, 25)
	explorerFishingRestore(t, s)
	ship := p*3 + 1
	explorerFishingAct(t, s, p, Action{Type: "catan_fish_ship", Edge: buildEdge, Slot: ship, Tokens: []int{21, 22}})
	if !slices.Contains(s.Catan.Explorer.Cargo.Turn.BuildStopped, ship) {
		t.Fatal("new discovery ship did not stop")
	}
	explorerFishingAct(t, s, p, Action{Type: "catan_explorer_begin_move"})
	if !s.Catan.Explorer.Fleet.Turn.Ships[ship].Closed {
		t.Fatal("discovery stop lost at movement start")
	}
	explorerFishingAct(t, s, p, Action{Type: "catan_fish_voyage", Slot: ship, Tokens: []int{23, 24, 25}})
	move := s.Catan.Explorer.Fleet.Turn.Ships[ship]
	if move.Second == nil || !move.Second.Stopped || move.Remaining != 4 || move.Spent != 0 {
		t.Fatal("zero-spent discovery restart", move)
	}
	to := explorerFishingClearStep(t, s, ship)
	explorerFishingAct(t, s, p, Action{Type: "catan_explorer_sail", Slot: ship, Targets: []int{to}})
	explorerFishingAct(t, s, p, Action{Type: "catan_end"})
	explorerEventReady(t, s)
	explorerFishingAct(t, s, s.Turn, Action{Type: "catan_explorer_begin_move"})
}

func TestCatanExplorerFishingSecondVoyageSwiftBudget(t *testing.T) {
	g, fleet, gold, bank := explorerSailingFixture(t, 3)
	g.Fishing = &CatanFishing{Explorer: catanExplorerFishingRules}
	if _, err := fleet.sail(g, 0, 1, 0, []int{1, 2, 1, 0}, -1, -1, gold, bank); err != nil {
		t.Fatal(err)
	}
	move := &fleet.Turn.Ships[0]
	move.Second, move.Remaining = &catanExplorerSecondVoyage{Spent: 4}, 4
	if err := fleet.swiftVoyage(g, 0, 1, 1); err != nil {
		t.Fatal(err)
	}
	if move.Remaining != 5 {
		t.Fatal("swift bonus not added to second voyage")
	}
	if err := fleet.wool(g, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	if move.Remaining != 7 {
		t.Fatal("second voyage wool budget")
	}
	if _, err := fleet.sail(g, 0, 1, 0, []int{1, 2, 1, 0, 1, 2, 1}, -1, -1, gold, bank); err != nil {
		t.Fatal(err)
	}
	if err := fleet.validate(g); err != nil {
		t.Fatal(err)
	}
	if move.Spent != 11 || move.Remaining != 0 {
		t.Fatal("combined voyage budget", move)
	}
}

func TestCatanExplorerFishingPiratePassDestinationsAndAllShips(t *testing.T) {
	s := explorerFishingGame(t, 3, "explorers-and-pirates", false, true, false)
	explorerEventReady(t, s)
	g, x := s.Catan, s.Catan.Explorer
	p := s.Turn
	found := false
	for _, tile := range g.Tiles {
		if !catanExplorerPirateTile(g, x.Board, tile.ID) {
			continue
		}
		for _, edge := range g.Edges {
			if !slices.Contains(edge.Tiles, tile.ID) {
				continue
			}
			fog := false
			for _, h := range g.Tiles {
				if h.Resource == CatanFog && catanExplorerTouches(g, edge.ID, h.ID) {
					fog = true
				}
			}
			if !fog {
				x.Pirate.Owner, x.Pirate.Tile = (p+1)%3, tile.ID
				x.Fleet.Positions[p*3], x.Fleet.Positions[p*3+1] = edge.ID, edge.ID
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("no safe pirate fixture sea")
	}
	x.Economy.GoldBank += x.Economy.Gold[p]
	x.Economy.Gold[p] = 0
	explorerFishingHand(t, s, p, 11, 12)
	explorerFishingAct(t, s, p, Action{Type: "catan_explorer_begin_move"})
	g, x = s.Catan, s.Catan.Explorer
	if len(x.Fleet.destinations(g, p, g.TurnSerial, p*3)) != 0 {
		t.Fatal("zero-gold ship escaped toll before payment")
	}
	before := clone(x.Pirate)
	explorerFishingAct(t, s, p, Action{Type: "catan_fish_pirate", Tokens: []int{11}})
	explorerFishingReject(t, s, p, Action{Type: "catan_fish_pirate", Tokens: []int{12}, Prompt: int(s.Catan.TurnSerial)})
	for ship := p * 3; ship < p*3+2; ship++ {
		g, x = s.Catan, s.Catan.Explorer
		if len(x.Fleet.destinations(g, p, g.TurnSerial, ship)) == 0 {
			t.Fatal("paid fish pass still pruned all destinations")
		}
		to := explorerFishingClearStep(t, s, ship)
		q, err := x.Fleet.quote(g, p, g.TurnSerial, ship, []int{to}, x.Pirate.Owner, x.Pirate.Tile)
		if err != nil || q.Gold != 0 {
			t.Fatal("fish pass did not waive toll", q, err)
		}
		explorerFishingAct(t, s, p, Action{Type: "catan_explorer_sail", Slot: ship, Targets: []int{to}})
	}
	if !reflect.DeepEqual(before, s.Catan.Explorer.Pirate) || s.Catan.Explorer.Economy.Gold[p] != 0 {
		t.Fatal("fish pass moved pirate or spent gold")
	}
	copy := clone(*s)
	copy.Catan.Fishing = nil
	copy.Catan.Explorer.Board.Fishing = ""
	copy.Catan.Explorer.Board.FishingLakes = false
	if copy.Catan.Explorer.Fleet.validate(copy.Catan) == nil {
		t.Fatal("base fleet accepted fish pass")
	}
	explorerFishingAct(t, s, p, Action{Type: "catan_end"})
	explorerEventReady(t, s)
	explorerFishingAct(t, s, s.Turn, Action{Type: "catan_explorer_begin_move"})
	if s.Catan.Explorer.Fleet.Turn.FishPirate {
		t.Fatal("fish pirate pass carried into another turn")
	}
}
