package pokepath_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func projectedDamageTotals(t *testing.T, state *engine.State) (float64, float64) {
	t.Helper()
	encoded, err := json.Marshal(engine.Project(state))
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err = json.Unmarshal(encoded, &view); err != nil {
		t.Fatal(err)
	}
	actual, actualOK := view["totalDamage"].(float64)
	resolved, resolvedOK := view["totalOverdamage"].(float64)
	if !actualOK || !resolvedOK {
		t.Fatal("view omitted independent original wave damage denominators", view["totalDamage"], view["totalOverdamage"])
	}
	return actual, resolved
}

func TestNativeFinalSharedDamageAndMagmaContributeToHealingThresholdsE2E(t *testing.T) {
	for _, kind := range []string{"eonUnion", "magmaStorm"} {
		t.Run(kind, func(t *testing.T) {
			state := waScenario(t)
			key := "charmander"
			if kind == "magmaStorm" {
				key = "heatran"
			}
			owner := waDeploy(t, state, key, 1)
			healingEquip(t, state, owner, "shellBell")
			waBattle(t, state)
			state.Health = 10
			origin := engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y}
			first := waEnemy(t, state, origin)
			second := waEnemy(t, state, engine.Point{X: origin.X + 40, Y: origin.Y})
			if kind == "eonUnion" {
				persistentAreaEnemy(t, state, first).PassiveID = "eonUnion"
				persistentAreaEnemy(t, state, second).PassiveID = "eonUnion"
				persistentAreaEnemy(t, state, first).Distance = 100
				state.World.Towers[owner].WaveDamage = 49980
			} else {
				state.Towers[0].CooldownMS = 100000
				state.World.Towers[owner].WaveDamage = 49950
				engine.ApplyConditions(&persistentAreaEnemy(t, state, first).EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: .005, Stacks: 1}}}, 0)
				persistentAreaEnemy(t, state, first).HP = 0
			}
			waAdvance(t, state, 20)
			if state.Health != 11 || !state.World.HealingItemUsed["shellBell"] || state.World.HealingDamage[owner] <= 50000 {
				t.Fatal("actual raw hit missed the original shared getDamaged healing path", state.Health, state.World.HealingDamage[owner])
			}
			waSnapshotContinuation(t, state, 40)
			if state.Health != 11 {
				t.Fatal("raw hit claimed the wave healing item more than once")
			}
			waEvidence(t, "final-healing-"+kind, state, map[string]any{"heals": 1, "strictDamageGoal": 50000, "source": kind})
		})
	}
}

func TestNativeFinalStalwartUsesActualForcedTargetModeE2E(t *testing.T) {
	state := waScenario(t)
	owner := waDeploy(t, state, "megaSkarmory", 30)
	healingEquip(t, state, owner, "silphScope")
	waBattle(t, state)
	origin := state.Towers[0]
	visible := waEnemy(t, state, engine.Point{X: origin.X - 300, Y: origin.Y})
	hidden := waEnemy(t, state, engine.Point{X: origin.X + 300, Y: origin.Y})
	persistentAreaEnemy(t, state, visible).Distance = 100
	persistentAreaEnemy(t, state, hidden).Distance = 10
	persistentAreaEnemy(t, state, hidden).Invisible = true
	waAdvance(t, state, 20)
	if state.World.Towers[owner].FlightTarget != hidden || state.Towers[0].X <= origin.X {
		t.Fatal("Stalwart chase ignored Silph Scope's actual invisible-first policy", state.World.Towers[owner].FlightTarget, hidden)
	}
	waSnapshotContinuation(t, state, 80)
	waEvidence(t, "final-stalwart-scope", state, map[string]any{"flightTarget": hidden, "forcedMode": "invisible"})
}

