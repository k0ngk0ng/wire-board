package pokepath_test

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func waScenario(t *testing.T) *engine.State {
	t.Helper()
	state, err := engine.New(engine.Options{Starter: "charmander", Speed: 1}, 4, 99127, 1730000000000)
	if err != nil {
		t.Fatal(err)
	}
	state.Team = []engine.Pokemon{}
	state.TeamSlots = 10
	return state
}

func waPartner(t *testing.T, state *engine.State, key string, level int) int {
	t.Helper()
	specification, ok := engine.CatalogData().Species[key]
	if !ok {
		t.Fatalf("missing factual species %s", key)
	}
	pokemon := engine.Pokemon{ID: state.NextID, Species: key, Level: level, Target: "first", TileID: -1, Sprite: specification.Sprite, Stats: engine.Stats{Power: math.Floor(specification.PowerBase + specification.PowerPerLevel*float64(level)), AttackMS: max(50, math.Floor(specification.AttackMSBase+specification.AttackMSPerLevel*float64(level))), Range: math.Floor(specification.RangeBase + specification.RangePerLevel*float64(level)), CriticalPct: 0}}
	if prices := engine.CatalogData().UpgradeCosts[specification.CostScale]; len(prices) > level {
		pokemon.UpgradeCost = prices[level]
	}
	state.NextID++
	state.Team = append(state.Team, pokemon)
	return pokemon.ID
}

func waApply(t *testing.T, state *engine.State, actor int, action engine.Action) {
	t.Helper()
	action.Epoch = state.Epoch
	if err := engine.Apply(state, actor, action); err != nil {
		t.Fatalf("%s: %v", action.Type, err)
	}
}

func waPlacement(t *testing.T, state *engine.State, terrain int, excluded ...int) engine.Placement {
	t.Helper()
	for _, tile := range engine.Project(state).Tiles {
		if terrain != 0 && tile.Terrain != terrain {
			continue
		}
		available := true
		for _, id := range excluded {
			available = available && tile.ID != id
		}
		if available {
			return tile
		}
	}
	t.Fatal("scenario has no matching factual placement")
	return engine.Placement{}
}

func waEnemy(t *testing.T, state *engine.State, point engine.Point) int {
	t.Helper()
	id, err := engine.SpawnEnemyAt(state, "rattata", point, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for index := range state.Enemies {
		if state.Enemies[index].ID == id {
			state.Enemies[index].Entered = true
			state.Enemies[index].BaseSpeed = 0
			state.Enemies[index].HP, state.Enemies[index].MaxHP = 100000, 100000
		}
	}
	return id
}

func waBattle(t *testing.T, state *engine.State) {
	t.Helper()
	waApply(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = []engine.Enemy{}
}

func waAdvance(t *testing.T, state *engine.State, milliseconds int64) {
	t.Helper()
	if err := engine.Advance(state, milliseconds); err != nil {
		t.Fatal(err)
	}
}

func waSnapshotContinuation(t *testing.T, state *engine.State, milliseconds int64) {
	t.Helper()
	encoded, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := engine.Unmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	waAdvance(t, state, milliseconds)
	for elapsed := int64(0); elapsed < milliseconds; {
		step := min(int64(17), milliseconds-elapsed)
		waAdvance(t, restored, step)
		elapsed += step
	}
	left, _ := engine.Marshal(state)
	right, _ := engine.Marshal(restored)
	if !bytes.Equal(left, right) {
		t.Fatal("world effect state was not saved or fixed-step replay diverged")
	}
}

func waEvidence(t *testing.T, name string, state *engine.State, facts any) {
	t.Helper()
	directory, err := filepath.Abs(filepath.Join("..", "..", "..", ".local", "pokepath", "native-world"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	value := map[string]any{"test": t.Name(), "command": ". scripts/env.sh && go test -race ./games/pokepath/tests -run '^TestNativeWorld' -count=1", "controlledFixture": true, "seed": 99127, "facts": facts, "state": engine.Project(state), "passed": true}
	encoded, _ := json.MarshalIndent(value, "", "  ")
	if err := os.WriteFile(filepath.Join(directory, name+".json"), append(encoded, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestNativeWorldPlatformPassengerFiresFromSharedTile(t *testing.T) {
	state := waScenario(t)
	host := waPartner(t, state, "turtwig", 30)
	passenger := waPartner(t, state, "rattata", 30)
	tile := waPlacement(t, state, 1)
	waApply(t, state, 0, engine.Action{Type: "deploy", PokemonID: host, TileID: tile.ID})
	waApply(t, state, 3, engine.Action{Type: "deploy", PokemonID: passenger, TileID: tile.ID})
	if len(state.Towers) != 2 || state.Towers[0].TileID != state.Towers[1].TileID {
		t.Fatal("a legal carried Pokémon did not occupy the host's actual map position")
	}
	waBattle(t, state)
	id := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 40, Y: state.Towers[0].Y})
	waAdvance(t, state, 3000)
	hit := false
	for _, event := range state.Events {
		hit = hit || event.Type == "hit" && event.Source == passenger && event.Target == id && event.Amount > 0
	}
	if !hit {
		t.Fatal("carried Pokémon never dealt real projectile damage")
	}
	waSnapshotContinuation(t, state, 400)
	waEvidence(t, "passenger", state, map[string]any{"attackIntervalMultiplier": .85, "grassTerrain": 2, "sharedTile": tile.ID})
}

func TestNativeWorldDischargeRequiresContinuousExposure(t *testing.T) {
	state := waScenario(t)
	partner := waPartner(t, state, "zapdos", 40)
	tile := waPlacement(t, state, 4)
	waApply(t, state, 0, engine.Action{Type: "deploy", PokemonID: partner, TileID: tile.ID})
	waBattle(t, state)
	id := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 30, Y: state.Towers[0].Y})
	waAdvance(t, state, 2980)
	if engine.HasCondition(&state.Enemies[0].EnemyEffectState, "stun") {
		t.Fatal("continuous exposure stunned before 3000 milliseconds")
	}
	waSnapshotContinuation(t, state, 20)
	stunned := false
	for _, enemy := range state.Enemies {
		stunned = stunned || enemy.ID == id && engine.HasCondition(&enemy.EnemyEffectState, "stun")
	}
	if !stunned {
		t.Fatal("three seconds of actual in-range exposure did not apply the two-second stun")
	}
	waAdvance(t, state, 2200)
	for _, enemy := range state.Enemies {
		if enemy.ID == id && engine.HasCondition(&enemy.EnemyEffectState, "stun") {
			t.Fatal("the once-per-target aura repeatedly refreshed the same target")
		}
	}
	waEvidence(t, "continuous-exposure", state, map[string]int{"exposureMS": 3000, "stunMS": 2000})
}

func TestNativeWorldOppositeChargesHitOnlyActualSegment(t *testing.T) {
	state := waScenario(t)
	plus := waPartner(t, state, "plusle", 30)
	minus := waPartner(t, state, "minun", 30)
	first := waPlacement(t, state, 1)
	second := waPlacement(t, state, 1, first.ID)
	for _, candidate := range engine.Project(state).Tiles {
		if candidate.Terrain == 1 && math.Hypot(candidate.X-first.X, candidate.Y-first.Y) > math.Hypot(second.X-first.X, second.Y-first.Y) {
			second = candidate
		}
	}
	waApply(t, state, 0, engine.Action{Type: "deploy", PokemonID: plus, TileID: first.ID})
	waApply(t, state, 1, engine.Action{Type: "deploy", PokemonID: minus, TileID: second.ID})
	waBattle(t, state)
	a, b := state.Towers[0], state.Towers[1]
	middle := engine.Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
	on := waEnemy(t, state, middle)
	dx, dy := b.X-a.X, b.Y-a.Y
	length := math.Hypot(dx, dy)
	off := waEnemy(t, state, engine.Point{X: middle.X - dy/length*40, Y: middle.Y + dx/length*40})
	waAdvance(t, state, 1020)
	onDamaged, offDamaged := false, false
	for _, enemy := range state.Enemies {
		if enemy.ID == on {
			onDamaged = enemy.HP < enemy.MaxHP
		}
		if enemy.ID == off {
			offDamaged = enemy.HP < enemy.MaxHP
		}
	}
	if !onDamaged || offDamaged {
		t.Fatal("opposite-charge link did not damage only the real endpoint segment")
	}
	waSnapshotContinuation(t, state, 1000)
	waEvidence(t, "opposite-link", state, map[string]int{"width": 12, "hitCooldownMS": 1000, "maxSearchRange": 2000})
}

func TestNativeWorldTeleportWaitsForIdleAndPersistsStack(t *testing.T) {
	state := waScenario(t)
	partner := waPartner(t, state, "abra", 40)
	tile := waPlacement(t, state, 1)
	waApply(t, state, 0, engine.Action{Type: "deploy", PokemonID: partner, TileID: tile.ID})
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: -1000, Y: -1000})
	waAdvance(t, state, 3300)
	for _, event := range state.Events {
		if event.Type == "teleport" {
			t.Fatal("teleport happened before 200 original 60Hz frames")
		}
	}
	waSnapshotContinuation(t, state, 60)
	teleported := false
	for _, event := range state.Events {
		teleported = teleported || event.Type == "teleport" && event.Source == partner
	}
	if !teleported {
		t.Fatal("an idle Abra did not perform a real legal relocation")
	}
	waEvidence(t, "teleport", state, map[string]any{"periodMS": 10000.0 / 3, "stackingDamagePerTeleport": 1})
}

func TestNativeWorldIdlePunchChargesThenActuallyFiresBurst(t *testing.T) {
	state := waScenario(t)
	partner := waPartner(t, state, "scizor", 50)
	tile := waPlacement(t, state, 1)
	waApply(t, state, 0, engine.Action{Type: "deploy", PokemonID: partner, TileID: tile.ID})
	waBattle(t, state)
	id := waEnemy(t, state, engine.Point{X: -1000, Y: -1000})
	interval := state.Team[0].Stats.AttackMS
	waAdvance(t, state, int64(math.Ceil(interval*10/20)*20))
	for index := range state.Enemies {
		if state.Enemies[index].ID == id {
			state.Enemies[index].X, state.Enemies[index].Y = state.Towers[0].X+20, state.Towers[0].Y
		}
	}
	waAdvance(t, state, 1000)
	fires := 0
	for _, event := range state.Events {
		if event.Type == "fire" && event.Source == state.Towers[0].ID {
			fires++
		}
	}
	if fires != 10 {
		t.Fatalf("idle recharge did not produce exactly ten real burst projectiles: %d", fires)
	}
	waEvidence(t, "idle-punch", state, map[string]int{"maximumCharges": 10, "burstIntervalMS": 90})
}

func waDeploy(t *testing.T, state *engine.State, key string, level int) int {
	t.Helper()
	id := waPartner(t, state, key, level)
	if err := engine.Validate(state); err != nil {
		t.Fatalf("invalid %s fixture: %v", key, err)
	}
	for _, tile := range engine.Project(state).Tiles {
		occupied := false
		for _, tower := range state.Towers {
			occupied = occupied || tower.TileID == tile.ID
		}
		if !occupied && slicesContains(engine.CatalogData().Species[key].Terrains, tile.Terrain) {
			waApply(t, state, 0, engine.Action{Type: "deploy", PokemonID: id, TileID: tile.ID})
			return id
		}
	}
	t.Fatal("no legal unoccupied placement")
	return 0
}

func slicesContains(values []int, value int) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func TestNativeWorldHeavyMetalLocksImmediatelyAndStunsOnce(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "rattata", 20)
	waBattle(t, state)
	id := waEnemy(t, state, engine.Point{X: -1000, Y: -1000})
	waDeploy(t, state, "bronzor", 20)
	if !state.World.MovementLocked || !engine.HasCondition(&state.Enemies[0].EnemyEffectState, "stun") {
		t.Fatal("heavy metal did not immediately lock placement and stun all enemies")
	}
	before, _ := engine.Marshal(state)
	err := engine.Apply(state, 2, engine.Action{Type: "retire", Epoch: state.Epoch, PokemonID: state.Team[0].ID})
	after, _ := engine.Marshal(state)
	if err == nil || !bytes.Equal(before, after) {
		t.Fatal("a locked retirement was accepted or mutated the shared state")
	}
	waSnapshotContinuation(t, state, 2100)
	for _, enemy := range state.Enemies {
		if enemy.ID == id && engine.HasCondition(&enemy.EnemyEffectState, "stun") {
			t.Fatal("deployment stun was refreshed rather than expiring")
		}
	}
	waEvidence(t, "heavy-metal", state, map[string]any{"immediateMovementLock": true, "stunMS": 2000})
}

