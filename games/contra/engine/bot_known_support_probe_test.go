package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// An unknown leading-foot cell must not trap a stationary actor forever when
// there is still visible support ahead of its center. Keep several pixels of
// known support for presentation lag; never cross into the unknown cell.
func TestBotApproachesUnknownEdgeOnlyWithinKnownSupport(t *testing.T) {
	f, e := os.Open("testdata/bot-energy-top-unknown-front.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var base BotObservation
	if e = json.NewDecoder(z).Decode(&base); e != nil {
		t.Fatal(e)
	}
	// Isolate the short probe: the separate shaft test covers a known
	// lower landing, which legitimately permits a controlled drop.
	for x := range base.Terrain[13] {
		base.Terrain[13][x] = TerrainEmpty
		base.TerrainKnown[13][x] = false
	}
	for _, advance := range []int{0, 3} {
		o := base
		o.Players[0].X += advance
		b := Bot{LastTick: o.Tick - 1, LastX: o.Players[0].X, LastY: o.Players[0].Y, LastViewX: o.ViewX, LastViewY: o.ViewY, Phase: "battle", Stage: 5, StageKnown: true, GridKnown: true}
		pad := b.Decide(o)
		if advance == 0 && pad&ButtonRight == 0 {
			t.Fatal("known support cannot be approached to reveal hidden edge", pad)
		}
		if advance != 0 && pad&(ButtonRight|ButtonA) != 0 {
			t.Fatal("probe leaves confirmed support", pad)
		}
	}
}
