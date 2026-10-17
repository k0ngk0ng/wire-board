package engine

// Failure cases observed before the fix: action68/variant63 legitimately
// reaches cursors3/4 after A release, beyond its normal three-frame animation;
// contact lookup stops the entire game. Do not shorten that animation, make the
// player invulnerable, accept arbitrary cursors, or widen other attack domains.
import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalRunningReleaseContactTail(t *testing.T) {
	raw, e := os.ReadFile("testdata/running-release-contact.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Kind, Action, Variant, Frame, Flags uint8
		Shape                               [8]uint8
	}
	if e = json.Unmarshal(raw, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 40 {
		t.Fatal("missing original geometry calls")
	}
	for _, r := range rows {
		want := observedContactProfile(observedContactObject{Shape: r.Shape, AttackFlags: r.Flags, ReceiveFlags: r.Flags})
		got, e := OriginalContactProfile(ContactProfileKey{Kind: r.Kind, Action: r.Action | 128, Variant: r.Variant, Frame: r.Frame})
		if e != nil || got != want {
			t.Fatalf("kind%d frame%d got%+v want%+v err%v", r.Kind, r.Frame, got, want, e)
		}
	}
	for _, key := range []ContactProfileKey{{Kind: 0, Action: 104, Variant: 63, Frame: 5}, {Kind: 0, Action: 103, Variant: 63, Frame: 3}, {Kind: 0, Action: 104, Variant: 255}} {
		if _, e = OriginalContactProfile(key); e == nil {
			t.Fatal("unknown contact domain accepted", key)
		}
	}
}
func TestRuntimeNativeRunningReleasePrestateContinues(t *testing.T) {
	raw, e := os.ReadFile("testdata/running-release-browser-state.json")
	if e != nil {
		t.Fatal(e)
	}
	var state RuntimeState
	if e = json.Unmarshal(raw, &state); e != nil {
		t.Fatal(e)
	}
	initial := state.Playing.World.Preparation.Actions.Actors[0]
	if state.VideoFrames != 560 || initial.Player.Action != 104 || initial.AttackVariant != 63 || initial.Player.Animation.Cursor != 3 {
		t.Fatal("native browser failure before-state changed")
	}
	sawRelease := false
	for tick := 0; tick < 100; tick++ {
		held := [4]Buttons{}
		if tick < 5 {
			held[0] = ButtonA
		}
		next, e := StepVideo(state, held)
		if e != nil {
			t.Fatal("native release continuation", tick, e)
		}
		checkpoint, _ := json.Marshal(state)
		var restored RuntimeState
		_ = json.Unmarshal(checkpoint, &restored)
		replay, e := StepVideo(restored, held)
		if e != nil || !reflect.DeepEqual(next, replay) {
			t.Fatal("release replay divergence", tick, e)
		}
		state = next.State
		aux := state.Playing.World.Preparation.Actions.Auxiliary[0]
		sawRelease = sawRelease || aux.Action == 11 && aux.Owner == -1
	}
	if !sawRelease {
		t.Fatal("release never advanced original flying object")
	}
}
