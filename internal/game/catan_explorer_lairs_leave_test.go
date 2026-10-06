package game

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"testing"
)

func explorerNetworkState(t *testing.T, name string) *State {
	t.Helper()
	raw, err := os.ReadFile("../server/testdata/catan_explorer_lairs_" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var s State
	if err = json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return explorerStateRestore(t, &s)
}

// Three visible crew are placed in a controlled fixture, then the final
// landing uses Apply. This does not claim natural recruitment/sailing coverage.
func explorerLairsReadyDeparture(t *testing.T) *State {
	t.Helper()
	s := explorerLairsStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	x := g.Explorer
	actor := s.Turn
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
	x.Fleet.Positions[actor*3] = edge
	x.Cargo.Units[actor*11] = catanExplorerCargoLocation{"supply", -1}
	other := (actor + 1) % 3
	for _, unit := range []int{other*11 + 2, other*11 + 3} {
		x.Cargo.Units[unit] = catanExplorerCargoLocation{"lair", tile}
	}
	x.Cargo.Units[actor*11+2] = catanExplorerCargoLocation{"ship", actor * 3}
	for _, a := range []Action{{Type: "catan_explorer_begin_move", Prompt: 1}, {Type: "catan_explorer_land", Prompt: 1, Target: tile, Slot: actor * 3, Cards: []int{actor*11 + 2}}} {
		if err := s.Apply(actor, a); err != nil {
			t.Fatal(err)
		}
	}
	return explorerStateRestore(t, s)
}

func explorerPassOrdinaryTurn(t *testing.T, s *State) {
	t.Helper()
	if s.Phase == "catan_roll" {
		if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
			t.Fatal(err)
		}
	}
	if s.Phase == "catan_turn" {
		if err := s.Apply(s.Turn, Action{Type: "catan_explorer_begin_move", Prompt: int(s.Catan.TurnSerial)}); err != nil {
			t.Fatal(err)
		}
	}
	if s.Phase != "catan_explorer_move" {
		t.Fatal("unexpected response", s.Phase)
	}
	if err := s.Apply(s.Turn, Action{Type: "catan_end", Prompt: int(s.Catan.TurnSerial)}); err != nil {
		t.Fatal(err)
	}
}

func TestCatanExplorerLairsDeparturePirateAndRotation(t *testing.T) {
	for n := 2; n <= 4; n++ {
		for _, phase := range []string{"catan_roll", "catan_turn", "catan_explorer_move"} {
			for _, owner := range []string{"self", "other"} {
				t.Run(fmt.Sprintf("%d/%s/%s", n, phase, owner), func(t *testing.T) {
					s := explorerLairsStarted(t, n)
					actor := s.Turn
					if phase != "catan_roll" {
						if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
							t.Fatal(err)
						}
					}
					if phase == "catan_explorer_move" {
						if err := s.Apply(actor, Action{Type: "catan_explorer_begin_move", Prompt: 1}); err != nil {
							t.Fatal(err)
						}
					}
					x := s.Catan.Explorer
					x.Pirate.Owner = actor
					if owner == "other" {
						x.Pirate.Owner = (actor + 1) % n
					}
					x.Pirate.Tile = x.Pirate.destinations(s.Catan, x.Board)[0]
					oldPirate := clone(*x.Pirate)
					if err := s.EliminateCatan(actor); err != nil {
						t.Fatal(err)
					}
					x = s.Catan.Explorer
					if !x.Lairs.Retired[actor] || s.Turn != (actor+1)%n || s.Catan.TurnSerial != 2 {
						t.Fatal("departure did not transfer turn")
					}
					if owner == "self" {
						if x.Pirate.Owner != -1 || x.Pirate.Tile != -1 {
							t.Fatal("departed pirate remained")
						}
					} else if !reflect.DeepEqual(*x.Pirate, oldPirate) {
						t.Fatal("unrelated pirate changed")
					}
					s = explorerStateRestore(t, s)
					for step := 0; step < 6 && !s.Finished; step++ {
						if s.Turn == actor {
							t.Fatal("departed player got turn")
						}
						explorerPassOrdinaryTurn(t, s)
						s = explorerStateRestore(t, s)
					}
					if n == 2 && (!s.Finished || !slices.Equal(s.Winners, []int{(actor + 1) % n})) {
						t.Fatal("survivor not awarded")
					}
					if sum(s.Catan.Players[actor].Resources) != 0 || s.Catan.Explorer.Economy.Gold[actor] != 0 {
						t.Fatal("departed player produced")
					}
				})
			}
		}
	}
}

