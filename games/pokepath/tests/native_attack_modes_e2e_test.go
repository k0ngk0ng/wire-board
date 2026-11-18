package pokepath_test

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func attackModeEvidence(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		path := filepath.Join("..", "..", "..", ".local", "pokepath", "verification", "native-attack-modes")
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Error(err)
			return
		}
		value, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "command": ". scripts/env.sh && go test ./games/pokepath/tests -run '^TestNativeAttackMode' -count=1", "fixture": "declared owned species and stationary entered enemies; normal deploy/aim/wave/advance/save interface"}, "", "  ")
		if err := os.WriteFile(filepath.Join(path, strings.ReplaceAll(t.Name(), "/", "-")+".json"), append(value, '\n'), 0600); err != nil {
			t.Error(err)
		}
	})
}

func attackModeFixture(t *testing.T, key string) *engine.State {
	t.Helper()
	attackModeEvidence(t)
	state, err := engine.New(engine.Options{Starter: "charmander", Speed: 1}, 4, 87654321, 1794919872358)
	if err != nil {
		t.Fatal(err)
	}
	species := engine.CatalogData().Species[key]
	if species.Key == "" {
		t.Fatal("unknown declared owned fixture species", key)
	}
	state.Team[0].Species = key
	state.Team[0].Sprite = species.Sprite
	state.Team[0].Stats = engine.Stats{Power: math.Floor(species.PowerBase + species.PowerPerLevel), Range: math.Floor(species.RangeBase + species.RangePerLevel), AttackMS: math.Floor(species.AttackMSBase + species.AttackMSPerLevel), CriticalPct: species.CriticalBase + species.CriticalPerLevel}
	state.Team[0].UpgradeCost = engine.CatalogData().UpgradeCosts[species.CostScale][1]
	if species.AttackType == "bombardment" {
		state.Team[0].Target = "bombardment"
	}
	tile := -1
	for _, placement := range engine.Project(state).Tiles {
		if slices.Contains(species.Terrains, placement.Terrain) {
			tile = placement.ID
			break
		}
	}
	if tile < 0 {
		t.Fatal("declared species has no legal initial-route placement")
	}
	attackModeAct(t, state, 0, engine.Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: tile})
	return state
}

func attackModeAct(t *testing.T, state *engine.State, actor int, action engine.Action) {
	t.Helper()
	action.Epoch = state.Epoch
	if err := engine.Apply(state, actor, action); err != nil {
		t.Fatalf("%s: %v", action.Type, err)
	}
}

func attackModeEnemy(state *engine.State, point engine.Point) int {
	specification := engine.CatalogData().Enemies["rattata"]
	id := state.NextID
	state.NextID++
	enemy := engine.Enemy{ID: id, Key: "rattata", Species: "rattata", X: point.X, Y: point.Y, Entered: true, PathIndex: 0, Waypoint: 1, Power: specification.Power, Gold: specification.Gold, Sprite: specification.Sprite, EnemyEffectState: engine.EnemyEffectState{HP: specification.Health, MaxHP: specification.Health, Armor: specification.Armor, MaxArmor: specification.Armor, BaseSpeed: specification.Speed, CanStun: specification.CanStun, CanSlow: specification.CanSlow, CanBurn: specification.CanBurn, CanPoison: specification.CanPoison, PassiveID: specification.PassiveID, Conditions: []engine.Condition{{Kind: "stun", RemainingMS: 10000, Magnitude: 1, Stacks: 1}}}}
	state.Enemies = append(state.Enemies, enemy)
	return id
}

func attackModeAdvance(t *testing.T, state *engine.State, milliseconds int64) {
	t.Helper()
	if err := engine.Advance(state, milliseconds); err != nil {
		t.Fatal(err)
	}
}