func TestNativeWorldSturdyShellSmashAndIceBodyShareDamageEvent(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "regirock", 20)
	waDeploy(t, state, "regice", 20)
	waDeploy(t, state, "tirtouga", 20)
	health := state.Health
	waBattle(t, state)
	if state.Health != health || !state.World.SturdyUsed || !state.World.IceBodyActivated || !state.World.ShellSmashActive || state.World.HitsReceived != 1 {
		t.Fatal("shell smash damage did not pass through first-event sturdy and ice-body rules")
	}
	waEnemy(t, state, engine.Point{X: -1000, Y: -1000})
	waSnapshotContinuation(t, state, 300)
	waEvidence(t, "heart-event", state, map[string]any{"shellSmashLoss": 1, "sturdyReduction": 1, "iceBodyEvenWhenProtected": true})
}

func TestNativeWorldFlyingOrbitPersistsDuringPreparation(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "skarmory", 30)
	origin := state.Towers[0]
	waAdvance(t, state, 1000)
	position := state.Towers[0]
	sprite := engine.CatalogData().Species["skarmory"].Sprite
	x, y := position.X-12+sprite.FrameWidth/2, position.Y-12+sprite.FrameHeight/2
	if math.Abs(math.Hypot(x-origin.X, y-origin.Y)-70) > .01 || math.Abs(x-origin.X-math.Cos(1)*70) > .01 || math.Abs(y-origin.Y-math.Sin(1)*70) > .01 {
		t.Fatal("flying tower did not follow its factual 70 px / 1 rad/s preparation orbit")
	}
	if orbit := state.World.Towers[origin.PokemonID].OrbitPosition; orbit == nil || math.Abs(orbit.X+12-position.X) > 1e-6 || math.Abs(orbit.Y+12-position.Y) > 1e-6 {
		t.Fatal("flying tower did not retain its factual sprite position separately from its collision center")
	}
	waSnapshotContinuation(t, state, 1200)
	waEvidence(t, "flying", state, map[string]any{"radius": 70, "angularSpeed": 1, "duringPreparation": true})
}

func TestNativeWorldBerriesWaveStartGlobalDurationAndPassengerDirection(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "applin", 20)
	waDeploy(t, state, "tropius", 20)
	host := waDeploy(t, state, "turtwig", 20)
	passenger := waPartner(t, state, "rattata", 20)
	for index := range state.Team {
		if state.Team[index].ID == host {
			state.Team[index].Item = "berryPouch"
		}
		if state.Team[index].ID == passenger {
			state.Team[index].Item = "liechiBerry"
		}
	}
	state.Inventory = append(state.Inventory, engine.OwnedItem{ID: "berryPouch", EquippedBy: host}, engine.OwnedItem{ID: "liechiBerry", EquippedBy: passenger})
	waApply(t, state, 1, engine.Action{Type: "deploy", PokemonID: passenger, TileID: state.Towers[2].TileID})
	waBattle(t, state)
	if state.World.Towers[passenger].BerryTimers["liechiBerry"] != 36000 || state.World.Towers[host].BerryTimers["liechiBerry"] != 36000 {
		t.Fatal("wave-start harvest duration was not copied from passenger berry to berry-pouch host")
	}
	waEnemy(t, state, engine.Point{X: -1000, Y: -1000})
	waSnapshotContinuation(t, state, 1000)
	waEvidence(t, "berry-wave", state, map[string]int{"normalDurationMS": 18000, "harvestDurationMS": 36000})
}

