package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// A nozzle penalty must not permanently forbid the only visible forward
// platform. Keep preferring safe progress when available; otherwise plan the
// hazardous forward landing and let the visible firing-window gate wait.
func TestBotNozzlePenaltyDoesNotPreferEndlessSamePlaceJump(t *testing.T) {
	f, e := os.Open("testdata/bot-energy-crate-landing.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var o BotObservation
	if e = json.NewDecoder(z).Decode(&o); e != nil {
		t.Fatal(e)
	}
	p := o.Players[0]
	b := Bot{LastX: p.X, LastY: p.Y, ViewStillTicks: 1000}
	x, y, _, found := b.planJump(o, false)
	if !found || x <= p.X+12 || y >= p.Y-12 {
		t.Fatal("no progress despite a visible higher forward platform", x, y, found)
	}
	if pad := b.play(o); pad&ButtonA != 0 {
		t.Fatal("forward plan bypassed the active flame gate", pad)
	}
}