func TestNativeAttackModeBombardmentAimFlightAndSplashE2E(t *testing.T) {
	state := attackModeFixture(t, "numel")
	owner := state.Team[0].ID
	tower := state.Towers[0]
	attackModeAct(t, state, 1, engine.Action{Type: "aim", PokemonID: owner, X: tower.X + 1000, Y: tower.Y})
	aim := state.AttackModes.Towers[owner]
	if aim == nil || math.Abs(aim.Aim.X-tower.X-state.Team[0].Stats.Range) > 0.00001 || aim.Aim.Y != tower.Y {
		t.Fatal("shared bombardment aim did not clamp to the actual tower range")
	}
	attackModeAct(t, state, 2, engine.Action{Type: "wave"})
	state.Enemies = nil
	for index := 0; index < 3; index++ {
		attackModeEnemy(state, engine.Point{X: 650, Y: 500})
	}
	attackModeAdvance(t, state, 20)
	if len(state.AttackModes.Effects) != 1 {
		t.Fatal("bombardment with no ordinary in-range target failed to launch a native shell")
	}
	shell := state.AttackModes.Effects[0]
	if shell.Kind != "bombardment" || shell.DurationMS != 500 || shell.Impacted || state.TotalDamage != 0 || math.Hypot(shell.Destination.X-aim.Aim.X, shell.Destination.Y-aim.Aim.Y) > aim.AreaRadius {
		t.Fatal("native shell timing or sampled aim geometry differs from the original")
	}
	enemyRadius := math.Max(state.Enemies[0].Sprite.FrameWidth, state.Enemies[0].Sprite.FrameHeight) / 2
	state.Enemies[0].X, state.Enemies[0].Y = shell.Destination.X, shell.Destination.Y
	state.Enemies[1].X, state.Enemies[1].Y = shell.Destination.X+shell.Radius+enemyRadius-0.5, shell.Destination.Y
	state.Enemies[2].X, state.Enemies[2].Y = shell.Destination.X+shell.Radius+enemyRadius+1, shell.Destination.Y
	attackModeAdvance(t, state, 220)
	saved, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := engine.Unmarshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	attackModeAdvance(t, state, 240)
	attackModeAdvance(t, restored, 240)
	if state.TotalDamage != 0 {
		t.Fatal("bombardment hit before the fixed 500 ms flight completed")
	}
	attackModeAdvance(t, state, 20)
	attackModeAdvance(t, restored, 20)
	if state.TotalDamage <= 0 || len(state.Enemies) != 3 || state.Enemies[0].ReceivedHits != 1 || state.Enemies[1].ReceivedHits != 1 || state.Enemies[2].ReceivedHits != 0 || !state.AttackModes.Effects[0].Impacted {
		t.Fatal("native splash did not include the inner boundary and exclude the outer boundary")
	}
	first, _ := engine.Marshal(state)
	second, _ := engine.Marshal(restored)
	if !bytes.Equal(first, second) {
		t.Fatal("mid-flight save restoration changed native shell motion, random outcomes or hit results")
	}
	attackModeAdvance(t, state, 400)
	if len(state.AttackModes.Effects) != 0 || state.Enemies[0].ReceivedHits != 1 || state.Enemies[1].ReceivedHits != 1 || state.Enemies[2].ReceivedHits != 0 {
		t.Fatal("impact effect failed to settle after 400 ms or damaged the boundary survivor again")
	}
}

func TestNativeAttackModeBombardmentReaimCancelsFlightAtomicallyE2E(t *testing.T) {
	state := attackModeFixture(t, "numel")
	owner := state.Team[0].ID
	tower := state.Towers[0]
	attackModeAct(t, state, 0, engine.Action{Type: "aim", PokemonID: owner, X: tower.X + 80, Y: tower.Y})
	attackModeAct(t, state, 1, engine.Action{Type: "wave"})
	state.Enemies = nil
	attackModeEnemy(state, engine.Point{X: 650, Y: 500})
	attackModeAdvance(t, state, 20)
	if len(state.AttackModes.Effects) == 0 {
		t.Fatal("fixture did not emit a live shell")
	}
	cooldown := state.Towers[0].CooldownMS
	attackModeAct(t, state, 2, engine.Action{Type: "aim", PokemonID: owner, X: tower.X, Y: tower.Y + 80})
	if len(state.AttackModes.Effects) != 0 || state.Towers[0].CooldownMS != cooldown {
		t.Fatal("reaim did not cancel current flight or unexpectedly reset the shared cooldown")
	}
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 3, engine.Action{Type: "aim", PokemonID: owner + 999, X: 100, Y: 100, Epoch: state.Epoch}); err == nil {
		t.Fatal("aim accepted an unowned stable identifier")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("rejected aim changed live shell state or random generator")
	}
}

func TestNativeAttackModeRouteChangeClearsOldAimAndWorldE2E(t *testing.T) {
	state := attackModeFixture(t, "numel")
	owner := state.Team[0].ID
	tower := state.Towers[0]
	attackModeAct(t, state, 0, engine.Action{Type: "aim", PokemonID: owner, X: tower.X + 80, Y: tower.Y})
	state.Stars = 10000
	state.World.Honey = 7
	attackModeAct(t, state, 1, engine.Action{Type: "route", Route: 1})
	if len(state.AttackModes.Towers) != 0 || len(state.AttackModes.Effects) != 0 || state.World.Honey != 0 || len(state.Towers) != 0 {
		t.Fatal("changing routes retained previous tower aim, effects or route-owned world counters")
	}
}

