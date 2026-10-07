package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

// This is a shared-city-rules boundary fixture, not a playable Explorer+CK
// constructor. The combined setup/economy/event flow remains separately gated.
func mixedCityHarborFixture(t *testing.T) *State {
	t.Helper()
	s := ckEmptyTurn(t)
	g := s.Catan
	g.Vertices = []CatanVertex{
		{ID: 0, Owner: 0, Level: 2},
		{ID: 1, Owner: 0, Level: 2, Harbor: true},
		{ID: 2, Owner: 1, Level: 2, Harbor: true},
		{ID: 3, Owner: 2, Level: 1},
	}
	g.Edges = nil
	g.Tiles = []CatanTile{{ID: 0, Resource: 0, Number: 6, Vertices: []int{0, 1, 2, 3}}}
	return s
}

func TestCatanBuildingKindCityInventoryWallsAndMetropolis(t *testing.T) {
	s := mixedCityHarborFixture(t)
	g, k := s.Catan, s.Catan.CitiesKnights
	for _, v := range []int{-1, 99} {
		if g.cityAt(v) || g.harborAt(v) || catanExplorerHarborAt(g, v) {
			t.Fatal("invalid vertex classified")
		}
	}
	if !g.cityAt(0) || g.harborAt(0) || g.cityAt(1) || !g.harborAt(1) || catanExplorerHarborAt(g, 0) || !catanExplorerHarborAt(g, 1) {
		t.Fatal("city/harbor classification")
	}
	cargo, fleet := catanExplorerCargo{}, &catanExplorerSailing{}
	if cargo.holder(g, fleet, 0, catanExplorerCargoLocation{"harbor", 0}) || !cargo.holder(g, fleet, 0, catanExplorerCargoLocation{"harbor", 1}) || cargo.holder(g, fleet, 1, catanExplorerCargoLocation{"harbor", 1}) {
		t.Fatal("cargo berth permits a city or foreign harbor")
	}
	if _, villages, cities := g.pieces(0); villages != 0 || cities != 1 || g.cityPiecesLeft(0) != 3 {
		t.Fatal("harbor spent a city piece")
	}
	if !slices.Equal(g.cityMetropolisSites(0), []int{0}) || !slices.Equal(g.pillageSites(0), []int{0}) || len(g.cityMetropolisSites(1)) != 0 || len(g.pillageSites(1)) != 0 {
		t.Fatal("harbor is a metropolis or pillage candidate")
	}
	k.Walls = []int{1} // Even a stale/corrupt wall on a harbor grants no limit.
	if g.catanDiscardLimit(0) != 7 {
		t.Fatal("harbor grants wall discard bonus")
	}
	k.Walls = nil
	for r := range g.Bank {
		g.Players[0].Resources[r] = 3
		g.Bank[r] -= 3
		g.Players[1].Resources[r] = 3
		g.Bank[r] -= 3
	}
	before := clone(*s)
	if err := s.catanCityBuild(0, Action{Type: "catan_wall", Vertex: 1}, 0); err == nil {
		t.Fatal("wall accepted on harbor")
	}
	if !reflect.DeepEqual(*s, before) {
		t.Fatal("rejected harbor wall mutated state")
	}
	if err := s.catanCityBuild(0, Action{Type: "catan_wall", Vertex: 0}, 0); err != nil {
		t.Fatal(err)
	}
	if g.catanDiscardLimit(0) != 9 {
		t.Fatal("real city wall missing")
	}
	s.Turn = 1
	before = clone(*s)
	if err := s.catanCityBuild(1, Action{Type: "catan_improvement", Color: 0}, 0); err == nil {
		t.Fatal("harbor alone unlocked city improvements")
	}
	if !reflect.DeepEqual(*s, before) {
		t.Fatal("rejected improvement changed state")
	}
	if len(g.cityEconomyBotChoices(1)) != 0 {
		t.Fatal("bot offers improvements/walls without a city")
	}
	if g.canCityUpgrade(0, 1) {
		t.Fatal("harbor can be converted to a city")
	}
	ckSupply(t, g)
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if !restored.Catan.harborAt(1) || !restored.Catan.cityAt(0) {
		t.Fatal("save lost distinct buildings")
	}
}

