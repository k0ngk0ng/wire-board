package contra

// Before visible carrier aiming: a boss behind and above the player must not
// be ignored by an always-right traversal policy. Only a positively recognized
// visual family may select this behavior, never a reused logical Kind number.
import "testing"

func carrierAimObservation(x int, family string) BotObservation {
	o := BotObservation{Tick: 4, Actor: 0, Seats: 1, Phase: "battle", Stage: 4, StageKnown: true, SceneKnown: true}
	o.Players[0] = BotPlayer{X: x, Y: 164, Alive: true, Visible: true, Sprite: 13}
	o.Enemies = []BotEnemy{{X: 128, Y: 80, Kind: 20, Family: family}}
	return o
}

func TestBotVisibleCarrierBehindPlayerGetsAlignedFire(t *testing.T) {
	var b Bot
	if pad := b.Decide(carrierAimObservation(160, "snow-carrier")); pad&(ButtonLeft|ButtonUp|ButtonB) != ButtonLeft|ButtonUp|ButtonB || pad&ButtonRight != 0 {
		t.Fatal("visible carrier behind the player was ignored", pad)
	}
}

func TestBotVisibleCarrierAlignedKeepsVerticalFire(t *testing.T) {
	var b Bot
	if pad := b.Decide(carrierAimObservation(128, "snow-carrier")); pad != ButtonUp|ButtonB {
		t.Fatal("aligned carrier firing walked away or fired sideways", pad)
	}
}

func TestBotCarrierBehaviorDoesNotUseBareLogicalKind(t *testing.T) {
	var b Bot
	if pad := b.Decide(carrierAimObservation(160, "")); pad&ButtonLeft != 0 {
		t.Fatal("reused numeric kind became a hidden carrier identity", pad)
	}
}