func TestNativeAttackModeLightningWeatherFlightAndRestoreE2E(t *testing.T) {
	state := attackModeFixture(t, "thundurus")
	owner := state.Team[0].ID
	tower := state.Towers[0]
	attackModeAct(t, state, 0, engine.Action{Type: "aim", PokemonID: owner, X: tower.X + 80, Y: tower.Y})
	attackModeAct(t, state, 1, engine.Action{Type: "wave"})
	state.Enemies = nil
	attackModeEnemy(state, engine.Point{X: 650, Y: 500})
	attackModeAdvance(t, state, 20)
	if len(state.AttackModes.Effects) != 0 {
		t.Fatal("wildbolt storm emitted actual lightning outside thunderstorm weather")
	}
	state.Weather = "thunderStorm"
	for elapsed := 0; len(state.AttackModes.Effects) == 0 && elapsed < 4100; elapsed += 20 {
		attackModeAdvance(t, state, 20)
	}
	if len(state.AttackModes.Effects) < 3 || len(state.AttackModes.Effects) > 7 {
		t.Fatal("actual thunderstorm volley was outside the original three-to-seven range")
	}
	for _, effect := range state.AttackModes.Effects {
		if effect.Kind != "lightningArea" || effect.DurationMS != 150 || effect.Origin.X != effect.Destination.X || effect.Origin.Y != effect.Destination.Y-260 || len(effect.BoltPoints) != 7 || effect.Impacted {
			t.Fatal("native lightning did not preserve its exact source geometry and flight duration")
		}
	}
	saved, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := engine.Unmarshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	attackModeAdvance(t, state, 140)
	attackModeAdvance(t, restored, 140)
	for _, effect := range state.AttackModes.Effects {
		if !effect.Impacted {
			t.Fatal("lightning failed to impact at its fixed 150 ms flight boundary")
		}
	}
	first, _ := engine.Marshal(state)
	second, _ := engine.Marshal(restored)
	if !bytes.Equal(first, second) {
		t.Fatal("lightning restoration rerolled bolt geometry or native outcomes")
	}
	state.Weather = ""
	attackModeAdvance(t, state, 4000)
	if len(state.AttackModes.Effects) != 0 {
		t.Fatal("lightning failed to settle or fired a new volley after thunderstorm ended")
	}
}

func TestNativeAttackModeLightningZoomLensAddsOneActualBoltE2E(t *testing.T) {
	state := attackModeFixture(t, "thundurus")
	owner := state.Team[0].ID
	tower := state.Towers[0]
	attackModeAct(t, state, 0, engine.Action{Type: "aim", PokemonID: owner, X: tower.X + 80, Y: tower.Y})
	attackModeAct(t, state, 1, engine.Action{Type: "wave"})
	state.Weather = "thunderStorm"
	state.Enemies = nil
	attackModeEnemy(state, engine.Point{X: 650, Y: 500})
	withLens, err := engine.Clone(state)
	if err != nil {
		t.Fatal(err)
	}
	withLens.Inventory = append(withLens.Inventory, engine.OwnedItem{ID: "zoomLens", EquippedBy: owner})
	withLens.Team[0].Item = "zoomLens"
	attackModeAdvance(t, state, 20)
	attackModeAdvance(t, withLens, 20)
	if len(withLens.AttackModes.Effects) != len(state.AttackModes.Effects)+1 {
		t.Fatalf("zoom lens actual bolt count: normal=%d lens=%d random=%d/%d", len(state.AttackModes.Effects), len(withLens.AttackModes.Effects), state.Random, withLens.Random)
	}
}

func TestNativeAttackModeBombardmentZoomLensAddsOneActualShellE2E(t *testing.T) {
	state := attackModeFixture(t, "numel")
	owner := state.Team[0].ID
	attackModeAct(t, state, 1, engine.Action{Type: "wave"})
	state.Enemies = nil
	attackModeEnemy(state, engine.Point{X: 650, Y: 500})
	withLens, err := engine.Clone(state)
	if err != nil {
		t.Fatal(err)
	}
	withLens.Inventory = append(withLens.Inventory, engine.OwnedItem{ID: "zoomLens", EquippedBy: owner})
	withLens.Team[0].Item = "zoomLens"
	attackModeAdvance(t, state, 20)
	attackModeAdvance(t, withLens, 20)
	if len(state.AttackModes.Effects) != 1 || len(withLens.AttackModes.Effects) != 2 {
		t.Fatal("zoom lens did not add a real shell to an ordinary bombardment volley")
	}
}

func TestNativeAttackModeUTurnVisitsMovesReturnsAndRestoresE2E(t *testing.T) {
	state := attackModeFixture(t, "scyther")
	owner := state.Team[0].ID
	home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = nil
	for _, distance := range []float64{60, 120, 180} {
		attackModeEnemy(state, engine.Point{X: home.X + distance, Y: home.Y})
	}
	attackModeEnemy(state, engine.Point{X: 650, Y: 500})
	attackModeAdvance(t, state, 20)
	mode := state.AttackModes.Towers[owner]
	if mode == nil || mode.Chain == nil || mode.Chain.Phase != "moving" || state.Enemies[0].ReceivedHits != 1 || state.Enemies[1].ReceivedHits != 0 {
		t.Fatal("u-turn did not deal its immediate first hit and begin a real movement chain")
	}
	attackModeAdvance(t, state, 120)
	if state.Towers[0].X <= home.X || state.Towers[0].X >= home.X+60 || state.Enemies[1].ReceivedHits != 0 {
		t.Fatal("u-turn did not interpolate physically toward the first target before waiting")
	}
	saved, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := engine.Unmarshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	attackModeAdvance(t, state, 2000)
	attackModeAdvance(t, restored, 2000)
	if state.AttackModes.Towers[owner].Chain != nil || state.Towers[0].X != home.X || state.Towers[0].Y != home.Y || state.Towers[0].CooldownMS <= 0 {
		t.Fatal("u-turn failed to settle at its actual home and restart cooldown after returning")
	}
	for index := 0; index < 3; index++ {
		enemy := state.Enemies[index]
		base := state.Team[0].Stats.Power
		if enemy.LastHitCritical {
			base = math.Ceil(base * 1.5)
		}
		expected := base + math.Ceil(base*float64(index)*0.05)
		if enemy.ReceivedHits != 1 || enemy.LastHPDamage != expected {
			t.Fatalf("u-turn target %d did not receive one linearly increased post-critical hit: got %.2f want %.2f", index, enemy.LastHPDamage, expected)
		}
	}
	if state.Enemies[3].ReceivedHits != 0 {
		t.Fatal("u-turn chained to an enemy outside range from its last real position")
	}
	first, _ := engine.Marshal(state)
	second, _ := engine.Marshal(restored)
	if !bytes.Equal(first, second) {
		t.Fatal("u-turn continuation diverged after saving movement, visited targets or return phase")
	}
}

