package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Visible overhead claws should not induce a jump off a flat floor. The
// bounded walking probe may bridge obscured pixels between visible supports,
// but must never reinterpret an explicitly visible gap as supporting ground.
func TestBotWalksBelowVisibleClaws(t *testing.T) {
	f, err := os.Open("testdata/bot-hangar-claw-ground.json.gz")
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
	o, b := data.Observation, data.Bot
	if pad := b.Decide(o); pad&ButtonA != 0 || pad&ButtonRight == 0 {
		t.Fatalf("jump under visible claws: %v", pad)
	}
	running, runningBot := o, data.Bot
	running.Players[0].Sprite = 2
	if pad := runningBot.Decide(running); pad&ButtonA != 0 || pad&ButtonRight == 0 {
		t.Errorf("running pose loses ground route: %v", pad)
	}
	known := data.Bot.publicMemory(o)
	hidden, recent := known, data.Bot
	hidden.Players[0].Visible = false
	recent.PlayerSeenAt[0] = hidden.Tick - 1
	if _, ok := recent.clawGroundRoute(hidden); !ok {
		t.Error("one-frame body occlusion cancels the observed floor route")
	}
	recent.PlayerSeenAt[0] = hidden.Tick - 9
	if _, ok := recent.clawGroundRoute(hidden); ok {
		t.Error("stale body memory controls the floor route")
	}
	p := known.Players[0]
	row, col := (p.Y+16+int(known.ViewY&15))/16, (p.X+8+int(known.ViewX&15))/16
	known.Terrain[row][col], known.TerrainKnown[row][col] = TerrainEmpty, true
	if _, ok := data.Bot.clawGroundRoute(known); ok {
		t.Error("claw route crosses a visible gap")
	}
}
