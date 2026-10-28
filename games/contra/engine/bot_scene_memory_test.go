package contra

import "testing"

// Failure cases before the repair: a temporarily unclassified frame sequence
// erases a disclosed indoor scene; a later visible body then walks sideways
// instead of through the open door. Conversely a new visible stage title must
// invalidate the previous scene, even if its own scene field is unknown.
func TestBotIndoorSceneSurvivesUnclassifiedFrames(t *testing.T) {
	var b Bot
	o := botBaseObservation(1)
	b.Decide(o)
	for tick := uint64(2); tick <= 25; tick++ {
		b.Decide(BotObservation{Tick: tick, Actor: 0, Seats: 1})
	}
	if !b.SceneKnown || b.Scene != 1 {
		t.Fatal("lack of recognizable sprites erased the disclosed indoor scene", b.SceneKnown, b.Scene)
	}
	o = BotObservation{Tick: 26, Actor: 0, Seats: 1, Phase: "battle"}
	o.Players[0] = BotPlayer{X: 128, Y: 168, Sprite: 81, Visible: true, Alive: true}
	if pad := b.Decide(o); pad&ButtonUp == 0 || pad&(ButtonLeft|ButtonRight) != 0 {
		t.Fatal("open indoor lane became sideways outdoor movement", pad)
	}
}

func TestBotVisibleStageChangeInvalidatesOldScene(t *testing.T) {
	var b Bot
	b.Decide(botBaseObservation(1))
	b.Decide(BotObservation{Tick: 2, Actor: 0, Seats: 1, Phase: "intro", StageKnown: true, Stage: 2})
	if b.SceneKnown {
		t.Fatal("new disclosed stage retained the old indoor scene")
	}
}