func TestCatanExplorerLairsDepartureResetsUnresolvedCaptureAndCanRecapture(t *testing.T) {
	s := explorerLairsReadyDeparture(t)
	actor := s.Turn
	other := (actor + 1) % 3
	x := s.Catan.Explorer
	tile := -1
	for _, site := range x.Lairs.Sites {
		if site.Ready > 0 {
			tile = site.Tile
		}
	}
	if tile < 0 {
		t.Fatal("no ready lair")
	}
	beforeGold := slices.Clone(x.Economy.Gold)
	if err := s.EliminateCatan(actor); err != nil {
		t.Fatal(err)
	}
	s = explorerStateRestore(t, s)
	g := s.Catan
	x = g.Explorer
	site := x.Lairs.Sites[x.Lairs.site(tile)]
	if site.Ready != 0 || site.Captor != -1 || site.Resolved != 0 || len(site.Contributions) != 0 || sum(x.Lairs.Progress) != 0 || x.Economy.Gold[other] != beforeGold[other] {
		t.Fatal("unresolved capture rewarded or retained")
	}
	if got := x.Cargo.contents(catanExplorerCargoLocation{"lair", tile}); !slices.Equal(got, []int{other*11 + 2, other*11 + 3}) {
		t.Fatal("other crews lost", got)
	}
	// The next player may finish the remaining two-crew site normally. Give
	// their third crew a touching ship; use real production/landing/resolution.
	ship := other * 3
	for _, e := range g.Edges {
		if catanExplorerSeaEdge(g, e.ID) && catanExplorerTouches(g, e.ID, tile) {
			x.Fleet.Positions[ship] = e.ID
			break
		}
	}
	x.Cargo.Units[other*11] = catanExplorerCargoLocation{"supply", -1}
	x.Cargo.Units[other*11+4] = catanExplorerCargoLocation{"ship", ship}
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	for _, a := range []Action{{Type: "catan_explorer_begin_move", Prompt: 2}, {Type: "catan_explorer_land", Prompt: 2, Target: tile, Slot: ship, Cards: []int{other*11 + 4}}, {Type: "catan_end", Prompt: 2}, {Type: "catan_explorer_resolve", Prompt: 2, Target: tile}} {
		if err := s.Apply(other, a); err != nil {
			t.Fatal(err)
		}
	}
	s = explorerStateRestore(t, s)
	x = s.Catan.Explorer
	site = x.Lairs.Sites[x.Lairs.site(tile)]
	if site.Resolved != 2 || site.Hero != other || x.Lairs.Progress[actor] != 0 || x.Lairs.Progress[other] != 2 {
		t.Fatal("recapture failed")
	}
}

func TestCatanExplorerLairsDeparturePreservesHistoryAndTransfersLeader(t *testing.T) {
	s := explorerNetworkState(t, "resolve")
	s.AutoCatanPending() // three contributors, ready for hero dice
	g := s.Catan
	x := g.Explorer
	hero := s.Turn
	if s.Phase != "catan_explorer_battle" {
		t.Fatal("missing battle")
	}
	// Deterministic hero, via the mission controller's trusted random source.
	dieAt := 0
	if err := x.Lairs.apply(g, x.Board, x.Fleet, x.Cargo, x.Economy, hero, g.TurnSerial, "roll", x.Lairs.Battle.Tile, 0, nil, func(int) int {
		dieAt++
		if dieAt == 1 {
			return 5
		}
		return 0
	}); err != nil {
		t.Fatal(err)
	}
	s.catanExplorerMissionScore()
	if err := s.catanExplorerNextTurn(); err != nil {
		t.Fatal(err)
	}
	for s.Turn != hero {
		explorerPassOrdinaryTurn(t, s)
	}
	s = explorerStateRestore(t, s)
	x = s.Catan.Explorer
	old := clone(*x.Lairs)
	scores := x.Lairs.scores()
	if old.leader() != hero {
		t.Fatal("fixture hero must lead")
	}
	nextLeader := (hero + 1) % 3
	if err := s.EliminateCatan(hero); err != nil {
		t.Fatal(err)
	}
	s = explorerStateRestore(t, s)
	x = s.Catan.Explorer
	if x.Lairs.leader() != nextLeader || x.Lairs.scores()[nextLeader] != scores[nextLeader]+1 || x.Lairs.scores()[hero] != scores[hero]-1 {
		t.Fatal("mission leadership not transferred")
	}
	if !reflect.DeepEqual(old.Sites, x.Lairs.Sites) || !slices.Equal(old.Progress, x.Lairs.Progress) || !slices.Equal(old.Arrival, x.Lairs.Arrival) || old.Serial != x.Lairs.Serial {
		t.Fatal("departure rewrote battle history")
	}
	view := s.View(-1)["catan"].(map[string]any)["explorer"].(map[string]any)["lairs"].(catanExplorerLairsView)
	if view.Leader != nextLeader {
		t.Fatal("public leader stale")
	}
	for _, mutate := range []func(*State){
		func(b *State) { b.Catan.Explorer.Lairs.Retired = nil },
		func(b *State) { b.Catan.Explorer.Lairs.Retired = []bool{true} },
		func(b *State) { b.Catan.Explorer.Lairs.Retired[hero] = false },
		func(b *State) { b.Catan.Explorer.Economy.Gold[hero]++; b.Catan.Explorer.Economy.GoldBank-- },
		func(b *State) { b.Catan.Players[hero].Resources[0]++; b.Catan.Bank[0]-- },
		func(b *State) {
			b.Catan.Explorer.Cargo.Units[hero*11+2] = catanExplorerCargoLocation{"lair", old.Sites[0].Tile}
		},
	} {
		bad := clone(*s)
		mutate(&bad)
		if err := bad.validateCatanExplorer(); err == nil {
			t.Fatal("corrupted retired state accepted")
		}
	}
}

func TestCatanExplorerLairsDepartureRejectsMandatoryResponses(t *testing.T) {
	for _, name := range []string{"setup", "discard", "chase", "resolve", "battle"} {
		t.Run(name, func(t *testing.T) {
			fixture := name
			if name == "battle" {
				fixture = "resolve"
			}
			s := explorerNetworkState(t, fixture)
			if name == "battle" {
				s.AutoCatanPending()
			}
			before, _ := json.Marshal(s)
			if err := s.EliminateCatan(s.Turn); err == nil {
				t.Fatal("removed a required responder")
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("rejection mutated state")
			}
			s.AutoCatanPending()
			explorerStateRestore(t, s)
		})
	}
}
