package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Failure cases: prolonged overlap must not deadlock; fresh uncertainty and
// absence of a visible landing must not cause blind movement. The fixture
// contains only submitted visual observations and public bot memory.
func TestBotOverlapUnknownSupportRecovery(t *testing.T) {
	f, err := os.Open("testdata/bot-coop-overlap.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var data struct {
		Observation BotObservation
		Bots        [2]Bot
	}
	if err := json.NewDecoder(z).Decode(&data); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"stalled", "fresh", "no-landing"} {
		t.Run(mode, func(t *testing.T) {
			o, b := data.Observation, data.Bots[1]
			o.Actor = 1
			if mode == "fresh" {
				b.ViewStillTicks = 0
				b.StillTicks = 0
			}
			if mode == "no-landing" {
				// Retain pose and identity but remove all observed terrain.
				var empty BotObservation
				o.Terrain = empty.Terrain
			}
			pad := b.Decide(o)
			if mode == "stalled" {
				if pad&ButtonA == 0 || pad&ButtonRight == 0 {
					t.Fatalf("overlapping actors remain stuck: %v", pad)
				}
			} else if pad&(ButtonA|ButtonRight|ButtonLeft) != 0 {
				t.Fatalf("unsafe movement: %v", pad)
			}
		})
	}
}
