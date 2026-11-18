package engine

import (
	"bytes"
	"testing"
)

func coreApply(t *testing.T, state *State, actor int, action Action) {
	t.Helper()
	action.Epoch = state.Epoch
	if err := Apply(state, actor, action); err != nil {
		t.Fatalf("%s: %v", action.Type, err)
	}
}

func coreNew(t *testing.T, players int) *State {
	t.Helper()
	state, err := New(Options{Starter: "charmander"}, players, 123456, 1730000000000)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func coreTile(t *testing.T, state *State) int {
	t.Helper()
	for _, tile := range Project(state).Tiles {
		if tile.Terrain == 1 {
			return tile.ID
		}
	}
	t.Fatal("the first route has no grass placement")
	return -1
}

func TestCoreSharedCampaignE2E(t *testing.T) {
	state := coreNew(t, 4)
	if state.Gold != 50 || state.Health != 14 || len(state.Team) != 1 {
		t.Fatal("invalid initial shared world", state.Gold, state.Health, len(state.Team))
	}
	pokemonID := state.Team[0].ID
	coreApply(t, state, 0, Action{Type: "deploy", PokemonID: pokemonID, TileID: coreTile(t, state)})
	beforeGold := state.Gold
	coreApply(t, state, 3, Action{Type: "upgrade", PokemonID: pokemonID})
	if state.Team[0].Level != 2 || state.Gold >= beforeGold || len(state.Towers) != 1 {
		t.Fatal("the second participant did not use the shared world")
	}
	coreApply(t, state, 2, Action{Type: "wave"})
	if !state.WaveActive || len(state.Enemies) == 0 {
		t.Fatal("wave did not spawn actual enemies")
	}
	firstX, firstY := state.Enemies[0].X, state.Enemies[0].Y
	if err := Advance(state, 1000); err != nil {
		t.Fatal(err)
	}
	if len(state.Enemies) > 0 && state.Enemies[0].X == firstX && state.Enemies[0].Y == firstY {
		t.Fatal("enemies did not follow the route")
	}
	for elapsed := 0; elapsed < 180000 && state.WaveActive; elapsed += 100 {
		if err := Advance(state, 100); err != nil {
			t.Fatal(err)
		}
	}
	if state.WaveActive || state.Wave != 2 || state.Health <= 0 {
		t.Fatal("the first complete wave did not resolve", state.Wave, state.Health, state.WaveActive)
	}
	if err := Validate(state); err != nil {
		t.Fatal(err)
	}
}

func TestCoreInvalidActionsAreAtomicE2E(t *testing.T) {
	state := coreNew(t, 2)
	before, _ := Marshal(state)
	for _, attempt := range []struct {
		actor  int
		action Action
	}{
		{-1, Action{Type: "wave", Epoch: state.Epoch}},
		{2, Action{Type: "wave", Epoch: state.Epoch}},
		{0, Action{Type: "wave", Epoch: state.Epoch + 1}},
		{0, Action{Type: "deploy", Epoch: state.Epoch, PokemonID: 99999, TileID: 0}},
		{0, Action{Type: "upgrade", Epoch: state.Epoch, PokemonID: 99999}},
		{0, Action{Type: "route", Epoch: state.Epoch, Route: 46}},
		{0, Action{Type: "setGold", Epoch: state.Epoch, Value: 999999}},
	} {
		if err := Apply(state, attempt.actor, attempt.action); err == nil {
			t.Fatal("invalid action accepted", attempt)
		}
		after, _ := Marshal(state)
		if !bytes.Equal(before, after) {
			t.Fatal("invalid action changed the shared world", attempt)
		}
	}
}

func TestCoreSnapshotContinuationAndFixedStepE2E(t *testing.T) {
	state := coreNew(t, 1)
	coreApply(t, state, 0, Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: coreTile(t, state)})
	coreApply(t, state, 0, Action{Type: "wave"})
	if err := Advance(state, 1557); err != nil {
		t.Fatal(err)
	}
	saved, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Unmarshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 10; index++ {
		if err := Advance(state, 100); err != nil {
			t.Fatal(err)
		}
		for _, duration := range []int64{17, 23, 60} {
			if err := Advance(restored, duration); err != nil {
				t.Fatal(err)
			}
		}
		left, _ := Marshal(state)
		right, _ := Marshal(restored)
		if !bytes.Equal(left, right) {
			t.Fatal("snapshot continuation or substep split changed deterministic rules", index)
		}
	}
	clone, err := Clone(state)
	if err != nil {
		t.Fatal(err)
	}
	clone.Team[0].Level++
	if clone.Team[0].Level == state.Team[0].Level {
		t.Fatal("clone shares mutable owned Pokémon")
	}
}

func TestCorePauseRetainsRulesStateE2E(t *testing.T) {
	state := coreNew(t, 1)
	coreApply(t, state, 0, Action{Type: "wave"})
	if err := Advance(state, 500); err != nil {
		t.Fatal(err)
	}
	coreApply(t, state, 0, Action{Type: "pause"})
	before, _ := Marshal(state)
	if err := Advance(state, 600000); err != nil {
		t.Fatal(err)
	}
	after, _ := Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("paused world advanced time, random state or resources")
	}
	coreApply(t, state, 0, Action{Type: "resume"})
	if err := Advance(state, 100); err != nil {
		t.Fatal(err)
	}
	if state.Clock != 600 {
		t.Fatal("resume replayed offline time", state.Clock)
	}
}

func TestCoreFrozenUpgradePricesE2E(t *testing.T) {
	state := coreNew(t, 1)
	if state.Team[0].UpgradeCost != 40 {
		t.Fatalf("level-one Charmander price = %d, want 40", state.Team[0].UpgradeCost)
	}
	coreApply(t, state, 0, Action{Type: "upgrade", PokemonID: state.Team[0].ID})
	if state.Gold != 10 || state.Team[0].Level != 2 || state.Team[0].UpgradeCost != 49 {
		t.Fatalf("level-two state = gold %d level %d next price %d", state.Gold, state.Team[0].Level, state.Team[0].UpgradeCost)
	}
}

func TestCoreFrozenBulkUpgradeQuotesE2E(t *testing.T) {
	state := coreNew(t, 1)
	quotes := Project(state).Team[0].UpgradeCosts
	if quotes["1"] != 40 || quotes["5"] != 302 || quotes["10"] != 919 {
		t.Fatal("the native UI did not receive the recorded bulk prices", quotes)
	}
}

func TestCoreNewPartnersKeepRecordedDefaultTargetsE2E(t *testing.T) {
	state := coreNew(t, 1)
	for _, scenario := range []struct{ key, target string }{
		{"charmander", "notBurned"},
		{"spinda", "random"},
		{"yamask", "curseable"},
		{"fuecoco", "cursed"},
		{"spoink", "invisible"},
		{"sunflora", "aura"},
	} {
		pokemon, err := newPokemon(state, scenario.key, 1)
		if err != nil {
			t.Fatal(err)
		}
		if pokemon.Target != scenario.target {
			t.Fatal("new partner targeting differs from its recorded default", scenario.key, pokemon.Target, scenario.target)
		}
	}
}
