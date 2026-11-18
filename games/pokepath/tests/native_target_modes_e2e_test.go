package pokepath_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func targetOptions(t *testing.T, state *engine.State, id int) []string {
	t.Helper()
	pokemon, err := engine.ProjectPokemon(state, id)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(pokemon)
	if err != nil {
		t.Fatal(err)
	}
	var view struct {
		TargetModes []string `json:"targetModes"`
	}
	if err = json.Unmarshal(encoded, &view); err != nil {
		t.Fatal(err)
	}
	return view.TargetModes
}

func TestNativeTargetModeProjectionAndAuthorizationUseTheSameRulesE2E(t *testing.T) {
	for _, scenario := range []struct {
		key, item string
		count     int
	}{{"charmander", "", 20}, {"spoink", "", 21}, {"mareep", "", 0}, {"cyndaquil", "", 0}, {"spinda", "", 0}, {"spinda", "ringTarget", 20}, {"charmander", "quickClaw", 0}, {"charmander", "silphScope", 0}, {"charmander", "spindaCocktail", 0}} {
		t.Run(scenario.key+scenario.item, func(t *testing.T) {
			state := waScenario(t)
			owner := waDeploy(t, state, scenario.key, 1)
			if scenario.item != "" {
				healingEquip(t, state, owner, scenario.item)
			}
			options := targetOptions(t, state, owner)
			if len(options) != scenario.count {
				t.Fatal("native inspect did not expose the actual legal target-mode set", len(options), scenario.count)
			}
			before, _ := engine.Marshal(state)
			err := engine.Apply(state, 0, engine.Action{Type: "target", Epoch: state.Epoch, PokemonID: owner, Mode: "last"})
			if scenario.count == 0 {
				if err == nil {
					t.Fatal("a locked target mode was accepted by the authoritative rule")
				}
				after, _ := engine.Marshal(state)
				if !bytes.Equal(before, after) {
					t.Fatal("invalid locked target request left a partial mutation")
				}
			} else if err != nil {
				t.Fatal("a projected legal target mode was rejected", err)
			}
		})
	}
}

func TestNativeRingTargetActuallyStopsSpindaRandomTargetOverrideE2E(t *testing.T) {
	state := waScenario(t)
	owner := waDeploy(t, state, "spinda", 1)
	healingEquip(t, state, owner, "ringTarget")
	waApply(t, state, 0, engine.Action{Type: "target", PokemonID: owner, Mode: "first"})
	waBattle(t, state)
	first := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 80, Y: state.Towers[0].Y})
	second := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 100, Y: state.Towers[0].Y})
	persistentAreaEnemy(t, state, first).Distance = 100
	persistentAreaEnemy(t, state, second).Distance = 10
	state.Random = 13000
	waAdvance(t, state, 20)
	if len(state.Projectiles) != 1 || state.Projectiles[0].TargetID != first {
		t.Fatal("ring target exposed manual selection but Spinda still overrode it with random targeting")
	}
	waEvidence(t, "target-ring-spinda", state, map[string]any{"mode": "first", "actualTarget": first, "randomOverride": false})
}

func TestNativeCopiedSpindaKeepsTheOriginalTargetLockE2E(t *testing.T) {
	state := waScenario(t)
	waPartner(t, state, "spinda", 1)
	copy := waPartner(t, state, "ditto", 1)
	state.Progression.DNA[copy] = "spinda"
	if len(targetOptions(t, state, copy)) != 0 {
		t.Fatal("copied Spinda unexpectedly allowed a manual target policy")
	}
	if err := engine.Apply(state, 0, engine.Action{Type: "target", Epoch: state.Epoch, PokemonID: copy, Mode: "first"}); err == nil {
		t.Fatal("authoritative target action ignored the DNA target lock")
	}
}

func TestNativeSilphScopeUsesTheForcedInvisiblePriorityE2E(t *testing.T) {
	state := waScenario(t)
	owner := waDeploy(t, state, "charmander", 1)
	healingEquip(t, state, owner, "silphScope")
	waBattle(t, state)
	visible := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 80, Y: state.Towers[0].Y})
	hidden := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 100, Y: state.Towers[0].Y})
	persistentAreaEnemy(t, state, visible).Distance = 100
	persistentAreaEnemy(t, state, hidden).Distance = 10
	persistentAreaEnemy(t, state, hidden).Invisible = true
	waAdvance(t, state, 20)
	if len(state.Projectiles) != 1 || state.Projectiles[0].TargetID != hidden {
		t.Fatal("locked Silph Scope target label did not match the actual invisible-first attack")
	}
	waEvidence(t, "target-silph-scope", state, map[string]any{"forcedMode": "invisible", "actualTarget": hidden})
}
