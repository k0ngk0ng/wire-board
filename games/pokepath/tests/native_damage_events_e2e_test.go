package pokepath_test

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func damageEvent(t *testing.T, state *engine.State, target int, kind string) map[string]any {
	t.Helper()
	for index := len(state.Events) - 1; index >= 0; index-- {
		event := state.Events[index]
		if event.Type != "hit" || event.Target != target {
			continue
		}
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err = json.Unmarshal(encoded, &result); err != nil {
			t.Fatal(err)
		}
		if result["sourceKind"] == kind {
			for _, key := range []string{"amount", "hpDamage", "armorDamage", "shieldDamage", "attemptedDamage", "resolvedDamage", "displayHPDamage", "displayArmorDamage"} {
				if _, exists := result[key]; !exists {
					result[key] = float64(0)
				}
			}
			return result
		}
	}
	t.Fatalf("missing actual %s hit event for %d", kind, target)
	return nil
}

func TestNativeDamageEventSeparatesActualLossFromOriginalOverkillLabelsE2E(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "charmander", 1)
	state.Team[0].Stats.CriticalPct = 100
	waBattle(t, state)
	id := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	enemy := persistentAreaEnemy(t, state, id)
	enemy.HP, enemy.MaxHP, enemy.Armor, enemy.MaxArmor = 3, 70, 5, 5
	waAdvance(t, state, 20)
	event := damageEvent(t, state, id, "physical")
	if event["critical"] != true || event["hpDamage"] != float64(3) || event["armorDamage"] != float64(5) || event["attemptedDamage"] != float64(23) || event["displayHPDamage"] != float64(18) || event["displayArmorDamage"] != float64(5) || event["amount"] != float64(8) {
		t.Fatal("hit display metadata guessed capped HP, lost armor, or rerolled critical", event)
	}
	if state.TotalDamage != 8 || persistentAreaEnemy(t, state, id).HP != 0 || persistentAreaEnemy(t, state, id).Armor != 0 {
		t.Fatal("display event changed the actual overkill settlement")
	}
	waEvidence(t, "damage-event-overkill", state, map[string]any{"actualHP": 3, "actualArmor": 5, "displayHP": 18, "displayArmor": 5, "critical": true})
}

func TestNativeDamageEventsCarryEachActualConditionSourceE2E(t *testing.T) {
	for _, scenario := range []struct {
		kind      string
		magnitude float64
		resolved  float64
	}{{"burn", .005, 50}, {"poison", .001, 10}, {"nightmare", 25, 3}, {"scorch", .005, 10}} {
		t.Run(scenario.kind, func(t *testing.T) {
			state := waScenario(t)
			owner := waPartner(t, state, "charmander", 1)
			waBattle(t, state)
			id := waEnemy(t, state, engine.Point{X: 300, Y: 300})
			enemy := persistentAreaEnemy(t, state, id)
			enemy.HP, enemy.MaxHP, enemy.Armor, enemy.MaxArmor = 10000, 10000, 10, 2000
			engine.ApplyConditions(&enemy.EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: scenario.kind, Chance: 1, DurationMS: 10000, Magnitude: scenario.magnitude, Stacks: 1, SourcePokemonID: owner}}}, 0)
			waAdvance(t, state, 1000)
			if state.TotalDamage != 0 {
				t.Fatal("condition damage ignored the fractional frame before its first pulse")
			}
			waAdvance(t, state, 20)
			event := damageEvent(t, state, id, scenario.kind)
			armor := min(10.0, scenario.resolved)
			if event["source"] != float64(owner) || event["critical"] == true || event["hpDamage"] != scenario.resolved-armor || event["armorDamage"] != armor || event["attemptedDamage"] != scenario.resolved || event["amount"] != scenario.resolved {
				t.Fatal("condition event did not describe the settled source and split damage", event)
			}
			if state.TotalDamage != scenario.resolved {
				t.Fatal("condition display duplicated or lost attributed damage")
			}
			waEvidence(t, "damage-event-"+scenario.kind, state, map[string]any{"sourceKind": scenario.kind, "resolved": scenario.resolved, "actualArmor": armor})
		})
	}
}

func TestNativeDamageEventIncludesActualUnattributedWeatherDamageE2E(t *testing.T) {
	state := waScenario(t)
	waBattle(t, state)
	id := waEnemy(t, state, engine.Point{X: 300, Y: 300})
	enemy := persistentAreaEnemy(t, state, id)
	enemy.HP, enemy.MaxHP = 1000, 1000
	state.Weather = "sandstorm"
	waAdvance(t, state, 1000)
	event := damageEvent(t, state, id, "sandstorm")
	if event["hpDamage"] != float64(3) || event["attemptedDamage"] != float64(3) || event["amount"] != float64(3) || persistentAreaEnemy(t, state, id).HP != 997 || state.TotalDamage != 0 {
		t.Fatal("weather display changed physical loss or invented a player damage owner", event)
	}
	waEvidence(t, "damage-event-weather", state, map[string]any{"sourceKind": "sandstorm", "actualHP": 3, "playerAttribution": false})
}

func TestNativeOverdamageAccumulatesResolvedBeforeTheHealthCapE2E(t *testing.T) {
	state := waScenario(t)
	owner := waDeploy(t, state, "charmander", 1)
	state.Team[0].Stats.CriticalPct = 100
	waBattle(t, state)
	id := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	enemy := persistentAreaEnemy(t, state, id)
	enemy.HP, enemy.MaxHP, enemy.Armor, enemy.MaxArmor = 3, 70, 5, 5
	waAdvance(t, state, 20)
	var pokemon map[string]any
	encoded, err := json.Marshal(engine.Project(state).Team[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(encoded, &pokemon); err != nil {
		t.Fatal(err)
	}
	if pokemon["overdamage"] != float64(23) || pokemon["damage"] != float64(8) {
		t.Fatal("overdamage was missing or incorrectly represented only the overkill difference", pokemon["overdamage"], pokemon["damage"])
	}
	state.Enemies = []engine.Enemy{}
	for index := range state.Towers {
		state.Towers[index].CooldownMS = 100000
	}
	id = waEnemy(t, state, engine.Point{X: 300, Y: 300})
	enemy = persistentAreaEnemy(t, state, id)
	enemy.HP, enemy.MaxHP, enemy.Armor, enemy.MaxArmor = 3, 10000, 5, 5
	engine.ApplyConditions(&enemy.EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: .005, Stacks: 1, SourcePokemonID: owner}}}, 0)
	waAdvance(t, state, 1000)
	if state.TotalDamage != 8 {
		t.Fatal("DOT overdamage was accumulated before its first real pulse")
	}
	waAdvance(t, state, 20)
	encoded, err = json.Marshal(engine.Project(state).Team[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(encoded, &pokemon); err != nil {
		t.Fatal(err)
	}
	if pokemon["overdamage"] != float64(73) || pokemon["damage"] != float64(16) {
		t.Fatal("attributed armor-plus-DOT overdamage did not accumulate independently of actual lost health", pokemon["overdamage"], pokemon["damage"])
	}
	waEvidence(t, "damage-overdamage", state, map[string]any{"resolvedTotal": 73, "actualTotal": 16, "sources": []string{"physical", "burn"}})
}
