package pokepath_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func projectedBeamWidth(t *testing.T, projectile engine.Projectile) float64 {
	t.Helper()
	encoded, err := json.Marshal(projectile)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err = json.Unmarshal(encoded, &value); err != nil {
		t.Fatal(err)
	}
	width, ok := value["beamWidth"].(float64)
	if !ok {
		t.Fatal("actual beam omitted its canonical collision/display width")
	}
	return width
}

func TestNativeBeamGeometryUsesOriginalWidthsLengthCapsAndFirstHitTimingE2E(t *testing.T) {
	for _, scenario := range []struct {
		key   string
		width float64
		count int
	}{{"bulbasaur", 24, 0}, {"magearna", 24, 0}, {"registeel", 24, 0}, {"dratini", 50, 4}, {"spheal", 20, 4}, {"tapuKoko", 20, 39}, {"magnemite", 8, 0}, {"minior", 8, 0}, {"baltoy", 20, 0}} {
		for _, item := range []string{"", "terrainExtender"} {
			t.Run(scenario.key+item, func(t *testing.T) {
				state := waScenario(t)
				owner := waDeploy(t, state, scenario.key, 1)
				if item != "" {
					healingEquip(t, state, owner, item)
				}
				waBattle(t, state)
				state.Towers[0].AttackCount = scenario.count
				origin := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
				width := scenario.width
				if item != "" {
					width *= 2
				}
				tolerance := width/2 + 12
				primary := waEnemy(t, state, engine.Point{X: origin.X + 80, Y: origin.Y})
				persistentAreaEnemy(t, state, primary).Distance = 100
				inside := waEnemy(t, state, engine.Point{X: origin.X + 80, Y: origin.Y + tolerance})
				outside := waEnemy(t, state, engine.Point{X: origin.X + 80, Y: origin.Y + tolerance + .1})
				far := waEnemy(t, state, engine.Point{X: origin.X + 500, Y: origin.Y})
				waAdvance(t, state, 20)
				if len(state.Projectiles) == 0 || persistentAreaEnemy(t, state, primary).ReceivedHits != 0 {
					t.Fatal("ordinary beam hit immediately instead of waiting its original 140 ms cooldown")
				}
				beam := state.Projectiles[0]
				length := 2000.0
				if scenario.key == "baltoy" {
					length = beam.Profile.Range
				}
				if beam.RayLength != length || projectedBeamWidth(t, beam) != width {
					t.Fatal("actual beam length or width differed from the original", beam.RayLength, length, projectedBeamWidth(t, beam), width)
				}
				endpoint := waEnemy(t, state, engine.Point{X: origin.X + length + tolerance, Y: origin.Y})
				state.Towers[0].CooldownMS = 100000
				waAdvance(t, state, 146)
				if persistentAreaEnemy(t, state, primary).ReceivedHits != 0 {
					t.Fatal("beam hit before its first original scan crossed 140 ms")
				}
				waAdvance(t, state, 1)
				farHits := 1
				if scenario.key == "baltoy" {
					farHits = 0
				}
				if persistentAreaEnemy(t, state, primary).ReceivedHits != 1 || persistentAreaEnemy(t, state, inside).ReceivedHits != 1 || persistentAreaEnemy(t, state, outside).ReceivedHits != 0 || persistentAreaEnemy(t, state, far).ReceivedHits != farHits || persistentAreaEnemy(t, state, endpoint).ReceivedHits != 1 {
					t.Fatal("beam used a truncated range, common narrow line, missing enemy tolerance or square endpoint")
				}
				waSnapshotContinuation(t, state, 120)
				if len(state.Projectiles) != 0 || persistentAreaEnemy(t, state, primary).ReceivedHits != 1 {
					t.Fatal("expired beam scanned or repeated a forbidden final hit")
				}
				waEvidence(t, "beam-geometry-"+scenario.key+item, state, map[string]any{"width": width, "length": length, "hitCooldownMS": 140, "firstScanClockMS": 167, "lifeMS": 260, "endpointTolerance": tolerance})
			})
		}
	}
}

