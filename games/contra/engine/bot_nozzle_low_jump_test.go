package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// A committed A precedes the visible airborne pose. Re-evaluating a new
// standing launch in that interval must not erase the accepted landing.
func TestBotNozzlePendingJumpKeepsLanding(t *testing.T) {
	f, err := os.Open("testdata/bot-energy-pending-jump.json.gz")
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
		Bot         Bot
	}
	if err := json.NewDecoder(z).Decode(&data); err != nil {
		t.Fatal(err)
	}
	goal := data.Bot.JumpTargetX
	pad := data.Bot.Decide(data.Observation)
	if !data.Bot.JumpTargetActive || data.Bot.JumpTargetX != goal || pad&ButtonRight == 0 || pad&ButtonDown != 0 {
		t.Fatalf("pending launch lost landing: pad=%d goal=%d active=%v", pad, data.Bot.JumpTargetX, data.Bot.JumpTargetActive)
	}
}

// Failure cases: a distant low mouth blocks a jump to higher visible support
// forever; the exception incorrectly permits an active, close or higher beam.
func TestBotJumpsAboveDistantLowNozzle(t *testing.T) {
	f, err := os.Open("testdata/bot-energy-late-jump.json.gz")
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
		Bot         Bot
	}
	if err := json.NewDecoder(z).Decode(&data); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"low-distant", "active", "close", "higher"} {
		t.Run(mode, func(t *testing.T) {
			o, b := data.Observation, data.Bot
			o.Enemies = append([]BotEnemy(nil), o.Enemies...)
			if mode == "active" {
				o.Enemies = append(o.Enemies, BotEnemy{ID: 20, X: 150, Y: 200, Kind: 17, Family: "machine-flame"})
			}
			for i := range o.Enemies {
				if o.Enemies[i].Family == "machine-flame-nozzle-left" && o.Enemies[i].X == 200 {
					if mode == "close" {
						o.Enemies[i].X = 150
					}
					if mode == "higher" {
						o.Enemies[i].Y = 160
					}
				}
			}
			pad := b.Decide(o)
			if mode == "low-distant" {
				probe := data.Bot
				known := probe.publicMemory(o)
				x, y, _, found := probe.planJump(known, false)
				t.Logf("position=%+v landing=%d,%d found=%v opening=%v", known.Players[0], x, y, found, probe.nozzleJumpOpening(known, x))
				if pad&ButtonA == 0 || pad&ButtonRight == 0 {
					t.Fatalf("safe upward route remains blocked: %v", pad)
				}
			} else if pad&ButtonA != 0 {
				t.Fatalf("unsafe nozzle exception: %v", pad)
			}
		})
	}
}