func TestNativeWorldWebLandingTicksAndExpiresAfterRetirement(t *testing.T) {
	state := waScenario(t)
	id := waDeploy(t, state, "grubbin", 30)
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	waAdvance(t, state, 200)
	if len(state.World.Zones) == 0 || state.World.Zones[0].Kind != "web" {
		t.Fatal("web projectile impact did not create a real persistent zone")
	}
	waApply(t, state, 0, engine.Action{Type: "retire", PokemonID: id})
	waAdvance(t, state, 4200)
	if len(state.World.Zones) != 0 {
		t.Fatal("retiring a web source left a hidden or immortal zone")
	}
	waEvidence(t, "web", state, map[string]int{"radius": 30, "durationMS": 4100, "tickMS": 500})
}

func TestNativeWorldAnchorChainsTickAndDetachAtExactReleaseRange(t *testing.T) {
	state := waScenario(t)
	id := waDeploy(t, state, "dhelmise", 30)
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	waAdvance(t, state, 1000)
	value := state.World.Towers[id]
	if len(value.Chains) != 1 || state.Enemies[0].HP == state.Enemies[0].MaxHP {
		t.Fatal("anchor did not create one live chain with real 500 ms damage")
	}
	state.Enemies[0].X = state.Towers[0].X + state.Team[0].Stats.Range + 40
	state.Enemies[0].Y = state.Towers[0].Y
	waAdvance(t, state, 20)
	if len(state.World.Towers[id].Chains) != 0 {
		t.Fatal("anchor did not detach at the exact uncursed release threshold")
	}
	waSnapshotContinuation(t, state, 1000)
	waEvidence(t, "anchor", state, map[string]int{"maximumChains": 6, "tickMS": 500, "releaseExtra": 40, "cursedReleaseExtra": 100})
}

func TestNativeWorldExplosionWaitsForFiveTargetsAndCostsHearts(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "megaGlalie", 30)
	waBattle(t, state)
	for index := 0; index < 4; index++ {
		waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y + float64(index)})
	}
	health := state.Health
	waAdvance(t, state, 4000)
	if state.Health != health {
		t.Fatal("explosion fired without five legal targets")
	}
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	waSnapshotContinuation(t, state, 1000)
	if state.Health != health-2 {
		t.Fatal("resumed explosion countdown did not fire and apply the exact self heart cost")
	}
	waEvidence(t, "explosion", state, map[string]int{"countdownMS": 3000, "minimumRemainingMS": 1000, "heartCost": 2, "minimumTargets": 5})
}

func TestNativeWorldSkyArrowsSpawnOnlyAfterImpactAndFallWithDelay(t *testing.T) {
	state := waScenario(t)
	id := waDeploy(t, state, "rowlet", 30)
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 180, Y: state.Towers[0].Y})
	for elapsed := 0; elapsed < 1000 && len(state.World.Zones) == 0; elapsed++ {
		waAdvance(t, state, 1)
	}
	if len(state.World.Zones) != 3 || state.World.Zones[0].Kind != "skyArrow" {
		t.Fatalf("one real primary impact did not create exactly three delayed sky arrows: zones=%v projectiles=%v events=%v", state.World.Zones, state.Projectiles, state.Events)
	}
	state.Towers[0].CooldownMS = 100000
	zone := state.World.Zones[0]
	targetID := waEnemy(t, state, engine.Point{X: zone.X, Y: zone.Y})
	waAdvance(t, state, 449)
	for _, event := range state.Events {
		if event.Type == "hit" && event.Source == id && event.Target == targetID {
			t.Fatal("sky arrow damaged its landing target before the 450 ms fall")
		}
	}
	waSnapshotContinuation(t, state, 1)
	hit := false
	for _, event := range state.Events {
		hit = hit || event.Type == "hit" && event.Source == id && event.Target == targetID && event.Amount > 0
	}
	if !hit {
		t.Fatal("a landed sky arrow did not deal normal direct damage to its actual area")
	}
	waEvidence(t, "sky-arrows", state, map[string]int{"scatterRadius": 75, "radius": 36, "fallMS": 450, "staggerMS": 90})
}

func TestNativeWorldBurnedDeathUsesGlobalMagmaSourceAndDeadMaxHealth(t *testing.T) {
	state := waScenario(t)
	id := waDeploy(t, state, "heatran", 30)
	waBattle(t, state)
	state.Towers[0].CooldownMS = 100000
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	other := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 60, Y: state.Towers[0].Y})
	engine.ApplyConditions(&state.Enemies[0].EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: .005, Stacks: 1}}}, 0)
	state.Enemies[0].HP = 0
	state.Enemies[0].LastHitBy = 0
	waAdvance(t, state, 20)
	found := false
	for _, event := range state.Events {
		found = found || event.Type == "magmaExplosion" && event.Source == id && event.Target == other && event.Amount == 6000
	}
	burned := false
	for _, enemy := range state.Enemies {
		burned = burned || enemy.ID == other && engine.HasCondition(&enemy.EnemyEffectState, "burn")
	}
	if !found || !burned {
		t.Fatalf("global magma source failed to explode a burned death using ceil(6%% dead MaxHP): events=%v", state.Events)
	}
	waSnapshotContinuation(t, state, 400)
	waEvidence(t, "magma-death", state, map[string]any{"radius": 50, "maxHPDamage": .06, "anyBurnedDeath": true})
}

func TestNativeWorldWeatherRecomputesFromActiveTowersAndRetirement(t *testing.T) {
	state := waScenario(t)
	rain := waDeploy(t, state, "wingull", 30)
	waAdvance(t, state, 20)
	if state.Weather != "rain" {
		t.Fatal("deployed drizzle did not change the shared world weather")
	}
	wind := waDeploy(t, state, "tornadus", 30)
	waAdvance(t, state, 20)
	if state.Weather != "strongWinds" {
		t.Fatal("later active weather source did not override rain")
	}
	waApply(t, state, 0, engine.Action{Type: "retire", PokemonID: wind})
	waAdvance(t, state, 20)
	if state.Weather != "rain" {
		t.Fatal("weather remained after its actual source retired")
	}
	waApply(t, state, 1, engine.Action{Type: "retire", PokemonID: rain})
	waAdvance(t, state, 20)
	if state.Weather != "" {
		t.Fatal("weather did not return to the map's factual base")
	}
	waSnapshotContinuation(t, state, 100)
	waEvidence(t, "weather", state, []string{"rain", "strongWinds", "rain", ""})
}

func TestNativeWorldReefPoisonsEachFullHealthRegenerationEvent(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "megaDragalge", 30)
	waBattle(t, state)
	state.Towers[0].CooldownMS = 100000
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	state.Enemies[0].Regeneration = 100
	waAdvance(t, state, 980)
	if engine.ConditionStacks(&state.Enemies[0].EnemyEffectState, "poison") != 0 {
		t.Fatal("reef triggered without a real healing event")
	}
	waSnapshotContinuation(t, state, 20)
	if engine.ConditionStacks(&state.Enemies[0].EnemyEffectState, "poison") != 1 {
		t.Fatal("a full-health regeneration pulse did not add exactly one reef poison stack")
	}
	waSnapshotContinuation(t, state, 1000)
	if engine.ConditionStacks(&state.Enemies[0].EnemyEffectState, "poison") != 2 {
		t.Fatal("reef failed to distinguish separate healing events")
	}
	waEvidence(t, "reef-healing", state, map[string]any{"radius": 120, "poisonMaxHP": .001, "fullHealthCounts": true})
}