func TestNativeBeamTrackingUses390MSCooldownAndOriginalFourSecondLifeE2E(t *testing.T) {
	for _, key := range []string{"squirtle", "lunala"} {
		state := waScenario(t)
		waDeploy(t, state, key, 1)
		waBattle(t, state)
		origin := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
		primary := waEnemy(t, state, engine.Point{X: origin.X + 80, Y: origin.Y})
		waAdvance(t, state, 20)
		state.Towers[0].CooldownMS = 100000
		if len(state.Projectiles) != 1 || state.Projectiles[0].Profile.BeamTickMS != 390 || state.Projectiles[0].RayLength != max(100, state.Projectiles[0].Profile.Range) || persistentAreaEnemy(t, state, primary).ReceivedHits != 0 {
			t.Fatal("tracking beam did not use the original range, cooldown and delayed first hit", key)
		}
		waAdvance(t, state, 396)
		if persistentAreaEnemy(t, state, primary).ReceivedHits != 0 {
			t.Fatal("tracking beam hit before the 390 ms gate reached its next 50 ms scan", key)
		}
		waAdvance(t, state, 1)
		if persistentAreaEnemy(t, state, primary).ReceivedHits != 1 {
			t.Fatal("tracking beam missed the first 60 Hz scan at or after its cooldown", key)
		}
		waAdvance(t, state, 399)
		if persistentAreaEnemy(t, state, primary).ReceivedHits != 1 {
			t.Fatal("tracking beam reused the old 250 ms interval", key)
		}
		waAdvance(t, state, 1)
		if persistentAreaEnemy(t, state, primary).ReceivedHits != 2 {
			t.Fatal("tracking beam lost the next 390 ms per-enemy cooldown crossing", key)
		}
		waSnapshotContinuation(t, state, 3183)
		if len(state.Projectiles) != 0 || persistentAreaEnemy(t, state, primary).ReceivedHits != 9 {
			t.Fatal("tracking beam did not expire before its 4000 ms scan with exactly nine real hits", key)
		}
		waEvidence(t, "beam-tracking-"+key, state, map[string]any{"hitCooldownMS": 390, "scanMS": 50, "firstScanClockMS": 417, "lifeMS": 4000, "hits": 9})
	}
}

func TestNativeBeamTrackingFollowsThenKeepsLastDirectionAfterTargetDeathE2E(t *testing.T) {
	state := waScenario(t)
	waDeploy(t, state, "squirtle", 1)
	waBattle(t, state)
	origin := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
	primary := waEnemy(t, state, engine.Point{X: origin.X + 80, Y: origin.Y})
	waAdvance(t, state, 20)
	state.Towers[0].CooldownMS = 100000
	persistentAreaEnemy(t, state, primary).X, persistentAreaEnemy(t, state, primary).Y = origin.X, origin.Y+80
	waAdvance(t, state, 420)
	if persistentAreaEnemy(t, state, primary).ReceivedHits != 1 || math.Abs(state.Projectiles[0].RayDX) > .000001 || state.Projectiles[0].RayDY != 1 {
		t.Fatal("hydro cannon failed to follow the real attached target")
	}
	persistentAreaEnemy(t, state, primary).HP = 0
	secondary := waEnemy(t, state, engine.Point{X: origin.X, Y: origin.Y + 80})
	waAdvance(t, state, 60)
	if len(state.Projectiles) != 1 || persistentAreaEnemy(t, state, secondary).ReceivedHits != 1 || math.Abs(state.Projectiles[0].RayDX) > .000001 || state.Projectiles[0].RayDY != 1 {
		t.Fatal("dead tracking target canceled the original persistent last-position beam")
	}
	waSnapshotContinuation(t, state, 400)
}

func TestNativeBeamSignalAndCryogenicStaySingleAttachedAndDetachAtBoundaryE2E(t *testing.T) {
	for _, key := range []string{"volbeat", "ironBundle"} {
		state := waScenario(t)
		waDeploy(t, state, key, 1)
		waBattle(t, state)
		origin := engine.Point{X: state.Towers[0].X, Y: state.Towers[0].Y}
		primary := waEnemy(t, state, engine.Point{X: origin.X + 80, Y: origin.Y})
		persistentAreaEnemy(t, state, primary).Distance = 100
		bystander := waEnemy(t, state, engine.Point{X: origin.X + 60, Y: origin.Y})
		waAdvance(t, state, 20)
		state.Towers[0].CooldownMS = 0
		waAdvance(t, state, 20)
		if len(state.Projectiles) != 1 || persistentAreaEnemy(t, state, primary).ReceivedHits != 0 || persistentAreaEnemy(t, state, bystander).ReceivedHits != 0 {
			t.Fatal("attached beam immediately damaged, pierced a bystander or duplicated its active owner beam", key)
		}
		state.Towers[0].CooldownMS = 100000
		waSnapshotContinuation(t, state, 5000)
		if len(state.Projectiles) != 1 || persistentAreaEnemy(t, state, primary).ReceivedHits == 0 || persistentAreaEnemy(t, state, bystander).ReceivedHits != 0 {
			t.Fatal("attached beam lost its unbounded lifetime or single-target damage restriction", key)
		}
		persistentAreaEnemy(t, state, primary).X = origin.X + state.Projectiles[0].Profile.Range + 20
		waAdvance(t, state, 20)
		if len(state.Projectiles) != 0 {
			t.Fatal("attached beam survived its original inclusive range+20 detach boundary", key)
		}
		waEvidence(t, "beam-attached-"+key, state, map[string]any{"maximumActive": 1, "onlyAttachedEnemy": true, "life": "until death or detach", "bystanderDamage": 0})
	}
}
