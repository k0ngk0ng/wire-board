package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Lair wall fill is deliberately ambiguous in the original artwork, while
// the elevated red platform tops are identifiable. Unknown fill must neither
// become a fabricated solid wall nor forbid a jump to a known forward top.
func TestBotCanJumpToKnownTopAcrossUnknownFill(t *testing.T) {
	f, e := os.Open("testdata/bot-alien-known-top.json.gz")
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
	if o.terrainKnownAt(p.X+8, p.Y+16) || !o.terrainKnownAt(144, 160) || o.terrainAt(144, 160) != TerrainPlatform {
		t.Fatal("fixture lost ambiguous fill / known top distinction")
	}
	b := Bot{LastTick: o.Tick - 1, LastX: p.X, LastY: p.Y, LastViewX: o.ViewX, LastViewY: o.ViewY, Phase: "battle", Stage: 7, StageKnown: true, GridKnown: true, ViewStillTicks: 2240}
	pad := b.Decide(o)
	if pad&ButtonA == 0 || pad&ButtonRight == 0 {
		t.Fatal("known higher platform blocked by unknown fill", pad)
	}
	if o.terrainKnownAt(144, 176) {
		t.Fatal("navigation invented hidden wall material")
	}
}
