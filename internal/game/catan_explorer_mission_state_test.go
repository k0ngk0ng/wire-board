package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func explorerLairsStarted(t *testing.T, n int) *State {
	t.Helper()
	// Artificial component numbers for private integration acceptance only.
	s, err := newCatanExplorerLairsState(n, "fixed", []int{2, 3, 4, 5, 6, 8})
	if err != nil {
		t.Fatal(err)
	}
	for s.Catan.Explorer.Setup != nil {
		choices := s.catanExplorerChoices(s.Turn)
		if len(choices) == 0 {
			t.Fatal("setup state has no choices")
		}
		raw, _ := json.Marshal(catanExplorerChoiceView(choices)[0])
		var a Action
		if err = json.Unmarshal(raw, &a); err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(s)
		if err = s.Apply((s.Turn+1)%n, a); err == nil {
			t.Fatal("non-actor setup accepted")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("bad setup mutated")
		}
		if err = s.Apply(s.Turn, a); err != nil {
			t.Fatal(err)
		}
		s = explorerStateRestore(t, s)
	}
	return s
}
func TestCatanExplorerLairsStateSetupChoicesRollsAndRounds(t *testing.T) {
	for n := 2; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerLairsStarted(t, n)
			start := s.Catan.StartPlayer
			for i := 0; i < 100; i++ {
				if err := s.validateCatanExplorer(); err != nil {
					t.Fatal(i, err)
				}
				actor := s.Turn
				var a Action
				if s.Phase == "catan_discard" {
					for p, due := range s.Catan.DiscardDue {
						if due > 0 {
							actor = p
							break
						}
					}
					var err error
					a, err = s.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					wanted := map[string]string{"catan_roll": "catan_roll", "catan_turn": "catan_explorer_begin_move", "catan_explorer_move": "catan_end"}[s.Phase]
					choices := s.catanExplorerChoices(actor)
					found := false
					for _, candidate := range choices {
						if wanted == "" || candidate.Type == wanted {
							a = candidate
							found = true
							break
						}
					}
					if !found {
						t.Fatal("missing required state choice", s.Phase)
					}
				}
				if err := s.Apply(actor, a); err != nil {
					t.Fatal(i, s.Phase, a, err)
				}
				if i%9 == 0 {
					s = explorerStateRestore(t, s)
				}
				view := s.View(-1)["catan"].(map[string]any)["explorer"].(map[string]any)
				if len(view["choices"].([]map[string]any)) != 0 {
					t.Fatal("spectator choices leaked")
				}
			}
			if s.Round < 3 || s.Catan.StartPlayer != start || s.Catan.Explorer.Setup != nil {
				t.Fatal("round progression or randomized first player lost")
			}
		})
	}
}
func TestCatanExplorerLairsSevenSuspendsOrdinaryActions(t *testing.T) {
	s := explorerLairsStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{3, 4}); err != nil {
		t.Fatal(err)
	}
	if err := s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_explorer_pirate_place" {
		t.Fatal("seven did not activate pirate", s.Phase)
	}
	s = explorerStateRestore(t, s)
	for _, kind := range []string{"catan_explorer_begin_move", "catan_end", "catan_roll"} {
		before, _ := json.Marshal(s)
		if err := s.Apply(s.Turn, Action{Type: kind, Prompt: int(s.Catan.TurnSerial)}); err == nil {
			t.Fatal("bypassed pirate")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("illegal response mutated state")
		}
	}
	a, err := s.BotAction(s.Turn)
	if err != nil {
		t.Fatal(err)
	}
	if a.Type != "catan_explorer_pirate_place" {
		t.Fatal("bot cannot satisfy required pirate response")
	}
	if err = s.Apply(s.Turn, a); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_turn" || s.Catan.Explorer.Pirate.Owner != s.Turn {
		t.Fatal("did not resume building")
	}
}
func TestCatanExplorerLairsMissionStateEndBattleAndProjection(t *testing.T) {
	s := explorerLairsStarted(t, 2)
	g := s.Catan
	x := g.Explorer
	actor := s.Turn
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	// Explicit mission integration fixture: reveal through the real controller,
	// then put two own crew on one lair and the third on a touching ship.
	for _, h := range slices.Clone(x.Board.Hidden) {
		if _, err := x.discover(g, actor, []int{h.Tile}); err != nil {
			t.Fatal(err)
		}
	}
	tile, edge := -1, -1
	for _, site := range x.Lairs.Sites {
		for _, e := range g.Edges {
			if catanExplorerSeaEdge(g, e.ID) && catanExplorerTouches(g, e.ID, site.Tile) && !slices.Contains(x.Fleet.Positions, e.ID) {
				tile, edge = site.Tile, e.ID
				break
			}
		}
		if tile >= 0 {
			break
		}
	}
	if tile < 0 {
		t.Fatal("no lair coast")
	}
	ship := actor * 3
	x.Fleet.Positions[ship] = edge
	x.Cargo.Units[actor*11] = catanExplorerCargoLocation{"supply", -1}
	x.Cargo.Units[actor*11+2] = catanExplorerCargoLocation{"lair", tile}
	x.Cargo.Units[actor*11+3] = catanExplorerCargoLocation{"lair", tile}
	x.Cargo.Units[actor*11+4] = catanExplorerCargoLocation{"ship", ship}
	if err := s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	for _, a := range []Action{{Type: "catan_explorer_begin_move", Prompt: 1}, {Type: "catan_explorer_land", Prompt: 1, Target: tile, Slot: ship, Cards: []int{actor*11 + 4}}, {Type: "catan_end", Prompt: 1}} {
		if err := s.Apply(actor, a); err != nil {
			t.Fatal(a, err)
		}
	}
	if s.Phase != "catan_explorer_resolve" || s.Turn != actor || s.Catan.TurnSerial != 1 {
		t.Fatal("turn advanced before battle")
	}
	s = explorerStateRestore(t, s)
	v := s.View(-1)["catan"].(map[string]any)["explorer"].(map[string]any)["lairs"].(catanExplorerLairsView)
	for _, site := range v.Sites {
		if site.Number != 0 {
			t.Fatal("unresolved secret number leaked")
		}
	}
	a, err := s.BotAction(actor)
	if err != nil {
		t.Fatal(err)
	}
	if a.Type != "catan_explorer_resolve" || a.Target != tile {
		t.Fatal("bot required resolution")
	}
	if err = s.Apply(actor, a); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	x = g.Explorer
	if s.Turn == actor || g.TurnSerial != 2 || s.Phase != "catan_roll" || g.Players[actor].Score != 5 || x.Lairs.Sites[x.Lairs.site(tile)].Hero != actor || g.Tiles[tile].Number == 0 {
		t.Fatal("mission did not score and hand off", s.Phase, g.Players[actor].Score)
	}
	s = explorerStateRestore(t, s)
	v = s.View(-1)["catan"].(map[string]any)["explorer"].(map[string]any)["lairs"].(catanExplorerLairsView)
	if v.Sites[x.Lairs.site(tile)].Number != g.Tiles[tile].Number {
		t.Fatal("resolved number missing")
	}
}

