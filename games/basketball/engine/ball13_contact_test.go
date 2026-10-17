package engine

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Failure inventory: assuming geometry arrays end with adjacent arrays; giving
// ball13 cursor2 the visible ball box; losing cursor4's disabled box; widening
// arbitrary other actions/cursors; changing the world cursor to force lookup.
func TestOriginalBall13ContactGeometry(t *testing.T) {
	var rows []struct {
		Action, Frame, Flags uint8
		Shape                [8]uint8
	}
	data, e := os.ReadFile("testdata/ball13-contact.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 12 {
		t.Fatal("incomplete original contact oracle")
	}
	for _, r := range rows {
		got, e := OriginalContactProfile(ContactProfileKey{Kind: 8, Action: r.Action, Frame: r.Frame})
		want := observedContactProfile(observedContactObject{Shape: r.Shape, AttackFlags: r.Flags, ReceiveFlags: r.Flags})
		if e != nil || got != want {
			t.Fatal(r, got, want, e)
		}
	}
	for _, frame := range []uint8{3, 7, 255} {
		if _, e := OriginalContactProfile(ContactProfileKey{Kind: 8, Action: 13, Frame: frame}); e == nil {
			t.Fatal("accepted unreachable cursor", frame)
		}
	}
}
func TestOriginalBall13ContactRuntimeContinuation(t *testing.T) {
	var s RuntimeState
	readPropAttackFixture(t, "testdata/ball13-contact-runtime-before.json.gz", &s)
	if s.VideoFrames != 13067 {
		t.Fatal("wrong original failure", s.VideoFrames)
	}
	for frame := 0; frame < 360; frame++ {
		data, _ := json.Marshal(s)
		var restored RuntimeState
		if e := json.Unmarshal(data, &restored); e != nil {
			t.Fatal(e)
		}
		a, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(frame, s.VideoFrames, e)
		}
		b, e := StepVideo(restored, [4]Buttons{})
		if e != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("restore differs", frame, e)
		}
		s = a.State
	}
}
