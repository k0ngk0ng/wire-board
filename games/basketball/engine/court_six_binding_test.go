package engine

// Failure inventory before Court6 binding: wrong playable depth72, New York
// passage fallback, slot reel bytes treated as passages, no-op84D6 substituted
// by another court device, loss of other-object writes, and auxiliary rejection.
import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestCourtSixOriginalMotionAndDeviceBinding(t *testing.T) {
	data, e := os.ReadFile("testdata/court-six-motion.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Object  uint8
		Sounds  []uint8
		Variant int
	}
	if e = json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 2080 {
		t.Fatal("missing court6 source rows", len(rows))
	}
	f, e := os.Open("testdata/court-six-motion.ram.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var objects [13]int
	triggers := 0
	for index, row := range rows {
		var raw [4096]byte
		if _, e = io.ReadFull(z, raw[:]); e != nil {
			t.Fatal(e)
		}
		before, want := courtFloorRAM(raw[:2048]), courtFloorRAM(raw[2048:])
		result := WorldActionStep{}
		court, e := originalWorldMotionCourt(&result, WorldActionContext{CourtID: 6}, row.Object)
		if e != nil {
			t.Fatal(index, e)
		}
		// Supply exact independent source state at the binding boundary. Its
		// returned complete state and committed callback must both match EEFF.
		stored := false
		court.originalMotion.load = func() CourtFloorState { return before }
		court.originalMotion.store = func(got CourtFloorState, selected uint8) {
			stored = true
			if got != want || selected != row.Object {
				t.Errorf("case%d bound commit differs", index)
			}
		}
		got, e := court.originalMotion.run(before)
		if e != nil || !stored || got.State != want || len(got.SoundIDs) != len(row.Sounds) || len(row.Sounds) > 0 && !reflect.DeepEqual(got.SoundIDs, row.Sounds) {
			t.Fatalf("case%d object%d before%+v\ngot%+v\nwant%+v\nerr%v", index, row.Object, before.Objects[row.Object], got.State.Objects[row.Object], want.Objects[row.Object], e)
		}
		if len(result.SoundIDs) != len(row.Sounds) || len(row.Sounds) > 0 && !reflect.DeepEqual(result.SoundIDs, row.Sounds) {
			t.Fatal("binding sound order", index)
		}
		if row.Object < 4 && court.originalMotion.resetJumpPhase != (before.JumpPhases[row.Object] != want.JumpPhases[row.Object]) {
			t.Fatal("selected jump-phase write lost", index)
		}
		if !got.HasSurface || len(result.MotionSurfaceWrites) != 1 || result.MotionSurfaceWrites[0].Origin != before.Objects[row.Object].Body.Position {
			t.Fatal("motion surface event boundary", index)
		}
		objects[row.Object]++
		triggers += len(row.Sounds)
	}
	if triggers != 0 {
		t.Fatal("Court6 unexpectedly used another court device")
	}
	t.Logf("matched %d original Court6 EEFF calls, per-object %v, device sound requests %d", len(rows), objects, triggers)
}

func TestCourtSixAuxiliaryRequiresOriginalSurface(t *testing.T) {
	s := AuxiliaryState{Action: 0, Entered: true, Owner: -1, NewYorkPassage: 17, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, Velocity: Velocity{X: Pixel}}}
	c := AuxiliaryContext{Object: 4, Court: 6, SpecialHolder: -1}
	if got, e := StepAuxiliaryRest(s, c); e == nil || got.State != s {
		t.Fatal("court6 silently used NY motion", got, e)
	}
	world := WorldActionStep{}
	world.State.Auxiliary[0] = s
	world.State.Holder = 6
	court, e := originalWorldMotionCourt(&world, WorldActionContext{CourtID: 6}, 4)
	if e != nil {
		t.Fatal(e)
	}
	got, e := stepAuxiliaryRestOnCourt(s, c, court)
	if e != nil || got.State.Body.Position.X != 201*Pixel || got.State.NewYorkPassage != 17 {
		t.Fatal("aux shared switch timer used as NY passage", got, e)
	}
}
