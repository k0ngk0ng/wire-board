package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type observedDunk struct {
	observedPlayer
	Control, TargetObject, ActiveDunker, AuxiliaryMode, CarriedPlayer, DetachedCount uint8
	Durabilities, Actions, Owners                                                    [13]uint8
	Ball                                                                             observedBall
}

func TestOriginalDunkAndReleasedBallTrace(t *testing.T) {
	var rows []struct {
		Kind          string
		Frame         int
		Actor         uint8
		Before, After observedDunk
	}
	raw, err := os.ReadFile("testdata/dunk-chain-player.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 450 {
		t.Fatal("missing original dunk-chain calls", len(rows))
	}
	players, balls := 0, 0
	for _, r := range rows {
		b, a := r.Before, r.After
		if r.Kind == "player" {
			ctx := DunkContext{Actor: r.Actor, TargetObject: b.TargetObject, TargetOwner: b.Owners[b.TargetObject], TargetDurability: int8(b.Durabilities[b.TargetObject]), AuxiliaryOwners: [2]int8{int8(b.Owners[4]), int8(b.Owners[5])}, CarriedPlayer: int8(b.CarriedPlayer), DetachedCount: b.DetachedCount, Attachment: uint8(b.Attachment), Ball: ballFromObservation(b.Ball), BallHolder: int8(b.Holder), AuxiliaryRelease: b.AuxiliaryMode&128 != 0, Computer: b.Control&128 != 0}
			got, err := StepDunkPlayer(playerFromObservation(b.observedPlayer), ctx, NewYorkGroundCourt())
			want := playerFromObservation(a.observedPlayer)
			active := b.ActiveDunker
			if got.SetActiveDunker {
				active = got.ActiveDunker
			}
			if err != nil || got.Player != want || got.Ball != ballFromObservation(a.Ball) || got.TargetDurability != int8(a.Durabilities[b.TargetObject]) || got.AuxiliaryRelease != (a.AuxiliaryMode&128 != 0) || got.Revisit != (a.Attachment&64 == 0) || active != a.ActiveDunker {
				t.Fatalf("player frame %d before %+v\ngot %+v\nwant %+v err=%v", r.Frame, b, got, a, err)
			}
			actions := b.Actions
			actions[r.Actor] = uint8(got.Player.Action)
			if got.Player.Entered {
				actions[r.Actor] |= 128
			}
			actions[6] = uint8(got.Ball.Action)
			if got.Ball.Entered {
				actions[6] |= 128
			}
			for _, c := range got.Objects {
				actions[c.Object] = c.Action
			}
			if actions != a.Actions || b.Owners != a.Owners || b.Holder != a.Holder {
				t.Fatal("unrepresented dunk world change", r.Frame, actions, a.Actions)
			}
			players++
		} else {
			got, err := StepDunkReleasedBall(ballFromObservation(b.Ball), NewYorkBallCourt())
			holder := b.Holder
			if got.ReleaseHolder {
				holder = 6
			}
			if err != nil || got.Ball != ballFromObservation(a.Ball) || holder != a.Holder {
				t.Fatalf("ball frame %d before %+v\ngot %+v\nwant %+v err=%v", r.Frame, b.Ball, got, a.Ball, err)
			}
			balls++
		}
	}
	if players != 190 || balls != 260 {
		t.Fatal(players, balls)
	}
	t.Logf("matched all %d dunk player calls and %d released-ball calls", players, balls)
}
func TestDunkPreservesFractionsAndBallEntryAddition(t *testing.T) {
	p := Player{Action: 0x58, Heading: HeadingNone, Body: Body{Position: Position{X: 450*Pixel + 77, Depth: 40*Pixel + 99, Height: 120*Pixel + 111}}}
	ctx := DunkContext{Actor: 3, TargetObject: 8, TargetOwner: 8, TargetDurability: 1, CarriedPlayer: -1, AuxiliaryOwners: [2]int8{-1, -1}, BallHolder: 3, Ball: Ball{Action: BallHeld, Body: Body{Position: Position{X: 450*Pixel + 33}}}}
	got, err := StepDunkPlayer(p, ctx, NewYorkGroundCourt())
	if err != nil || got.Player.Body.Position.X != 451*Pixel+77 || got.Player.Body.Position.Depth != 43*Pixel+99 || got.Player.Body.Position.Height != 74*Pixel+111 || got.Ball.Body.Position.X != 459*Pixel+33 || got.TargetDurability != -1 {
		t.Fatal(got, err)
	}
	ball := Ball{Action: 10, Airborne: true, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel, Height: 100 * Pixel}, Velocity: Velocity{Height: 4 * Pixel}, Gravity: Pixel / 4}}
	released, err := StepDunkReleasedBall(ball, NewYorkBallCourt())
	if err != nil || !released.ReleaseHolder || released.Ball.Body.Gravity != 3*Pixel/4 || released.Ball.Body.Velocity.Height != 4*Pixel+Pixel/16-3*Pixel/4 {
		t.Fatal(released, err)
	}
}
