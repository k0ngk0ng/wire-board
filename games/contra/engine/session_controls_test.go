package contra

// Before platform pad sampling: derive edges on the authoritative frame, keep
// pads separate in co-op, restore held buttons, and do not consume a changed
// input on a duplicate frame. Exercise actual movement and shooting from a new
// standard session rather than injecting a player state.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSessionNativePadsDriveAndRestore(t *testing.T) {
	resources := levelResources(t, "stage-1.json")
	s, e := NewSession(42, 2, "pad-session")
	if e != nil {
		t.Fatal(e)
	}
	var previous [2]Buttons
	shotFrames := 0
	for frame := uint64(1); frame <= 160; frame++ {
		var pads [2]Buttons
		if frame == 2 {
			pads[0] = ButtonStart
		}
		if frame >= 60 {
			pads[0] = ButtonRight | ButtonB
			pads[1] = ButtonB
			if frame%45 == 0 {
				pads[0] |= ButtonA
			}
		}
		before, _ := json.Marshal(s)
		var restored Session
		if e = json.Unmarshal(before, &restored); e != nil {
			t.Fatal(e)
		}
		one, e := s.StepPads(frame, pads, resources)
		if e != nil {
			t.Fatal(frame, e)
		}
		two, e := restored.StepPads(frame, pads, resources)
		if e != nil || !reflect.DeepEqual(one, two) || !reflect.DeepEqual(*s, restored) {
			t.Fatal(frame, "pad restore diverged", e)
		}
		for i := range pads {
			if s.Controls.Held[i] != pads[i] || s.Controls.Pressed[i] != pads[i]&^previous[i] {
				t.Fatal(frame, "source edges differ", s.Controls)
			}
		}
		previous = pads
		for _, command := range s.Audio.Pending {
			if command.Kind == "sound" && command.ID == 0x1a {
				shotFrames++
			}
		}
		state, _ := json.Marshal(s)
		if _, e = s.StepPads(frame, [2]Buttons{ButtonLeft, ButtonA}, resources); e != nil {
			t.Fatal(e)
		}
		after, _ := json.Marshal(s)
		if string(state) != string(after) {
			t.Fatal("retry changed held input")
		}
	}
	if shotFrames != 2 {
		t.Fatal("held normal gun manufactured extra edges", shotFrames)
	}
	if s.World.Active.Combat.Players[0].X == s.World.Active.Combat.Players[1].X {
		t.Fatal("separate co-op movement did not affect real actors")
	}
}
