package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalBallCatchActions(t *testing.T) {
	b, err := os.ReadFile("testdata/ball-player.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Scenario      string
		Frame, Actor  int
		Before, After struct {
			observedHoopPlayer
			BallAction uint8
		}
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2006 {
		t.Fatal("unexpected candidate catch record count", len(rows))
	}
	count := 0
	for _, r := range rows {
		a := r.Before.Action & 127
		if a != 29 && a != 31 {
			continue
		}
		if r.Before.Attachment&128 != 0 || r.Before.Boost != 0 || r.Before.Holder != int32(r.Actor) {
			t.Fatal("fixture leaves ordinary catch context")
		}
		original := Carrier{Player: playerFromObservation(r.Before.observedPlayer), ShotCycle: int8(r.Before.ShotCycle)}
		got, err := StepCarrierAction(original, CarrierContext{HasBall: true, BallAction: BallAction(r.Before.BallAction & 127)}, KunioProfile(), NewYorkGroundCourt())
		if err != nil {
			t.Fatalf("%s frame %d: %v", r.Scenario, r.Frame, err)
		}
		want := Carrier{Player: playerFromObservation(r.After.observedPlayer), ShotCycle: int8(r.After.ShotCycle)}
		if got.Carrier != want || got.BindBall || got.Revisit != (r.After.Attachment&64 == 0) {
			t.Fatalf("%s frame %d got %+v want %+v attachment %d", r.Scenario, r.Frame, got, want, r.After.Attachment)
		}
		action := r.Before.BallAction
		if got.Ball.Change {
			action = uint8(got.Ball.Action)
		}
		if action != r.After.BallAction {
			t.Fatal("catch ball command differs", r.Frame, got.Ball)
		}
		count++
	}
	if count != 25 {
		t.Fatal(count)
	}
	t.Log("matched 25 original ground/air catch action calls, including same-update revisit")
}

func TestPickupBallUsesBothAnimationStages(t *testing.T) {
	s := Carrier{Player: Player{Action: ActionPickUp, Heading: HeadingNone, Body: Body{Position: Position{X: 100 * Pixel, Depth: 40 * Pixel}, Velocity: Velocity{X: Pixel}}}, ShotCycle: -1}
	ctx := CarrierContext{}
	bindings := 0
	for i := 0; i < 9; i++ {
		step, err := StepCarrierAction(s, ctx, KunioProfile(), NewYorkGroundCourt())
		if err != nil {
			t.Fatal(i, err)
		}
		s = step.Carrier
		if step.BindBall {
			ctx.HasBall = true
			bindings++
		}
		if step.Ball.Change {
			ctx.BallAction = step.Ball.Action
		}
		if s.Player.Body.Position != (Position{X: 100 * Pixel, Depth: 40 * Pixel}) {
			t.Fatal("pickup moved player", i, s)
		}
		if i == 3 && (s.Player.Action != ActionCatchBall || s.Player.Entered) {
			t.Fatal("pickup skipped catch transition", s)
		}
	}
	if bindings != 1 || s.Player.Action != ActionDribbleIdle || s.Player.Entered {
		t.Fatal(bindings, s)
	}
}

func TestCatchHeldBallOffsetsPreserveBallFractions(t *testing.T) {
	// Fixed bank $ECF5[action] selects $ED8D: {6, 0, 8, 1} for
	// all three catch actions, including the initial receive transition ($1E).
	for _, action := range []Action{ActionReceiveBall, ActionCatchBall, ActionCatchAir, ActionPickUp} {
		for _, mirrored := range []bool{false, true} {
			b := Ball{Action: BallHeld, Entered: true, Airborne: true, Body: Body{Position: Position{X: 200*Pixel + 119, Depth: 50*Pixel + 33, Height: 10*Pixel + 11}}}
			p := Player{Action: action, Pose: Pose{Mirrored: mirrored}, Body: Body{Position: Position{X: 100*Pixel + 77, Depth: 40*Pixel + 99, Height: 16*Pixel + 128}}}
			got, err := StepHeldBall(b, p)
			x := 106*Pixel + 119
			if mirrored {
				x = 94*Pixel + 119
			}
			if action == ActionPickUp {
				x = 100*Pixel + 119
			}
			if err != nil || got.Body.Position != (Position{X: x, Depth: 39*Pixel + 33, Height: 24*Pixel + 11}) {
				t.Fatal(action, mirrored, got, err)
			}
		}
	}
}
