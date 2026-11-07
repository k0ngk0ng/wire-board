package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Holding Down cannot itself move the actor off the known platform. Losing
// the body or support to visual occlusion must not release that safe input;
// a newly visible jumping pose must resume normal movement decisions.
func TestBotCrouchIntentSurvivesVisibleBodyOcclusion(t *testing.T) {
	f, e := os.Open("testdata/bot-energy-crouch-occlusion.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []BotObservation
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	b := Bot{LastTick: 108036, LastX: 89, LastY: 196, LastViewX: 8, LastPad: ButtonDown | ButtonB, Phase: "battle", Stage: 5, StageKnown: true, GridKnown: true}
	b.PlayerMemory[0] = BotPlayer{X: 89, Y: 196, Sprite: 23, Visible: true, Alive: true, LivesKnown: true, Lives: 10}
	b.PlayerSeenAt[0] = 108035
	for _, o := range rows {
		if pad := b.Decide(o); pad&ButtonDown == 0 || pad&(ButtonLeft|ButtonRight|ButtonA) != 0 {
			t.Fatal("occlusion released crouch", o.Tick, pad)
		}
	}
	o := rows[len(rows)-1]
	o.Tick++
	o.Players[0] = BotPlayer{X: 89, Y: 190, Sprite: 8, Visible: true, Alive: true}
	if pad := b.Decide(o); pad&ButtonDown != 0 {
		t.Fatal("new visible jumping pose ignored", pad)
	}
}

// Explicit manual control may change the actor between policy calls; a stale
// held input from an earlier assistance interval must not be resumed blindly.
func TestBotNozzleCrouchDoesNotSurviveControlGap(t *testing.T) {
	o := BotObservation{Tick: 100, Actor: 0, Seats: 1, Phase: "battle", Enemies: []BotEnemy{{Family: "machine-flame-nozzle-left", X: 140, Y: 136}}}
	b := Bot{LastTick: 50, Phase: "battle", LastPad: ButtonDown}
	if b.holdOccludedNozzleCrouch(o) {
		t.Fatal("stale crouch carried across manual control gap")
	}
}