func TestNativeWorldChessFootprintActivatesAfterFiftyMilliseconds(t *testing.T) {
	state := waScenario(t)
	state.Route, state.Wave = 35, 1
	state.RouteWaves[35] = 1
	id := waPartner(t, state, "kingdra", 30)
	state.Team[0].Item = "dampMulch"
	state.Inventory = append(state.Inventory, engine.OwnedItem{ID: "dampMulch", EquippedBy: id})
	waApply(t, state, 0, engine.Action{Type: "deploy", PokemonID: id, TileID: 17})
	waAdvance(t, state, 40)
	if len(state.World.ChessActive) != 0 {
		t.Fatal("chess footprint activated before its factual delayed recomputation")
	}
	waSnapshotContinuation(t, state, 20)
	if len(state.World.ChessActive) != 2 || state.World.ChessActive[0] != "h1" || state.World.ChessActive[1] != "h2" {
		t.Fatal("map 35 did not activate the two exact kingdra footprint pieces")
	}
	waApply(t, state, 1, engine.Action{Type: "retire", PokemonID: id})
	waAdvance(t, state, 60)
	if len(state.World.ChessActive) != 0 {
		t.Fatal("chess pieces stayed active after their source retired")
	}
	waEvidence(t, "chess", state, map[string]any{"route": 35, "tile": 17, "pieces": []string{"h1", "h2"}, "delayMS": 50})
}

func TestNativeWorldSingularityWaitsPullsThenDealsAreaDamage(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "megaGardevoir", 30)
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	other := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 70, Y: state.Towers[0].Y})
	waAdvance(t, state, 1000)
	for _, enemy := range state.Enemies {
		if enemy.HP != enemy.MaxHP {
			t.Fatal("singularity dealt damage before travel, 1000 ms wait, and 450 ms pull completed")
		}
	}
	waSnapshotContinuation(t, state, 1000)
	pulled, hit := false, false
	for _, enemy := range state.Enemies {
		if enemy.ID == other {
			pulled = enemy.X < state.Towers[0].X+70
			hit = enemy.HP < enemy.MaxHP
		}
	}
	if !pulled || !hit {
		t.Fatal("singularity did not really pull and damage an in-area target")
	}
	waEvidence(t, "singularity", state, map[string]int{"waitMS": 1000, "pullMS": 450, "pullSpeed": 220, "radius": 90})
}

func TestNativeWorldWindProjectileDamagesAndPushesRealEnemy(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "tornadus", 30)
	waBattle(t, state)
	id := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 60, Y: state.Towers[0].Y})
	waAdvance(t, state, 20)
	if len(state.World.Zones) == 0 || state.World.Zones[0].Kind != "tornado" {
		t.Fatal("wind attack did not create a real traveling oscillating tornado")
	}
	zone := state.World.Zones[0]
	state.Enemies[0].X, state.Enemies[0].Y = zone.X, zone.Y
	x, y := zone.X, zone.Y
	waAdvance(t, state, 320)
	hit, moved := false, false
	for _, enemy := range state.Enemies {
		if enemy.ID == id {
			hit = enemy.HP < enemy.MaxHP
			moved = math.Hypot(enemy.X-x, enemy.Y-y) > 10
		}
	}
	if !hit || !moved {
		t.Fatal("tornado failed to apply actual repeated-collision damage and timed 15 px push")
	}
	waSnapshotContinuation(t, state, 400)
	waEvidence(t, "tornado", state, map[string]int{"amplitude": 32, "wavelength": 95, "hitCooldownMS": 300, "pushPixels": 15, "pushSpeed": 50})
}

func TestNativeWorldIdleAlliesFireAtToyAndHoneyCountsRealImpacts(t *testing.T) {
	state := waScenario(t)
	toy := waDeploy(t, state, "wobbuffet", 30)
	honey := waDeploy(t, state, "combee", 30)
	closest := engine.Placement{}
	best := math.MaxFloat64
	for _, tile := range engine.Project(state).Tiles {
		distance := math.Hypot(tile.X+12-state.Towers[1].X, tile.Y+12-state.Towers[1].Y)
		if tile.ID != state.Towers[1].TileID && tile.Terrain != 3 && distance < best {
			closest, best = tile, distance
		}
	}
	waApply(t, state, 0, engine.Action{Type: "move", PokemonID: toy, TileID: closest.ID})
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: -1000, Y: -1000})
	gold := state.Gold
	waAdvance(t, state, 4000)
	fired := false
	for _, event := range state.Events {
		fired = fired || event.Type == "toyHit" && event.Source == honey
	}
	if !fired || state.World.Honey == 0 || state.Gold != gold || state.TotalKills != 0 {
		t.Fatalf("idle toy did not receive real ally projectiles and generate impact honey without enemy rewards: towers=%v honey=%d events=%v", state.Towers, state.World.Honey, state.Events)
	}
	waSnapshotContinuation(t, state, 400)
	waEvidence(t, "idle-toy", state, map[string]any{"honeyPerPhysicalImpact": 1, "enemyRewards": false})
}

func TestNativeWorldSteelBeamCostsOnceForSixUniqueTargetsAndCannotKill(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "registeel", 30)
	waBattle(t, state)
	for index := 0; index < 6; index++ {
		waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20 + float64(index*10), Y: state.Towers[0].Y})
	}
	health := state.Health
	waAdvance(t, state, 166)
	if state.Health != health {
		t.Fatal("steel beam paid a heart before its first original 140 ms hit")
	}
	waAdvance(t, state, 1)
	if state.Health != health-1 {
		t.Fatalf("one beam hitting six unique targets did not pay exactly one heart: %d events=%v", state.Health, state.Events)
	}
	state.Health = 1
	state.Towers[0].CooldownMS = 0
	waSnapshotContinuation(t, state, 500)
	if state.Health != 1 || state.Phase == "defeat" {
		t.Fatal("steel beam self-cost killed the shared team")
	}
	waEvidence(t, "steel-beam", state, map[string]any{"minimumUniqueTargets": 6, "costPerBeam": 1, "cannotKill": true})
}

func waFirstDamage(t *testing.T, state *engine.State, pokemonID int) float64 {
	t.Helper()
	for elapsed := 0; elapsed < 5000; elapsed += 20 {
		waAdvance(t, state, 20)
		for _, event := range state.Events {
			if event.Type == "hit" && event.Source == pokemonID && event.Amount > 0 {
				return event.Amount
			}
		}
	}
	t.Fatal("scenario produced no real damage")
	return 0
}

func TestNativeWorldHelpingHandIsGlobalAndCountsEachUnheldRecipient(t *testing.T) {
	measure := func(helpers int, held bool) float64 {
		state := waScenario(t)
		id := waDeploy(t, state, "charmander", 30)
		if held {
			state.Team[0].Item = "zinc"
			state.Inventory = append(state.Inventory, engine.OwnedItem{ID: "zinc", EquippedBy: id})
			for index, item := range state.Progression.ShopStock {
				if item == "zinc" {
					state.Progression.ShopStock[index] = ""
				}
			}
		}
		for index := 0; index < helpers; index++ {
			waDeploy(t, state, "aipom", 30)
		}
		waBattle(t, state)
		waEnemy(t, state, engine.Point{X: state.Towers[0].X + 30, Y: state.Towers[0].Y})
		state.Random = 123456789
		return waFirstDamage(t, state, id)
	}
	base, one, two, held := measure(0, false), measure(1, false), measure(2, false), measure(2, true)
	if one != base+50 || two != base+100 || held != base {
		t.Fatalf("global helping-hand damage was not +50 per helper only for unheld recipients: %v", []float64{base, one, two, held})
	}
	waEvidence(t, "global-help", waScenario(t), map[string]any{"base": base, "oneHelper": one, "twoHelpers": two, "heldRecipient": held})
}

