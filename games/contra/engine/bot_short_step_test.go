package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// A visible short rise must remain usable after camera stagnation even when
// the identical brown wall fill is correctly kept unknown.
func TestBotShortKnownRiseBreaksStagnation(t *testing.T) {
	f, err := os.Open("testdata/bot-alien-short-step.json.gz")
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
	o := data.Bot.publicMemory(data.Observation)
	fresh := data.Bot
	fresh.ViewStillTicks = 0
	if fresh.play(o)&ButtonA != 0 {
		t.Error("fresh unobstructed approach triggers stalled-rise recovery")
	}
	flat := o
	for row := range flat.Terrain {
		if row*16-int(o.ViewY&15) < o.Players[0].Y+16-8 {
			flat.Terrain[row] = [17]TerrainKind{}
		}
	}
	withoutRise := data.Bot
	if withoutRise.play(flat)&ButtonA != 0 {
		t.Error("recovery invents an elevated landing")
	}
	x, y, _, found := data.Bot.planJump(o, false)
	t.Logf("position=%+v landing=%d,%d found=%v", o.Players[0], x, y, found)
	if pad := data.Bot.play(o); pad&ButtonA == 0 || pad&ButtonRight == 0 {
		t.Fatalf("visible forward rise not used: %d", pad)
	}
}