func TestNativeAttackModeUTurnRetireCancelsRemainingChainE2E(t *testing.T) {
	state := attackModeFixture(t, "scyther")
	owner := state.Team[0].ID
	home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = nil
	attackModeEnemy(state, engine.Point{X: home.X + 60, Y: home.Y})
	attackModeEnemy(state, engine.Point{X: home.X + 120, Y: home.Y})
	attackModeAdvance(t, state, 140)
	if state.AttackModes.Towers[owner] == nil || state.AttackModes.Towers[owner].Chain == nil {
		t.Fatal("fixture did not start a native u-turn chain")
	}
	damage := state.TotalDamage
	attackModeAct(t, state, 1, engine.Action{Type: "retire", PokemonID: owner})
	attackModeAdvance(t, state, 1000)
	if state.AttackModes.Towers[owner] != nil || len(state.Towers) != 0 || state.TotalDamage != damage || state.Enemies[1].ReceivedHits != 0 {
		t.Fatal("retiring u-turn left a hidden chain capable of moving or dealing damage")
	}
}

func TestNativeAttackModeMultipleChainsHaveStableOrderE2E(t *testing.T) {
	state := attackModeFixture(t, "scyther")
	second := state.Team[0]
	second.ID, second.Species, second.TileID = state.NextID, "ditto", -1
	state.NextID++
	second.UpgradeCost = engine.CatalogData().UpgradeCosts[engine.CatalogData().Species["ditto"].CostScale][1]
	state.Progression.DNA[second.ID] = "scyther"
	state.Team = append(state.Team, second)
	for _, placement := range engine.Project(state).Tiles {
		if placement.ID != state.Towers[0].TileID && slices.Contains(engine.CatalogData().Species["scyther"].Terrains, placement.Terrain) {
			attackModeAct(t, state, 1, engine.Action{Type: "deploy", PokemonID: second.ID, TileID: placement.ID})
			break
		}
	}
	home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = nil
	for _, distance := range []float64{60, 120, 180, 240} {
		attackModeEnemy(state, engine.Point{X: home.X + distance, Y: home.Y})
	}
	attackModeEnemy(state, engine.Point{X: 650, Y: 500})
	attackModeAdvance(t, state, 140)
	restored, err := engine.Clone(state)
	if err != nil {
		t.Fatal(err)
	}
	for tick := 0; tick < 100; tick++ {
		attackModeAdvance(t, state, 20)
		attackModeAdvance(t, restored, 20)
		first, _ := engine.Marshal(state)
		second, _ := engine.Marshal(restored)
		if !bytes.Equal(first, second) {
			t.Fatal("parallel owned u-turn chains consumed random rolls in unstable map order")
		}
	}
}

func TestNativeAttackModeMoveCancelsOldUTurnGeometryE2E(t *testing.T) {
	state := attackModeFixture(t, "scyther")
	owner := state.Team[0].ID
	home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = nil
	attackModeEnemy(state, engine.Point{X: home.X + 60, Y: home.Y})
	attackModeEnemy(state, engine.Point{X: home.X + 120, Y: home.Y})
	attackModeAdvance(t, state, 140)
	for _, placement := range engine.Project(state).Tiles {
		if placement.ID == state.Towers[0].TileID || !slices.Contains(engine.CatalogData().Species["scyther"].Terrains, placement.Terrain) {
			continue
		}
		attackModeAct(t, state, 1, engine.Action{Type: "move", PokemonID: owner, TileID: placement.ID})
		if value := state.AttackModes.Towers[owner]; value != nil && value.Chain != nil {
			t.Fatal("moving a tower left its previous u-turn movement and return-home geometry active")
		}
		return
	}
	t.Fatal("fixture has no second legal placement")
}

