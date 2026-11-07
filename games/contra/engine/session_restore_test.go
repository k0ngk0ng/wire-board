package contra

// Before exposing restore to the native/Wasm bridge: reject inconsistent
// versions/clocks, in-flight campaign work and impossible final markers without
// advancing a frame; accept both a fresh session and a saved live session.
import (
	"encoding/json"
	"testing"
)

func TestSessionCheckpointValidation(t *testing.T) {
	s, e := NewSession(7, 2, "restore-epoch")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Validate(); e != nil {
		t.Fatal("fresh", e)
	}
	if _, e = s.StepPads(1, [2]Buttons{}, levelResources(t, "stage-1.json")); e != nil {
		t.Fatal(e)
	}
	if e = s.Validate(); e != nil {
		t.Fatal("live", e)
	}
	for _, change := range []func(*Session){
		func(v *Session) { v.Version++ }, func(v *Session) { v.Audio.Epoch = "" },
		func(v *Session) { v.Random.Entropy++ }, func(v *Session) { v.Audio.Tick++ },
		func(v *Session) { v.World.Campaign.Pending = CampaignCombat },
		func(v *Session) { v.Finalized = true }, func(v *Session) { v.FinalFlushTicks = 6 },
		func(v *Session) { v.World.Active.Presentation.Queue.Used++ },
	} {
		copy := *s
		change(&copy)
		before, _ := json.Marshal(copy)
		if e = copy.Validate(); e == nil {
			t.Fatal("invalid checkpoint accepted")
		}
		after, _ := json.Marshal(copy)
		if string(before) != string(after) {
			t.Fatal("validation mutated checkpoint")
		}
	}
}
