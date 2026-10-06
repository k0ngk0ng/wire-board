package game

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCatanExplorerFullNetworkSetupFixture(t *testing.T) {
	raw, err := os.ReadFile("../server/testdata/catan_explorer_full_setup.json")
	if err != nil {
		t.Fatal(err)
	}
	var s State
	if err = json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if err = s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	x := s.Catan.Explorer
	if s.Phase != "catan_explorer_setup" || x.Setup == nil || x.Setup.Step != 0 || len(s.Catan.Players) != 3 || s.Catan.RollID != 0 || x.Board.Scenario != "explorers-and-pirates" || x.Lairs == nil || x.Fish == nil || x.Spice == nil || x.Board.Target != 17 {
		t.Fatal("not a normal full-scenario setup")
	}
	for _, p := range s.Catan.Players {
		if sum(p.Resources) != 0 || p.Score != 0 {
			t.Fatal("fixture injected resources or scores")
		}
	}
	for step := 0; s.CatanPendingActor() >= 0; step++ {
		if step >= 12 {
			t.Fatal("automatic setup stalled")
		}
		s.AutoCatanPending()
		s = *explorerStateRestore(t, &s)
	}
	if s.Phase != "catan_roll" || s.Catan.TurnSerial != 1 {
		t.Fatal("fixture failed normal setup")
	}
}

// Controlled response snapshot: revealed map, one shared ready lair, permanent
// farm crew and loaded fish/spice coexist. It is not a natural match fixture;
// artificial lair token values remain explicitly outside the release recipe.
func TestCatanExplorerFullNetworkResponseFixture(t *testing.T) {
	raw, err := os.ReadFile("../server/testdata/catan_explorer_full_resolve.json")
	if err != nil {
		t.Fatal(err)
	}
	var s State
	if err = json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if err = s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	x := s.Catan.Explorer
	actor, other, serial := s.Turn, (s.Turn+1)%3, s.Catan.TurnSerial
	if s.Phase != "catan_explorer_resolve" || x.Board.Target != 17 || x.Lairs == nil || x.Spice == nil || x.Fish == nil {
		t.Fatal("not a full response fixture")
	}
	ready, permanent, spice, fish := 0, 0, 0, 0
	for _, site := range x.Lairs.Sites {
		if site.Ready > 0 {
			ready++
		}
	}
	for _, loc := range x.Cargo.Units {
		if loc.Kind == "farm" {
			permanent++
		}
	}
	for _, sack := range x.Cargo.Spice {
		if sack.At.Kind == "ship" {
			spice++
		}
	}
	for _, loc := range x.Cargo.Fish {
		if loc.Kind == "ship" {
			fish++
		}
	}
	if ready != 1 || permanent != 1 || spice != 1 || fish != 1 {
		t.Fatal("missing combined boundary", ready, permanent, spice, fish)
	}
	before := clone(*x)
	a, err := s.BotAction(actor)
	if err != nil || a.Type != "catan_explorer_resolve" {
		t.Fatal(a, err)
	}
	explorerSpiceApply(t, &s, a)
	if s.Phase != "catan_explorer_battle" || len(s.Catan.Explorer.Lairs.Battle.Candidates) != 2 {
		t.Fatal("missing shared hero battle")
	}
	for step := 0; s.Phase == "catan_explorer_battle"; step++ {
		if step >= 60 {
			t.Fatal("battle stalled")
		}
		a, err = s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		explorerSpiceApply(t, &s, a)
	}
	x = s.Catan.Explorer
	if s.Phase != "catan_roll" || s.Turn != other || s.Catan.TurnSerial != serial+1 {
		t.Fatal("response did not finish turn")
	}
	if !reflect.DeepEqual(x.Cargo.Spice, before.Cargo.Spice) || !reflect.DeepEqual(x.Cargo.Fish, before.Cargo.Fish) || !reflect.DeepEqual(x.Spice, before.Spice) || !reflect.DeepEqual(x.Fish, before.Fish) {
		t.Fatal("lair response changed other mission cargo or records")
	}
	for p := 0; p < 3; p++ {
		want := before.Economy.Gold[p]
		if p == actor || p == other {
			want += 2
		}
		if x.Economy.Gold[p] != want {
			t.Fatal("incorrect contribution reward", p)
		}
	}
}