func TestNativeAttackModeSoundWavesExpandDelayAndDeduplicateE2E(t *testing.T) {
	state := attackModeFixture(t, "chimecho")
	home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = nil
	for _, point := range []engine.Point{{X: home.X + 60, Y: home.Y}, {X: home.X + 120, Y: home.Y}, {X: home.X - 60, Y: home.Y}, {X: home.X + 60, Y: home.Y}} {
		attackModeEnemy(state, point)
	}
	state.Enemies[3].Invisible = true
	attackModeAdvance(t, state, 20)
	if len(state.AttackModes.Effects) != 3 || state.AttackModes.Effects[0].Kind != "wave" || state.AttackModes.Effects[1].DelayMS != 450 || state.AttackModes.Effects[2].DelayMS != 900 {
		t.Fatal("boomburst did not launch the original three delayed outward waves")
	}
	attackModeAdvance(t, state, 300)
	if state.Enemies[0].ReceivedHits != 0 || state.Enemies[1].ReceivedHits != 0 {
		t.Fatal("sound wave damaged an enemy before the advancing ring reached it")
	}
	attackModeAdvance(t, state, 380)
	if state.Enemies[0].ReceivedHits != 1 || state.Enemies[1].ReceivedHits != 1 || state.Enemies[2].ReceivedHits != 1 || state.Enemies[3].ReceivedHits != 0 {
		t.Fatal("wavefront geometry, visibility or per-ring hit deduplication was incorrect")
	}
	saved, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := engine.Unmarshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	attackModeAdvance(t, state, 1100)
	attackModeAdvance(t, restored, 1100)
	if len(state.AttackModes.Effects) != 0 || state.Enemies[0].ReceivedHits != 3 || state.Enemies[1].ReceivedHits != 3 || state.Enemies[2].ReceivedHits != 3 || state.Enemies[3].ReceivedHits != 0 {
		t.Fatal("delayed rings failed to finish with one hit each and no invisible target damage")
	}
	first, _ := engine.Marshal(state)
	second, _ := engine.Marshal(restored)
	if !bytes.Equal(first, second) {
		t.Fatal("sound-wave continuation changed delays, hit sets or native damage after restore")
	}
}

func TestNativeAttackModeSweepUsesEachOriginalGeometryE2E(t *testing.T) {
	for _, scenario := range []struct {
		key    string
		points []engine.Point
		hits   []int
	}{
		{key: "heatmor", points: []engine.Point{{X: 60}, {X: 30, Y: 60}, {X: -30}, {X: 116}}, hits: []int{1, 1, 0, 0}},
		{key: "keldeo", points: []engine.Point{{X: 100}, {X: 100, Y: 73.5}, {X: 100, Y: 75}, {X: -10}, {X: 181}}, hits: []int{1, 1, 0, 0, 0}},
	} {
		t.Run(scenario.key, func(t *testing.T) {
			state := attackModeFixture(t, scenario.key)
			home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
			attackModeAct(t, state, 0, engine.Action{Type: "wave"})
			state.Enemies = nil
			for _, point := range scenario.points {
				attackModeEnemy(state, engine.Point{X: home.X + point.X, Y: home.Y + point.Y})
			}
			attackModeAdvance(t, state, 20)
			if len(state.AttackModes.Effects) != 1 || state.AttackModes.Effects[0].Kind != "sweep" {
				t.Fatal("original directional sweep did not create an explicit native geometry")
			}
			for index, expected := range scenario.hits {
				if state.Enemies[index].ReceivedHits != expected {
					t.Fatalf("sweep %s point %d violated its original hit boundary", scenario.key, index)
				}
			}
			attackModeAdvance(t, state, 400)
			if len(state.AttackModes.Effects) != 0 {
				t.Fatal("sweep visual did not settle within its original lifetime")
			}
			for index, expected := range scenario.hits {
				if state.Enemies[index].ReceivedHits != expected {
					t.Fatal("lingering sweep visual repeated a direct hit")
				}
			}
		})
	}
}

func TestNativeAttackModeShadowClawTargetAreaAndFinalMultipliersE2E(t *testing.T) {
	state := attackModeFixture(t, "mimikyu")
	home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = nil
	for _, point := range []engine.Point{{X: home.X + 100, Y: home.Y}, {X: home.X + 201.5, Y: home.Y}, {X: home.X + 203, Y: home.Y}, {X: home.X + 100, Y: home.Y + 20}} {
		attackModeEnemy(state, point)
	}
	state.Enemies[0].Conditions = append(state.Enemies[0].Conditions, engine.Condition{Kind: "nightmare", RemainingMS: -1, Magnitude: 1, Stacks: 10})
	state.Enemies[3].Invisible = true
	attackModeAdvance(t, state, 20)
	if len(state.AttackModes.Effects) != 1 || state.AttackModes.Effects[0].Radius != 78 || state.AttackModes.Effects[0].Origin.X != home.X+100 {
		t.Fatal("empowered shadow claw did not center its 78px area on the actual selected target")
	}
	for index, multiplier := range []float64{4, 2} {
		base := state.Team[0].Stats.Power
		if state.Enemies[index].LastHitCritical {
			base = math.Ceil(base * 1.5)
		}
		if state.Enemies[index].ReceivedHits != 1 || state.Enemies[index].LastIncomingDamage != math.Floor(base*multiplier) {
			t.Fatal("shadow claw lost its main-target empowerment or per-target post-critical nightmare multiplier")
		}
	}
	if state.Enemies[2].ReceivedHits != 0 || state.Enemies[3].ReceivedHits != 0 {
		t.Fatal("shadow claw hit beyond its target-centered boundary or revealed an invisible incidental target")
	}
}