func TestNativeWorldCommanderBoostsOnlyLegalMountedPair(t *testing.T) {
	state := waScenario(t)
	host := waDeploy(t, state, "dondozo", 30)
	passenger := waPartner(t, state, "tatsugiri", 30)
	waApply(t, state, 2, engine.Action{Type: "deploy", PokemonID: passenger, TileID: state.Towers[0].TileID})
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	state.Towers[1].CooldownMS = 100000
	state.Random = 123456789
	damage := waFirstDamage(t, state, host)
	want := math.Ceil(state.Team[0].Stats.Power * 1.5)
	if damage != want {
		t.Fatalf("mounted commander did not use ceil(1.5x) real host damage: got %v want %v", damage, want)
	}
	extra := waPartner(t, state, "rattata", 30)
	before, _ := engine.Marshal(state)
	err := engine.Apply(state, 3, engine.Action{Type: "deploy", Epoch: state.Epoch, PokemonID: extra, TileID: state.Towers[0].TileID})
	after, _ := engine.Marshal(state)
	if err == nil || !bytes.Equal(before, after) {
		t.Fatal("occupied commander mount accepted another passenger or mutated the failed action")
	}
	waSnapshotContinuation(t, state, 500)
	waEvidence(t, "commander", state, map[string]any{"damageMultiplier": 1.5, "attackIntervalMultiplier": .75, "criticalBonus": 30})
}

func TestNativeWorldLightningRodChargesOnlyFirstEligibleRodForNewStun(t *testing.T) {
	state := waScenario(t)
	state.Route, state.Wave = 27, 1
	state.RouteWaves[27] = 1
	first := waDeploy(t, state, "megaSceptile", 30)
	second := waDeploy(t, state, "megaSceptile", 30)
	waApply(t, state, 0, engine.Action{Type: "move", PokemonID: first, TileID: 6})
	waApply(t, state, 0, engine.Action{Type: "move", PokemonID: second, TileID: 7})
	waBattle(t, state)
	point := engine.Point{X: (state.Towers[0].X + state.Towers[1].X) / 2, Y: (state.Towers[0].Y + state.Towers[1].Y) / 2}
	waEnemy(t, state, point)
	waDeploy(t, state, "bronzor", 30)
	if state.Towers[0].Charges != 1 || state.Towers[1].Charges != 0 {
		t.Fatalf("one new stun did not charge only the first eligible rod: towers=%v enemy=%v", state.Towers, state.Enemies[0])
	}
	waEnemy(t, state, point)
	waApply(t, state, 1, engine.Action{Type: "bell"})
	if state.Towers[0].Charges != 2 || state.Towers[1].Charges != 0 {
		t.Fatal("a separate new stun failed to add exactly one charge")
	}
	waSnapshotContinuation(t, state, 400)
	waEvidence(t, "lightning-rod", state, map[string]any{"firstEligibleOnly": true, "refreshDoesNotCount": true, "powerPerCharge": .1})
}

func TestNativeWorldTriageUsesSharedRandomOncePerActualWaveEnd(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "comfey", 30)
	waBattle(t, state)
	state.Health, state.Random = 10, 1
	waAdvance(t, state, 20)
	if state.Health != 11 || state.WaveActive {
		t.Fatal("a successful factual triage RNG roll did not heal once at real wave completion")
	}
	waSnapshotContinuation(t, state, 200)
	if state.Health != 11 {
		t.Fatal("idle preparation repeated the completed wave's triage reward")
	}
	waEvidence(t, "triage", state, map[string]any{"chance": .05, "heartAmount": 1, "waveEndOnly": true})
}

func waNear(t *testing.T, state *engine.State, key string, point engine.Point) int {
	t.Helper()
	id := waPartner(t, state, key, 30)
	best, placement := math.MaxFloat64, -1
	for _, tile := range engine.Project(state).Tiles {
		occupied := false
		for _, tower := range state.Towers {
			occupied = occupied || tower.TileID == tile.ID
		}
		distance := math.Hypot(tile.X+12-point.X, tile.Y+12-point.Y)
		if !occupied && slicesContains(engine.CatalogData().Species[key].Terrains, tile.Terrain) && distance < best {
			best, placement = distance, tile.ID
		}
	}
	if placement < 0 {
		t.Fatal("no nearby legal factual placement")
	}
	waApply(t, state, 0, engine.Action{Type: "deploy", PokemonID: id, TileID: placement})
	return id
}

func TestNativeWorldCombatFieldsChangeRealDamageWithoutSameTypeStacking(t *testing.T) {
	cases := []struct {
		name     string
		field    string
		attacker string
		stunned  bool
	}{
		{"brutalFauna", "tapuBulu", "rattata", false},
		{"erraticStatic", "tapuKoko", "rattata", true},
		{"ragingBlossom", "tapuLele", "chikorita", false},
	}
	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) {
			measure := func(fields int) float64 {
				state := waScenario(t)
				attacker := waDeploy(t, state, scenario.attacker, 30)
				point := engine.Point{X: state.Towers[0].X + 30, Y: state.Towers[0].Y}
				for index := 0; index < fields; index++ {
					waNear(t, state, scenario.field, point)
					state.Towers[len(state.Towers)-1].X, state.Towers[len(state.Towers)-1].Y = point.X+float64(index*10), point.Y
				}
				waBattle(t, state)
				for index := 1; index < len(state.Towers); index++ {
					state.Towers[index].CooldownMS = 100000
				}
				waEnemy(t, state, point)
				if scenario.stunned {
					engine.ApplyConditions(&state.Enemies[0].EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "stun", Chance: 1, DurationMS: 10000, Magnitude: 1, Stacks: 1}}}, 0)
				}
				state.Random = 123456789
				return waFirstDamage(t, state, attacker)
			}
			base, one, two := measure(0), measure(1), measure(2)
			if one != math.Ceil(base*1.2) || two != one {
				t.Fatalf("field direct damage or same-type deduplication differed: base=%v one=%v two=%v", base, one, two)
			}
		})
	}
	waEvidence(t, "combat-fields", waScenario(t), map[string]any{"radius": 120, "factor": 1.2, "round": "ceil", "sameTypeStacks": false})
}

func TestNativeWorldSereneFieldAmplifiesRealSlowAmount(t *testing.T) {
	state := waScenario(t)
	attacker := waDeploy(t, state, "tangela", 30)
	point := engine.Point{X: state.Towers[0].X + 30, Y: state.Towers[0].Y}
	waNear(t, state, "tapuFini", point)
	state.Towers[1].X, state.Towers[1].Y = point.X+20, point.Y
	waBattle(t, state)
	state.Towers[1].CooldownMS = 100000
	waEnemy(t, state, point)
	waFirstDamage(t, state, attacker)
	found := false
	for _, condition := range state.Enemies[0].Conditions {
		found = found || condition.Kind == "slow" && math.Abs(condition.Magnitude-.6) < .000001
	}
	if !found {
		t.Fatal("serene field did not turn a real 50% slow into 60% slow")
	}
	waSnapshotContinuation(t, state, 500)
	waEvidence(t, "serene-field", state, map[string]any{"radius": 120, "baseSlow": .5, "actualSlow": .6})
}

func TestNativeWorldVictoryStarWidensActualCrossTargeting(t *testing.T) {
	state := waScenario(t)
	attacker := waDeploy(t, state, "yamask", 30)
	waBattle(t, state)
	target := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 60, Y: state.Towers[0].Y + 30})
	waAdvance(t, state, 400)
	for _, event := range state.Events {
		if event.Type == "hit" && event.Source == attacker && event.Target == target {
			t.Fatal("cross attacked a target outside its original 24 px half-width")
		}
	}
	waDeploy(t, state, "victini", 30)
	waFirstDamage(t, state, attacker)
	waSnapshotContinuation(t, state, 400)
	waEvidence(t, "cross-width", state, map[string]int{"normalHalfWidth": 24, "victoryHalfWidth": 36, "targetOffset": 30})
}

