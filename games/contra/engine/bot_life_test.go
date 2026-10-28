package contra

// Before borrowing support: only the visible per-seat GAME OVER message and
// publicly displayed teammate reserves may motivate an A-edge attempt. It
// must release between attempts, respect pause, and never infer elimination
// from zero reserves alone or write either player's life count.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func botGameOverObservation(t *testing.T) BotObservation {
	t.Helper()
	var o BotObservation
	err := json.Unmarshal([]byte(`{"Tick":1,"Actor":0,"Seats":2,"Phase":"battle","Players":[{"Lives":0,"LivesKnown":true,"AliveKnown":true,"Alive":false,"GameOverKnown":true,"GameOver":true},{"X":120,"Y":164,"Visible":true,"Alive":true,"Sprite":13,"Lives":1,"LivesKnown":true}]}`), &o)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestBotVisibleGameOverAttemptsLegalBorrow(t *testing.T) {
	o := botGameOverObservation(t)
	before := o
	var b Bot
	if pad := b.Decide(o); pad != ButtonA {
		t.Fatal("visible eliminated seat did not try the legal borrow button", pad)
	}
	if !reflect.DeepEqual(o, before) {
		t.Fatal("borrowing changed public life values")
	}
	o.Tick = 2
	if pad := b.Decide(o); pad != 0 {
		t.Fatal("borrow attempt did not release its edge", pad)
	}
}

func TestBotBorrowRequiresVisibleEligibilityAndUnpausedPlay(t *testing.T) {
	for _, reason := range []string{"unknown-reserves", "zero-reserves", "paused", "no-seat-message"} {
		o := botGameOverObservation(t)
		switch reason {
		case "unknown-reserves":
			o.Players[1].LivesKnown = false
		case "zero-reserves":
			o.Players[1].Lives = 0
		case "paused":
			o.Paused, o.PausedKnown = true, true
		case "no-seat-message":
			o.Players[0] = BotPlayer{LivesKnown: true, Lives: 0}
		}
		var b Bot
		if pad := b.Decide(o); pad&ButtonA != 0 {
			t.Fatal("borrow was inferred without visible eligibility", reason, pad)
		}
	}
}

func TestBotObservedLastLifeDeathRetainsBorrowAttempt(t *testing.T) {
	for _, sprite := range []uint8{10, 11, 12, 86} {
		o := botGameOverObservation(t)
		o.Players[0] = BotPlayer{X: 60, Y: 180, Visible: true, Alive: true, Sprite: sprite, LivesKnown: true, Lives: 0}
		var b Bot
		if pad := b.Decide(o); pad != ButtonA {
			t.Fatal("visible last-life death did not permit a legal borrow attempt", sprite, pad)
		}
		o.Tick = 2
		o.Players[0] = BotPlayer{LivesKnown: true, Lives: 0}
		b.Decide(o)
		o.Tick = 3
		if pad := b.Decide(o); pad != ButtonA {
			t.Fatal("known death was forgotten when the body disappeared", sprite, pad)
		}
		o.Tick = 4
		o.Players[0] = BotPlayer{X: 48, Y: 32, Visible: true, Alive: true, Sprite: 8, LivesKnown: true, Lives: 0}
		b.Decide(o)
		o.Tick = 5
		if pad := b.Decide(o); pad&ButtonA != 0 {
			t.Fatal("borrow attempts continued after visible revival", sprite, pad)
		}
	}
}

func TestBotUnknownLifeAndNonfatalPosesDoNotEstablishBorrow(t *testing.T) {
	for _, sprite := range []uint8{10, 84, 85} {
		o := botGameOverObservation(t)
		o.Players[0] = BotPlayer{X: 60, Y: 180, Visible: true, Alive: true, Sprite: sprite, LivesKnown: sprite != 10, Lives: 0}
		var b Bot
		if pad := b.Decide(o); pad&ButtonA != 0 {
			t.Fatal("unknown lives/crouch/shock were treated as elimination", sprite, pad)
		}
	}
}
