package engine

import (
	"bytes"
	"testing"
)

func TestCoreIncenseUsesTheActualTowerWaveLifetimeE2E(t *testing.T) {
	state := coreNew(t, 1)
	state.Speed, state.WaveActive, state.Phase = 1, true, "battle"
	id, err := SpawnEnemyAt(state, "rattata", Point{X: 700, Y: 600}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	enemyFor(state, id).BaseSpeed = 0
	if err := Advance(state, 5000); err != nil {
		t.Fatal(err)
	}
	state.Team[0].Item = "fullIncense"
	state.Inventory = append(state.Inventory, OwnedItem{ID: "fullIncense", EquippedBy: state.Team[0].ID})
	coreApply(t, state, 0, Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: coreTile(t, state)})
	for _, scenario := range []struct {
		duration int64
		bonus    float64
	}{{500, 0}, {500, 0}, {100, 0}, {17, 2}} {
		if err := Advance(state, scenario.duration); err != nil {
			t.Fatal(err)
		}
		profile, err := profileFor(state, &state.Towers[0], &state.Team[0], nil, .5)
		if err != nil {
			t.Fatal(err)
		}
		if profile.Power != state.Team[0].Stats.Power+scenario.bonus {
			t.Fatal("incense skipped its completed-second or published-stat boundary", profile.Power, scenario.bonus)
		}
	}
}