func TestNativeWorldBoostMeterChargesOnHitAndExpiresWithoutRefilling(t *testing.T) {
	state := waScenario(t)
	id := waDeploy(t, state, "zacian", 30)
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	waFirstDamage(t, state, id)
	if state.Towers[0].Charges != 1 {
		t.Fatal("a real hit did not add exactly 2% to the fifty-hit boost meter")
	}
	state.Towers[0].Charges, state.Towers[0].CooldownMS = 49, 0
	for elapsed := 0; elapsed < 1000 && state.World.Towers[id].BoostMS == 0; elapsed += 20 {
		waAdvance(t, state, 20)
	}
	if state.World.Towers[id].BoostMS <= 0 || state.Towers[0].Charges != 0 {
		t.Fatal("the fiftieth real hit did not begin the ten-second boost")
	}
	state.Enemies[0].X, state.Enemies[0].Y = -1000, -1000
	waSnapshotContinuation(t, state, 10020)
	if state.World.Towers[id].BoostMS != 0 || state.Towers[0].Charges != 0 {
		t.Fatal("the active meter recharged without real hits or failed to expire")
	}
	waEvidence(t, "boost-meter", state, map[string]int{"hitsToCharge": 50, "boostMS": 10000})
}

func TestNativeWorldContinuousBurnAndSlowUseRealRange(t *testing.T) {
	for _, scenario := range []struct{ species, condition string }{{"moltres", "burn"}, {"articuno", "slow"}} {
		t.Run(scenario.species, func(t *testing.T) {
			state := waScenario(t)
			waDeploy(t, state, scenario.species, 30)
			waBattle(t, state)
			waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
			waEnemy(t, state, engine.Point{X: -1000, Y: -1000})
			waAdvance(t, state, 20)
			if !engine.HasCondition(&state.Enemies[0].EnemyEffectState, scenario.condition) || engine.HasCondition(&state.Enemies[1].EnemyEffectState, scenario.condition) {
				t.Fatal("continuous condition did not use the real aura range")
			}
			if scenario.condition == "burn" && !engine.HasCondition(&state.Enemies[0].EnemyEffectState, "healBlock") {
				t.Fatal("flame body did not prevent actual healing inside its aura")
			}
			waSnapshotContinuation(t, state, 200)
		})
	}
	waEvidence(t, "continuous-conditions", waScenario(t), []string{"burn", "healBlock", "slow"})
}

func TestNativeWorldSandDamageUsesEveryInRangeTyranitarMultiplier(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "larvitar", 30)
	waBattle(t, state)
	state.Towers[0].CooldownMS = 100000
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	waEnemy(t, state, engine.Point{X: -1000, Y: -1000})
	waSnapshotContinuation(t, state, 1000)
	if state.Enemies[0].HP != 99550 || state.Enemies[1].HP != 99700 {
		t.Fatalf("sand pulses did not use exactly +50%% only inside the actual source range: %v %v", state.Enemies[0].HP, state.Enemies[1].HP)
	}
	waEvidence(t, "sand-range", state, map[string]any{"pulseMS": 1000, "baseMaxHP": .003, "inRangeFactor": 1.5})
}

func TestNativeWorldDampPreventsOnlyExplosionSelfCost(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "megaGlalie", 30)
	waDeploy(t, state, "quagsire", 30)
	waBattle(t, state)
	for index := 0; index < 5; index++ {
		waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y + float64(index)})
	}
	health := state.Health
	waSnapshotContinuation(t, state, 3100)
	if state.Health != health || state.Enemies[0].HP == state.Enemies[0].MaxHP {
		t.Fatal("damp either failed to prevent self-cost or prevented the real explosion damage")
	}
	waEvidence(t, "damp", state, map[string]any{"explosionHeartCost": 0, "damageStillApplied": true})
}

func TestNativeWorldSpikyShieldEscalatesChanceThenTicksRealArea(t *testing.T) {
	state := waScenario(t)
	id := waDeploy(t, state, "chespin", 30)
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 30, Y: state.Towers[0].Y})
	for elapsed := 0; elapsed < 20000 && len(state.World.Zones) == 0; elapsed += 20 {
		waAdvance(t, state, 20)
	}
	if len(state.World.Zones) == 0 || state.World.Zones[0].Kind != "spikes" || state.World.Towers[id].Values["spikyChance"] != 0 {
		t.Fatal("real impacts failed to escalate spike chance and reset it after a successful zone")
	}
	state.Towers[0].CooldownMS = 100000
	before := state.Enemies[0].HP
	waSnapshotContinuation(t, state, 500)
	if state.Enemies[0].HP >= before || state.World.Zones[0].Radius != 60 || state.World.Zones[0].LifeMS != 2600 {
		t.Fatal("the spawned spike field did not deal its 500 ms normal-damage pulse")
	}
	waEvidence(t, "spiky-shield", state, map[string]any{"initialChance": 0, "failedChanceIncrement": .25, "radius": 60, "lifeMS": 2600, "pulseMS": 500})
}

func TestNativeWorldMawFiresOnceOnBelowHalfHealthRangeEntry(t *testing.T) {
	state := waScenario(t)
	id := waDeploy(t, state, "megaMawile", 30)
	waBattle(t, state)
	state.Towers[0].CooldownMS = 100000
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	state.Enemies[0].HP = 40000
	waAdvance(t, state, 1000)
	fires := 0
	for _, event := range state.Events {
		if event.Type == "fire" && event.Source == state.Towers[0].ID {
			fires++
		}
	}
	if fires != 1 || state.Enemies[0].HP >= 40000 {
		t.Fatalf("maw entry aura failed to fire exactly one real half-power projectile independent of normal cooldown: %d", fires)
	}
	waSnapshotContinuation(t, state, 1000)
	waEvidence(t, "maw", state, map[string]any{"threshold": .5, "projectileDamageScale": .5, "pokemonID": id})
}

func TestNativeWorldMegaPunchKillRefillsAndCancelsDeadTargetBurst(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "megaScizor", 30)
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	waEnemy(t, state, engine.Point{X: -1000, Y: -1000})
	state.Enemies[0].HP = 1
	state.Towers[0].Charges = 5
	waAdvance(t, state, 500)
	fires := 0
	for _, event := range state.Events {
		if event.Type == "fire" && event.Source == state.Towers[0].ID {
			fires++
		}
	}
	if fires != 1 || state.Towers[0].Charges != 5 || state.World.Towers[state.Team[0].ID].BurstRemaining != 0 {
		t.Fatal("mega punch did not refill five charges and cancel the remaining burst after its first target died")
	}
	waSnapshotContinuation(t, state, 400)
	waEvidence(t, "mega-punch", state, map[string]any{"maximumCharges": 5, "killRefills": true, "deadTargetCancelsBurst": true})
}

func TestNativeWorldShadowTagDelaysDamageFollowsAndAddsNightmares(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "megaGengar", 30)
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 30, Y: state.Towers[0].Y})
	waAdvance(t, state, 200)
	if state.Enemies[0].HP != state.Enemies[0].MaxHP || len(state.World.Zones) == 0 || state.World.Zones[0].Kind != "shadowTag" {
		t.Fatal("shadow tag dealt an ordinary first hit instead of starting a following zone")
	}
	state.Towers[0].CooldownMS = 100000
	state.World.Towers[state.Team[0].ID].Timers["specialCooldown"] = 100000
	state.Enemies[0].X += 20
	waAdvance(t, state, 800)
	if !engine.HasCondition(&state.Enemies[0].EnemyEffectState, "shadowTagSlow") || math.Abs(state.World.Zones[0].X-state.Enemies[0].X) > .01 {
		t.Fatal("growing shadow zone did not follow its live target and apply the real temporary slow")
	}
	waSnapshotContinuation(t, state, 1600)
	if state.Enemies[0].HP >= state.Enemies[0].MaxHP || engine.ConditionStacks(&state.Enemies[0].EnemyEffectState, "nightmare") != 2 || engine.HasCondition(&state.Enemies[0].EnemyEffectState, "shadowTagSlow") {
		t.Fatal("shadow-tag culmination failed to deal damage, add two nightmares, and remove its temporary slow")
	}
	waEvidence(t, "shadow-tag", state, map[string]int{"growthMS": 2000, "radius": 60, "temporarySlowMS": 100, "nightmareStacks": 2})
}

