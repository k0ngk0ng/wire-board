package contra

// Failure scenarios before generator code: timer carry changes random scans;
// scan starts/steps reverse or stop early; script probabilities use new RNG;
// edge protection ignores GameOver; random soldiers allocate slots7..15;
// burst count/attributes or the inherited variant work byte are invented;
// screen counter increments on a three-soldier burst when the original does not.
import "testing"

func TestOriginalSoldierGeneration(t *testing.T) {
	compareOriginalSoldierGeneration(t, "testdata/original-enemy-generators.json.gz", 1900)
}

func TestControlledSoldierGeneration(t *testing.T) {
	// The controlled original matrix includes nonzero/wrapping inherited burst
	// seeds, all eight timer profiles, carry boundaries, slot6 cap and edge gates.
	compareOriginalSoldierGeneration(t, "testdata/original-enemy-generator-cases.json.gz", 1163)
}

func compareOriginalSoldierGeneration(t *testing.T, path string, count int) {
	rows, maps := loadEnemyFixture(t, path)
	if len(rows) != count {
		t.Fatal("generator coverage", len(rows), count)
	}
	for _, r := range rows {
		w := r.Before.World
		window := maps[r.Before.Map]
		s := EnemySystem{Slots: enemyPoolFromOracle(r.Before.Enemies), Generator: SoldierGenerator{Routine: w[16], Timer: w[17], X: w[18], Y: w[19], Screen: w[27], Generated: w[28]}}
		c := EnemyStepContext{Level: w[3], GameLoop: w[4], Frame: w[0], Random: w[5], Vertical: w[8] != 0, Screen: w[9], FrameScroll: w[11], AttackEnabled: w[22] != 0, WeaponStrength: w[29], SpawnVariantSeed: w[30], Terrain: &window, Scroll: CollisionScroll{Y: w[24], X: w[25], NameTable: w[26]}}
		for i, p := range r.Before.Players {
			c.Players[i] = EnemyTarget{State: p.State, X: p.X, Y: p.Y, GameOver: p.GameOver}
		}
		result, err := s.SpawnSoldiers(c)
		if err != nil {
			t.Fatalf("generator original%d: %v", r.ID, err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		a := r.After.World
		if s.Slots != want {
			for i, e := range s.Slots {
				if e != want[i] {
					t.Fatalf("generator original%d frame%d slot%d got%+v want%+v world%v", r.ID, r.Frame, i, e, want[i], w)
				}
			}
		}
		wanted := SoldierGenerator{Routine: a[16], Timer: a[17], X: a[18], Y: a[19], Screen: a[27], Generated: a[28]}
		if s.Generator != wanted || result.NextVariantSeed != a[30] {
			t.Fatalf("generator original%d frame%d got%+v seed%d want%+v seed%d before%v", r.ID, r.Frame, s.Generator, result.NextVariantSeed, wanted, a[30], w)
		}
		if window != maps[r.After.Map] {
			t.Fatal("generator changed collision map")
		}
	}
}
