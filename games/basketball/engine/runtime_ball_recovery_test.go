package engine

// Before runtime wiring: new matches must enable the user-approved repair;
// old missing-policy saves must keep original behavior; context tampering must
// fail restore, and the option must survive an actual opening/world handoff.
import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRuntimeBallRecoveryPolicyAndLegacyRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	if !c.RecoverUnsupportedBall {
		t.Fatal("new games must enable the approved repair")
	}
	s, e := NewRuntime(c)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 600 && s.Playing == nil; i++ {
		next, err := StepVideo(s, [4]Buttons{})
		if err != nil {
			t.Fatal(i, err)
		}
		s = next.State
	}
	if s.Playing == nil || !s.Playing.Context.Actions.RecoverUnsupportedBall {
		t.Fatal("opening lost repair policy")
	}
	encoded, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var restored RuntimeState
	if e = json.Unmarshal(encoded, &restored); e != nil {
		t.Fatal(e)
	}
	if e = ValidateRuntimeState(restored); e != nil {
		t.Fatal(e)
	}
	if !restored.Config.RecoverUnsupportedBall || !restored.Playing.Context.Actions.RecoverUnsupportedBall {
		t.Fatal("restore lost repair policy")
	}
	restored.Playing.Context.Actions.RecoverUnsupportedBall = false
	if e = ValidateRuntimeState(restored); e == nil {
		t.Fatal("accepted altered playing policy")
	}
	restored.Config.RecoverUnsupportedBall = false
	encoded, e = json.Marshal(restored)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(encoded, []byte("recoverUnsupportedBall")) {
		t.Fatal("legacy zero policy must stay omitted")
	}
	var legacy RuntimeState
	if e = json.Unmarshal(encoded, &legacy); e != nil {
		t.Fatal(e)
	}
	if e = ValidateRuntimeState(legacy); e != nil {
		t.Fatal("legacy save rejected", e)
	}
	if legacy.Config.RecoverUnsupportedBall || legacy.Playing.Context.Actions.RecoverUnsupportedBall {
		t.Fatal("missing policy changed original behavior")
	}
}
