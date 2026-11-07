package contra

// Failure cases established before enemy-bullet implementation: inactive slots
// cleared too aggressively, fractional carry/sign lost, scroll applied on the
// wrong axis, offscreen removal skips/reorders coordinate writes, initialization
// moves bullets a frame early, and post-hit transitions erase the kind/state.

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type enemyOracleStep struct {
	ID, Frame, Actor int
	Sounds           []uint8
	Before, After    struct {
		Graphics struct {
			Used, Mode        uint8
			Bytes, Supertiles string
		}
		Enemies [][]uint8
		World   []uint8
		Map     int
		Players []struct {
			State, X, Y                                                uint8
			Hidden, Jump, Water, SpawnInvincible, Invincible, GameOver uint8
		}
	}
}

func TestControlledEnemyExplosions(t *testing.T) {
	// Sound, animation, slot removal and scroll must retain one timeline; the
	// native initializer can emit sound even when its source sprite is absent.
	rows, _ := loadEnemyFixture(t, "testdata/original-enemy-explosions.json.gz")
	if len(rows) != 43 {
		t.Fatal("controlled destruction coverage", len(rows))
	}
	for _, r := range rows {
		before := enemyPoolFromOracle(r.Before.Enemies)
		w := r.Before.World
		s := EnemySystem{Slots: before}
		if err := s.StepExplosion(r.Actor, EnemyStepContext{Vertical: w[8] != 0, FrameScroll: w[11]}, ExplosionPhase(before[r.Actor].Routine-7)); err != nil {
			t.Fatal(err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if s.Slots != want {
			for i, e := range s.Slots {
				if e != want[i] {
					t.Fatalf("original explosion %d slot%d got %+v want %+v before %+v", r.ID, i, e, want[i], before[i])
				}
			}
		}
		if len(s.PendingSounds) != len(r.Sounds) {
			t.Fatalf("original explosion %d sounds %v want %v", r.ID, s.PendingSounds, r.Sounds)
		}
		for i, sound := range s.PendingSounds {
			if sound != r.Sounds[i] {
				t.Fatal("wrong original explosion sound", sound, r.Sounds)
			}
		}
	}
}

func loadEnemySteps(t *testing.T) ([]enemyOracleStep, map[int]CollisionWindow) {
	return loadEnemyFixture(t, "testdata/original-enemy-step.json.gz")
}
func loadEnemyFixture(t *testing.T, path string) ([]enemyOracleStep, map[int]CollisionWindow) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle struct {
		Original bool `json:"source_is_original_execution"`
		Records  []enemyOracleStep
		Maps     []struct {
			ID    int
			Bytes string
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if !oracle.Original {
		t.Fatal("expected original execution")
	}
	maps := map[int]CollisionWindow{}
	for _, m := range oracle.Maps {
		data, err := hex.DecodeString(m.Bytes)
		if err != nil || len(data) != 128 {
			t.Fatal("invalid oracle map", m.ID, err)
		}
		maps[m.ID] = CollisionWindow(data)
	}
	return oracle.Records, maps
}

func TestControlledEnemyBulletStates(t *testing.T) {
	rows, maps := loadEnemyFixture(t, "testdata/original-enemy-bullet-cases.json.gz")
	if len(rows) != 63 {
		t.Fatalf("controlled original coverage changed: %d", len(rows))
	}
	for _, r := range rows {
		before := enemyPoolFromOracle(r.Before.Enemies)
		window := maps[r.Before.Map]
		w := r.Before.World
		system := EnemySystem{Slots: before}
		err := system.StepBullet(r.Actor, EnemyStepContext{Level: w[3], Frame: w[0], Vertical: w[8] != 0, FrameScroll: w[11], SolidBulletBackground: w[29]&0x80 != 0, Terrain: &window, Scroll: CollisionScroll{Y: w[24], X: w[25], NameTable: w[26]}})
		if err != nil {
			t.Fatalf("controlled original %d: %v", r.ID, err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if system.Slots != want {
			for i, e := range system.Slots {
				if e != want[i] {
					t.Fatalf("controlled original %d slot%d got %+v want %+v before %+v", r.ID, i, e, want[i], before[i])
				}
			}
		}
	}
}
func TestOriginalEnemyBullets(t *testing.T) {
	rows, maps := loadEnemySteps(t)
	count := 0
	for _, r := range rows {
		before := enemyPoolFromOracle(r.Before.Enemies)
		if before[r.Actor].Kind != 1 {
			continue
		}
		w := r.Before.World
		if w[3] != 0 {
			t.Fatal("fixture needs explicit per-level solid flags")
		}
		window := maps[r.Before.Map]
		system := EnemySystem{Slots: before}
		err := system.StepBullet(r.Actor, EnemyStepContext{Level: w[3], Frame: w[0], Vertical: w[8] != 0, FrameScroll: w[11], Terrain: &window, Scroll: CollisionScroll{Y: w[24], X: w[25], NameTable: w[26]}})
		if err != nil {
			t.Fatalf("original bullet %d: %v", r.ID, err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if system.Slots != want {
			for i, e := range system.Slots {
				if e != want[i] {
					t.Fatalf("original bullet %d frame%d slot%d: got %+v want %+v before %+v", r.ID, r.Frame, i, e, want[i], before[i])
				}
			}
		}
		count++
	}
	if count != 924 {
		t.Fatalf("original bullet coverage %d want924", count)
	}
	t.Logf("%d original enemy bullet calls matched", count)
}

func TestOriginalEnemyRunners(t *testing.T) {
	// New behavior is driven by natural original calls, including edge turns,
	// random jump choices, fractional movement and both water-splash phases.
	rows, maps := loadEnemySteps(t)
	count := 0
	for _, r := range rows {
		before := enemyPoolFromOracle(r.Before.Enemies)
		if before[r.Actor].Kind != 5 {
			continue
		}
		w := r.Before.World
		window := maps[r.Before.Map]
		c := EnemyStepContext{Level: w[3], Frame: w[0], Vertical: w[8] != 0, FrameScroll: w[11], Random: w[5], AttackEnabled: w[22] != 0, Terrain: &window, Scroll: CollisionScroll{Y: w[24], X: w[25], NameTable: w[26]}}
		for i, p := range r.Before.Players {
			c.Players[i] = EnemyTarget{State: p.State, X: p.X, Y: p.Y}
		}
		system := EnemySystem{Slots: before}
		if err := system.StepRunner(r.Actor, c); err != nil {
			t.Fatalf("original runner %d: %v", r.ID, err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if system.Slots != want {
			for i, e := range system.Slots {
				if e != want[i] {
					t.Fatalf("original runner %d frame%d slot%d got %+v want %+v before %+v world%+v", r.ID, r.Frame, i, e, want[i], before[i], w)
				}
			}
		}
		count++
	}
	if count != 1123 {
		t.Fatalf("original runner coverage %d want1123", count)
	}
}

func TestOriginalFlyingCapsules(t *testing.T) {
	// Preserve anchor-based acceleration rather than substituting a sine wave;
	// initialization and per-frame scroll happen at different original points.
	rows, _ := loadEnemySteps(t)
	count := 0
	for _, r := range rows {
		before := enemyPoolFromOracle(r.Before.Enemies)
		if before[r.Actor].Kind != 3 {
			continue
		}
		w := r.Before.World
		s := EnemySystem{Slots: before}
		if err := s.StepCapsule(r.Actor, EnemyStepContext{Vertical: w[8] != 0, FrameScroll: w[11]}); err != nil {
			t.Fatal(err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if s.Slots != want {
			for i, e := range s.Slots {
				if e != want[i] {
					t.Fatalf("original capsule %d slot%d got %+v want %+v before %+v", r.ID, i, e, want[i], before[i])
				}
			}
		}
		count++
	}
	if count != 257 {
		t.Fatalf("original capsule coverage %d want257", count)
	}
}

func TestOriginalCapsuleDestruction(t *testing.T) {
	// Integration found the originally unobserved third capsule phase: Japanese
	// $8385 jumps to $82D4, sharing sound/explosion/pickup conversion with a box.
	rows, _ := loadEnemyFixture(t, "testdata/original-enemy-capsule-destruction.json.gz")
	if len(rows) != 1 {
		t.Fatal("capsule destruction coverage", len(rows))
	}
	for _, r := range rows {
		before := enemyPoolFromOracle(r.Before.Enemies)
		w := r.Before.World
		s := EnemySystem{Slots: before}
		if err := s.StepCapsule(r.Actor, EnemyStepContext{Level: w[3], Vertical: w[8] != 0, FrameScroll: w[11]}); err != nil {
			t.Fatal(err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if s.Slots != want {
			for i, e := range s.Slots {
				if e != want[i] {
					t.Fatalf("original capsule destruction %d slot%d got%+v want%+v", r.ID, i, e, want[i])
				}
			}
		}
		if len(s.PendingSounds) != len(r.Sounds) {
			t.Fatalf("capsule sound got%v want%v", s.PendingSounds, r.Sounds)
		}
		for i, v := range s.PendingSounds {
			if v != r.Sounds[i] {
				t.Fatal("capsule sound", v, r.Sounds)
			}
		}
	}
}

func TestControlledRunnerDestruction(t *testing.T) {
	// Destruction must retain original impulse, edge stop, scroll order,
	// decrement-underflow and shared explosion sound/removal behavior.
	rows, maps := loadEnemyFixture(t, "testdata/original-enemy-runner-death.json.gz")
	if len(rows) != 81 {
		t.Fatal("runner destruction coverage", len(rows))
	}
	for _, r := range rows {
		before := enemyPoolFromOracle(r.Before.Enemies)
		w := r.Before.World
		window := maps[r.Before.Map]
		s := EnemySystem{Slots: before}
		if err := s.StepRunner(r.Actor, EnemyStepContext{Vertical: w[8] != 0, FrameScroll: w[11], Terrain: &window}); err != nil {
			t.Fatal(err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if s.Slots != want {
			for i, e := range s.Slots {
				if e != want[i] {
					t.Fatalf("original runner death %d slot%d got %+v want %+v before %+v", r.ID, i, e, want[i], before[i])
				}
			}
		}
		if len(s.PendingSounds) != len(r.Sounds) {
			t.Fatalf("runner destruction sounds got %v want %v", s.PendingSounds, r.Sounds)
		}
		for i, v := range s.PendingSounds {
			if v != r.Sounds[i] {
				t.Fatal("runner destruction sound", v, r.Sounds)
			}
		}
	}
}