func TestCatanBuildingKindInvalidHarborMarkerRejected(t *testing.T) {
	s, err := newCatanExplorerState(3)
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []int{0, 1} {
		candidate := clone(*s)
		found := false
		for i := range candidate.Catan.Vertices {
			v := &candidate.Catan.Vertices[i]
			if v.Level == level {
				v.Harbor = true
				found = true
				break
			}
		}
		if !found {
			t.Fatal("missing candidate", level)
		}
		if err := candidate.validateCatanExplorer(); err == nil {
			t.Fatal("harbor marker accepted on empty vertex/village", level)
		}
	}
}

func TestCatanBuildingKindHarborsDoNotDefendOrAttractBarbarians(t *testing.T) {
	s := mixedCityHarborFixture(t)
	k := s.Catan.CitiesKnights
	k.Event = &CatanCityEvent{}
	k.Knights = []CatanKnight{{Owner: 0, Vertex: 3, Strength: 1, Active: true}}
	s.catanPrepareBarbarians()
	// One active level-one knight beats one city; counting either harbor would
	// incorrectly lose this battle and trigger pillaging.
	if k.Players[0].DefenderPoints != 1 || len(k.Event.Tasks) != 0 {
		t.Fatal("harbors increased barbarian strength", k.Event)
	}
	s = mixedCityHarborFixture(t)
	k = s.Catan.CitiesKnights
	k.Event = &CatanCityEvent{}
	s.catanPrepareBarbarians()
	if len(k.Event.Tasks) != 1 || k.Event.Tasks[0].Kind != "pillage" || k.Event.Tasks[0].Player != 0 {
		t.Fatal("harbor-only owner was pillaged", k.Event.Tasks)
	}
}

func TestCatanBuildingKindHarborProductionIsOneResource(t *testing.T) {
	for _, terrain := range []int{0, 1, 2, 3, 4} {
		s := mixedCityHarborFixture(t)
		g := s.Catan
		g.Tiles[0].Resource = terrain
		if err := s.catanRollProductionEffect(6, false); err != nil {
			t.Fatal(err)
		}
		// Player zero has city+harbor, player one only a harbor, player two a village.
		commodity := -1
		switch terrain {
		case 0:
			commodity = 5
		case 2:
			commodity = 6
		case 4:
			commodity = 7
		}
		expected := make([]int, 8)
		expected[terrain] = 3
		if commodity >= 0 {
			expected[terrain] = 2
			expected[commodity] = 1
		}
		if !slices.Equal(g.Players[0].Resources, expected) {
			t.Fatal("city plus harbor production", terrain, g.Players[0].Resources, expected)
		}
		for _, p := range []int{1, 2} {
			want := make([]int, 8)
			want[terrain] = 1
			if !slices.Equal(g.Players[p].Resources, want) {
				t.Fatal("harbor/village produced commodity or doubled", terrain, p, g.Players[p].Resources)
			}
		}
		ckSupply(t, g)
	}
}

func TestCatanBuildingKindLegacyExplorerHarbors(t *testing.T) {
	s, err := newCatanExplorerState(3)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	count := 0
	for i := range g.Vertices {
		if g.Vertices[i].Level == 2 {
			count++
			if !g.Vertices[i].Harbor {
				t.Fatal("new harbor missing marker")
			}
			g.Vertices[i].Harbor = false
		}
	}
	if count != 3 {
		t.Fatal("fixture harbor count")
	}
	restored := explorerStateRestore(t, s)
	for _, v := range restored.Catan.Vertices {
		if v.Level == 2 && (!restored.Catan.harborAt(v.ID) || restored.Catan.cityAt(v.ID) || !catanExplorerHarborAt(restored.Catan, v.ID)) {
			t.Fatal("old level-only harbor lost")
		}
	}
	for p := range g.Players {
		if _, _, cities := restored.Catan.pieces(p); cities != 0 {
			t.Fatal("legacy harbor consumes city inventory")
		}
	}
}
