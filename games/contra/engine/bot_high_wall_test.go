package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// The top of this visible column cannot be reached from the current ledge.
// A head above the wall does not make a body still inside its side a landing.
func TestBotCannotLandBodyInsideVisibleHighWall(t *testing.T) {
	f, e := os.Open("testdata/bot-energy-high-wall.json.gz")
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
	b := Bot{LastX: o.Players[0].X, LastY: o.Players[0].Y, ViewStillTicks: 1000}
	x, y, _, found := b.planJump(o, false)
	if found && o.terrainAt(x, y) == TerrainSolid {
		t.Fatal("predicted landing body remains inside visible wall", x, y)
	}
}
