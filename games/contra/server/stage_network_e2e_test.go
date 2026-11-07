package contra

// Failure inventory before implementation: an unready/disconnected peer stops
// the running clock; ready seats lose inputs when another seat finishes loading;
// loading inputs leak into replay; late ready resets time; restore reintroduces
// a global gate; a ready seat cannot pause while its peer is loading.
// Uses a declared stage-transition fixture and real module callbacks, not a
// natural campaign victory. The initial platform loading barrier is unchanged.
import (
	"encoding/json"
	"testing"
	"time"

	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
)

func TestContraRunningStageSlowPeerDoesNotBlockReadySeat(t *testing.T) {
	m, c, g := initial(t, 2)
	g = ticks(t, m, &c, boundary(t, g, 0), 50)
	ready := func(actor int) {
		c.Actor = actor
		v := wire(t, m, g, actor)
		g = action(t, m, c, g, map[string]any{"type": "stage-ready", "epoch": v.Epoch, "sequence": v.Inputs[actor].Sequence + 1, "level": v.Stage.Level, "bundleID": v.Bundle.ID, "rulesBuildID": v.RulesBuildID, "runtimeBuildID": v.RuntimeBuildID, "stageSHA256": v.Stage.SHA256})
	}
	ready(0)
	start := saved(t, g).Session.Random.Frames
	for n := 0; n < 60; n++ {
		c.Actor = 0
		g = input(t, m, c, g, engine.ButtonRight|engine.ButtonB)
		c.Actor = 1
		g = input(t, m, c, g, engine.ButtonLeft|engine.ButtonA)
		g = ticks(t, m, &c, g, 50)
		s := saved(t, g)
		if s.Session.Random.Frames != start+uint64((n+1)*3) || s.StageLoading == nil || !s.StageLoading.Ready[0] || s.StageLoading.Ready[1] {
			t.Fatal("slow peer stopped another player's authoritative frames", n)
		}
		if s.Frames[len(s.Frames)-1].Pads != [2]engine.Buttons{engine.ButtonRight | engine.ButtonB, 0} || s.Inputs[1].Held != 0 || s.Inputs[1].Automated {
			t.Fatal("loading peer leaked input or was automatically taken over")
		}
	}
	before := saved(t, g)
	// Ready arrives between ticks. It must neither erase elapsed time nor clear
	// the ready player's held buttons when the last loading flag is removed.
	c.Now = c.Now.Add(25 * time.Millisecond)
	ready(1)
	after := saved(t, g)
	if after.StageLoading != nil || after.ServerAt != before.ServerAt || after.Remainder != before.Remainder || after.Inputs[0].Held != before.Inputs[0].Held {
		t.Fatal("late readiness changed another player's controls or clock")
	}
	c.Actor = 1
	g = input(t, m, c, g, engine.ButtonB)
	g = ticks(t, m, &c, g, 25)
	after = saved(t, g)
	if after.Session.Random.Frames != before.Session.Random.Frames+3 || after.Frames[len(after.Frames)-1].Pads != [2]engine.Buttons{engine.ButtonRight | engine.ButtonB, engine.ButtonB} {
		t.Fatal("recovered player could not rejoin the uninterrupted timeline")
	}
	encoded, _ := json.Marshal(map[string]any{"passed": true, "slowPeerElapsedMS": 3000, "fastPeerFrames": before.Session.Random.Frames - start, "lateReadyPreservedClock": true, "automaticTakeover": false})
	t.Log(string(encoded))
}

func TestContraRunningStageAllowsReadyPlayerPauseAndRestore(t *testing.T) {
	m, c, g := initial(t, 2)
	g = ticks(t, m, &c, boundary(t, g, 0), 50)
	v := wire(t, m, g, 0)
	g = action(t, m, c, g, map[string]any{"type": "stage-ready", "epoch": v.Epoch, "sequence": 1, "level": v.Stage.Level, "bundleID": v.Bundle.ID, "rulesBuildID": v.RulesBuildID, "runtimeBuildID": v.RuntimeBuildID, "stageSHA256": v.Stage.SHA256})
	g = ticks(t, m, &c, g, 50)
	g = input(t, m, c, g, engine.ButtonStart)
	g = ticks(t, m, &c, g, 50)
	g = input(t, m, c, g, 0)
	for n := 0; n < 100 && saved(t, g).Session.World.Active.Combat.World.LevelRoutine != 4; n++ {
		g = ticks(t, m, &c, g, 50)
	}
	if saved(t, g).Session.World.Active.Combat.World.LevelRoutine != 4 {
		t.Fatal("unready peer prevented the stage from reaching play")
	}
	g = input(t, m, c, g, engine.ButtonStart)
	g = ticks(t, m, &c, g, 50)
	if !g.Paused || saved(t, g).StageLoading == nil {
		t.Fatal("ready player cannot explicitly pause while peer loads")
	}
	restored, err := m.Restore(c, g)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Paused || saved(t, restored).Session.Random.Frames != saved(t, g).Session.Random.Frames {
		t.Fatal("restore changed the explicit pause or timeline")
	}
}
