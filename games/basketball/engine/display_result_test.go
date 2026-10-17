package engine

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

// Failure cases: BA40 background initialization leaves the preceding ball's
// fourth CHR page active; raw 0x86 reselects a gameplay page; restore drops page
// 13; the result write unnecessarily discards retained court-ring columns.
func TestRuntimeResultInstallsOriginalCHRBank(t *testing.T) {
	f, err := os.Open("testdata/match-result.ram.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var raw [4096]byte
	if _, err = io.ReadFull(z, raw[:]); err != nil {
		t.Fatal(err)
	}
	before, after := raw[:2048], raw[2048:]
	if before[0x589] != 6 || after[0x589] != 0x86 || before[0x54] != 32 || after[0x54] != 13 {
		t.Fatal("original BCC3 capture changed", before[0x589], after[0x589], before[0x54], after[0x54])
	}
	for _, previous := range []uint8{32, 33, 34, 35} {
		s, err := NewRuntime(DefaultRuntimeConfig())
		if err != nil {
			t.Fatal(err)
		}
		o := *s.Opening
		o.Transition.Match.Phase = 6
		o.Transition.Match.Period = 2
		o.Transition.Scores = [2]uint16{0, 12}
		o.Projection = resetOriginalWorldDrawList(o.Projection)
		s.Opening = nil
		s.Transition = &RuntimeTransitionState{Scene: o}
		s.LogicSteps = 1
		s.Display.BallCHR = previous
		s.Display.CourtColumns = []CourtColumnCommit{{Column: 7, Phase: 1}, {Column: 8, Phase: 1}}
		s.Display.CourtSerial = 2
		originalDisplay := s.Display
		r, err := StepLogic(s, [4]ControllerFrame{})
		if err != nil {
			t.Fatal(err)
		}
		if r.State.Display.BallCHR != after[0x54] {
			t.Fatalf("BA40/BCC3 must install observed CHR page %d, preceding=%d got=%d", after[0x54], previous, r.State.Display.BallCHR)
		}
		if !reflect.DeepEqual(r.State.Display.CourtColumns, originalDisplay.CourtColumns) || r.State.Display.CourtSerial != originalDisplay.CourtSerial || r.State.Display.CourtEpoch != originalDisplay.CourtEpoch {
			t.Fatal("result CHR write reset court history")
		}
		data, _ := json.Marshal(r.State)
		var restored RuntimeState
		if err = json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		if err = ValidateRuntimeState(restored); err != nil {
			t.Fatal(err)
		}
		for n := 0; n < 4; n++ {
			r, err = StepVideo(restored, [4]Buttons{})
			if err != nil {
				t.Fatal(err)
			}
			restored = r.State
		}
		if restored.Display.BallCHR != 13 || restored.Display.Visible.BallCHR != 13 || restored.Display.Visible.Result == nil {
			t.Fatal("result display did not preserve/publish initialized CHR", restored.Display.BallCHR, restored.Display.Visible.BallCHR)
		}
	}
}
