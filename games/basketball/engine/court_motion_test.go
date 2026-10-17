package engine

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

func courtFloorRAM(r []byte) CourtFloorState {
	var s CourtFloorState
	w := func(lo, hi int) Fixed { return Fixed(uint16(r[lo]) | uint16(r[hi])<<8) }
	pos := func(i int) Position {
		return Position{X: signed24(w(0x68+i, 0x75+i) | Fixed(r[0x82+i])<<16), Depth: w(0x8f+i, 0x9c+i), Height: signed24(w(0xa9+i, 0xb6+i) | Fixed(r[0xc3+i])<<16)}
	}
	for i := range s.Objects {
		s.Objects[i] = CourtFloorObject{Body: Body{Position: pos(i), Velocity: Velocity{X: signed16(w(0x478+i, 0x485+i)), Depth: signed16(w(0x492+i, 0x49f+i)), Height: signed16(w(0x4ac+i, 0x4b9+i))}, Gravity: signed16(w(0x4c6+i, 0x4d3+i))}, Travel: FloorPoint{X: signed24(w(0x58e+i, 0x59b+i) | Fixed(r[0x5a8+i])<<16), Depth: w(0x5b5+i, 0x5c2+i)}, PreviousHeight: signed16(w(0x5cf+i, 0x5dc+i)) * Pixel, Action: r[0x512+i] & 127, Entered: r[0x512+i] >= 128, Request: ActionRequest{Action: Action(r[0x51f+i] & 127), Pending: r[0x51f+i] < 128}, Pose: Pose{Frame: r[0x5b+i] & 127, Mirrored: r[0x5b+i] >= 128}, FacingLeft: r[0x46b+i] >= 128, Flags: r[0x5f7+i], Kind: r[0x3d6+i], Attachment: r[0x642+i]}
		if i < 4 {
			s.Objects[i].Controller = r[0x539+i]
		} else {
			s.Objects[i].Owner = int8(r[0x539+i])
		}
	}
	copy(s.CourtFlags[:], r[0x6d7:0x6e4])
	s.LastShooter = r[0x6d1]
	s.BallDeflection = DeflectionAxes(r[0x564])
	for i := 0; i < 4; i++ {
		s.LinkedActors[i] = int8(r[0x550+i])
		s.JumpPhases[i] = int8(r[0x576+i])
	}
	s.Random = RandomState(uint32(r[0x48])<<24 | uint32(r[0x49])<<16 | uint32(r[0x4a])<<8 | uint32(r[0x4b]))
	return s
}
func TestOriginalCourtFloorNaturalNewYork(t *testing.T) {
	verifyCourtFloorRecords(t, "court-floor-natural", 14458)
}
func TestOriginalCourtFloorConfiguredMachines(t *testing.T) {
	verifyCourtFloorRecords(t, "court-floor-machines", 68094)
}
func verifyCourtFloorRecords(t *testing.T, name string, count int) {
	data, e := os.ReadFile("testdata/" + name + ".json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Frame  int
		Object uint8
		Sounds []uint8
	}
	if e = json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open("testdata/" + name + ".ram.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var counts [13]int
	for _, row := range rows {
		var raw [4096]byte
		if _, e = io.ReadFull(z, raw[:]); e != nil {
			t.Fatal(e)
		}
		before, want := courtFloorRAM(raw[:2048]), courtFloorRAM(raw[2048:])
		got, e := ResolveOriginalCourtSurface(before, row.Object, raw[0x58b])
		if e != nil {
			t.Fatal(row, e)
		}
		if got.State != want {
			for i := range want.Objects {
				if got.State.Objects[i] != want.Objects[i] {
					t.Errorf("frame %d selected %d object %d before%+v got%+v want%+v", row.Frame, row.Object, i, before.Objects[i], got.State.Objects[i], want.Objects[i])
				}
			}
			if got.State.CourtFlags != want.CourtFlags {
				t.Errorf("court flags got%v want%v", got.State.CourtFlags, want.CourtFlags)
			}
			t.Fatalf("floor state differs frame%d object%d", row.Frame, row.Object)
		}
		if got.FloorCode != CourtSurface(raw[2048+0x604+int(row.Object)]) || got.FloorHeight != raw[2048+0x5ea+int(row.Object)] {
			t.Fatal("floor cache differs", row)
		}
		if len(got.SoundIDs) != len(row.Sounds) || (len(row.Sounds) > 0 && !reflect.DeepEqual(got.SoundIDs, row.Sounds)) {
			t.Fatal("floor sound differs", row, got.SoundIDs)
		}
		counts[row.Object]++
	}
	if (count > 0 && len(rows) != count) || len(rows) < 1000 {
		t.Fatal("natural floor coverage incomplete", len(rows))
	}
	t.Logf("matched %d %s floor resolutions by object %v", len(rows), name, counts)
}

// Complete wrapper failures: double integration, premature XY commit, omitted
// New York passage advancement, attached integer following, lost fractions,
// changed shared state and terrain events emitted on bypass paths.
func TestCompleteOriginalCourtMotionNatural(t *testing.T) {
	f, e := os.Open("testdata/plateau-motion.ram.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var counts [13]int
	total := 0
	for {
		var raw [4101]byte
		_, e = io.ReadFull(z, raw[:])
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		object := raw[4]
		b, a := raw[5:2053], raw[2053:]
		before, want := courtFloorRAM(b), courtFloorRAM(a)
		got, e := StepOriginalCourtMotion(before, object, b[0x58b])
		if e != nil {
			t.Fatal(total, object, e)
		}
		if got.State != want {
			for i := range want.Objects {
				if got.State.Objects[i] != want.Objects[i] {
					t.Errorf("record%d selected%d object%d before%+v got%+v want%+v", total, object, i, before.Objects[i], got.State.Objects[i], want.Objects[i])
				}
			}
			t.Fatalf("complete motion state differs record%d object%d", total, object)
		}
		bypass := before.Objects[object].Attachment&128 != 0 || before.CourtFlags[object] != 0
		if got.HasSurface == bypass {
			t.Fatal("incorrect terrain event boundary", total, object, bypass, got.HasSurface)
		}
		if got.HasSurface && (got.SurfaceOrigin != before.Objects[object].Body.Position || got.FloorHeight != a[0x5ea+int(object)] || got.FloorCode != CourtSurface(a[0x604+int(object)])) {
			t.Fatal("incorrect old-point terrain observation", total)
		}
		counts[object]++
		total++
	}
	if total != 13229 {
		t.Fatal("complete natural motion coverage changed", total)
	}
	t.Logf("matched %d complete natural EEFF calls by object %v", total, counts)
}