func TestNativeFinalZeroPowerAppliesPoisonWithoutInventingPhysicalDamageE2E(t *testing.T) {
	state := waScenario(t)
	owner := waDeploy(t, state, "gulpin", 1)
	waBattle(t, state)
	id := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	enemy := persistentAreaEnemy(t, state, id)
	enemy.LastHPDamage, enemy.LastIncomingDamage, enemy.LastResolvedDamage = 987, 987, 987
	waAdvance(t, state, 20)
	enemy = persistentAreaEnemy(t, state, id)
	event := damageEvent(t, state, id, "physical")
	if enemy.HP != 100000 || !engine.HasCondition(&enemy.EnemyEffectState, "poison") || event["amount"] != float64(0) || event["attemptedDamage"] != float64(0) || state.Team[0].Damage != 0 || state.Team[0].Overdamage != 0 {
		t.Fatal("real zero-power poison field invented physical damage or retained prior damage metadata", enemy.HP, event)
	}
	waAdvance(t, state, 1000)
	if persistentAreaEnemy(t, state, id).HP != 100000 {
		t.Fatal("zero-power poison advanced before the real fractional pulse boundary")
	}
	waAdvance(t, state, 20)
	if persistentAreaEnemy(t, state, id).HP >= 100000 || state.Team[0].Damage <= 0 || state.Team[0].ID != owner {
		t.Fatal("zero physical power accidentally disabled its later real poison tick")
	}
	waEvidence(t, "final-zero-power", state, map[string]any{"species": "gulpin", "powerBase": 0, "powerScale": 0, "directDamage": 0, "poisonStillTicks": true})
}

func TestNativeFinalDamageDenominatorsSurviveRetirementTeamSwapAndSaveE2E(t *testing.T) {
	state := waScenario(t)
	owner := waDeploy(t, state, "charmander", 1)
	state.Team[0].Stats.CriticalPct = 100
	waBattle(t, state)
	id := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	enemy := persistentAreaEnemy(t, state, id)
	enemy.HP, enemy.MaxHP, enemy.Armor, enemy.MaxArmor = 3, 70, 5, 5
	waAdvance(t, state, 20)
	assertTotals := func(actual, resolved float64) {
		t.Helper()
		gotActual, gotResolved := projectedDamageTotals(t, state)
		if gotActual != actual || gotResolved != resolved {
			t.Fatal("wave denominators were rebuilt from current members or lost overkill", gotActual, gotResolved, actual, resolved)
		}
	}
	assertTotals(8, 23)
	waApply(t, state, 0, engine.Action{Type: "retire", PokemonID: owner})
	if state.Team[0].Damage != 8 || state.Team[0].Overdamage != 23 {
		t.Fatal("tower retirement reset a partner that is still in the team")
	}
	waApply(t, state, 0, engine.Action{Type: "team", PokemonID: owner, Mode: "box"})
	if state.Box[0].Damage != 0 || state.Box[0].Overdamage != 0 {
		t.Fatal("removing a partner from the team kept original wave display counters")
	}
	assertTotals(8, 23)
	waApply(t, state, 0, engine.Action{Type: "team", PokemonID: owner, Mode: "team"})
	if state.Team[0].Damage != 0 || state.Team[0].Overdamage != 0 {
		t.Fatal("adding a partner retained stale display damage")
	}
	assertTotals(8, 23)
	waSnapshotContinuation(t, state, 40)
	assertTotals(8, 23)
	state.Enemies = []engine.Enemy{}
	waAdvance(t, state, 20)
	waBattle(t, state)
	assertTotals(0, 0)
	if state.Team[0].Damage != 0 || state.Team[0].Overdamage != 0 {
		t.Fatal("new wave failed to reset current team damage displays")
	}
	waEvidence(t, "final-damage-denominators", state, map[string]any{"actualBeforeReset": 8, "resolvedBeforeReset": 23, "retirementKeepsCounters": true, "teamSwapKeepsGlobal": true, "newWaveResets": true})
}

