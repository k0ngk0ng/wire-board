package engine

import (
	"bytes"
	"math"
	"testing"
)

func coreMotionOwner(t *testing.T, key string) *State {
	t.Helper()
	state := coreNew(t, 1)
	pokemon, err := newPokemon(state, key, 1)
	if err != nil {
		t.Fatal(err)
	}
	state.Team = []Pokemon{pokemon}
	route, _ := routeForState(state)
	for _, tile := range route.Placements {
		if terrainAllowed(pokemon, CatalogData().Species[key], tile.Terrain) {
			coreApply(t, state, 0, Action{Type: "deploy", PokemonID: pokemon.ID, TileID: tile.ID})
			state.Speed = 1
			return state
		}
	}
	t.Fatal("owner has no legal deployment")
	return nil
}

func TestCoreOrbitalsPersistBetweenPreparationAndCombatE2E(t *testing.T) {
	state := coreMotionOwner(t, "cyndaquil")
	if len(state.Projectiles) != 3 {
		t.Fatal("deploying Cyndaquil did not create its three permanent orbitals", len(state.Projectiles))
	}
	origin := Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	radius := state.Team[0].Stats.Range
	for _, projectile := range state.Projectiles {
		if projectile.Kind != "orbit" || math.Abs(math.Hypot(projectile.X-origin.X, projectile.Y-origin.Y)-radius) > 0.000001 {
			t.Fatal("orbital was not on the recorded circular radius")
		}
	}
	if err := Advance(state, 500); err != nil {
		t.Fatal(err)
	}
	before := state.Projectiles[0]
	coreApply(t, state, 0, Action{Type: "wave"})
	if state.Projectiles[0].ID != before.ID || state.Projectiles[0].Angle != before.Angle {
		t.Fatal("starting a wave recreated permanent orbital angles")
	}
	snapshot, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Unmarshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := Advance(state, 500); err != nil {
		t.Fatal(err)
	}
	if err := Advance(restored, 500); err != nil {
		t.Fatal(err)
	}
	left, _ := Marshal(state)
	right, _ := Marshal(restored)
	if !bytes.Equal(left, right) {
		t.Fatal("orbital restoration changed angular position or source state")
	}
	coreApply(t, state, 0, Action{Type: "retire", PokemonID: state.Team[0].ID})
	if len(state.Projectiles) != 0 {
		t.Fatal("retiring the source left live orbitals")
	}
}

