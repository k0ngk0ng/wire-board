package contra

// Failure scenarios before extending spawn: per-level stream confusion,
// missed or early triggers, full-pool mandatory replacement drift, repeat
// truncation, wrong vertical coordinates, indoor half-tile bits ignored,
// repeated indoor resets, stale base counters, and invented screen clearing.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalAllStageEnemySpawning(t *testing.T) {
	f, err := os.Open("testdata/original-enemy-all-spawn.json.gz")
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
		Records  []struct {
			Case          int
			Before, After struct {
				Enemies [][]uint8
				World   []uint8
			}
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if !oracle.Original || len(oracle.Records) != 3230 {
		t.Fatal("all-stage original spawning coverage")
	}
	for _, r := range oracle.Records {
		w := r.Before.World
		cleared := w[6]
		s := EnemySystem{Slots: enemyPoolFromOracle(r.Before.Enemies), ScreenReadOffset: w[5], Base: BaseEnemyState{w[7], w[8], w[9], w[10], w[11]}}
		c := EnemySpawnContext{Level: w[0], SceneType: w[1], Vertical: w[2] != 0, Screen: w[3], Scroll: w[4], ScreenCleared: &cleared}
		if err := s.SpawnScreen(c); err != nil {
			t.Fatalf("case%d world%v: %v", r.Case, w, err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if s.Slots != want {
			for i, e := range s.Slots {
				if e != want[i] {
					t.Fatalf("case%d world%v slot%d got%+v want%+v", r.Case, w, i, e, want[i])
				}
			}
		}
		aw := r.After.World
		wantBase := BaseEnemyState{aw[7], aw[8], aw[9], aw[10], aw[11]}
		if s.ScreenReadOffset != aw[5] || cleared != aw[6] || s.Base != wantBase {
			t.Fatalf("case%d cursor%d/%d clear%d/%d base%+v/%+v", r.Case, s.ScreenReadOffset, aw[5], cleared, aw[6], s.Base, wantBase)
		}
	}
}