func TestNativeFinalNewWaveAndRouteResetExistingTeamDamageDisplaysE2E(t *testing.T) {
	for _, action := range []string{"wave", "route", "loadTeam"} {
		t.Run(action, func(t *testing.T) {
			state := waScenario(t)
			waDeploy(t, state, "charmander", 1)
			waApply(t, state, 0, engine.Action{Type: "saveTeam", Slot: 0})
			waBattle(t, state)
			waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
			waAdvance(t, state, 20)
			if state.Team[0].Damage != 15 {
				t.Fatal("controlled original direct hit did not reach display counters")
			}
			state.Enemies = []engine.Enemy{}
			waAdvance(t, state, 20)
			if action == "wave" {
				waBattle(t, state)
			} else {
				waApply(t, state, 0, engine.Action{Type: action, Route: 0, Slot: 0})
			}
			if state.Team[0].Damage != 0 || state.Team[0].Overdamage != 0 {
				t.Fatal("original wave or team transition retained stale damage display", action, state.Team[0].Damage, state.Team[0].Overdamage)
			}
			actual, resolved := projectedDamageTotals(t, state)
			want := 0.0
			if action == "loadTeam" {
				want = 15
			}
			if actual != want || resolved != want {
				t.Fatal("transition reset the wrong global denominator", action, actual, resolved)
			}
		})
	}
}

func TestNativeFinalFireLashAnimationUsesScaledTimeAndOneInstantHitE2E(t *testing.T) {
	for _, speed := range []float64{.4, 5} {
		state := waScenario(t)
		waDeploy(t, state, "heatmor", 1)
		waBattle(t, state)
		state.Speed = speed
		id := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
		waAdvance(t, state, 17)
		if len(state.AttackModes.Effects) != 1 || state.AttackModes.Effects[0].WallClock || state.AttackModes.Effects[0].AgeMS != int64(1000*speed/60) {
			t.Fatal("fire lash used an unscaled visual clock", speed, state.AttackModes.Effects)
		}
		waAdvance(t, state, 33)
		if state.AttackModes.Effects[0].AgeMS != int64(50*speed) || persistentAreaEnemy(t, state, id).ReceivedHits != 1 {
			t.Fatal("fire lash phase or instant hit changed with game speed", speed)
		}
		finalFrame := math.Ceil(400 / (1000 * speed / 60))
		waSnapshotContinuation(t, state, int64(math.Ceil(finalFrame*1000/60))-state.Clock)
		if len(state.AttackModes.Effects) != 0 || persistentAreaEnemy(t, state, id).ReceivedHits != 1 {
			t.Fatal("fire lash did not expire after its scaled 400 ms or repeated damage", speed)
		}
		waEvidence(t, "final-fire-lash-clock", state, map[string]any{"speed": speed, "scaledLifetimeMS": 400, "hits": 1})
	}
}

func TestNativeFinalDNAResetsBothPartnerCountersWithoutRemovingGlobalDamageE2E(t *testing.T) {
	state := waScenario(t)
	target := waPartner(t, state, "charmander", 1)
	owner := waPartner(t, state, "ditto", 1)
	waApply(t, state, 0, engine.Action{Type: "dna", PokemonID: owner, TargetID: target})
	tile := waPlacement(t, state, 1)
	waApply(t, state, 0, engine.Action{Type: "deploy", PokemonID: owner, TileID: tile.ID})
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	waAdvance(t, state, 20)
	waApply(t, state, 0, engine.Action{Type: "retire", PokemonID: owner})
	waApply(t, state, 0, engine.Action{Type: "dna", PokemonID: owner, TargetID: target})
	pokemon, err := engine.ProjectPokemon(state, owner)
	if err != nil {
		t.Fatal(err)
	}
	if pokemon.Damage != 0 || pokemon.Overdamage != 0 || state.World.HealingDamage[owner] != 0 {
		t.Fatal("DNA transformation only reset one display or retained the old healing threshold", pokemon.Damage, pokemon.Overdamage, state.World.HealingDamage[owner])
	}
	actual, resolved := projectedDamageTotals(t, state)
	if actual != 15 || resolved != 15 {
		t.Fatal("DNA transformation removed the previous form's global contribution", actual, resolved)
	}
}
