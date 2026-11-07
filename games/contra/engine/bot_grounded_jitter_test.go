package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Actual current-frame pattern alignment can move a crouched body's anchor
// by one pixel. Stable visible support and a non-jumping pose must keep Down;
// a real jumping pose must not be classified as grounded by this tolerance.
func TestBotGroundedCrouchSurvivesOnePixelVisualJitter(t *testing.T) {
	f, e := os.Open("testdata/bot-energy-duck-jitter.json.gz")
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
	b := Bot{LastTick: 104208, LastX: 91, LastY: 196, LastViewX: 8, LastPad: ButtonDown | ButtonB, Phase: "battle", Stage: 5, StageKnown: true, GridKnown: true}
	for _, o := range rows {
		if pad := b.Decide(o); pad&ButtonDown == 0 || pad&(ButtonLeft|ButtonRight|ButtonA) != 0 {
			t.Fatal("safe crouch released after visual anchor jitter", o.Tick, pad)
		}
	}
	o := rows[1]
	o.Tick++
	o.Players[0].Sprite = 8
	if pad := b.Decide(o); pad&ButtonDown != 0 {
		t.Fatal("jumping pose treated as grounded", pad)
	}
}
