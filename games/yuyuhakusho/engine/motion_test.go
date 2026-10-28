package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// These tests and both original-ROM fixtures precede the recovered code.
type originalMotionCases struct {
	Random []struct {
		Seed     uint32 `json:"seed"`
		Next     uint32 `json:"next_seed"`
		Returned uint32 `json:"returned"`
	} `json:"rng"`
	Projection []struct{ Before, After ProjectionState } `json:"projection"`
	Walk       []struct {
		Before, After MotionState
		Speed         int32 `json:"speed"`
	} `json:"walk"`
	Air []struct {
		Before, After MotionState
		Landing       bool `json:"landing"`
	} `json:"air"`
}

func readMotionCases(t *testing.T, name string) originalMotionCases {
	t.Helper()
	f, err := os.Open("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var cases originalMotionCases
	if err := json.NewDecoder(z).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases.Random) == 0 || len(cases.Projection) == 0 || len(cases.Walk) == 0 || len(cases.Air) == 0 {
		t.Fatal("incomplete original fixture")
	}
	return cases
}

func TestOriginalMotionStages(t *testing.T) {
	for _, file := range []string{"motion-oracle", "motion-boundaries"} {
		t.Run(file, func(t *testing.T) {
			cases := readMotionCases(t, file)
			for i, r := range cases.Random {
				next, value := NextRandom(r.Seed)
				if next != r.Next || value != r.Returned {
					t.Fatalf("random %d: got %08x/%08x want %08x/%08x", i, next, value, r.Next, r.Returned)
				}
			}
			for i, r := range cases.Projection {
				if got := ProjectFighter(r.Before); got != r.After {
					t.Fatalf("projection %d: got %+v want %+v", i, got, r.After)
				}
			}
			for i, r := range cases.Walk {
				if got := StepWalk(r.Before, r.Speed); got != r.After {
					t.Fatalf("walk %d: got %+v want %+v", i, got, r.After)
				}
			}
			for i, r := range cases.Air {
				got, landing := IntegrateAir(r.Before)
				if got != r.After || landing != r.Landing {
					t.Fatalf("air %d: got %+v/%v want %+v/%v", i, got, landing, r.After, r.Landing)
				}
			}
		})
	}
}

// A reset is only accepted where the original trace's next input actually
// differs from its preceding output; within each chain Go owns its state.
func TestOriginalRandomContinuousChains(t *testing.T) {
	rows := readMotionCases(t, "motion-oracle").Random
	var state uint32
	for i, r := range rows {
		if i == 0 || rows[i-1].Next != r.Seed {
			state = r.Seed
		}
		next, value := NextRandom(state)
		if next != r.Next || value != r.Returned {
			t.Fatalf("random chain diverged at call %d", i)
		}
		state = next
	}
}
