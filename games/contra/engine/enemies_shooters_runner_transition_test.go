package contra

// Failure scenarios before runner state3's firing transition: attack-disabled
// calls still decrement the timer; zero underflow is treated as expiry; burst
// size ignores strength/random bits; entering fire phase incorrectly stops the
// same frame's sprite/velocity work; recoil/collision inputs are overwritten.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalRunnerFireTransition(t *testing.T) {
	f, err := os.Open("testdata/original-enemy-shooters-runner-transition.json.gz")
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
		Original                  bool `json:"source_is_original_execution"`
		Terrain                   string
		ScrollX, ScrollY, Control uint8
		Cases                     []struct {
			Enabled, Strength, Random, FrameCounter uint8
			Players                                 [2]EnemyTarget
		}
		Records []struct {
			Case          int
			Before, After [][]uint8
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if !oracle.Original || len(oracle.Records) != 48 {
		t.Fatal("transition coverage changed")
	}
	raw, err := hex.DecodeString(oracle.Terrain)
	if err != nil || len(raw) != 128 {
		t.Fatal("invalid original collision map")
	}
	terrain := CollisionWindow(raw)
	for _, r := range oracle.Records {
		input := oracle.Cases[r.Case-1]
		s := EnemySystem{Slots: enemyPoolFromOracle(r.Before)}
		c := EnemyStepContext{AttackEnabled: input.Enabled != 0, WeaponStrength: input.Strength, Random: input.Random, Frame: input.FrameCounter, Players: input.Players, Terrain: &terrain, Scroll: CollisionScroll{X: oracle.ScrollX, Y: oracle.ScrollY, NameTable: oracle.Control}}
		if err = s.StepRunner(15, c); err != nil {
			t.Fatalf("transition%d: %v", r.Case, err)
		}
		want := enemyPoolFromOracle(r.After)
		if s.Slots != want {
			for i, v := range s.Slots {
				if v != want[i] {
					t.Fatalf("transition%d slot%d got%+v want%+v before%+v", r.Case, i, v, want[i], enemyFromOracle(r.Before[i]))
				}
			}
		}
	}
}