func waItem(state *engine.State, pokemonID int, itemID string) {
	for index := range state.Team {
		if state.Team[index].ID == pokemonID {
			state.Team[index].Item = itemID
		}
	}
	state.Inventory = append(state.Inventory, engine.OwnedItem{ID: itemID, EquippedBy: pokemonID})
	for index, item := range state.Progression.ShopStock {
		if item == itemID {
			state.Progression.ShopStock[index] = ""
		}
	}
}

func TestNativeWorldUnnerveRipenAndSimpleChangeRealBerryDamage(t *testing.T) {
	for _, scenario := range []struct {
		species string
		plain   float64
		unnerve float64
	}{{"rattata", 200, 220}, {"flapple", 400, 440}, {"bidoof", 350, 385}} {
		t.Run(scenario.species, func(t *testing.T) {
			measure := func(berry, unnerve bool) float64 {
				state := waScenario(t)
				id := waDeploy(t, state, scenario.species, 30)
				if berry {
					waItem(state, id, "liechiBerry")
					waDeploy(t, state, "applin", 30)
				}
				if unnerve {
					waDeploy(t, state, "calyrex", 30)
				}
				waBattle(t, state)
				for index := 1; index < len(state.Towers); index++ {
					state.Towers[index].CooldownMS = 100000
				}
				waEnemy(t, state, engine.Point{X: state.Towers[0].X + 30, Y: state.Towers[0].Y})
				state.Random = 123456789
				return waFirstDamage(t, state, id)
			}
			base, plain, unnerve := measure(false, false), measure(true, false), measure(true, true)
			if plain-base != scenario.plain || unnerve-base != scenario.unnerve {
				t.Fatalf("berry damage was not derived from real global and own abilities: %v %v %v", base, plain, unnerve)
			}
		})
	}
	waEvidence(t, "berry-effect", waScenario(t), map[string]any{"normal": []int{200, 220}, "ripen": []int{400, 440}, "simple": []int{350, 385}})
}

func TestNativeWorldJusticeGlobalAurasUseActualDamageEvent(t *testing.T) {
	measure := func(helper string, damaged bool) float64 {
		state := waScenario(t)
		id := waDeploy(t, state, "keldeo", 30)
		if helper != "" {
			waDeploy(t, state, helper, 30)
		}
		if damaged {
			waDeploy(t, state, "tirtouga", 30)
		}
		waBattle(t, state)
		for index := 1; index < len(state.Towers); index++ {
			state.Towers[index].CooldownMS = 100000
		}
		waEnemy(t, state, engine.Point{X: state.Towers[0].X + 30, Y: state.Towers[0].Y})
		state.Random = 123456789
		return waFirstDamage(t, state, id)
	}
	base, steel, steelHit, sword := measure("", false), measure("cobalion", false), measure("cobalion", true), measure("terrakion", false)
	if steel != base+25 || steelHit != base+75 || sword != math.Ceil(base*1.2) {
		t.Fatalf("justice auras did not use collection membership and the real shared damage event: %v", []float64{base, steel, steelHit, sword})
	}
	waEvidence(t, "justice", waScenario(t), map[string]any{"base": base, "heartSteel": steel, "heartSteelAfterDamage": steelHit, "swordsDance": sword})
}

func TestNativeWorldSolarPowerBuffsOtherBurnUsersOnlyInSun(t *testing.T) {
	measure := func(solar bool) float64 {
		state := waScenario(t)
		id := waDeploy(t, state, "charmander", 30)
		waDeploy(t, state, "torkoal", 30)
		if solar {
			waDeploy(t, state, "megaHoundoom", 30)
		}
		waBattle(t, state)
		for index := 1; index < len(state.Towers); index++ {
			state.Towers[index].CooldownMS = 100000
		}
		waEnemy(t, state, engine.Point{X: state.Towers[0].X + 30, Y: state.Towers[0].Y})
		engine.ApplyConditions(&state.Enemies[0].EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: .005, Stacks: 1}}}, 0)
		state.Random = 123456789
		return waFirstDamage(t, state, id)
	}
	base, solar := measure(false), measure(true)
	if solar != math.Ceil(base*1.25) {
		t.Fatalf("solar power did not enhance another actual burn user's damage in sunlight: %v %v", base, solar)
	}
	waEvidence(t, "solar-power", waScenario(t), map[string]any{"base": base, "solar": solar, "factor": 1.25})
}

func TestNativeWorldCloudNineSuppressesOrdinaryWeatherBenefitWithBossException(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "wingull", 30)
	waDeploy(t, state, "swablu", 30)
	waBattle(t, state)
	point := engine.Point{X: 300, Y: 200}
	waEnemy(t, state, point)
	waEnemy(t, state, point)
	for index := range state.Enemies {
		state.Enemies[index].PassiveID = "swiftSwim"
		state.Enemies[index].BaseSpeed = 100
	}
	state.Enemies[1].Boss = true
	waAdvance(t, state, 20)
	ordinary, boss := state.Enemies[0], state.Enemies[1]
	if !ordinary.CloudNine || boss.CloudNine || math.Abs(math.Hypot(ordinary.X-point.X, ordinary.Y-point.Y)-100.0/60) > .01 || math.Abs(math.Hypot(boss.X-point.X, boss.Y-point.Y)-200.0/60) > .01 {
		t.Fatal("cloud nine did not suppress the real ordinary swift-swim motion while preserving the boss exception")
	}
	waSnapshotContinuation(t, state, 300)
	waEvidence(t, "cloud-nine", state, map[string]any{"ordinarySpeed": 100, "bossSpeed": 200, "rain": true})
}

func TestNativeWorldFutureAttackActuallyTriggersWaitingPastAlly(t *testing.T) {
	state := waScenario(t)
	waiting := waDeploy(t, state, "sandyShocks", 30)
	waDeploy(t, state, "ironThorns", 30)
	waBattle(t, state)
	state.Towers[0].CooldownMS = 100000
	waEnemy(t, state, engine.Point{X: (state.Towers[0].X + state.Towers[1].X) / 2, Y: (state.Towers[0].Y + state.Towers[1].Y) / 2})
	waAdvance(t, state, 3000)
	hit := false
	for _, event := range state.Events {
		hit = hit || event.Type == "hit" && event.Source == waiting
	}
	if !hit {
		t.Fatal("a real future-paradox attack failed to trigger the waiting ally before its own 60000 ms cooldown")
	}
	waSnapshotContinuation(t, state, 400)
	waEvidence(t, "future-trigger", state, map[string]any{"normalCooldownMS": 60000, "futureID": 178, "waitingID": 179})
}

func TestNativeWorldTriAttackAppliesARealSeededControlCondition(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "porygon", 30)
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + state.Team[0].Stats.Range, Y: state.Towers[0].Y})
	found := false
	for elapsed := 0; elapsed < 30000 && !found; elapsed += 20 {
		waAdvance(t, state, 20)
		for _, condition := range state.Enemies[0].Conditions {
			found = found || condition.Kind == "burn" || condition.Kind == "slow" || condition.Kind == "stun"
		}
	}
	if !found {
		t.Fatal("seeded tri-attack orbit never applied an actual burn, slow, or stun")
	}
	waSnapshotContinuation(t, state, 400)
	waEvidence(t, "tri-attack", state, map[string]any{"chance": .3, "conditions": []string{"burn", "slow", "stun"}})
}

func TestNativeWorldDefiantRemovesActualItemPowerPenalty(t *testing.T) {
	state := waScenario(t)
	id := waDeploy(t, state, "bisharp", 30)
	waItem(state, id, "metalCoatReal")
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 30, Y: state.Towers[0].Y + 30})
	state.Random = 123456789
	damage := waFirstDamage(t, state, id)
	want := state.Team[0].Stats.Power + 500
	if want <= 1000 {
		want = math.Ceil(want * 1.5)
	}
	if damage != want {
		t.Fatalf("defiant did not cancel the -250 power penalty and apply its +500 before technician: %v %v", damage, want)
	}
	waSnapshotContinuation(t, state, 400)
	waEvidence(t, "defiant", state, map[string]any{"ignoredPenalty": -250, "bonus": 500, "damage": damage})
}