func TestNativeAttackModeDirectionalAreaWindowAndBoundsE2E(t *testing.T) {
	state := attackModeFixture(t, "glalie")
	home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = nil
	for _, point := range []engine.Point{{X: 60}, {X: 189}, {X: 191}, {X: 60, Y: 61}, {X: -60}} {
		attackModeEnemy(state, engine.Point{X: home.X + point.X, Y: home.Y + point.Y})
	}
	attackModeAdvance(t, state, 40)
	if len(state.AttackModes.Effects) != 1 || state.AttackModes.Effects[0].DurationMS != 520 || state.Enemies[0].ReceivedHits != 1 || state.Enemies[1].ReceivedHits != 1 {
		t.Fatal("icy wind failed its original 500ms cone hitbox and 520ms visual configuration")
	}
	for _, enemy := range state.Enemies[2:] {
		if enemy.ReceivedHits != 0 {
			t.Fatal("directional area hit outside range+10, 45-degree half angle or the forward direction")
		}
	}
	attackModeAdvance(t, state, 500)
	if len(state.AttackModes.Effects) != 1 || state.Enemies[0].ReceivedHits != 1 || state.Enemies[1].ReceivedHits != 1 {
		t.Fatal("directional area expired before its original 520 ms visual duration or repeated damage")
	}
	attackModeAdvance(t, state, 10)
	if len(state.AttackModes.Effects) != 0 || state.Enemies[0].ReceivedHits != 1 || state.Enemies[1].ReceivedHits != 1 {
		t.Fatal("directional area failed to expire or hit the same enemy twice")
	}
}

func TestNativeAttackModePsychoCutArcMovesAndExcludesBacksideE2E(t *testing.T) {
	state := attackModeFixture(t, "gallade")
	home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = nil
	for _, point := range []engine.Point{{X: 90}, {X: 90, Y: 20}, {X: -50}, {X: 80, Y: 60}} {
		attackModeEnemy(state, engine.Point{X: home.X + point.X, Y: home.Y + point.Y})
	}
	attackModeAdvance(t, state, 100)
	if len(state.AttackModes.Effects) != 1 || state.AttackModes.Effects[0].Kind != "arc" || state.Enemies[0].ReceivedHits != 0 {
		t.Fatal("psycho cut failed to launch a moving native arc before reaching its target")
	}
	attackModeAdvance(t, state, 80)
	if state.Enemies[0].ReceivedHits != 1 || state.Enemies[1].ReceivedHits != 1 || state.Enemies[2].ReceivedHits != 0 || state.Enemies[3].ReceivedHits != 0 {
		t.Fatal("psycho cut did not preserve the 45px leading arc, 12px band and original half-angle")
	}
	attackModeAdvance(t, state, 260)
	if len(state.AttackModes.Effects) != 0 {
		t.Fatal("psycho cut did not settle at range minus arc radius")
	}
}

func TestNativeAttackModeDuneWeatherBandAndSinglePassE2E(t *testing.T) {
	state := attackModeFixture(t, "landorus")
	home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = nil
	for _, point := range []engine.Point{{X: 30}, {X: 60, Y: 79}, {X: 60, Y: 81}, {X: -79}, {X: -81}} {
		attackModeEnemy(state, engine.Point{X: home.X + point.X, Y: home.Y + point.Y})
	}
	attackModeEnemy(state, engine.Point{X: 650, Y: 500})
	withWeather, err := engine.Clone(state)
	if err != nil {
		t.Fatal(err)
	}
	attackModeAdvance(t, state, 20)
	if len(state.AttackModes.Effects) != 0 || state.Towers[0].CooldownMS <= 0 {
		t.Fatal("sandsear storm emitted its real dune outside sand avalanche weather")
	}
	state = withWeather
	state.Weather = "sandAvalanche"
	attackModeAdvance(t, state, 500)
	if len(state.AttackModes.Effects) != 1 || state.AttackModes.Effects[0].Kind != "dune" {
		t.Fatal("sandsear storm did not launch a moving 160px-wide native wavefront")
	}
	for index, expected := range []int{1, 1, 0, 1, 0, 0} {
		if state.Enemies[index].ReceivedHits != expected {
			t.Fatalf("dune boundary point %d expected %d actual %d", index, expected, state.Enemies[index].ReceivedHits)
		}
	}
	attackModeAdvance(t, state, 7400)
	if len(state.AttackModes.Effects) != 0 {
		t.Fatal("dune failed to settle after traversing the actual map diagonal")
	}
}

func TestNativeAttackModeSupersonicCreatesHitCenteredWavesE2E(t *testing.T) {
	state := attackModeFixture(t, "megaChimecho")
	home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = nil
	attackModeEnemy(state, engine.Point{X: home.X + 60, Y: home.Y})
	attackModeEnemy(state, engine.Point{X: home.X + 90, Y: home.Y})
	attackModeEnemy(state, engine.Point{X: 650, Y: 500})
	for elapsed := 0; state.Enemies[0].ReceivedHits == 0 && elapsed < 1000; elapsed += 20 {
		attackModeAdvance(t, state, 20)
	}
	if len(state.AttackModes.Effects) != 3 || state.Enemies[0].ReceivedHits != 1 || state.Enemies[1].ReceivedHits != 0 {
		t.Fatal("supersonic failed to create exactly three waves after its normal projectile hit")
	}
	for _, effect := range state.AttackModes.Effects {
		if effect.Kind != "wave" || effect.Origin.X != state.Enemies[0].X || effect.Origin.Y != state.Enemies[0].Y || effect.Origin.X == home.X {
			t.Fatal("supersonic ring origin followed the tower instead of the actual impact point")
		}
	}
	attackModeAdvance(t, state, 220)
	if state.Enemies[1].ReceivedHits != 1 || len(state.AttackModes.Effects) != 3 {
		t.Fatal("supersonic did not damage a nearby enemy with its real advancing ring or recursively spawned extra rings")
	}
}

