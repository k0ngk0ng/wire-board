package pokepath_test

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func profileStart(t *testing.T) *engine.State {
	t.Helper()
	state, err := engine.New(engine.Options{Starter: "charmander"}, 2, 123456, 1730000000000)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func profileAct(t *testing.T, state *engine.State, action engine.Action) {
	t.Helper()
	action.Epoch = state.Epoch
	if err := engine.Apply(state, 1, action); err != nil {
		t.Fatal(err)
	}
}

func profileProjection(t *testing.T, state *engine.State) map[string]any {
	t.Helper()
	before, _ := engine.Marshal(state)
	raw, err := json.Marshal(engine.Project(state).Progression)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("reading trainer statistics mutated the live world")
	}
	var projection map[string]any
	if err := json.Unmarshal(raw, &projection); err != nil {
		t.Fatal(err)
	}
	stats, ok := projection["profileStats"].(map[string]any)
	if !ok {
		t.Fatal("trainer statistics are missing from the native projection")
	}
	return stats
}

func TestNativeProfileInitialFactsAndOriginalSettings(t *testing.T) {
	state := profileStart(t)
	stats := profileProjection(t, state)
	for key, expected := range map[string]float64{"pokemonOwned": 1, "highestPokemonLevel": 1, "totalPokemonLevel": 1, "totalGold": 50, "timePlayed": 0, "highestHit": 0, "defeatedEnemies": 0, "defeatedSpecies": 0, "wavesCompleted": 0, "appliedStuns": 0, "appliedSlows": 0, "appliedBurns": 0, "appliedPoisons": 0, "appliedCurses": 0, "resets": 0, "stolenGold": 0, "stars": 0, "maxStars": 4000, "enemyTotal": 345, "shinyAmount": 0} {
		if stats[key] != expected {
			t.Fatalf("initial %s = %v, expected %v", key, stats[key], expected)
		}
	}
	if state.Progression.Settings.Font != "PressStart2P" {
		t.Fatal("default font differs from the original menu setting")
	}
	for _, theme := range []string{"original", "classic", "halloween", "monochrome"} {
		profileAct(t, state, engine.Action{Type: "settings", Name: "theme", Mode: theme})
	}
	for _, font := range []string{"PressStart2P", "Rubik", "Lexend", "DynaPuff", "Unown"} {
		profileAct(t, state, engine.Action{Type: "settings", Name: "font", Mode: font})
	}
	for _, invalid := range []engine.Action{{Type: "settings", Name: "theme", Mode: "modern"}, {Type: "settings", Name: "theme", Mode: "dark"}, {Type: "settings", Name: "font", Mode: "system"}, {Type: "settings", Name: "font", Mode: "NGC"}} {
		invalid.Epoch = state.Epoch
		before, _ := engine.Marshal(state)
		if err := engine.Apply(state, 0, invalid); err == nil {
			t.Fatal("accepted a setting absent from the reference")
		}
		after, _ := engine.Marshal(state)
		if !bytes.Equal(before, after) {
			t.Fatal("invalid settings changed trainer data")
		}
	}
}

func TestNativeProfileNamePortraitAndRecordActionsAreAtomic(t *testing.T) {
	state := profileStart(t)
	profileAct(t, state, engine.Action{Type: "profile", Name: "巡径共同训练家十一字"})
	profileAct(t, state, engine.Action{Type: "portrait", Value: 79})
	raw, _ := json.Marshal(engine.Project(state).Progression)
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	if fields["portrait"] != float64(79) {
		t.Fatal("a partner's portrait choice was not shared")
	}
	for _, action := range []engine.Action{{Type: "profile", Name: "一二三四五六七八九十一二"}, {Type: "portrait", Value: -1}, {Type: "portrait", Value: 80}, {Type: "portrait", Value: 1.5}, {Type: "portrait", Value: math.NaN()}, {Type: "profileRecordReset", Value: 18}} {
		action.Epoch = state.Epoch
		before, _ := engine.Marshal(state)
		if err := engine.Apply(state, 1, action); err == nil {
			t.Fatalf("accepted invalid trainer action: %+v", action)
		}
		after, _ := engine.Marshal(state)
		if !bytes.Equal(before, after) {
			t.Fatal("invalid trainer action changed the snapshot")
		}
	}
	profileAct(t, state, engine.Action{Type: "profileRecordReset", Value: 19})
	profileAct(t, state, engine.Action{Type: "profileRecordReset", Value: 20})
	if err := engine.Apply(state, -1, engine.Action{Type: "portrait", Value: 1, Epoch: state.Epoch}); err == nil {
		t.Fatal("spectator changed the shared portrait")
	}
}

