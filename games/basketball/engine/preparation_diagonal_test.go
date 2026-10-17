package engine

// Failure inventory: the positive-X/negative-depth diagonal reads another
// player's X fraction; dropping that source changes launch speed. New York
// diagonal corner entry must use the original complete floor operation, not
// skip collision or invent a bounce. Saved full runtime states must advance
// atomically and reproduce after serialization with the normal video clock.
import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestDiagonalLaunchAgainstOriginalFunction(t *testing.T) {
	f, e := os.Open("testdata/diagonal-launch-function.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []struct {
		Position, Target   Position
		Arc, SecondPlayerX Fixed
		Velocity           Velocity
		Gravity            Fixed
	}
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 576 {
		t.Fatal("original launch matrix incomplete", len(rows))
	}
	for i, row := range rows {
		got, e := LaunchPass(row.Position, row.Target, row.Arc, PassLaunchContext{SecondPlayerX: row.SecondPlayerX})
		if e != nil {
			t.Fatalf("case %d: %v", i, e)
		}
		want := Body{Position: row.Position, Velocity: row.Velocity, Gravity: row.Gravity}
		if got != want {
			t.Fatalf("case %d %+v: got %+v want %+v", i, row, got, want)
		}
	}
}

func TestPreparedDiagonalRuntimeFirstDifferences(t *testing.T) {
	// Independently executed EEFF inputs use the two saved failing ball bodies.
	// Compare the complete bound motion before advancing the full video states.
	mf, e := os.Open("testdata/prepared-corner-motion.ram.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer mf.Close()
	mz, e := gzip.NewReader(mf)
	if e != nil {
		t.Fatal(e)
	}
	defer mz.Close()
	for i := 0; i < 2; i++ {
		var raw [4096]byte
		if _, e = io.ReadFull(mz, raw[:]); e != nil {
			t.Fatal(e)
		}
		before, want := courtFloorRAM(raw[:2048]), courtFloorRAM(raw[2048:])
		got, e := StepOriginalCourtMotion(before, 6, 1)
		if e != nil {
			t.Fatal(e)
		}
		if got.State != want {
			t.Fatalf("corner %d original EEFF mismatch: got %+v want %+v", i, got.State.Objects[6], want.Objects[6])
		}
		o := before.Objects[6]
		b := Ball{Body: o.Body, Action: BallAction(o.Action), Entered: o.Entered, Request: BallRequest{Action: BallAction(o.Request.Action), Pending: o.Request.Pending}, Pose: o.Pose, FacingLeft: o.FacingLeft, Airborne: o.Flags&128 != 0, Deflection: before.BallDeflection}
		binding := &courtMotionBinding{object: 6, court: 1, load: func() CourtFloorState { return before }, store: func(s CourtFloorState, _ uint8) {
			if s != want {
				t.Fatal("bound floor state differs from source")
			}
		}}
		if e = advanceBallOnCourt(&b, FlatCourt{originalMotion: binding}); e != nil {
			t.Fatal(e)
		}
		o = want.Objects[6]
		if b.Body != o.Body || b.Airborne != (o.Flags&128 != 0) || b.Deflection != want.BallDeflection {
			t.Fatal("bound ball differs from original corner movement")
		}
	}
	f, e := os.Open("testdata/preparation-diagonal-runtime.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []struct {
		Name, Failure string
		State         RuntimeState
	}
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 6 {
		t.Fatal("missing saved first differences", len(rows))
	}
	for _, row := range rows {
		t.Run(row.Name, func(t *testing.T) {
			encoded, e := json.Marshal(row.State)
			if e != nil {
				t.Fatal(e)
			}
			var restored RuntimeState
			if e = json.Unmarshal(encoded, &restored); e != nil {
				t.Fatal(e)
			}
			step, e := StepVideo(row.State, [4]Buttons{})
			if e != nil {
				t.Fatalf("saved first difference: %v", e)
			}
			replayed, e := StepVideo(restored, [4]Buttons{})
			if e != nil {
				t.Fatal(e)
			}
			if !step.LogicAdvanced || !reflect.DeepEqual(step, replayed) {
				t.Fatal("logic step or restored deterministic replay differs")
			}
		})
	}
}
