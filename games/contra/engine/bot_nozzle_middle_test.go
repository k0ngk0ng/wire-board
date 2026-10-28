package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Actual middle-platform input: no hidden World state is included. The next
// move must wait safely or ascend, rather than walking back toward the muzzle.
func TestBotRecordedMiddlePlatformDecision(t *testing.T) {
	f, e := os.Open("testdata/bot-energy-middle-decision.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var x struct {
		Bot         Bot
		Observation BotObservation
	}
	if e = json.NewDecoder(z).Decode(&x); e != nil {
		t.Fatal(e)
	}
	b := x.Bot
	o := b.publicMemory(x.Observation)
	b.trackVisualEntities(&o)
	gx, gy, _, found := b.planJump(o, false)
	t.Log("remembered nozzle ahead", botNozzleLandingRisk(b.nozzleHazardView(o), o.Players[0].X+16, o.Players[0].Y))
	t.Log("landing", gx, gy, found, "current", o.Players[o.Actor], "nozzle ahead", botNozzleLandingRisk(o, o.Players[0].X+16, o.Players[0].Y))
	if pad := x.Bot.Decide(x.Observation); pad&(ButtonA|ButtonDown) == 0 {
		t.Fatal("walks toward nozzle instead of waiting or ascending", pad)
	}
}