func TestCoreReturningProjectileReachesHomeAfterTargetDiesE2E(t *testing.T) {
	state := coreMotionOwner(t, "cubone")
	state.WaveActive, state.Phase = true, "battle"
	tower := state.Towers[0]
	id, err := SpawnEnemyAt(state, "rattata", Point{X: tower.X + 70, Y: tower.Y}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	enemy := enemyFor(state, id)
	enemy.Entered, enemy.BaseSpeed = true, 0
	enemy.HP, enemy.MaxHP = 10000, 10000
	if err := Advance(state, 100); err != nil {
		t.Fatal(err)
	}
	if len(state.Projectiles) != 1 || !state.Projectiles[0].Profile.Returning {
		t.Fatal("Cubone did not launch its actual returning projectile")
	}
	projectileID := state.Projectiles[0].ID
	enemyFor(state, id).HP = 0
	if err := Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, projectile := range state.Projectiles {
		if projectile.ID == projectileID {
			found = true
			if projectile.MotionPhase != "return" {
				t.Fatal("dead-target boomerang kept moving outward", projectile.MotionPhase)
			}
		}
	}
	if !found {
		t.Fatal("a dead target deleted the returning projectile instead of sending it home")
	}
	state.Towers[0].CooldownMS = 10000
	if err := Advance(state, 2000); err != nil {
		t.Fatal(err)
	}
	for _, projectile := range state.Projectiles {
		if projectile.ID == projectileID {
			t.Fatal("returning projectile failed to reach its source")
		}
	}
}

func TestCoreProjectileRetargetsNearbyAfterItsTargetDiesE2E(t *testing.T) {
	state := coreMotionOwner(t, "charmander")
	state.WaveActive, state.Phase = true, "battle"
	tower := state.Towers[0]
	ids := []int{}
	for _, offset := range []float64{80, 120} {
		id, err := SpawnEnemyAt(state, "rattata", Point{X: tower.X + offset, Y: tower.Y}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		enemy := enemyFor(state, id)
		enemy.Entered, enemy.BaseSpeed = true, 0
		enemy.HP, enemy.MaxHP = 10000, 10000
		enemy.Distance = 200 - offset
		ids = append(ids, id)
	}
	if err := Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	if len(state.Projectiles) != 1 || state.Projectiles[0].TargetID != ids[0] {
		t.Fatal("owner did not fire at the first enemy")
	}
	enemyFor(state, ids[0]).HP = 0
	if err := Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	if len(state.Projectiles) != 1 || state.Projectiles[0].TargetID != ids[1] {
		t.Fatal("the in-flight projectile disappeared rather than retargeting within 200 px")
	}
}

func TestCoreRicochetCannotCrossMoreThanTwoHundredPixelsE2E(t *testing.T) {
	state := coreMotionOwner(t, "froaki")
	state.WaveActive, state.Phase = true, "battle"
	tower := state.Towers[0]
	ids := []int{}
	for _, offset := range []float64{60, 300} {
		id, err := SpawnEnemyAt(state, "rattata", Point{X: tower.X + offset, Y: tower.Y}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		enemy := enemyFor(state, id)
		enemy.Entered, enemy.BaseSpeed = true, 0
		enemy.HP, enemy.MaxHP = 10000, 10000
		enemy.Distance = 400 - offset
		ids = append(ids, id)
	}
	if err := Advance(state, 250); err != nil {
		t.Fatal(err)
	}
	if enemyFor(state, ids[0]).HP >= 10000 {
		t.Fatal("the actual primary projectile did not impact")
	}
	if len(state.Projectiles) != 0 || enemyFor(state, ids[1]).HP != 10000 {
		t.Fatal("a ricochet targeted an enemy beyond its 200 px search radius")
	}
}

func coreOrdinaryProjectileScene(t *testing.T, offsets ...Point) (*State, []int) {
	t.Helper()
	state := coreMotionOwner(t, "froaki")
	if state.Towers[0].TileID != 1 {
		coreApply(t, state, 0, Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: 1})
	}
	state.WaveActive, state.Phase = true, "battle"
	state.Towers[0].CooldownMS = 0
	ids := []int{}
	for index, p := range offsets {
		id, err := SpawnEnemyAt(state, "rattata", p, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		enemy := enemyFor(state, id)
		enemy.Entered, enemy.BaseSpeed = true, 0
		enemy.HP, enemy.MaxHP, enemy.Distance = 10000, 10000, 500-float64(index)
		ids = append(ids, id)
	}
	coreApply(t, state, 0, Action{Type: "target", PokemonID: state.Team[0].ID, Mode: "first"})
	return state, ids
}

func TestCoreOrdinaryProjectileLaunchKeepsTheFractionalSpriteCenter(t *testing.T) {
	state, _ := coreOrdinaryProjectileScene(t, Point{X: 340.25, Y: 130})
	if err := Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	if len(state.Projectiles) != 1 {
		t.Fatal("ordinary Froakie shot missing")
	}
	p := state.Projectiles[0]
	if math.Abs(p.X-257.25) > .000001 || p.Y != 130 {
		t.Fatalf("first displayed center = (%v, %v), want (257.25, 130)", p.X, p.Y)
	}
}

func TestCoreOrdinaryProjectileContactRequiresAnInteriorSample(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		speed     float64
		firstHits bool
	}{{"exact boundary", 480, false}, {"crosses entire diameter", 4800, true}} {
		t.Run(scenario.name, func(t *testing.T) {
			state, ids := coreOrdinaryProjectileScene(t, Point{X: 312, Y: 132})
			if err := Advance(state, 20); err != nil {
				t.Fatal(err)
			}
			if len(state.Projectiles) != 1 {
				t.Fatal("ordinary shot missing")
			}
			projectile := &state.Projectiles[0]
			projectile.X, projectile.Y, projectile.Speed = 280, 132, scenario.speed
			if err := Advance(state, 20); err != nil {
				t.Fatal(err)
			}
			if hit := enemyFor(state, ids[0]).HP < 10000; hit != scenario.firstHits {
				t.Fatalf("first step hit = %v, want %v", hit, scenario.firstHits)
			}
			if !scenario.firstHits {
				if len(state.Projectiles) != 1 || state.Projectiles[0].X != 288 {
					t.Fatal("boundary contact did not preserve the projectile at x=288")
				}
				if err := Advance(state, 20); err != nil {
					t.Fatal(err)
				}
			}
			if hp := enemyFor(state, ids[0]).HP; hp != 9994 || len(state.Projectiles) != 0 {
				t.Fatalf("expected exactly one 6 HP contact, remaining HP = %v", hp)
			}
		})
	}
}

func TestCoreOrdinaryProjectileRicochetStartsANewFlight(t *testing.T) {
	state, ids := coreOrdinaryProjectileScene(t, Point{X: 318, Y: 130}, Point{X: 395, Y: 130}, Point{X: 243, Y: 130})
	for range 20 {
		if err := Advance(state, 20); err != nil {
			t.Fatal(err)
		}
		if enemyFor(state, ids[0]).HP == 10000 {
			continue
		}
		if len(state.Projectiles) != 1 {
			t.Fatal("first contact did not produce exactly one ricochet")
		}
		p := state.Projectiles[0]
		if p.TargetID != ids[2] {
			t.Fatal("ricochet must select the nearest enemy to the previous target center", p.TargetID)
		}
		if p.X < 331.25 || p.X > 337.25 || p.Y < 143 || p.Y > 149 || p.LifeMS != 0 {
			t.Fatalf("new flight began at (%v, %v) with age %v, want a fresh flight from the target sprite", p.X, p.Y, p.LifeMS)
		}
		return
	}
	t.Fatal("primary shot never struck the target")
}

func TestCoreShiftGearAccelerationResetsAtTheWaveBoundaryE2E(t *testing.T) {
	state := coreMotionOwner(t, "klink")
	state.WaveActive, state.Phase = true, "battle"
	projectile := state.Projectiles[0]
	id, err := SpawnEnemyAt(state, "rattata", Point{X: projectile.X, Y: projectile.Y}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	enemy := enemyFor(state, id)
	enemy.Entered, enemy.BaseSpeed = true, 0
	enemy.HP, enemy.MaxHP = 10000, 10000
	if err := Advance(state, 400); err != nil {
		t.Fatal(err)
	}
	if state.Towers[0].Stacks["orbitalSpeed"] <= 0 {
		t.Fatal("the real stationary gear contact did not start orbital acceleration")
	}
	state.Enemies = []Enemy{}
	if err := Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	if state.WaveActive || state.Towers[0].Stacks["orbitalSpeed"] != 0 {
		t.Fatal("orbital acceleration leaked across the completed wave")
	}
}
