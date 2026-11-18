package pokepath_test

import (
	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
	"testing"
)

func beginPassiveCampaign(t *testing.T) *engine.State {
	t.Helper()
	state, err := engine.New(engine.Options{Starter: "charmander"}, 2, 12113, 1794916800000)
	if err != nil {
		t.Fatal(err)
	}
	state.Speed = 1
	if err := engine.Apply(state, 0, engine.Action{Type: "wave", Epoch: state.Epoch}); err != nil {
		t.Fatal(err)
	}
	return state
}

func passiveWorldEnemy(state *engine.State, id int) *engine.Enemy {
	for index := range state.Enemies {
		if state.Enemies[index].ID == id {
			return &state.Enemies[index]
		}
	}
	return nil
}

func TestAbilitiesWorldRegeneratorActuallySurvivesAndCanBeInterrupted(t *testing.T) {
	state := beginPassiveCampaign(t)
	id, err := engine.SpawnEnemyAt(state, "hooh", engine.Point{X: 358.5, Y: 69}, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	enemy := passiveWorldEnemy(state, id)
	enemy.HP = 0
	beforeGold := state.Gold
	if err := engine.Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	enemy = passiveWorldEnemy(state, id)
	if enemy == nil || enemy.HP != 1 || enemy.ReviveRemainingMS <= 0 {
		t.Fatal("real world finalized HoOh before its regeneration phase")
	}
	if state.Gold != beforeGold {
		t.Fatal("first fatal hit awarded gold before final death")
	}
	enemy.HP = 0
	if err := engine.Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	enemy = passiveWorldEnemy(state, id)
	if enemy == nil || !enemy.Dead || enemy.HP > 0 || enemy.DeathRemainingMS <= 0 || state.Gold <= beforeGold {
		t.Fatal("early regeneration-window kill did not complete through real world settlement")
	}
	if err := engine.Advance(state, 560); err != nil {
		t.Fatal(err)
	}
	if passiveWorldEnemy(state, id) != nil {
		t.Fatal("settled HoOh remained after the original death fade duration")
	}
}

func TestAbilitiesWorldTimeTravelAndSchoolingExecuteTheirTypedEvents(t *testing.T) {
	state := beginPassiveCampaign(t)
	id, err := engine.SpawnEnemyAt(state, "celebi", engine.Point{X: 358.5, Y: 69}, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	enemy := passiveWorldEnemy(state, id)
	enemy.HP = 1000
	if err := engine.Advance(state, 8000); err != nil {
		t.Fatal(err)
	}
	enemy = passiveWorldEnemy(state, id)
	if enemy == nil || enemy.HP != 8000 {
		t.Fatal("real world did not apply time-travel healing")
	}
	found := false
	for _, event := range state.Events {
		if event.Type == "rewind" && event.Target == id {
			found = true
		}
	}
	if !found {
		t.Fatal("real world did not execute or record the rewind event")
	}
	school, err := engine.SpawnEnemyAt(state, "wishiwashi", engine.Point{X: 358.5, Y: 69}, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	passiveWorldEnemy(state, school).HP = 0
	if err := engine.Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	children := 0
	for _, entry := range state.Enemies {
		if entry.Key == "wishiwashiSolo" {
			children++
		}
	}
	if children != 100 {
		t.Fatalf("real world spawned %d schooling children, want 100", children)
	}
}

func passiveArena(t *testing.T, deploy bool) *engine.State {
	t.Helper()
	state := beginPassiveCampaign(t)
	state.Enemies, state.Projectiles = []engine.Enemy{}, []engine.Projectile{}
	if deploy {
		for _, placement := range engine.CatalogData().Routes["0"].Placements {
			if placement.Terrain != 1 {
				continue
			}
			if err := engine.Apply(state, 0, engine.Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: placement.ID, Epoch: state.Epoch}); err != nil {
				t.Fatal(err)
			}
			state.Towers[0].CooldownMS = 100000
			state.Team[0].Stats.Power, state.Team[0].Stats.AttackMS, state.Team[0].Stats.Range = 100, 1000, 300
			break
		}
		if len(state.Towers) == 0 {
			t.Fatal("arena has no legal land tower placement")
		}
	}
	return state
}

func passiveArenaEnemy(t *testing.T, state *engine.State, key string, point engine.Point) int {
	t.Helper()
	id, err := engine.SpawnEnemyAt(state, key, point, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	enemy := passiveWorldEnemy(state, id)
	enemy.HP, enemy.MaxHP, enemy.Armor, enemy.MaxArmor = 10000, 10000, 0, 0
	enemy.BaseSpeed, enemy.Regeneration, enemy.Entered = 0, 0, true
	return id
}

func passiveArenaImpact(t *testing.T, state *engine.State, target int, context engine.AbilityContext) {
	t.Helper()
	enemy := passiveWorldEnemy(state, target)
	context.PokemonID, context.Power, context.Range, context.AttackMS, context.Level = state.Team[0].ID, 100, 300, 1000, 1
	context.Random = .8
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	tower := 0
	if len(state.Towers) > 0 {
		tower = state.Towers[0].ID
	}
	state.Projectiles = append(state.Projectiles, engine.Projectile{ID: state.NextID, TowerID: tower, PokemonID: state.Team[0].ID, TargetID: target, X: enemy.X, Y: enemy.Y, Speed: 1000, Power: profile.Power, Context: context, Profile: profile, SourceProfile: profile, DamageScale: 1})
	state.NextID++
}

func TestAbilitiesWorldDamageRedirectionAndUnionUseActualProjectileClock(t *testing.T) {
	state := passiveArena(t, false)
	victim := passiveArenaEnemy(t, state, "rattata", engine.Point{X: 100, Y: 100})
	absorber := passiveArenaEnemy(t, state, "aegislash", engine.Point{X: 600, Y: 100})
	passiveWorldEnemy(state, victim).Invisible = true
	passiveArenaImpact(t, state, victim, engine.AbilityContext{AbilityID: "frisk"})
	if err := engine.Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	if passiveWorldEnemy(state, victim).HP != 10000 || passiveWorldEnemy(state, victim).Invisible || passiveWorldEnemy(state, absorber).HP != 9900 || state.TotalDamage != 100 {
		t.Fatalf("actual projectile did not redirect: victim=%+v absorber=%+v damage=%v projectiles=%+v", passiveWorldEnemy(state, victim), passiveWorldEnemy(state, absorber), state.TotalDamage, state.Projectiles)
	}
	state = passiveArena(t, false)
	first := passiveArenaEnemy(t, state, "latios", engine.Point{X: 100, Y: 100})
	second := passiveArenaEnemy(t, state, "latias", engine.Point{X: 200, Y: 100})
	passiveArenaImpact(t, state, first, engine.AbilityContext{})
	if err := engine.Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	if passiveWorldEnemy(state, first).HP != 9900 || passiveWorldEnemy(state, second).HP != 9900 || state.TotalDamage != 200 || state.Team[0].Damage != 200 || passiveWorldEnemy(state, second).LastHitBy != state.Team[0].ID {
		t.Fatal("union damage did not execute exactly once with real source statistics")
	}
}

func TestAbilitiesWorldArmorBreakModifiersLockAndRestoreAtWaveEnd(t *testing.T) {
	for _, key := range []string{"dhelmise", "dusknoir", "lopunny"} {
		t.Run(key, func(t *testing.T) {
			state := passiveArena(t, true)
			point := engine.Point{X: state.Towers[0].X + 30, Y: state.Towers[0].Y}
			id := passiveArenaEnemy(t, state, key, point)
			passiveWorldEnemy(state, id).Armor = 1
			passiveWorldEnemy(state, id).MaxArmor = 1
			state.Gold = 1000000
			stock := -1
			for index, item := range state.Progression.ShopStock {
				if item == "protein" {
					stock = index
					break
				}
			}
			if stock < 0 {
				t.Fatal("official first shelf is missing protein for the held-item fixture")
			}
			if err := engine.Apply(state, 0, engine.Action{Type: "buyItem", ItemID: "protein", Stock: stock, Epoch: state.Epoch}); err != nil {
				t.Fatal(err)
			}
			if err := engine.Apply(state, 0, engine.Action{Type: "equip", PokemonID: state.Team[0].ID, ItemID: "protein", Epoch: state.Epoch}); err != nil {
				t.Fatal(err)
			}
			passiveArenaImpact(t, state, id, engine.AbilityContext{AbilityID: "frisk", ItemID: "protein"})
			if err := engine.Advance(state, 20); err != nil {
				t.Fatal(err)
			}
			switch key {
			case "dhelmise":
				if !state.World.AnchorShotTriggered {
					t.Fatal("real armor break did not activate anchor shot")
				}
				state.Towers[0].CooldownMS = 0
				if err := engine.Advance(state, 20); err != nil || state.Towers[0].CooldownMS < 3900 {
					t.Fatal("anchor shot did not multiply actual tower firing interval")
				}
			case "dusknoir":
				if !state.World.Pressure {
					t.Fatalf("real armor break did not activate pressure: enemy=%+v projectiles=%+v", passiveWorldEnemy(state, id), state.Projectiles)
				}
				if err := engine.Apply(state, 0, engine.Action{Type: "retire", PokemonID: state.Team[0].ID, Epoch: state.Epoch}); err == nil {
					t.Fatal("pressure allowed retiring an actual deployed tower")
				}
			case "lopunny":
				if state.Team[0].Item != "" || state.Inventory[len(state.Inventory)-1].EquippedBy != 0 || state.World.StrippedItems[state.Team[0].ID] != "protein" {
					t.Fatal("klutz did not remove the real attacker's held item")
				}
			}
			for index := range state.Enemies {
				state.Enemies[index].HP = 0
			}
			if err := engine.Advance(state, 600); err != nil {
				t.Fatal(err)
			}
			if state.World.AnchorShotTriggered || state.World.Pressure || key == "lopunny" && state.Team[0].Item != "protein" {
				t.Fatal("wave settlement did not release passive locks or restore the stripped item")
			}
		})
	}
}

func TestAbilitiesWorldNaturalHealPulsesTriggerCorrosionEvenAtFullHealth(t *testing.T) {
	state := passiveArena(t, true)
	state.Team[0].Species = "megaDragalge"
	state.Team[0].Stats.Range = 300
	id := passiveArenaEnemy(t, state, "rattata", engine.Point{X: state.Towers[0].X + 30, Y: state.Towers[0].Y})
	enemy := passiveWorldEnemy(state, id)
	enemy.Regeneration, enemy.CanPoison = 100, true
	if err := engine.Advance(state, 980); err != nil {
		t.Fatal(err)
	}
	if engine.ConditionStacks(&passiveWorldEnemy(state, id).EnemyEffectState, "poison") != 0 {
		t.Fatal("corrosion triggered before the first natural healing pulse")
	}
	if err := engine.Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	enemy = passiveWorldEnemy(state, id)
	if enemy.HealingEvents != 1 || engine.ConditionStacks(&enemy.EnemyEffectState, "poison") != 1 {
		t.Fatal("actual full-health healing pulse did not trigger corrosion exactly once")
	}
}

func TestAbilitiesWorldSynchronyAppliesConditionsAfterSplashDamage(t *testing.T) {
	state := passiveArena(t, false)
	primary := passiveArenaEnemy(t, state, "rattata", engine.Point{X: 100, Y: 100})
	secondary := passiveArenaEnemy(t, state, "cresselia", engine.Point{X: 150, Y: 100})
	passiveWorldEnemy(state, secondary).HP = 4000
	passiveWorldEnemy(state, primary).Conditions = []engine.Condition{{Kind: "burn", RemainingMS: 10000, Magnitude: .005, Stacks: 1, SourcePokemonID: state.Team[0].ID}}
	passiveArenaImpact(t, state, primary, engine.AbilityContext{AbilityID: "synchronySplash"})
	if err := engine.Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	if !engine.HasCondition(&passiveWorldEnemy(state, secondary).EnemyEffectState, "burn") {
		t.Fatal("synchrony transmission ran before lunar-blessing's damage-triggered cleansing")
	}
}

func TestAbilitiesWorldSchoolingChildrenHaveScatteredCentersAndSlicedRoutes(t *testing.T) {
	state := passiveArena(t, false)
	id := passiveArenaEnemy(t, state, "wishiwashi", engine.Point{X: 300, Y: 240})
	parent := passiveWorldEnemy(state, id)
	parent.Waypoint = 2
	path := engine.EnemyPath(state, parent)
	if len(path) <= parent.Waypoint {
		t.Fatal("schooling fixture has no remaining route segment")
	}
	firstRemaining := path[parent.Waypoint]
	parent.HP = 0
	if err := engine.Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	centers := map[engine.Point]bool{}
	count := 0
	for index := range state.Enemies {
		child := &state.Enemies[index]
		if child.Key != "wishiwashiSolo" {
			continue
		}
		count++
		dx, dy := child.X-300-child.Sprite.FrameWidth/2, child.Y-240-child.Sprite.FrameHeight/2
		if dx < -20 || dx >= 10 || dy < -20 || dy >= 10 {
			t.Fatal("schooling child did not use the original randomized top-left spawn bounds")
		}
		centers[engine.Point{X: child.X, Y: child.Y}] = true
		childPath := engine.EnemyPath(state, child)
		if len(childPath) == 0 || childPath[0] != firstRemaining || child.Waypoint != 0 || child.Distance != 0 {
			t.Fatal("schooling child inherited old progress instead of starting its sliced remaining path")
		}
	}
	if count != 100 || len(centers) != 100 {
		t.Fatal("schooling did not create one hundred independently scattered children")
	}
}

func TestAbilitiesWorldUnionAlsoSharesAttributedConditionDamage(t *testing.T) {
	state := passiveArena(t, false)
	first := passiveArenaEnemy(t, state, "latios", engine.Point{X: 100, Y: 100})
	second := passiveArenaEnemy(t, state, "latias", engine.Point{X: 200, Y: 100})
	passiveWorldEnemy(state, first).Conditions = []engine.Condition{{Kind: "burn", DurationMS: 10000, RemainingMS: 10000, Magnitude: .005, Stacks: 1, SourcePokemonID: state.Team[0].ID}}
	if err := engine.Advance(state, 1000); err != nil {
		t.Fatal(err)
	}
	if passiveWorldEnemy(state, first).HP != 10000 || passiveWorldEnemy(state, second).HP != 10000 {
		t.Fatal("union condition pulse ignored the native fractional frame boundary")
	}
	if err := engine.Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	if passiveWorldEnemy(state, first).HP != 9950 || passiveWorldEnemy(state, second).HP != 9950 || state.TotalDamage != 100 || state.Team[0].Damage != 100 || passiveWorldEnemy(state, second).LastHitBy != state.Team[0].ID {
		t.Fatal("condition damage did not traverse eon union with the original source and damage kind")
	}
}

func TestAbilitiesWorldFatalCurseLinkOpensRegenerationBeforeTheNextProjectile(t *testing.T) {
	state := passiveArena(t, false)
	primary := passiveArenaEnemy(t, state, "rattata", engine.Point{X: 100, Y: 100})
	reviver := passiveArenaEnemy(t, state, "hooh", engine.Point{X: 200, Y: 100})
	for _, id := range []int{primary, reviver} {
		passiveWorldEnemy(state, id).Conditions = []engine.Condition{{Kind: "curse", DurationMS: -1, RemainingMS: -1, Stacks: 1, SourcePokemonID: state.Team[0].ID}}
	}
	passiveWorldEnemy(state, reviver).HP = 100
	passiveArenaImpact(t, state, reviver, engine.AbilityContext{})
	older := &state.Projectiles[len(state.Projectiles)-1]
	older.Context.Power, older.Profile.Power, older.SourceProfile.Power, older.Power = 60000, 60000, 60000, 60000
	passiveArenaImpact(t, state, primary, engine.AbilityContext{AbilityID: "curse"})
	beforeGold := state.Gold
	if err := engine.Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	enemy := passiveWorldEnemy(state, reviver)
	if enemy == nil || !enemy.Dead || !enemy.PassiveFlags["regeneratorStarted"] || !enemy.PassiveFlags["revivalInterrupted"] || enemy.ReviveRemainingMS != 0 || state.Gold <= beforeGold {
		t.Fatal("fatal curse link deferred regeneration and discarded the next same-tick projectile")
	}
}

func TestAbilitiesWorldStrangeIdolLinksRecountCursedPeersAfterFatalPrimary(t *testing.T) {
	state := passiveArena(t, false)
	ids := []int{}
	for index := 0; index < 3; index++ {
		id := passiveArenaEnemy(t, state, "rattata", engine.Point{X: 100 + float64(index*100), Y: 100})
		passiveWorldEnemy(state, id).Conditions = []engine.Condition{{Kind: "curse", DurationMS: -1, RemainingMS: -1, Stacks: 1, SourcePokemonID: state.Team[0].ID}}
		ids = append(ids, id)
	}
	passiveWorldEnemy(state, ids[0]).HP = 100
	passiveArenaImpact(t, state, ids[0], engine.AbilityContext{AbilityID: "curse", ItemID: "strangeIdol"})
	if err := engine.Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	if !passiveWorldEnemy(state, ids[0]).Dead || passiveWorldEnemy(state, ids[1]).HP != 9851 || passiveWorldEnemy(state, ids[2]).HP != 9851 || state.TotalDamage != 398 || state.Team[0].Damage != 398 {
		t.Fatalf("idol links reused pre-death counts or compounded the primary bonus: first=%v second=%v total=%v", passiveWorldEnemy(state, ids[1]).HP, passiveWorldEnemy(state, ids[2]).HP, state.TotalDamage)
	}
}
