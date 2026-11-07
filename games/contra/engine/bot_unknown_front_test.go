package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Before unknown-front handling: partial player pixels obscure one supporting
// cell, which becomes a fabricated cliff and repeated backward jumps. Wait on
// that uncertain edge, then resume when current pixels disclose the floor.
func TestBotUnknownFrontDoesNotInventCliff(t *testing.T) {
	f, err := os.Open("testdata/bot-energy-unknown-front.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var observations []BotObservation
	if err = json.NewDecoder(z).Decode(&observations); err != nil {
		t.Fatal(err)
	}
	b := Bot{LastTick: 91855, LastX: 35, LastY: 196, LastViewX: 8, Phase: "battle", Stage: 5, StageKnown: true, SceneKnown: true, GridKnown: true, ViewStillTicks: 1000}
	b.PlayerMemory[0] = BotPlayer{X: 35, Y: 196, Sprite: 14, Visible: true, Alive: true}
	b.PlayerSeenAt[0] = 91854
	pad := b.Decide(observations[0])
	if pad&(ButtonA|ButtonLeft|ButtonRight) != 0 {
		t.Fatal("unknown patch became an invented jump or blind movement", pad)
	}
	pad = b.Decide(observations[1])
	if pad&ButtonRight == 0 || pad&ButtonA != 0 {
		t.Fatal("disclosed floor did not release uncertainty hold", pad)
	}
}
