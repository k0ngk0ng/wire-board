package pokepath_test

import (
	"math"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func persistentAreaEnemy(t *testing.T, state *engine.State, id int) *engine.Enemy {
	t.Helper()
	for index := range state.Enemies {
		if state.Enemies[index].ID == id {
			return &state.Enemies[index]
		}
	}
	t.Fatalf("missing scenario enemy %d", id)
	return nil
}

func TestNativePersistentSpikesWaitTickPersistAndExpireE2E(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "pineco", 1)
	waBattle(t, state)
	id := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 40, Y: state.Towers[0].Y})
	waAdvance(t, state, 20)
	if persistentAreaEnemy(t, state, id).ReceivedHits != 0 || len(state.World.Zones) != 1 || state.World.Zones[0].Kind != "spikes" || state.World.Zones[0].LifeMS != 5000 || state.World.Zones[0].TickMS != 500 {
		t.Fatal("spikes dealt an immediate one-off area hit instead of creating the original persistent field")
	}
	waAdvance(t, state, 460)
	if persistentAreaEnemy(t, state, id).ReceivedHits != 0 {
		t.Fatal("spikes hit before 500 ms")
	}
	waAdvance(t, state, 20)
	if persistentAreaEnemy(t, state, id).ReceivedHits != 1 {
		t.Fatal("the first persistent spike tick was missing")
	}
	waSnapshotContinuation(t, state, 1200)
	waAdvance(t, state, 3280)
	if persistentAreaEnemy(t, state, id).ReceivedHits != 9 || len(state.World.Zones) != 1 {
		t.Fatal("spike lifetime or repeated tick count changed before expiry")
	}
	waAdvance(t, state, 20)
	if persistentAreaEnemy(t, state, id).ReceivedHits != 9 || len(state.World.Zones) != 0 {
		t.Fatal("expired spikes dealt a forbidden final tick or remained active")
	}
	waEvidence(t, "persistent-spikes", state, map[string]any{"lifeMS": 5000, "tickMS": 500, "directHits": 9, "firstHitMS": 500})
}

func TestNativePersistentSpikesHelmetChangesTheRealTickPeriodE2E(t *testing.T) {
	state := waScenario(t)
	owner := waDeploy(t, state, "pineco", 1)
	for index, key := range state.Progression.ShopStock {
		if key == "rockyHelmetSpikes" {
			state.Progression.ShopStock[index] = ""
		}
	}
	state.Inventory = append(state.Inventory, engine.OwnedItem{ID: "rockyHelmetSpikes"})
	waApply(t, state, 1, engine.Action{Type: "equip", PokemonID: owner, ItemID: "rockyHelmetSpikes"})
	waBattle(t, state)
	id := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 40, Y: state.Towers[0].Y})
	waAdvance(t, state, 240)
	if persistentAreaEnemy(t, state, id).ReceivedHits != 0 {
		t.Fatal("helmet spikes dealt damage before their 250 ms threshold")
	}
	waAdvance(t, state, 20)
	if len(state.World.Zones) != 1 || state.World.Zones[0].TickMS != 250 || persistentAreaEnemy(t, state, id).ReceivedHits != 1 {
		t.Fatal("helmet did not change the actual persistent damage interval")
	}
	waAdvance(t, state, 240)
	if persistentAreaEnemy(t, state, id).ReceivedHits != 2 {
		t.Fatal("helmet spike remainder did not preserve the second 500 ms crossing")
	}
	waEvidence(t, "persistent-spikes-helmet", state, map[string]any{"tickMS": 250, "fixedStepFirstHitMS": 260, "hitsAt500MS": 2})
}

func TestNativePersistentMagmaSpikesUseRingTicksAndDoNotRefreshBurnE2E(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "groudon", 1)
	waBattle(t, state)
	center := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	inner := waEnemy(t, state, engine.Point{X: center.X + 100, Y: center.Y})
	inside := waEnemy(t, state, engine.Point{X: center.X + 200, Y: center.Y})
	outside := waEnemy(t, state, engine.Point{X: center.X + 300, Y: center.Y})
	waAdvance(t, state, 440)
	if persistentAreaEnemy(t, state, inside).ReceivedHits != 0 || len(state.World.Zones) != 1 || state.World.Zones[0].Kind != "magmaSpikes" {
		t.Fatal("precipice blades was an immediate area hit rather than a persistent magma ring")
	}
	waAdvance(t, state, 20)
	if persistentAreaEnemy(t, state, inner).ReceivedHits != 0 || persistentAreaEnemy(t, state, outside).ReceivedHits != 0 || persistentAreaEnemy(t, state, inside).ReceivedHits != 1 || !engine.HasCondition(&persistentAreaEnemy(t, state, inside).EnemyEffectState, "burn") {
		t.Fatal("magma field hit its empty center/outside or missed its 450 ms burn tick")
	}
	waAdvance(t, state, 440)
	remaining := int64(0)
	elapsed := 0.0
	for _, condition := range persistentAreaEnemy(t, state, inside).Conditions {
		if condition.Kind == "burn" {
			remaining = condition.RemainingMS
			elapsed = condition.TickElapsedMS
		}
	}
	if persistentAreaEnemy(t, state, inside).ReceivedHits != 2 || remaining != 10000 || math.Abs(elapsed-450) > 1e-8 {
		t.Fatal("magma repeated tick refreshed an existing burn or used the wrong interval", remaining, elapsed)
	}
	waSnapshotContinuation(t, state, 600)
	if persistentAreaEnemy(t, state, inside).ReceivedHits != 3 || len(state.World.Zones) != 0 {
		t.Fatal("magma ring did not expire after exactly three direct ticks")
	}
	waEvidence(t, "persistent-magma-spikes", state, map[string]any{"lifeMS": 1500, "tickMS": 450, "innerRadius": 150, "radius": 280, "hits": 3, "burnRefresh": false})
}

