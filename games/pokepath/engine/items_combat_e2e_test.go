package engine

import (
	"fmt"
	"math"
	"testing"
)

func TestCoreItemsRoundAtImpactBeforeCriticalAndKeepLaunchPowerE2E(t *testing.T) {
	for _, scenario := range []struct {
		species  string
		level    int
		item     string
		normal   float64
		critical float64
	}{
		{"mareep", 1, "eviolite", 8, 12},
		{"mareep", 1, "laggingTail", 11, 17},
		{"mareep", 1, "quickClaw", 3, 5},
		{"bulbasaur", 2, "eviolite", 21, 32},
		{"treecko", 1, "eviolite", 15, 23},
		{"charmander", 1, "quickClaw", 7, 11},
	} {
		for _, critical := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-%s-%t", scenario.species, scenario.item, critical), func(t *testing.T) {
				state := coreMotionOwner(t, scenario.species)
				route, _ := routeForState(state)
				for _, tile := range route.Placements {
					if tile.Terrain == 1 {
						coreApply(t, state, 0, Action{Type: "move", PokemonID: state.Team[0].ID, TileID: tile.ID})
						break
					}
				}
				if scenario.level > 1 {
					coreApply(t, state, 0, Action{Type: "upgrade", PokemonID: state.Team[0].ID, Count: scenario.level - 1})
				}
				coreHoldItem(t, state, scenario.item)
				coreApply(t, state, 0, Action{Type: "wave"})
				state.Enemies = nil
				if critical {
					state.Team[0].Stats.CriticalPct = 100
				} else {
					state.Team[0].Stats.CriticalPct = 0
				}
				tower := state.Towers[0]
				id, err := SpawnEnemyAt(state, "rattata", Point{X: tower.X + 40, Y: tower.Y}, 0, 0)
				if err != nil {
					t.Fatal(err)
				}
				enemy := enemyFor(state, id)
				enemy.HP, enemy.MaxHP, enemy.BaseSpeed, enemy.Entered = 10000, 10000, 0, true
				var hit *Event
				for elapsed := 0; elapsed < 2000 && hit == nil; elapsed += 17 {
					if err := Advance(state, 17); err != nil {
						t.Fatal(err)
					}
					for _, event := range state.Events {
						if event.Type == "hit" && event.Target == id && event.SourceKind == "physical" {
							value := event
							hit = &value
							break
						}
					}
				}
				want := scenario.normal
				if critical {
					want = scenario.critical
				}
				if hit == nil || hit.HPDamage != want || hit.AttemptedDamage != want || hit.Critical != critical {
					t.Fatal("the actual impact applied item rounding at the wrong damage stage", hit, want)
				}
				view := ProjectTransient(state)
				if view.Team[0].CombatStats == nil || view.Team[0].CombatStats.Power != state.Team[0].Stats.Power {
					t.Fatal("impact-only item bonus leaked into displayed or launch power")
				}
				t.Logf("species=%s level=%d item=%s critical=%t power=%g hit=%g", scenario.species, scenario.level, scenario.item, critical, state.Team[0].Stats.Power, hit.HPDamage)
			})
		}
	}
}