func TestNativeAttackModeCorruptGeometryAndChainSnapshotsAreRejectedE2E(t *testing.T) {
	state := attackModeFixture(t, "numel")
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	attackModeAdvance(t, state, 20)
	original, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []func(map[string]any){
		func(mode map[string]any) { mode["effects"].([]any)[0].(map[string]any)["kind"] = "unknown" },
		func(mode map[string]any) { mode["effects"].([]any)[0].(map[string]any)["pokemonID"] = 99999 },
		func(mode map[string]any) { mode["effects"].([]any)[0].(map[string]any)["hitIDs"] = []int{1, 1} },
		func(mode map[string]any) { mode["effects"].([]any)[0].(map[string]any)["speed"] = -1 },
		func(mode map[string]any) { mode["towers"].(map[string]any)["1"].(map[string]any)["areaRadius"] = -1 },
		func(mode map[string]any) {
			mode["towers"].(map[string]any)["1"].(map[string]any)["swordDirection"] = 42
		},
	} {
		var value map[string]any
		if err := json.Unmarshal(original, &value); err != nil {
			t.Fatal(err)
		}
		mutation(value["attackModes"].(map[string]any))
		encoded, _ := json.Marshal(value)
		if _, err := engine.Unmarshal(encoded); err == nil {
			t.Fatal("corrupt explicit attack geometry was accepted as a resumable snapshot")
		}
	}
	chainState := attackModeFixture(t, "scyther")
	home := engine.Point{X: chainState.Towers[0].X, Y: chainState.Towers[0].Y}
	attackModeAct(t, chainState, 0, engine.Action{Type: "wave"})
	chainState.Enemies = nil
	attackModeEnemy(chainState, engine.Point{X: home.X + 60, Y: home.Y})
	attackModeAdvance(t, chainState, 20)
	original, err = engine.Marshal(chainState)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []func(map[string]any){
		func(chain map[string]any) { chain["phase"] = "unknown" },
		func(chain map[string]any) { chain["durationMS"] = 0 },
		func(chain map[string]any) { chain["ageMS"] = -1 },
	} {
		var value map[string]any
		if err := json.Unmarshal(original, &value); err != nil {
			t.Fatal(err)
		}
		chain := value["attackModes"].(map[string]any)["towers"].(map[string]any)["1"].(map[string]any)["chain"].(map[string]any)
		mutation(chain)
		encoded, _ := json.Marshal(value)
		if _, err := engine.Unmarshal(encoded); err == nil {
			t.Fatal("corrupt explicit attack movement chain was accepted as a resumable snapshot")
		}
	}
}

func TestNativeAttackModeSwordSharesOneCriticalRollE2E(t *testing.T) {
	state := attackModeFixture(t, "keldeo")
	home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = nil
	for _, point := range []engine.Point{{X: 60}, {X: 100, Y: 20}, {X: 140, Y: -20}} {
		attackModeEnemy(state, engine.Point{X: home.X + point.X, Y: home.Y + point.Y})
	}
	state.Random = 129
	attackModeAdvance(t, state, 20)
	if len(state.AttackModes.Effects) != 1 || state.AttackModes.Effects[0].SharedCritical == nil || !*state.AttackModes.Effects[0].SharedCritical {
		t.Fatal("seeded sword sweep failed to freeze its one shared critical roll")
	}
	for _, enemy := range state.Enemies {
		if enemy.ReceivedHits != 1 || !enemy.LastHitCritical {
			t.Fatal("sword sweep independently rerolled one target instead of sharing the actual slash critical")
		}
	}
}

func TestNativeAttackModeSwordVisualDirectionAlternatesAfterRestoreE2E(t *testing.T) {
	state := attackModeFixture(t, "keldeo")
	home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = nil
	attackModeEnemy(state, engine.Point{X: home.X + 60, Y: home.Y})
	attackModeEnemy(state, engine.Point{X: 650, Y: 500})
	attackModeAdvance(t, state, 20)
	if len(state.AttackModes.Effects) != 1 || state.AttackModes.Effects[0].Direction != 1 {
		t.Fatal("first native sword sweep did not preserve the original visual direction")
	}
	saved, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := engine.Unmarshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	attackModeAdvance(t, state, 1700)
	attackModeAdvance(t, restored, 1700)
	if len(state.AttackModes.Effects) != 1 || state.AttackModes.Effects[0].Direction != -1 {
		t.Fatal("next sword sweep did not alternate its saved visual direction")
	}
	first, _ := engine.Marshal(state)
	second, _ := engine.Marshal(restored)
	if !bytes.Equal(first, second) {
		t.Fatal("sword visual direction diverged after restoring a live sweep")
	}
}

