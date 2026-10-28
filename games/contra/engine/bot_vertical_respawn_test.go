package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Original legal-input replay proves the left platform is reachable by holding
// current X. A vertical respawn must not drift toward screen center, and the
// normal horizontal boundary must not exclude its current spawn X below28.
func TestBotVerticalRespawnKeepsReachableLeftPlatform(t *testing.T) {
	f, e := os.Open("testdata/bot-waterfall-respawn-left.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []BotObservation
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	for _, base := range rows {
		for actor := 0; actor < 2; actor++ {
			o := base
			o.Actor = actor
			o.Seats = actor + 1
			o.Players[actor] = base.Players[0]
			p := o.Players[actor]
			b := Bot{LastTick: o.Tick - 1, LastX: p.X, LastY: p.Y - 1, LastViewX: o.ViewX, LastViewY: o.ViewY, Phase: "battle", Stage: 2, StageKnown: true, SceneKnown: true, GridKnown: true}
			if target, ok := b.planDescent(o); !ok || target != p.X {
				t.Error("current left spawn is excluded from safe landing", o.Tick, actor, target, ok)
			}
			if pad := b.Decide(o); pad&(ButtonRight|ButtonA) != 0 {
				t.Error("vertical respawn drifts off its reachable platform", o.Tick, actor, pad)
			}
		}
	}
}

// Releasing direction does not stop original airborne horizontal momentum.
// At the left spawn boundary the corrective Left input is blocked safely,
// whereas neutral or Right continues carrying the actor off the small ledge.
func TestBotVerticalRespawnCancelsLatchedRightMomentum(t *testing.T) {
	f, e := os.Open("testdata/bot-waterfall-respawn-left.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []BotObservation
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	o := rows[1]
	p := o.Players[0]
	b := Bot{LastTick: o.Tick - 1, LastX: p.X - 1, LastY: p.Y - 1, LastPad: ButtonRight | ButtonUp, LastViewX: o.ViewX, LastViewY: o.ViewY, Phase: "battle", Stage: 2, StageKnown: true, SceneKnown: true, GridKnown: true}
	pad := b.Decide(o)
	actual := Player{State: 1, X: uint8(p.X), Sprite: 9}
	actual.setY(uint8(p.Y))
	actual.Motion.Jump = 0x21
	actual.Motion.Velocity = 256
	world := OutdoorWorld{Level: 2, ScrollType: 1, StopScroll: 0xff}
	terrain := CollisionWindow{}
	work := BaseThrowWork{}
	_, e = actual.Step(&PlayerStepContext{World: &world, Terrain: &terrain, ThrowWork: &work, Input: PlayerInput{Held: pad}, Fire: func(FireRequest) FireResult { return FireResult{} }})
	if e != nil {
		t.Fatal(e)
	}
	if int(actual.X) > p.X {
		t.Fatal("neutral pad preserves fatal rightward momentum", pad, p.X, actual.X)
	}
}