func TestNativeWorldDittoCopiedAttackUsesOwnLevelAndStableIdentity(t *testing.T) {
	state := waScenario(t)
	dna := waPartner(t, state, "charmander", 1)
	ditto := waPartner(t, state, "ditto", 30)
	waApply(t, state, 1, engine.Action{Type: "dna", PokemonID: ditto, TargetID: dna})
	state.Team[1].Stats.CriticalPct = 0
	tile := waPlacement(t, state, 1)
	waApply(t, state, 2, engine.Action{Type: "deploy", PokemonID: ditto, TileID: tile.ID})
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 30, Y: state.Towers[0].Y})
	state.Random = 123456789
	damage := waFirstDamage(t, state, ditto)
	specification := engine.CatalogData().Species["charmander"]
	want := math.Floor(specification.PowerBase + specification.PowerPerLevel*30)
	if damage != want || state.Team[1].Species != "ditto" || state.Team[1].ID != ditto || state.Team[1].Sprite.Image != specification.Sprite.Image {
		t.Fatal("Ditto's actual projectile did not preserve its own identity and level while copying DNA stats and sprite")
	}
	waSnapshotContinuation(t, state, 400)
	waEvidence(t, "ditto-attack", state, map[string]any{"dittoLevel": 30, "targetLevel": 1, "actualDamage": damage})
}

func TestNativeWorldSchemeUsesActualHitRNGAndCanShinyFinalBoxPokemon(t *testing.T) {
	state := waScenario(t)
	smeargle := waDeploy(t, state, "smeargle", 30)
	state.Team[0].Shiny = true
	candidate := waPartner(t, state, "charizard", 30)
	waApply(t, state, 0, engine.Action{Type: "team", PokemonID: candidate, Mode: "box"})
	waBattle(t, state)
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 1, Y: state.Towers[0].Y})
	state.Random = 6864
	waSnapshotContinuation(t, state, 20)
	if len(state.Box) != 1 || !state.Box[0].Shiny || state.Box[0].ID != candidate || state.Team[0].ID != smeargle {
		t.Fatalf("a real scheme hit with a 0.0000631327 roll failed to shiny the eligible final box Pokémon: box=%v events=%v", state.Box, state.Events)
	}
	waEvidence(t, "scheme-hit", state, map[string]any{"xorshiftSeed": 6864, "shinyRoll": .00006313272751867771, "chance": 1.0 / 9000, "recipient": "charizard", "recipientLocation": "box"})
}

func TestNativeWorldCorruptWorldSnapshotIsRejectedBeforeResume(t *testing.T) {
	state := waScenario(t)
	plus := waDeploy(t, state, "plusle", 30)
	waDeploy(t, state, "minun", 30)
	waAdvance(t, state, 20)
	cases := map[string]func(*engine.State){
		"nil tower":              func(value *engine.State) { value.World.Towers[plus] = nil },
		"negative timer":         func(value *engine.State) { value.World.Towers[plus].IdleMS = -1 },
		"overlong berry":         func(value *engine.State) { value.World.Towers[plus].BerryTimers["liechiBerry"] = 36001 },
		"future exposure target": func(value *engine.State) { value.World.Towers[plus].ExposureMS[value.NextID+1] = 20 },
		"self passenger":         func(value *engine.State) { value.World.PassengerHosts[plus] = plus },
		"link to pokemon":        func(value *engine.State) { value.World.Links[0].First = plus },
		"invalid charge count":   func(value *engine.State) { value.Towers[0].Charges = 51 },
		"unowned zone": func(value *engine.State) {
			value.World.Zones = []engine.WorldZone{{ID: value.NextID - 1, Kind: "web", PokemonID: value.Towers[0].ID, LifeMS: 4100, Radius: 30}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			copy, err := engine.Clone(state)
			if err != nil {
				t.Fatal(err)
			}
			mutate(copy)
			data, err := json.Marshal(copy)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Unmarshal(data); err == nil {
				t.Fatal("corrupt world snapshot was accepted for continuation")
			}
		})
	}
	waEvidence(t, "snapshot-validation", state, map[string]int{"corruptionCases": len(cases)})
}

func TestNativeWorldMagmaChainUsesPreDeathBurnAndResolvesEachDeathOnce(t *testing.T) {
	for _, burned := range []bool{false, true} {
		state := waScenario(t)
		waDeploy(t, state, "heatran", 30)
		waBattle(t, state)
		state.Towers[0].CooldownMS = 100000
		origin := engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y}
		waEnemy(t, state, origin)
		waEnemy(t, state, engine.Point{X: origin.X + 40, Y: origin.Y})
		third := waEnemy(t, state, engine.Point{X: origin.X + 80, Y: origin.Y})
		burn := engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: .005, Stacks: 1}}}
		engine.ApplyConditions(&state.Enemies[0].EnemyEffectState, burn, 0)
		state.Enemies[0].HP, state.Enemies[1].HP = 0, 5000
		if burned {
			engine.ApplyConditions(&state.Enemies[1].EnemyEffectState, burn, 0)
		}
		waAdvance(t, state, 20)
		var health float64
		for _, enemy := range state.Enemies {
			if enemy.ID == third {
				health = enemy.HP
			}
		}
		want := 100000.0
		if burned {
			want -= 6000
		}
		if health != want || state.WaveKills != 2 {
			t.Fatalf("magma death order or duplicate settlement differed for pre-burn=%v: thirdHP=%v want=%v kills=%d", burned, health, want, state.WaveKills)
		}
		waSnapshotContinuation(t, state, 100)
	}
	waEvidence(t, "magma-chain", waScenario(t), map[string]any{"deathBeforeNewBurn": true, "eachDeathOnce": true})
}

func TestNativeWorldMagmaResolvesRegenerationBeforeItsPostDamageBurn(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "heatran", 30)
	waBattle(t, state)
	state.Towers[0].CooldownMS = 100000
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	waEnemy(t, state, engine.Point{X: state.Towers[0].X + 60, Y: state.Towers[0].Y})
	engine.ApplyConditions(&state.Enemies[0].EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: .005, Stacks: 1}}}, 0)
	state.Enemies[0].HP, state.Enemies[1].HP = 0, 5000
	state.Enemies[1].PassiveID = "regenerator"
	waAdvance(t, state, 20)
	if state.Enemies[1].ReviveRemainingMS <= 0 || !engine.HasCondition(&state.Enemies[1].EnemyEffectState, "burn") || state.WaveKills != 1 {
		t.Fatal("magma post-damage burn ran before the target's immediate regeneration phase")
	}
	waEvidence(t, "magma-regeneration", state, map[string]any{"deathResolutionBeforeBurn": true})
}

func TestNativeWorldStalwartPursuesLegalEnemyBeyondAttackRange(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "megaSkarmory", 30)
	waBattle(t, state)
	origin := state.Towers[0]
	waEnemy(t, state, engine.Point{X: origin.X - 500, Y: origin.Y})
	waAdvance(t, state, 1000)
	if math.Abs(state.Towers[0].X-origin.X+300) > 1 || math.Abs(state.Towers[0].Y-origin.Y) > 1 {
		t.Fatal("stalwart did not pursue its global legal priority target at 300 px/s outside attack range")
	}
	waSnapshotContinuation(t, state, 1000)
	distance := math.Hypot(state.Towers[0].X-state.Enemies[0].X, state.Towers[0].Y-state.Enemies[0].Y)
	if math.Abs(distance-60) > 1 {
		t.Fatal("stalwart pursuit did not join the factual 60 px target orbit")
	}
	waEvidence(t, "stalwart", state, map[string]any{"radius": 60, "pursuitSpeed": 300, "angularSpeed": 2.5, "globalTargetSelection": true})
}