func TestNativeAttackModeRockThrowSamplesTheRotatedStripE2E(t *testing.T) {
	state := attackModeFixture(t, "tyrunt")
	owner := state.Team[0].ID
	tower := state.Towers[0]
	attackModeAct(t, state, 0, engine.Action{Type: "aim", PokemonID: owner, X: tower.X + 80, Y: tower.Y})
	attackModeAct(t, state, 1, engine.Action{Type: "wave"})
	state.Enemies = nil
	attackModeEnemy(state, engine.Point{X: 650, Y: 500})
	state.Random = 1
	attackModeAdvance(t, state, 20)
	if len(state.AttackModes.Effects) != 1 {
		t.Fatal("rock throw did not emit one real shell for its one actual fossil teammate")
	}
	zone := state.AttackModes.Towers[owner]
	point := state.AttackModes.Effects[0].Destination
	dx, dy := math.Abs(point.X-zone.Aim.X), math.Abs(point.Y-zone.Aim.Y)
	if zone.AreaWidth != zone.AreaHeight*5 || dx > zone.AreaHeight/2 || dy <= zone.AreaRadius || dy > zone.AreaWidth/2 {
		t.Fatal("rock throw sampled a circle or an unrotated rectangle instead of its source strip")
	}
}

func TestNativeAttackModeMegaPsychoCutExtendsOnlyOnActualHitsE2E(t *testing.T) {
	state := attackModeFixture(t, "megaGallade")
	home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	rangeBefore := state.Team[0].Stats.Range
	attackModeAct(t, state, 0, engine.Action{Type: "wave"})
	state.Enemies = nil
	firstID := attackModeEnemy(state, engine.Point{X: home.X + 90, Y: home.Y})
	farID := attackModeEnemy(state, engine.Point{X: home.X + rangeBefore + 40, Y: home.Y})
	attackModeEnemy(state, engine.Point{X: 650, Y: 500})
	attackModeAdvance(t, state, 660)
	if len(state.AttackModes.Effects) != 1 {
		t.Fatal("mega psycho cut ended before its hit-based extension could reach the far enemy")
	}
	effect := state.AttackModes.Effects[0]
	if !slices.Contains(effect.HitIDs, firstID) || !slices.Contains(effect.HitIDs, farID) || effect.MaxDistance != rangeBefore-45+120 || state.Team[0].Stats.Range != rangeBefore {
		t.Fatal("mega psycho cut did not add 60px per real new hit or incorrectly changed permanent range")
	}
}

func TestNativeAttackModeProducesGoWasmParityTracesE2E(t *testing.T) {
	for _, scenario := range []struct {
		key     string
		weather string
	}{{"numel", ""}, {"thundurus", "thunderStorm"}, {"tyrunt", ""}, {"scyther", ""}, {"chimecho", ""}, {"heatmor", ""}, {"keldeo", ""}, {"mimikyu", ""}, {"glalie", ""}, {"gallade", ""}, {"megaGallade", ""}, {"landorus", "sandAvalanche"}, {"megaChimecho", ""}} {
		t.Run(scenario.key, func(t *testing.T) {
			state := attackModeFixture(t, scenario.key)
			home := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
			attackModeAct(t, state, 0, engine.Action{Type: "wave"})
			state.Enemies = nil
			attackModeEnemy(state, engine.Point{X: home.X + 60, Y: home.Y})
			attackModeEnemy(state, engine.Point{X: home.X + 120, Y: home.Y})
			attackModeEnemy(state, engine.Point{X: 650, Y: 500})
			state.Weather = scenario.weather
			initial, err := engine.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			type operation struct {
				Kind         string          `json:"kind"`
				Actor        int             `json:"actor,omitempty"`
				Action       *engine.Action  `json:"action,omitempty"`
				Milliseconds int64           `json:"milliseconds,omitempty"`
				State        json.RawMessage `json:"state"`
				View         engine.View     `json:"view"`
			}
			operations := []operation{}
			if engine.CatalogData().Species[scenario.key].AttackType == "bombardment" {
				action := engine.Action{Type: "aim", PokemonID: state.Team[0].ID, X: home.X + 80, Y: home.Y, Epoch: state.Epoch}
				attackModeAct(t, state, 2, action)
				encoded, _ := engine.Marshal(state)
				operations = append(operations, operation{Kind: "apply", Actor: 2, Action: &action, State: encoded, View: engine.Project(state)})
			}
			for _, milliseconds := range []int64{20, 140, 500, 900, 900, 6000} {
				attackModeAdvance(t, state, milliseconds)
				encoded, err := engine.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				operations = append(operations, operation{Kind: "advance", Milliseconds: milliseconds, State: encoded, View: engine.Project(state)})
			}
			path := filepath.Join("..", "..", "..", ".local", "pokepath", "verification", "native-attack-modes")
			if err := os.MkdirAll(path, 0755); err != nil {
				t.Fatal(err)
			}
			value, _ := json.MarshalIndent(map[string]any{"species": scenario.key, "initial": json.RawMessage(initial), "operations": operations}, "", "  ")
			if err := os.WriteFile(filepath.Join(path, "parity-"+scenario.key+".json"), append(value, '\n'), 0600); err != nil {
				t.Fatal(err)
			}
		})
	}
}
