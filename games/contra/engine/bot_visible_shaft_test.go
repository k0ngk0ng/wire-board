package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// The Energy shaft has an unknown dark opening and a visible lower floor.
// A drop predictor must first leave the current platform, respect its walls,
// and require a real lower landing rather than falling through existing floor.
func TestBotVisibleShaftAllowsDropToKnownFloor(t *testing.T) {
	f, e := os.Open("testdata/bot-energy-visible-shaft.json.gz")
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
	b := Bot{LastTick: o.Tick - 1, LastX: 128, LastY: 52, LastViewX: o.ViewX, Phase: "battle", Stage: 5, StageKnown: true, GridKnown: true}
	if pad := b.Decide(o); pad&ButtonRight == 0 || pad&ButtonA != 0 {
		t.Fatal("known lower floor cannot be reached through visible shaft", pad)
	}
}
func TestBotDropCannotFallThroughCurrentPlatform(t *testing.T) {
	o := botNavigationFloor(1, 73)
	for x := range o.Terrain[5] {
		o.Terrain[5][x] = TerrainPlatform
		o.Terrain[9][x] = TerrainPlatform
	}
	var b Bot
	if b.safeDrop(o) {
		t.Fatal("drop predictor fell through a continuous supporting platform")
	}
}
