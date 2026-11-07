package contra

// Failure scenarios before all-stage support: same numeric kind selects the
// wrong level's properties; poison survives initialization; neighbor slots are
// overwritten; reused base tables drift; invalid kind/level wraps into ROM data.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalAllStageEnemyInitialization(t *testing.T) {
	f, err := os.Open("testdata/original-enemy-all-initialization.json.gz")
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
		Cases    []struct {
			Level, Kind uint8
			Slot        int
		}
		Records []struct {
			Case          int
			Before, After [][]uint8
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if !oracle.Original || len(oracle.Cases) != 597 || len(oracle.Records) != 597 {
		t.Fatal("initializer original coverage")
	}
	for _, r := range oracle.Records {
		c := oracle.Cases[r.Case-1]
		s := EnemySystem{Slots: enemyPoolFromOracle(r.Before)}
		if err := s.Initialize(c.Slot, c.Kind, c.Level); err != nil {
			t.Fatalf("level%d kind%d: %v", c.Level, c.Kind, err)
		}
		want := enemyPoolFromOracle(r.After)
		if s.Slots != want {
			for i, e := range s.Slots {
				if e != want[i] {
					t.Fatalf("case%d level%d kind%d slot%d got%+v want%+v", r.Case, c.Level, c.Kind, i, e, want[i])
				}
			}
		}
	}
}

func TestAllStageInitializationRejectsUnmappedKinds(t *testing.T) {
	counts := [8]uint8{19, 33, 22, 33, 23, 21, 25, 23}
	for level, count := range counts {
		s := EnemySystem{}
		s.Slots[0] = Enemy{Kind: 5, Routine: 3, HP: 7}
		before := s.Slots
		if err := s.Initialize(0, count, uint8(level)); err == nil || s.Slots != before {
			t.Fatalf("invalid level%d kind%d accepted/mutated", level, count)
		}
	}
}