func TestCoreProjectileUpdatesUseNewestFirstAndDeferNewbornRicochetsE2E(t *testing.T) {
	state := coreMotionOwner(t, "froaki")
	coreHoldItem(t, state, "zoomLens")
	coreApply(t, state, 0, Action{Type: "wave"})
	state.Enemies = nil
	state.Team[0].Stats.CriticalPct = 0
	tower := state.Towers[0]
	ids := []int{}
	for _, distance := range []float64{20, 20} {
		id, err := SpawnEnemyAt(state, "rattata", Point{X: tower.X + distance, Y: tower.Y}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		enemy := enemyFor(state, id)
		enemy.HP, enemy.MaxHP, enemy.BaseSpeed, enemy.Entered = 10000, 10000, 0, true
		ids = append(ids, id)
	}
	if err := Advance(state, 17); err != nil {
		t.Fatal(err)
	}
	if len(state.Projectiles) != 2 {
		t.Fatal("the real two-target attack did not create two deferred ricochets")
	}
	for _, projectile := range state.Projectiles {
		if projectile.RicochetHits != 1 || projectile.LifeMS != 0 {
			t.Fatal("a newborn ricochet advanced in its creation step")
		}
	}
	targets := []int{}
	for _, event := range state.Events {
		if event.Type == "hit" && event.SourceKind == "physical" {
			targets = append(targets, event.Target)
		}
	}
	if len(targets) != 2 || targets[0] != ids[1] || targets[1] != ids[0] {
		t.Fatal("same-tower projectile impacts did not run from newest to oldest", targets, ids)
	}
	t.Logf("first impact target=%d second impact target=%d newborns=%d", targets[0], targets[1], len(state.Projectiles))
}

func coreHoldItem(t *testing.T, state *State, key string) {
	t.Helper()
	for index, stock := range state.Progression.ShopStock {
		if stock == key {
			state.Progression.ShopStock[index] = ""
		}
	}
	state.Inventory = append(state.Inventory, OwnedItem{ID: key})
	coreApply(t, state, 0, Action{Type: "equip", PokemonID: state.Team[0].ID, ItemID: key})
}

func TestCoreSoftSandCountsOnlyActuallyTargetableAreaEnemiesE2E(t *testing.T) {
	for _, count := range []int{1, 2} {
		state := coreMotionOwner(t, "mareep")
		coreHoldItem(t, state, "softSand")
		state.WaveActive, state.Phase = true, "battle"
		tower := state.Towers[0]
		ids := []int{}
		for index := 0; index < count+3; index++ {
			point := Point{X: tower.X + 40 + float64(index)*3, Y: tower.Y}
			if index == count+2 {
				point.X = tower.X + 400
			}
			id, err := SpawnEnemyAt(state, "rattata", point, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			enemy := enemyFor(state, id)
			enemy.HP, enemy.MaxHP, enemy.BaseSpeed, enemy.Entered = 1000, 1000, 0, true
			if index == count {
				enemy.InvulnerableMS = 1000
			}
			if index == count+1 {
				enemy.Invisible = true
			}
			ids = append(ids, id)
		}
		if err := Advance(state, 20); err != nil {
			t.Fatal(err)
		}
		want := 35.0
		if count == 2 {
			want = 21
		}
		for index, id := range ids {
			lost := 1000 - enemyFor(state, id).HP
			if index < count && lost != want || index >= count && lost != 0 {
				t.Fatal("soft sand used global/immune/invisible enemies or skipped its actual damage bonus", count, index, lost, want)
			}
		}
	}
}

func TestCoreStrangeIdolUsesLiveCursedEnemiesWithoutReducingBaseDamageE2E(t *testing.T) {
	for _, count := range []int{0, 10, 50, 60} {
		state := coreMotionOwner(t, "yamask")
		coreHoldItem(t, state, "strangeIdol")
		state.WaveActive, state.Phase = true, "battle"
		tower := state.Towers[0]
		id, err := SpawnEnemyAt(state, "rattata", Point{X: tower.X + 40, Y: tower.Y}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		primary := enemyFor(state, id)
		primary.HP, primary.MaxHP, primary.BaseSpeed, primary.Entered = 1000, 1000, 0, true
		for index := 0; index < count; index++ {
			otherID, err := SpawnEnemyAt(state, "rattata", Point{X: 1000 + float64(index), Y: 1000}, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			other := enemyFor(state, otherID)
			other.HP, other.MaxHP, other.BaseSpeed = 10000, 10000, 0
			ApplyConditions(&other.EnemyEffectState, AttackProfile{Conditions: []ConditionSpec{curseCondition()}}, 0)
		}
		if err := Advance(state, 100); err != nil {
			t.Fatal(err)
		}
		base := state.Team[0].Stats.Power
		want := base + math.Ceil(base*max(0, 50-float64(count))/100)
		if lost := 1000 - enemyFor(state, id).HP; lost != want {
			t.Fatal("strange idol omitted live curse count or reduced base damage after fifty cursed enemies", count, lost, want)
		}
	}
}

func TestCoreRoundedExtraDamageProfilesActuallyReachTheTargetE2E(t *testing.T) {
	for _, scenario := range []struct {
		ability, item string
		teleports     int
	}{{"contrary", "inverter", 0}, {"teleport", "twistedSpoon", 1}} {
		state := coreNew(t, 1)
		id, err := SpawnEnemyAt(state, "rattata", Point{X: 100, Y: 100}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		context := AbilityContext{AbilityID: scenario.ability, ItemID: scenario.item, PokemonID: state.Team[0].ID, Power: 15, Range: 100, AttackMS: 1000, Level: 1, Random: .5, Teleports: scenario.teleports}
		profile, err := BuildAttackProfile(context)
		if err != nil {
			t.Fatal(err)
		}
		projectile := Projectile{PokemonID: state.Team[0].ID, TargetID: id, Context: context, Profile: profile, SourceProfile: profile, DamageScale: 1}
		if err := applyProjectileHit(state, &projectile, enemyFor(state, id), 1); err != nil {
			t.Fatal(err)
		}
		if lost := 70 - enemyFor(state, id).HP; lost != 23 {
			t.Fatal("extra-damage profile was never consumed before the critical stage", scenario.item, lost)
		}
	}
}
