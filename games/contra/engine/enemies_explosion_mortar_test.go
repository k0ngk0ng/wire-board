package contra

// Failure cases before E683 support: retaining bit0 or bit6 in StateWidth,
// losing explosive player contact by retaining the old collision shape, using
// the wrong sound flag, or changing adjacent entity slots during conversion.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalMortarExplosionEntry(t *testing.T) {
	checked := 0
	for _, name := range []string{"natural", "small-cases"} {
		f, err := os.Open("testdata/original-enemies-snow-" + name + ".json.gz")
		if err != nil {
			t.Fatal(err)
		}
		z, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		var oracle struct {
			Original bool `json:"source_is_original_execution"`
			Records  []struct {
				ID, Slot      int
				Before, After struct {
					Enemies [][]uint8
					World   []uint8
				}
				Events []struct {
					Kind string
					Code uint8
				}
			}
		}
		err = json.NewDecoder(z).Decode(&oracle)
		z.Close()
		f.Close()
		if err != nil || !oracle.Original {
			t.Fatal("original snow oracle", err)
		}
		for _, r := range oracle.Records {
			before := enemyPoolFromOracle(r.Before.Enemies)
			if before[r.Slot].Kind != 17 || before[r.Slot].Routine != 3 {
				continue
			}
			s := EnemySystem{Slots: before}
			w := r.Before.World
			if err := s.StepExplosion(r.Slot, EnemyStepContext{Level: w[2], Vertical: w[7] != 0, FrameScroll: w[10]}, ExplosionMortarStart); err != nil {
				t.Fatal(err)
			}
			want := enemyPoolFromOracle(r.After.Enemies)
			if s.Slots != want {
				t.Fatalf("%s id%d got%+v want%+v", name, r.ID, s.Slots[r.Slot], want[r.Slot])
			}
			var sounds []uint8
			for _, e := range r.Events {
				if e.Kind == "sound" {
					sounds = append(sounds, e.Code)
				}
			}
			if !reflect.DeepEqual(s.PendingSounds, sounds) {
				t.Fatalf("id%d sounds%v want%v", r.ID, s.PendingSounds, sounds)
			}
			checked++
		}
	}
	if checked != 17 {
		t.Fatalf("E683 coverage %d want17", checked)
	}
}
