package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func explorerFullStarted(t *testing.T, n int) *State {
	t.Helper()
	// Explicit artificial lair numbers, not verified production components.
	s, err := newCatanExplorerFullState(n, []int{2, 3, 4, 5, 6, 8})
	if err != nil {
		t.Fatal(err)
	}
	for s.Catan.Explorer.Setup != nil {
		choices := s.catanExplorerChoices(s.Turn)
		if len(choices) == 0 {
			t.Fatal("full setup stalled")
		}
		if err := s.Apply(s.Turn, choices[0]); err != nil {
			t.Fatal(err)
		}
		s = explorerStateRestore(t, s)
	}
	return s
}

func explorerFullRevealed(t *testing.T) *State {
	t.Helper()
	s := explorerFullStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	// Controlled boundary fixture; revealing directly is not natural gameplay.
	explorerFishRevealExcept(t, s, -1)
	return s
}

func TestCatanExplorerFullFormalOpeningAndMissions(t *testing.T) {
	for n := 2; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerFullStarted(t, n)
			first := s.Turn
			x := s.Catan.Explorer
			if x.Board.Target != 17 || x.Lairs == nil || x.Spice == nil || x.Fish == nil || len(x.Cargo.Spice) != 24 || len(x.Cargo.Fish) != 6 || s.Phase != "catan_roll" {
				t.Fatal("missing full mission state")
			}
			for _, p := range s.Catan.Players {
				if p.Score != 3 {
					t.Fatal("initial score")
				}
			}
			for viewer := -1; viewer < n; viewer++ {
				v := s.View(viewer)["catan"].(map[string]any)["explorer"].(map[string]any)
				b := v["board"].(catanExplorerBoardView)
				if b.Unexplored != [2]int{16, 16} || len(b.Farms)+len(b.Shoals) > 0 || len(v["lairs"].(catanExplorerLairsView).Sites) > 0 {
					t.Fatal("hidden missions leaked")
				}
			}
			for turn := 0; turn < 2*n; turn++ {
				// Deterministic trusted production, actual Apply for phase transitions.
				if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
					t.Fatal(err)
				}
				explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
				explorerFishApply(t, s, Action{Type: "catan_explorer_fish_roll"})
				explorerSpiceApply(t, s, Action{Type: "catan_end"})
			}
			if s.Turn != first || s.Round != 3 || s.Catan.TurnSerial != uint64(2*n+1) {
				t.Fatal("full turn cycle")
			}
		})
	}
	if _, err := newCatanExplorerFullState(3, nil); err == nil {
		t.Fatal("invented missing lair inventory")
	}
}

func TestCatanExplorerFullDiscoveryAndComponentIsolation(t *testing.T) {
	s := explorerFullRevealed(t)
	g, x := s.Catan, s.Catan.Explorer
	if len(x.Lairs.Sites) != 6 || len(x.Lairs.Deck) != 0 || len(x.Board.publicView().Farms) != 6 || len(x.Board.publicView().Shoals) != 6 {
		t.Fatal("full discoveries")
	}
	allocated := 0
	for _, sack := range x.Cargo.Spice {
		if sack.Origin >= 0 {
			allocated++
		}
	}
	if allocated != 18 {
		t.Fatal("farm sacks not allocated per starting player")
	}
	for _, site := range x.Lairs.Sites {
		if g.Tiles[site.Tile].Number != 0 {
			t.Fatal("unconquered gold number leaked")
		}
	}
	for viewer := -1; viewer < 3; viewer++ {
		v := s.View(viewer)["catan"].(map[string]any)["explorer"].(map[string]any)
		for _, site := range v["lairs"].(catanExplorerLairsView).Sites {
			if site.Number != 0 {
				t.Fatal("secret lair token exposed")
			}
		}
	}
	for _, mutate := range []func(*State){
		func(s *State) { s.Catan.Explorer.Spice = nil },
		func(s *State) { s.Catan.Explorer.Fish = nil },
		func(s *State) { s.Catan.Explorer.Lairs = nil },
		func(s *State) { s.Catan.Explorer.Board.Target = 15 },
	} {
		bad := clone(*s)
		mutate(&bad)
		if bad.validateCatanExplorer() == nil {
			t.Fatal("incomplete full state accepted")
		}
	}
	explorerStateRestore(t, s)
}

