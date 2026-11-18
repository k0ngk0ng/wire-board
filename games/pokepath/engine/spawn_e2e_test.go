package engine

import (
	"math"
	"testing"
)

func TestCoreFullWaveSpacingAndSpriteOriginsE2E(t *testing.T) {
	state := coreNew(t, 1)
	coreApply(t, state, 0, Action{Type: "wave"})
	if len(state.Enemies) != 3 {
		t.Fatal("the whole first wave was not instantiated together", len(state.Enemies))
	}
	for index, enemy := range state.Enemies {
		if enemy.SpawnAt != 0 || math.Abs(enemy.X-382.5) > 0.000001 || math.Abs(enemy.Y-(-90-float64(index)*50)) > 0.000001 || enemy.Entered || enemy.Distance != 0 {
			t.Fatal("spawn does not match the recorded first-route sprite origin", index, enemy.X, enemy.Y, enemy.SpawnAt)
		}
	}
	if err := Advance(state, 2400); err != nil {
		t.Fatal(err)
	}
	if !state.Enemies[0].Entered || state.Enemies[1].Entered || state.Enemies[1].Distance != 0 {
		t.Fatal("offscreen enemies became legal targets or accumulated target progress")
	}
}

func TestCoreFiveWaveScalingAndFrozenExtraGoldE2E(t *testing.T) {
	for _, scenario := range []struct {
		wave   int
		health float64
		gold   int64
	}{
		{1, 70, 6},
		{6, 71, 6},
		{26, 77, 10},
		{100, 70, 23},
	} {
		state := coreNew(t, 1)
		state.Wave = scenario.wave
		state.Progression.ExtraGold = 2
		id, err := SpawnEnemyAt(state, "rattata", Point{X: 100, Y: 100}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		enemy := enemyFor(state, id)
		if enemy.HP != scenario.health || enemy.Gold != scenario.gold {
			t.Fatal("five-wave scaling order differs from the reference facts", scenario.wave, enemy.HP, enemy.Gold)
		}
		state.Progression.ExtraGold = 100
		if enemy.Gold != scenario.gold {
			t.Fatal("a later progression change rewrote an already spawned enemy reward")
		}
	}
}

func TestCoreZeroPowerEscapeDoesNotTriggerHeartLossAbilitiesE2E(t *testing.T) {
	state := coreNew(t, 1)
	route, _ := routeForState(state)
	for _, tile := range route.Placements {
		if tile.Terrain != 1 {
			continue
		}
		if len(state.Team) <= len(state.Towers) {
			pokemon, err := newPokemon(state, "charmander", 1)
			if err != nil {
				t.Fatal(err)
			}
			state.Team = append(state.Team, pokemon)
		}
		coreApply(t, state, 0, Action{Type: "deploy", PokemonID: state.Team[len(state.Towers)].ID, TileID: tile.ID})
		if len(state.Towers) == 3 {
			break
		}
	}
	id, err := SpawnEnemyAt(state, "mew", Point{X: float64(route.Width) + 100, Y: 100}, 0, len(route.Paths[0])-1)
	if err != nil {
		t.Fatal(err)
	}
	enemyFor(state, id).BaseSpeed = 0
	state.WaveActive, state.Phase = true, "battle"
	if err := Advance(state, 25); err != nil {
		t.Fatal(err)
	}
	if state.Health != state.MaxHealth || state.WaveHeartsLost != 0 || state.World.HitsReceived != 0 {
		t.Fatal("Mew's zero-power full-health escape triggered heart-loss abilities", state.Health, state.World.HitsReceived)
	}
}
