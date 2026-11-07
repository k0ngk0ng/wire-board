package contra

// Before the ambiguous-water fallback: fresh assistance must not choose a
// candidate by order or silently invent a seat identity, but must be able to
// issue its own legal input until visible body colors identify the player.
// Pause/menu states and unrelated ambiguous artwork must not trigger probing.
import "testing"

func waterProbeObservation() BotObservation {
	o := BotObservation{Tick: 1, Actor: 0, Seats: 2, Phase: "battle"}
	o.PlayerCandidates = []BotPlayerCandidate{
		{Mask: 3, Player: BotPlayer{X: 70, Y: 200, Visible: true, Sprite: 25}},
		{Mask: 3, Player: BotPlayer{X: 120, Y: 200, Visible: true, Sprite: 25}},
	}
	return o
}

func TestBotUnknownWaterHeadsProbeWithoutAssigningIdentity(t *testing.T) {
	o := waterProbeObservation()
	for actor := 0; actor < 2; actor++ {
		o.Actor = actor
		var first, reversed Bot
		pad := first.Decide(o)
		r := o
		r.PlayerCandidates = []BotPlayerCandidate{o.PlayerCandidates[1], o.PlayerCandidates[0]}
		if pad&ButtonRight == 0 || pad&(ButtonLeft|ButtonStart|ButtonSelect|ButtonA) != 0 || pad != reversed.Decide(r) {
			t.Fatal("fresh assistance failed to issue an order-independent legal probe", actor, pad)
		}
		if first.PlayerSeenAt != ([2]uint64{}) || reversed.PlayerSeenAt != ([2]uint64{}) {
			t.Fatal("probe guessed an ambiguous head's seat")
		}
		o.Tick++
		o.Players[actor] = BotPlayer{X: 90, Y: 180, Visible: true, Alive: true, Sprite: 8}
		first.Decide(o)
		if first.PlayerSeenAt[actor] != o.Tick || first.PlayerMemory[actor].X != 90 {
			t.Fatal("visible body feedback failed to establish its actual seat")
		}
		o = waterProbeObservation()
	}
}

func TestBotUnknownWaterProbeRespectsPauseAndMenus(t *testing.T) {
	for _, state := range []string{"paused", "continue", "game-over", "unrelated"} {
		o := waterProbeObservation()
		switch state {
		case "paused":
			o.Paused, o.PausedKnown = true, true
		case "unrelated":
			for i := range o.PlayerCandidates {
				o.PlayerCandidates[i].Player.Sprite = 90
			}
		default:
			o.Phase = state
		}
		var b Bot
		if pad := b.Decide(o); pad != 0 {
			t.Fatal("probe ignored visible state", state, pad)
		}
	}
}

func TestBotUnknownSoloWaterHeadProbeDoesNotInventIdentity(t *testing.T) {
	o := waterProbeObservation()
	o.Seats = 1
	o.PlayerCandidates = o.PlayerCandidates[:1]
	o.PlayerCandidates[0].Mask = 1
	var b Bot
	if pad := b.Decide(o); pad&ButtonRight == 0 {
		t.Fatal("single-seat head candidate caused permanent inaction", pad)
	}
	if b.PlayerSeenAt != ([2]uint64{}) {
		t.Fatal("occupancy alone promoted a possibly non-player head")
	}
}
