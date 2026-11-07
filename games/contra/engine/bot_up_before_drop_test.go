package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// An upward route can initially share the current X. Requiring immediate
// horizontal progress makes the actor drop back to the platform it just left.
func TestBotHigherLedgePrecedesWallDrop(t *testing.T) {
	f, err := os.Open("testdata/bot-energy-up-before-drop.json.gz")
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
	x, y, _, found := data.Bot.planJump(o, false)
	t.Logf("player=%+v planned landing=%d,%d found=%v", o.Players[0], x, y, found)
	if pad, handled := data.Bot.blockedWallDrop(o, 0); handled {
		t.Fatalf("downward route preempts reachable higher ledge: %v", pad)
	}
}
