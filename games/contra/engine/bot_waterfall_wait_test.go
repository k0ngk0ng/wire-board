package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Failure cases: the lower actor never approaches a higher visible ledge;
// the upper actor abandons its waiting position before its peer catches up.
// Memory was reconstructed with exact equality of all 38,399 preceding pads.
func TestBotWaterfallLowerPeerMakesProgress(t *testing.T) {
	f, err := os.Open("testdata/bot-coop-waterfall-wait.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture struct {
		Observation BotObservation
		Bots        [2]Bot
	}
	if err := json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	for actor := 0; actor < 2; actor++ {
		o, b := fixture.Observation, fixture.Bots[actor]
		o.Actor = actor
		probe := b
		known := probe.publicMemory(o)
		x, y, platform, found := probe.planJump(known, true)
		approach, hasApproach := botClimbApproach(known, known.Players[actor].X, known.Players[actor].Y+16)
		t.Logf("actor%d jump=%d,%d platform=%d found=%v approach=%d,%v", actor, x, y, platform, found, approach, hasApproach)
		pad := b.Decide(o)
		if actor == 0 && (pad&ButtonA == 0 || pad&ButtonLeft == 0) {
			t.Errorf("lower actor does not jump toward the visible lower ledge: %v", pad)
		}
		if actor == 1 && pad&ButtonA != 0 {
			t.Errorf("upper actor jumps away from waiting peer: %v", pad)
		}
		if actor == 0 {
			fresh := fixture.Bots[0]
			fresh.StillTicks = 0
			if fresh.Decide(o)&ButtonA != 0 {
				t.Error("fresh observation immediately triggers a detour")
			}
			missing := fixture.Bots[0]
			for row := range o.Terrain {
				if row*16-int(o.ViewY&15) >= 200 {
					o.Terrain[row] = [17]TerrainKind{}
				}
			}
			if missing.Decide(o)&ButtonA != 0 {
				t.Error("detour jumps without a visible lower landing")
			}
		}
	}
}