func TestCatanExplorerFullSpiceAndFishVictoryAt17(t *testing.T) {
	for _, kind := range []string{"spice", "fish"} {
		t.Run(kind, func(t *testing.T) {
			s := explorerFullRevealed(t)
			actor := s.Turn
			g, x := s.Catan, s.Catan.Explorer
			sacks := []int{}
			for i, farm := range x.Board.publicView().Farms {
				sacks = append(sacks, explorerSpiceClaimFixture(t, s, actor, farm.Tile, i+2, catanExplorerCargoLocation{"supply", -1}))
			}
			// All six permanent crews coexist with the remaining three at a lair.
			tile := x.Lairs.Sites[0].Tile
			x.Lairs.Sites[0].Ready, x.Lairs.Sites[0].Captor = g.TurnSerial, actor
			for i := 8; i < 11; i++ {
				x.Cargo.Units[actor*11+i] = catanExplorerCargoLocation{"lair", tile}
			}
			explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
			g, x = s.Catan, s.Catan.Explorer
			for _, id := range sacks[:2] {
				x.Spice.Deliveries = append(x.Spice.Deliveries, catanExplorerSpiceDelivery{actor, g.TurnSerial, id})
			}
			x.Fish.Deliveries = []catanExplorerFishDelivery{{actor, g.TurnSerial, 0}, {actor, g.TurnSerial, 0}}
			s.catanExplorerMissionScore()
			explorerSpiceApply(t, s, Action{Type: "catan_end"})
			explorerSpiceApply(t, s, Action{Type: "catan_explorer_resolve", Target: tile})
			for s.Turn != actor {
				explorerPassOrdinaryTurn(t, s)
			}
			if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
				t.Fatal(err)
			}
			explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
			explorerVictoryBuildings(t, s, actor, 10)
			g, x = s.Catan, s.Catan.Explorer
			if g.Players[actor].Score != 16 || x.Lairs.playerScore(g, actor) != 16 || s.Finished {
				t.Fatal("three mission score should be 16")
			}
			ship := actor * 3
			x.Cargo.Units[actor*11] = catanExplorerCargoLocation{"supply", -1}
			anchor := x.Board.Council.Anchors[0]
			explorerSpiceDock(t, s, ship, func(e CatanEdge) bool { return e.A == anchor || e.B == anchor })
			card := sacks[2]
			if kind == "spice" {
				x.Cargo.Spice[card].At = catanExplorerCargoLocation{"ship", ship}
			} else {
				card = 0
				x.Cargo.Fish[0] = catanExplorerCargoLocation{"ship", ship}
			}
			explorerSpiceApply(t, s, Action{Type: "catan_explorer_" + kind + "_deliver", Slot: ship, Card: card})
			if !s.Finished || s.Catan.Players[actor].Score != 17 || !slices.Equal(s.Winners, []int{actor}) {
				t.Fatal("full delivery failed immediate 17-point victory")
			}
		})
	}
}

func TestCatanExplorerFullLairRewardIncludesSpiceVictory(t *testing.T) {
	s := explorerFullRevealed(t)
	actor := s.Turn
	other := (actor + 1) % 3
	g, x := s.Catan, s.Catan.Explorer
	sacks := []int{}
	for i, farm := range x.Board.publicView().Farms[:3] {
		id := explorerSpiceClaimFixture(t, s, actor, farm.Tile, i+2, catanExplorerCargoLocation{"supply", -1})
		sacks = append(sacks, id)
	}
	explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
	g, x = s.Catan, s.Catan.Explorer
	for _, id := range sacks {
		x.Spice.Deliveries = append(x.Spice.Deliveries, catanExplorerSpiceDelivery{actor, g.TurnSerial, id})
	}
	x.Fish.Deliveries = []catanExplorerFishDelivery{{actor, g.TurnSerial, 0}}
	tile := x.Lairs.Sites[0].Tile
	x.Lairs.Sites[0].Ready, x.Lairs.Sites[0].Captor = g.TurnSerial, actor
	x.Cargo.Units[actor*11+5] = catanExplorerCargoLocation{"lair", tile}
	for i := 2; i < 4; i++ {
		x.Cargo.Units[other*11+i] = catanExplorerCargoLocation{"lair", tile}
	}
	s.catanExplorerMissionScore()
	explorerSpiceApply(t, s, Action{Type: "catan_end"})
	explorerVictoryBuildings(t, s, actor, 10)
	g, x = s.Catan, s.Catan.Explorer
	if g.Players[actor].Score != 15 || x.Lairs.playerScore(g, actor) != 15 || s.Finished {
		t.Fatal("combined pre-reward score")
	}
	// Only the active contributor's two-gold reward is available. The game must
	// end before paying other contributors or selecting a hero, not reject it.
	x.Economy.Gold[other] += x.Economy.GoldBank - 2
	x.Economy.GoldBank = 2
	before := slices.Clone(x.Economy.Gold)
	explorerSpiceApply(t, s, Action{Type: "catan_explorer_resolve", Target: tile})
	g, x = s.Catan, s.Catan.Explorer
	if !s.Finished || g.Players[actor].Score != 17 || !slices.Equal(s.Winners, []int{actor}) || x.Lairs.RewardVictory == nil || x.Lairs.Battle != nil || x.Lairs.Progress[other] != 0 || x.Economy.GoldBank != 0 {
		t.Fatal("lair ignored spice points or continued past victory")
	}
	before[actor] += 2
	if !reflect.DeepEqual(before, x.Economy.Gold) || x.Lairs.Sites[0].Resolved != 0 || g.Tiles[tile].Number != 0 {
		t.Fatal("later rewards or liberation continued")
	}
	explorerFishReject(t, s, actor, Action{Type: "catan_end", Prompt: int(g.TurnSerial)})
}