func TestCatanExplorerLairsCrewPreviewsAreExecutableAndDoNotLeak(t *testing.T) {
	s := explorerLairsStarted(t, 3)
	actor := s.Turn
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	explorerFlowResources(s, map[int][]int{actor: {4, 4, 4, 4, 4}})
	before, _ := json.Marshal(s)
	crew := 0
	choices := s.catanExplorerChoices(actor)
	wire, _ := json.Marshal(catanExplorerChoiceView(choices))
	var decoded []Action
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, a := range decoded {
		if a.Type == "catan_explorer_unit" && a.Card%11 >= 2 {
			crew++
		}
		next := clone(*s)
		if err := next.Apply(actor, a); err != nil {
			t.Fatal(a, err)
		}
	}
	if crew == 0 {
		t.Fatal("crew missing from affordable actions")
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("preview mutated state")
	}
	changed := clone(*s)
	slices.Reverse(changed.Catan.Explorer.Lairs.Deck)
	v1, _ := json.Marshal(s.View(actor))
	v2, _ := json.Marshal(changed.View(actor))
	if string(v1) != string(v2) {
		t.Fatal("lair secret order changes public view")
	}
}
func TestCatanExplorerMissionRoutesKeepPaidAndFreeAlternatives(t *testing.T) {
	for gold := 0; gold <= 1; gold++ {
		g, f, _, _ := explorerSailingFixture(t, 3, CatanHexSpec{Resource: CatanSea}, CatanHexSpec{Q: 1, Resource: CatanSea})
		for id := range f.Positions {
			f.Positions[id] = -1
		}
		for _, edge := range g.Edges {
			if slices.Contains(edge.Tiles, 1) && !slices.Contains(edge.Tiles, 0) {
				f.Positions[0] = edge.ID
				break
			}
		}
		f.Turn = nil
		if err := f.begin(g, 0, 1, 0); err != nil {
			t.Fatal(err)
		}
		g.Explorer = &catanExplorer{Fleet: f, Pirate: &catanExplorerPirate{Owner: 1, Tile: 0}, Economy: &catanExplorerEconomy{Gold: []int{gold, 0, 0}}}
		expected := map[[2]int]int{}
		var enumerate func(int, []int, int)
		enumerate = func(from int, path []int, cost int) {
			if len(path) == 4 {
				return
			}
			for _, edge := range g.Edges {
				if edge.ID == from || !catanExplorerAdjacentEdges(g.Edges[from], edge) {
					continue
				}
				nextCost := cost
				if slices.Contains(g.Edges[from].Tiles, 0) || slices.Contains(edge.Tiles, 0) {
					nextCost = 1
				}
				if nextCost > gold {
					continue
				}
				next := append(slices.Clone(path), edge.ID)
				key := [2]int{edge.ID, nextCost}
				if key != ([2]int{f.Positions[0], 0}) && (expected[key] == 0 || len(next) < expected[key]) {
					expected[key] = len(next)
				}
				enumerate(edge.ID, next, nextCost)
			}
		}
		enumerate(f.Positions[0], nil, 0)
		actual := map[[2]int]int{}
		for _, path := range f.destinations(g, 0, 1, 0) {
			quote, err := f.quote(g, 0, 1, 0, path, 1, 0)
			if err != nil || quote.Gold > gold {
				t.Fatal("unpayable advertised route", path, err)
			}
			actual[[2]int{path[len(path)-1], quote.Gold}] = len(path)
		}
		if len(actual) != len(expected) {
			t.Fatal("lost paid/free endpoints", gold, actual, expected)
		}
		for key, length := range expected {
			if actual[key] != length {
				t.Fatal("lost shortest path for toll state", gold, key, length, actual[key])
			}
		}
	}
}