func TestNativePersistentSpikesDeduplicateAndSurviveSourceRetirementE2E(t *testing.T) {
	state := waScenario(t)
	owner := waDeploy(t, state, "pineco", 1)
	waBattle(t, state)
	id := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 40, Y: state.Towers[0].Y})
	waAdvance(t, state, 20)
	if len(state.World.Zones) != 1 {
		t.Fatal("persistent spike field was never created")
	}
	zoneID := state.World.Zones[0].ID
	state.Towers[0].CooldownMS = 0
	waAdvance(t, state, 20)
	if len(state.World.Zones) != 1 || state.World.Zones[0].ID != zoneID || state.World.Zones[0].AgeMS != 33 {
		t.Fatal("duplicate same-source cast refreshed or stacked the active field")
	}
	waApply(t, state, 2, engine.Action{Type: "retire", PokemonID: owner})
	waSnapshotContinuation(t, state, 460)
	if len(state.World.Zones) != 1 || persistentAreaEnemy(t, state, id).ReceivedHits != 1 {
		t.Fatal("retirement deleted a living field or lost its persistent source")
	}
	waAdvance(t, state, 4500)
	if len(state.World.Zones) != 0 || persistentAreaEnemy(t, state, id).ReceivedHits != 9 {
		t.Fatal("retired spike field changed lifetime or damage after restoring its saved source")
	}
	waEvidence(t, "persistent-spikes-retired", state, map[string]any{"deduplicated": true, "retiredSource": owner, "hits": 9, "persistentSourceInSave": true})
}

func TestNativePersistentSteamBeamAndHarmlessSlowTrailHaveSeparateLifetimesE2E(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "volcanion", 1)
	waBattle(t, state)
	center := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	primary := waEnemy(t, state, engine.Point{X: center.X + 100, Y: center.Y})
	inside := waEnemy(t, state, engine.Point{X: center.X + 100, Y: center.Y + 19})
	outside := waEnemy(t, state, engine.Point{X: center.X + 100, Y: center.Y + 20})
	waAdvance(t, state, 20)
	if len(state.Projectiles) != 1 || state.Projectiles[0].RayLength != 2000 || state.Projectiles[0].Profile.BeamDurationMS != 220 || len(state.World.Zones) != 62 || persistentAreaEnemy(t, state, primary).ReceivedHits != 0 {
		t.Fatal("steam eruption did not separate its 220 ms beam from 62 persistent smoke samples")
	}
	for _, zone := range state.World.Zones {
		if zone.Kind != "smoke" || zone.Radius != 22 || zone.LifeMS != 3000 {
			t.Fatal("steam trail geometry or lifetime differed from the original field")
		}
	}
	waAdvance(t, state, 146)
	if persistentAreaEnemy(t, state, primary).ReceivedHits != 0 {
		t.Fatal("steam beam hit before its original 140 ms first-hit threshold")
	}
	waAdvance(t, state, 1)
	if persistentAreaEnemy(t, state, primary).ReceivedHits != 1 || persistentAreaEnemy(t, state, inside).ReceivedHits != 1 || persistentAreaEnemy(t, state, outside).ReceivedHits != 0 {
		t.Fatal("steam beam lost its original half-width plus enemy quarter-size collision tolerance")
	}
	waAdvance(t, state, 66)
	if len(state.Projectiles) != 1 || len(state.World.Zones) != 62 {
		t.Fatal("steam beam or smoke expired before the 220 ms beam lifetime")
	}
	waAdvance(t, state, 1)
	if len(state.Projectiles) != 0 || len(state.World.Zones) != 62 {
		t.Fatal("steam beam lingered beyond 220 ms or removed its independent smoke")
	}
	waSnapshotContinuation(t, state, 1000-state.Clock)
	waAdvance(t, state, 2000)
	if len(state.World.Zones) != 0 {
		t.Fatal("steam smoke did not expire at its own 3000 ms boundary")
	}
	waEvidence(t, "persistent-steam-trail", state, map[string]any{"beamLifeMS": 220, "beamHitCooldownMS": 140, "beamWidth": 15, "beamLength": 2000, "smokeLifeMS": 3000, "smokeRadius": 22, "samples": 62})
}

