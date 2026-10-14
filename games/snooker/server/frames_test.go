package snooker

// Written before sparse trajectory storage: stationary balls disappear,
// down/up transitions are dropped, frame times move, compression mutates input,
// reconstruction changes authoritative positions or duplicate IDs are created.
import (
	"reflect"
	"testing"
)

func TestCompactFramesReconstructExactly(t *testing.T) {
	initial := []Ball{testBall(0, .5, .8), testBall(1, 1.5, .8), testBall(21, 3, .8)}
	a := append([]Ball{}, initial...)
	a[0].Pos.X = .6
	b := append([]Ball{}, a...)
	b[1].Pos.X = 1.6
	b[2].Down = true
	full := []MotionFrame{{0, initial}, {.1, a}, {.2, b}, {.3, append([]Ball{}, b...)}}
	before := mustJSON(full)
	packed := compactFrames(full)
	if !reflect.DeepEqual(before, mustJSON(full)) {
		t.Fatal("compression mutated source")
	}
	if len(packed) != len(full) || len(packed[0].Balls) != 3 || len(packed[1].Balls) != 1 || len(packed[2].Balls) != 2 || len(packed[3].Balls) != 0 {
		t.Fatal("not sparse", packed)
	}
	state := map[int]Ball{}
	for i, f := range packed {
		if f.Time != full[i].Time {
			t.Fatal("changed frame time")
		}
		for _, b := range f.Balls {
			state[b.ID] = b
		}
		for _, b := range full[i].Balls {
			if state[b.ID] != b {
				t.Fatal("reconstruction changed ball", i, b, state[b.ID])
			}
		}
	}
}
