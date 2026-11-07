package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Before snow-edge repair: losing support pixels changes a standing actor to
// the default airborne Right action; a known same-height forward landing is
// rejected solely because the intervening fill is unknown. Both seats fail.
func TestBotSnowUnknownSupportAndKnownForwardLanding(t *testing.T) {
	f, e := os.Open("testdata/bot-coop-snow-edge.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var data struct {
		UnknownSupport []struct {
			Observation BotObservation
			PriorPlayer BotPlayer
			PriorSeenAt uint64
		}
		KnownJumpStart BotObservation
	}
	if e = json.NewDecoder(z).Decode(&data); e != nil {
		t.Fatal(e)
	}
	for _, c := range data.UnknownSupport {
		o := c.Observation
		p := c.PriorPlayer
		if o.terrainKnownAt(p.X, p.Y+16) {
			t.Fatal("fixture no longer has unknown supporting pixels")
		}
		b := Bot{LastTick: o.Tick - 1, LastX: p.X, LastY: p.Y, LastViewX: o.ViewX, LastViewY: o.ViewY, Phase: "battle", Stage: 4, StageKnown: true, SceneKnown: true, GridKnown: true}
		b.PlayerMemory[o.Actor] = p
		b.PlayerSeenAt[o.Actor] = c.PriorSeenAt
		if pad := b.Decide(o); pad&(ButtonRight|ButtonLeft|ButtonA) != 0 {
			t.Errorf("seat%d walks off unknown support: %v", o.Actor, pad)
		}
	}
	o := data.KnownJumpStart
	p := o.Players[o.Actor]
	b := Bot{LastTick: o.Tick - 1, LastX: p.X, LastY: p.Y, LastViewX: o.ViewX, LastViewY: o.ViewY, Phase: "battle", Stage: 4, StageKnown: true, SceneKnown: true, GridKnown: true}
	if pad := b.Decide(o); pad&ButtonA == 0 || pad&ButtonRight == 0 {
		t.Fatal("known same-height forward platform is not used", pad)
	}
}