func TestNativePersistentSteamSmokeSlowsWithoutDamageAndRestoresMovementE2E(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "volcanion", 1)
	waBattle(t, state)
	id, err := engine.SpawnEnemyAt(state, "poliwag", engine.Point{X: 400, Y: 213}, 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	enemy := persistentAreaEnemy(t, state, id)
	enemy.HP, enemy.MaxHP, enemy.BaseSpeed, enemy.Entered = 100000, 100000, 0, true
	waAdvance(t, state, 220)
	health := persistentAreaEnemy(t, state, id).HP
	persistentAreaEnemy(t, state, id).BaseSpeed = 60
	waAdvance(t, state, 100)
	if x := persistentAreaEnemy(t, state, id).X; x != 404.8 {
		t.Fatal("overlapping steam smoke did not apply one nonstacking 0.8 movement factor", x)
	}
	persistentAreaEnemy(t, state, id).BaseSpeed = 0
	waAdvance(t, state, 2660)
	enemy = persistentAreaEnemy(t, state, id)
	enemy.X, enemy.Y, enemy.Waypoint, enemy.BaseSpeed = 400, 213, 4, 60
	waAdvance(t, state, 20)
	if x := persistentAreaEnemy(t, state, id).X; x != 401.6 || len(state.World.Zones) != 0 {
		t.Fatal("smoke stopped slowing before its 3000 ms expiry", x)
	}
	waAdvance(t, state, 17)
	if x := persistentAreaEnemy(t, state, id).X; x != 402.6 || len(state.World.Zones) != 0 {
		t.Fatal("steam smoke failed to restore full speed on the step after expiry", x)
	}
	if persistentAreaEnemy(t, state, id).HP != health {
		t.Fatal("smoke dealt damage after the actual beam had expired")
	}
	waEvidence(t, "persistent-steam-slow", state, map[string]any{"slowFactor": .8, "overlapStacks": false, "trailDamage": 0, "restoredSpeed": 60})
}

func TestNativePersistentSteamTerrainExtenderDoublesItsActualBeamWidthE2E(t *testing.T) {
	state := waScenario(t)
	owner := waDeploy(t, state, "volcanion", 1)
	for index, key := range state.Progression.ShopStock {
		if key == "terrainExtender" {
			state.Progression.ShopStock[index] = ""
		}
	}
	state.Inventory = append(state.Inventory, engine.OwnedItem{ID: "terrainExtender"})
	waApply(t, state, 0, engine.Action{Type: "equip", PokemonID: owner, ItemID: "terrainExtender"})
	waBattle(t, state)
	center := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	waEnemy(t, state, engine.Point{X: center.X + 100, Y: center.Y})
	inside := waEnemy(t, state, engine.Point{X: center.X + 100, Y: center.Y + 27})
	outside := waEnemy(t, state, engine.Point{X: center.X + 100, Y: center.Y + 27.1})
	waAdvance(t, state, 167)
	if persistentAreaEnemy(t, state, inside).ReceivedHits != 1 || persistentAreaEnemy(t, state, outside).ReceivedHits != 0 {
		t.Fatal("terrain extender did not produce the original 30 px steam beam")
	}
	waEvidence(t, "persistent-steam-wide", state, map[string]any{"beamWidth": 30, "enemyQuarterSizeTolerance": 12})
}

func TestNativeProjectileOverrideReachesEachRealTrickOrTreatSpriteE2E(t *testing.T) {
	for _, scenario := range []struct {
		seed uint32
		key  string
	}{{1, "candy"}, {4000, "candy1"}, {10000, "spinarak"}, {13000, "apple"}} {
		state := waScenario(t)
		waDeploy(t, state, "pumpkaboo", 1)
		waBattle(t, state)
		waEnemy(t, state, engine.Point{X: state.Towers[0].X + 100, Y: state.Towers[0].Y})
		state.Random = scenario.seed
		waAdvance(t, state, 20)
		if len(state.Projectiles) != 1 || state.Projectiles[0].Sprite.Image != engine.CatalogData().Projectiles[scenario.key].Sprite.Image {
			t.Fatal("trick-or-treat profile selected an effect but the actual projectile kept the wrong sprite", scenario.key)
		}
		waEvidence(t, "projectile-override-"+scenario.key, state, map[string]any{"seed": scenario.seed, "projectileSprite": scenario.key})
	}
}