func TestCoreLuckyPunchCountsRealCriticalsAndExpiresAtSixSecondsE2E(t *testing.T) {
	state := coreMotionOwner(t, "charmander")
	state.Team[0].Item = "luckyPunch"
	state.Inventory = append(state.Inventory, OwnedItem{ID: "luckyPunch", EquippedBy: state.Team[0].ID})
	state.WaveActive, state.Phase = true, "battle"
	id, err := SpawnEnemyAt(state, "rattata", Point{X: 700, Y: 600}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	enemyFor(state, id).BaseSpeed = 0
	context := AbilityContext{ItemID: "luckyPunch", PokemonID: state.Team[0].ID, Power: 1, Range: 100, AttackMS: 1000, Level: 1, Random: .5, CriticalPct: 100}
	profile, err := BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	projectile := Projectile{TowerID: state.Towers[0].ID, PokemonID: state.Team[0].ID, TargetID: id, Context: context, Profile: profile, SourceProfile: profile, DamageScale: 1}
	if err := applyProjectileHit(state, &projectile, enemyFor(state, id), 1); err != nil {
		t.Fatal(err)
	}
	state.Towers[0].CooldownMS = 10000
	for _, scenario := range []struct {
		duration int64
		bonus    float64
	}{{0, 10}, {5999, 10}, {1, 0}} {
		if err := Advance(state, scenario.duration); err != nil {
			t.Fatal(err)
		}
		profile, err := profileFor(state, &state.Towers[0], &state.Team[0], nil, .5)
		if err != nil {
			t.Fatal(err)
		}
		if profile.Range != state.Team[0].Stats.Range+scenario.bonus {
			t.Fatal("lucky punch did not track the actual critical or its six-second expiry", profile.Range, scenario.bonus)
		}
	}
}

func TestCoreGoldDigitAbilityReadsActualEarnedGoldE2E(t *testing.T) {
	state := coreMotionOwner(t, "gholdengo")
	state.Progression.TotalGold = 12345
	context := contextFor(state, &state.Towers[0], &state.Team[0], nil, .5)
	if context.TotalGoldDigits != 5 {
		t.Fatal("make-it-rain received no actual earned-gold digit count", context.TotalGoldDigits)
	}
}

func TestCoreEffectiveCombatStatProjectionIsReadOnlyE2E(t *testing.T) {
	state := coreMotionOwner(t, "charmander")
	state.Team[0].Item = "fullIncense"
	state.Inventory = append(state.Inventory, OwnedItem{ID: "fullIncense", EquippedBy: state.Team[0].ID})
	coreApply(t, state, 0, Action{Type: "wave"})
	state.Enemies = nil
	id, err := SpawnEnemyAt(state, "rattata", Point{X: 700, Y: 600}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	enemyFor(state, id).BaseSpeed = 0
	if err := Advance(state, 1120); err != nil {
		t.Fatal(err)
	}
	before, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	view := ProjectTransient(state)
	if view.Team[0].CombatStats == nil || view.Team[0].CombatStats.Power != state.Team[0].Stats.Power+2 {
		t.Fatal("the view omitted the actual deployed item modifier")
	}
	after, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("effective stat projection mutated timers, random state or owned stats")
	}
}

func TestCoreIncenseRestoresBeforeTheNextPublishedStatRefreshE2E(t *testing.T) {
	state := coreMotionOwner(t, "charmander")
	coreHoldItem(t, state, "fullIncense")
	coreApply(t, state, 0, Action{Type: "wave"})
	state.Enemies = nil
	id, err := SpawnEnemyAt(state, "rattata", Point{X: 700, Y: 600}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	enemyFor(state, id).BaseSpeed = 0
	if err := Advance(state, 1020); err != nil {
		t.Fatal(err)
	}
	profile, err := profileFor(state, &state.Towers[0], &state.Team[0], nil, .5)
	if err != nil || profile.Power != state.Team[0].Stats.Power {
		t.Fatal("unpublished incense power became visible at the one-second pulse", profile.Power, err)
	}
	encoded, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Unmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := Advance(state, 100); err != nil {
		t.Fatal(err)
	}
	for _, duration := range []int64{31, 23, 46} {
		if err := Advance(restored, duration); err != nil {
			t.Fatal(err)
		}
	}
	left, _ := Marshal(state)
	right, _ := Marshal(restored)
	if !bytes.Equal(left, right) {
		t.Fatal("pending incense stat refresh changed across persistence or chunked advancement")
	}
	profile, err = profileFor(restored, &restored.Towers[0], &restored.Team[0], nil, .5)
	if err != nil || profile.Power != restored.Team[0].Stats.Power+2 {
		t.Fatal("restored incense did not publish the completed pulse", profile.Power, err)
	}
}

func TestCoreRealBurnPulseRetainsItsFractionalFrameAndLastTickE2E(t *testing.T) {
	state := coreMotionOwner(t, "charmander")
	coreApply(t, state, 0, Action{Type: "wave"})
	state.Enemies = nil
	tower := state.Towers[0]
	id, err := SpawnEnemyAt(state, "rattata", Point{X: tower.X + 40, Y: tower.Y}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	enemy := enemyFor(state, id)
	enemy.BaseSpeed, enemy.HP, enemy.MaxHP, enemy.Entered = 0, 1000, 1000, true
	for frame := 0; frame < 120 && !HasCondition(&enemyFor(state, id).EnemyEffectState, "burn"); frame++ {
		if err := Advance(state, 17); err != nil {
			t.Fatal(err)
		}
	}
	if !HasCondition(&enemyFor(state, id).EnemyEffectState, "burn") {
		t.Fatal("the real projectile did not apply burn")
	}
	coreApply(t, state, 0, Action{Type: "retire", PokemonID: state.Team[0].ID})
	count := func(value *State) int {
		amount := 0
		for _, event := range value.Events {
			if event.Type == "hit" && event.SourceKind == "burn" {
				amount++
			}
		}
		return amount
	}
	if err := Advance(state, 1000); err != nil {
		t.Fatal(err)
	}
	if count(state) != 0 {
		t.Fatal("integer milliseconds advanced the first real burn pulse by a frame")
	}
	encoded, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Unmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := Advance(state, 9020); err != nil {
		t.Fatal(err)
	}
	for remaining := int64(9020); remaining > 0; {
		duration := min(int64(137), remaining)
		if err := Advance(restored, duration); err != nil {
			t.Fatal(err)
		}
		remaining -= duration
	}
	left, _ := Marshal(state)
	right, _ := Marshal(restored)
	if !bytes.Equal(left, right) || count(state) != 10 || HasCondition(&enemyFor(state, id).EnemyEffectState, "burn") {
		t.Fatal("fractional burn pulses diverged after restore or omitted the last tick", count(state))
	}
}

func TestCoreSinglePokemonQuerySupportsBoxAndReturnsAnIndependentViewE2E(t *testing.T) {
	state := coreNew(t, 1)
	id := state.Team[0].ID
	state.Box = append(state.Box, state.Team[0])
	state.Team = []Pokemon{}
	state.Box[0].Item = "protein"
	state.Inventory = append(state.Inventory, OwnedItem{ID: "protein", EquippedBy: id})
	for index, itemID := range state.Progression.ShopStock {
		if itemID == "protein" {
			state.Progression.ShopStock[index] = ""
		}
	}
	before, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	pokemon, err := ProjectPokemon(state, id)
	if err != nil {
		t.Fatal(err)
	}
	if pokemon.CombatStats == nil || pokemon.CombatStats.Power != state.Box[0].Stats.Power+15 || pokemon.CombatRangeType == "" {
		t.Fatal("single-UID Box projection omitted authoritative held-item stats or geometry")
	}
	pokemon.UpgradeCosts["1"] = -1
	pokemon.CombatStats.Power = -1
	if _, err := ProjectPokemon(state, 999999); err == nil {
		t.Fatal("unknown owned UID was accepted")
	}
	after, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("single-Pokémon query changed rules state or exposed owned mutable maps")
	}
}