func TestNativeProfileNaturalFirstWaveAndColdRestoration(t *testing.T) {
	state := profileStart(t)
	profileAct(t, state, engine.Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: 1})
	profileAct(t, state, engine.Action{Type: "upgrade", PokemonID: state.Team[0].ID, Count: 1})
	profileAct(t, state, engine.Action{Type: "wave"})
	for index := 0; index < 500 && state.WaveActive; index++ {
		if err := engine.Advance(state, 1000); err != nil {
			t.Fatal(err)
		}
	}
	stats := profileProjection(t, state)
	if stats["highestHit"].(float64) <= 0 || stats["defeatedEnemies"].(float64) <= 0 || stats["defeatedSpecies"].(float64) <= 0 || stats["highestPokemonLevel"] != float64(2) || stats["totalPokemonLevel"] != float64(2) {
		t.Fatal("natural attacks, kills or paid upgrades did not reach trainer statistics")
	}
	if stats["wavesCompleted"] != float64(1) || stats["totalGold"].(float64) <= 50 || stats["maxGoldPerWave"].(map[string]any)["value"].(float64) <= 0 || stats["maxGoldPerWave"].(map[string]any)["routeTag"] != "R1-1 W1" || stats["maxGoldPerTime"].(map[string]any)["value"].(float64) <= 0 {
		t.Fatal("the real first-wave reward was absent from route-tagged trainer records")
	}
	profileAct(t, state, engine.Action{Type: "profileRecordReset", Value: 19})
	cleared := profileProjection(t, state)
	if cleared["maxGoldPerWave"].(map[string]any)["value"] != float64(0) || cleared["maxGoldPerTime"].(map[string]any)["value"] != stats["maxGoldPerTime"].(map[string]any)["value"] {
		t.Fatal("clearing one earned record erased the other record")
	}
	stats = cleared
	raw, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := engine.Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := json.Marshal(stats)
	right, _ := json.Marshal(profileProjection(t, restored))
	if !bytes.Equal(left, right) {
		t.Fatal("cold restore lost trainer statistics")
	}
}

func TestNativeProfileChallengeClearsThePreviousWorldAndAttackChains(t *testing.T) {
	state := profileStart(t)
	state.Gold = 1000000000
	state.Stars = 540
	state.TeamSlots = 10
	for route := 0; route < 5; route++ {
		state.Records[route] = 100
	}
	state.Records[5] = 40
	for len(state.Team)+len(state.Box) < 30 {
		profileAct(t, state, engine.Action{Type: "egg"})
	}
	id := state.Team[0].ID
	profileAct(t, state, engine.Action{Type: "deploy", PokemonID: id, TileID: 1})
	state.World.Honey = 42
	state.AttackModes.Towers = map[int]*engine.AttackModeTower{id: {TowerID: state.Towers[0].ID, HasAim: true, Aim: engine.Point{X: 252, Y: 132}}}
	profileAct(t, state, engine.Action{Type: "challenge", Settings: json.RawMessage(`{"levelCap":50,"slotLimit":6}`)})
	if state.World.Honey != 0 || len(state.World.Towers) != 0 || len(state.AttackModes.Towers) != 0 {
		t.Fatal("a challenge inherited the previous field or attack chain")
	}
}

func TestNativeProfileSuccessfulConditionsCountWithoutRefreshDuplicates(t *testing.T) {
	state := profileStart(t)
	profileAct(t, state, engine.Action{Type: "wave"})
	enemy := &state.Enemies[0]
	for _, kind := range []string{"stun", "slow", "burn", "poison", "curse"} {
		profile := engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: kind, Chance: 1, DurationMS: 1000, Magnitude: 0.2, Stacks: 1}}}
		engine.WorldApplyConditions(state, enemy, profile, 0)
		engine.WorldApplyConditions(state, enemy, profile, 0)
	}
	stats := profileProjection(t, state)
	for key, expected := range map[string]float64{"appliedStuns": 1, "appliedSlows": 1, "appliedBurns": 1, "appliedPoisons": 2, "appliedCurses": 1} {
		if stats[key] != expected {
			t.Fatalf("successful condition count %s = %v, want %v", key, stats[key], expected)
		}
	}
	enemy.Conditions = nil
	enemy.PassiveID = "mummy"
	engine.WorldApplyConditions(state, enemy, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "curse", Chance: 1, DurationMS: -1, Stacks: 1}}}, 0)
	if profileProjection(t, state)["appliedCurses"] != float64(1) {
		t.Fatal("an immune enemy increased curse statistics")
	}
}

func TestNativeProfileGreedCountsActualSharedGoldGains(t *testing.T) {
	state := profileStart(t)
	state.Team[0].Species = "meowth"
	profileAct(t, state, engine.Action{Type: "upgrade", PokemonID: state.Team[0].ID, Count: 1})
	profileAct(t, state, engine.Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: 1})
	profileAct(t, state, engine.Action{Type: "wave"})
	for index := 0; index < 200 && state.WaveActive; index++ {
		if err := engine.Advance(state, 1000); err != nil {
			t.Fatal(err)
		}
	}
	stats := profileProjection(t, state)
	if stats["stolenGold"].(float64) <= 0 || stats["stolenGold"].(float64) > stats["totalGold"].(float64)-50 {
		t.Fatal("real Greed hits were not reflected in the shared stolen-gold total")
	}
}
