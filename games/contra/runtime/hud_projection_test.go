package runtime

// Pending rule state must not overwrite the lives paired with applied pixels,
// including a checkpoint restored in between rule production and display commit.
import (
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"testing"
)

func TestRuntimeViewUsesSubmittedLives(t *testing.T) {
	s, err := engine.NewSession(42, 1, "hud-view")
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := engine.BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	s.World.Active.Combat.Players[0].Lives = 1 // isolated pending rule change
	code, resource := testRulesBuild(t), bundle.Manifest().ID
	v, err := View(s, resource, code)
	if err != nil {
		t.Fatal(err)
	}
	if v.Players[0].Lives != 2 {
		t.Fatal("runtime leaked pending rule lives", v.Players[0].Lives)
	}
	c, err := CheckpointOf(s, resource, code)
	if err != nil {
		t.Fatal(err)
	}
	if c.State.PresentedLives != s.PresentedLives {
		t.Fatal("checkpoint omitted displayed lives")
	}
	restored, err := View(c.State, resource, code)
	if err != nil || restored.Players[0].Lives != v.Players[0].Lives {
		t.Fatal("checkpoint HUD differs", err)
	}
}
