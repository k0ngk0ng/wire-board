package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCarrierRunningReceiveAndPassageAgainstOriginal(t *testing.T) {
	b, err := os.ReadFile("testdata/ball-player.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Scenario      string
		Frame, Actor  int
		Before, After struct {
			observedHoopPlayer
			BallAction, BallRequest uint8
		}
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, r := range rows {
		a := r.Before.Action & 127
		if a != 18 && a != 19 && a != 21 && a != 30 && r.Before.Passage == 0 && r.After.Passage == 0 {
			continue
		}
		if r.Before.Attachment&128 != 0 {
			t.Fatal("linked rider needs a separate world context")
		}
		profile := KunioProfile()
		profile.RunSpeed, err = RunSpeedForGrade(uint8(r.Before.RunGrade))
		if err != nil {
			t.Fatal(err)
		}
		profile.JumpVelocity, err = JumpVelocityForGrade(uint8(r.Before.JumpGrade))
		if err != nil {
			t.Fatal(err)
		}
		original := Carrier{Player: playerFromObservation(r.Before.observedPlayer), ShotCycle: int8(r.Before.ShotCycle)}
		var got CarrierStep
		if freeAction(original.Player.Action) {
			p, e := StepComputerFreePlayer(original.Player, profile, NewYorkGroundCourt())
			err = e
			got.Carrier = Carrier{Player: p, ShotCycle: original.ShotCycle}
		} else {
			got, err = StepCarrierAction(original, CarrierContext{HasBall: r.Before.Holder == int32(r.Actor), Computer: r.Before.Control&128 != 0, SkipJumpWindup: r.Before.Boost&128 != 0, BallAction: BallAction(r.Before.BallAction & 127)}, profile, NewYorkGroundCourt())
		}
		if err != nil {
			t.Fatalf("%s frame %d actor %d: %v", r.Scenario, r.Frame, r.Actor, err)
		}
		want := Carrier{Player: playerFromObservation(r.After.observedPlayer), ShotCycle: int8(r.After.ShotCycle)}
		if got.Carrier != want || got.Revisit != (r.After.Attachment&64 == 0) {
			t.Fatalf("%s frame %d actor %d\nbefore %+v\ngot %+v\nwant %+v", r.Scenario, r.Frame, r.Actor, original, got, want)
		}
		ball, request := r.Before.BallAction, r.Before.BallRequest
		if got.Ball.Change {
			ball = uint8(got.Ball.Action)
		}
		if got.Ball.ChangeRequest {
			request = uint8(got.Ball.Request.Action)
			if !got.Ball.Request.Pending {
				request |= 128
			}
		}
		if ball != r.After.BallAction || request != r.After.BallRequest {
			t.Fatal("ball effects differ", r.Frame, got.Ball, ball, request, r.After.BallAction, r.After.BallRequest)
		}
		count++
	}
	if count != 379 {
		t.Fatal("unexpected carrier/terrain observation count", count)
	}
	t.Log("matched 379 original running, receiving, braking, running-jump and passage action calls")
}