func TestNativeRidePagerAddsRealPassengerPowerAndDamageE2E(t *testing.T) {
	state := waScenario(t)
	host := waDeploy(t, state, "turtwig", 1)
	passenger := waPartner(t, state, "charmander", 1)
	healingEquip(t, state, host, "ridePager")
	waApply(t, state, 1, engine.Action{Type: "deploy", PokemonID: passenger, TileID: state.Towers[0].TileID})
	waBattle(t, state)
	state.Towers[0].CooldownMS = 100000
	id := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	waAdvance(t, state, 20)
	if lost := 100000 - persistentAreaEnemy(t, state, id).HP; lost != 65 {
		t.Fatal("ride pager's passenger power never reached the actual projectile", lost)
	}
	pokemon, err := engine.ProjectPokemon(state, passenger)
	if err != nil || pokemon.CombatStats == nil || pokemon.CombatStats.Power != 65 {
		t.Fatal("ride-pager passenger projection disagreed with its real damage", err, pokemon.CombatStats)
	}
	waEvidence(t, "ride-pager-power", state, map[string]any{"hostItem": "ridePager", "passengerFlatPower": 50, "actualDamage": 65})
}

func TestNativeBlimpKeysGuidedFlightUsesLegalAimAndSavedRealMotionE2E(t *testing.T) {
	state := waScenario(t)
	owner := waDeploy(t, state, "drifloon", 1)
	healingEquip(t, state, owner, "blimpKeys")
	origin := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	waApply(t, state, 3, engine.Action{Type: "aim", PokemonID: owner, X: origin.X + 100, Y: origin.Y})
	random := state.Random
	waAdvance(t, state, 1000)
	if math.Abs(state.Towers[0].X-(origin.X+80)) > 1e-6 || state.Towers[0].Y != origin.Y || state.Random != random {
		t.Fatal("guided flight used the wrong movement speed or kept drawing wandering targets", state.Towers[0].X, origin.X+80)
	}
	waSnapshotContinuation(t, state, 700)
	if distance := (origin.X + 100) - state.Towers[0].X; distance < 0 || distance >= 6 {
		t.Fatal("guided flight did not stop inside its original arrival threshold", distance)
	}
	waApply(t, state, 0, engine.Action{Type: "pause"})
	before := state.Towers[0].X
	waAdvance(t, state, 1000)
	if state.Towers[0].X != before {
		t.Fatal("paused guided flight consumed movement time")
	}
	if err := engine.Apply(state, 0, engine.Action{Type: "aim", Epoch: state.Epoch, PokemonID: owner, X: origin.X, Y: origin.Y}); err == nil {
		t.Fatal("paused guided flight accepted an active movement command")
	}
	waEvidence(t, "guided-blimp", state, map[string]any{"action": "aim", "movementSpeed": 80, "arrivalThreshold": 6, "randomFreeWhileGuided": true})
}

func TestNativeHugePowerUsesItsActualGlobalStarShapeFilterE2E(t *testing.T) {
	for _, scenario := range []struct {
		key       string
		self, far bool
		item      string
	}{{"megaStarmie", true, false, ""}, {"charmander", false, false, ""}, {"staryu", false, true, ""}, {"yamask", false, false, "starPiece"}} {
		t.Run(scenario.key, func(t *testing.T) {
			state := waScenario(t)
			caster := waDeploy(t, state, "megaStarmie", 1)
			receiver := caster
			if !scenario.self {
				receiver = waDeploy(t, state, scenario.key, 1)
			}
			if scenario.item != "" {
				healingEquip(t, state, receiver, scenario.item)
			}
			if scenario.far {
				moved := false
				for _, tile := range engine.Project(state).Tiles {
					if tile.Terrain == 3 && tile.X+12-state.Towers[0].X > 230 {
						waApply(t, state, 0, engine.Action{Type: "move", PokemonID: receiver, TileID: tile.ID})
						moved = true
						break
					}
				}
				if !moved {
					t.Fatal("no factual water placement beyond the local aura radius")
				}
			}
			waBattle(t, state)
			target := waEnemy(t, state, engine.Point{X: 300, Y: 300})
			healingHit(t, state, receiver, target)
			base := 0.0
			for _, pokemon := range state.Team {
				if pokemon.ID == receiver {
					base = pokemon.Stats.Power
				}
			}
			want := base
			if scenario.self {
				want = base * 2
			}
			if scenario.self || scenario.far || scenario.item != "" {
				want = math.Ceil(want * 1.15)
			}
			if lost := 100000 - persistentAreaEnemy(t, state, target).HP; lost != want {
				t.Fatal("huge power ignored self/global scope or leaked its star-only bonus to a circular partner", lost, want)
			}
			waEvidence(t, "huge-power-"+scenario.key, state, map[string]any{"global": true, "starOnly": true, "selfIncluded": true, "actualDamage": want})
		})
	}
}
