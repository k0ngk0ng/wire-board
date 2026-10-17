package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type observedHoopPlayer struct {
	observedPlayer
	ShotCycle, AimWindow, Control, RunGrade int32
	Owners, HoopActions, HoopFacings        [6]uint8
}

func hoopCarrierFromObservation(r observedHoopPlayer) HoopCarrier {
	return HoopCarrier{Player: playerFromObservation(r.observedPlayer), ShotCycle: int8(r.ShotCycle), AimWindow: uint8(r.AimWindow)}
}

func TestHoopCarrierAgainstOriginal(t *testing.T) {
	b, err := os.ReadFile("testdata/hoop-player.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Frame, Actor  int
		Before, After observedHoopPlayer
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 159 {
		t.Fatal("unexpected player-hoop call count", len(rows))
	}
	for _, r := range rows {
		if r.Before.Attachment&128 != 0 || r.Before.Boost != 0 || r.Before.Holder == int32(r.Actor) {
			t.Fatal("fixture leaves unlinked hoop-carry context")
		}
		held := -1
		for i, owner := range r.Before.Owners {
			if int(owner) == r.Actor {
				held = i
				break
			}
		}
		profile := KunioProfile()
		profile.ID = "original-opponent"
		profile.RunSpeed, err = RunSpeedForGrade(uint8(r.Before.RunGrade))
		if err != nil {
			t.Fatal(err)
		}
		got, err := StepHoopCarrierAction(hoopCarrierFromObservation(r.Before), HoopCarrierContext{Computer: r.Before.Control&128 != 0, HeldHoop: held}, profile, NewYorkGroundCourt())
		if err != nil {
			t.Fatalf("frame %d actor %d: %v", r.Frame, r.Actor, err)
		}
		if want := hoopCarrierFromObservation(r.After); got.State != want {
			t.Fatalf("frame %d actor %d\nbefore %+v\ngot %+v\nwant %+v", r.Frame, r.Actor, hoopCarrierFromObservation(r.Before), got.State, want)
		}
		actions, facings := r.Before.HoopActions, r.Before.HoopFacings
		if got.Hoop.ChangeAction {
			actions[got.Hoop.Index] = uint8(got.Hoop.Action)
		}
		if got.Hoop.ChangeFacing {
			facings[got.Hoop.Index] = 0
			if got.Hoop.FacingLeft {
				facings[got.Hoop.Index] = 128
			}
		}
		if actions != r.After.HoopActions || facings != r.After.HoopFacings || r.Before.Owners != r.After.Owners {
			t.Fatalf("frame %d actor %d: incorrect hoop command %+v", r.Frame, r.Actor, got.Hoop)
		}
	}
	t.Log("matched 159 original pickup, carry idle/run, aim and throw action calls")
}

func TestPickupHoopRequiresBindingBeforeAnimation(t *testing.T) {
	original := HoopCarrier{Player: Player{Action: ActionPickUpHoop}}
	got, err := StepHoopCarrierAction(original, HoopCarrierContext{HeldHoop: -1}, KunioProfile(), NewYorkGroundCourt())
	if err == nil || got.State != original || got.Hoop.ChangeAction {
		t.Fatal(got, err)
	}
}

func TestHoopCarrierRetainsOriginalRejectedRequests(t *testing.T) {
	b, err := os.ReadFile("testdata/player-request-acceptance.json")
	if err != nil {
		t.Fatal(err)
	}
	var allowed [][]Action
	if err = json.Unmarshal(b, &allowed); err != nil {
		t.Fatal(err)
	}
	actions := []Action{ActionHoopCarryIdle, ActionHoopCarryRun, ActionHoopAim, ActionHoopThrow, ActionPickUpHoop}
	rejected := 0
	for _, action := range actions {
		before := HoopCarrier{Player: Player{Action: action, Heading: HeadingRight, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, Velocity: Velocity{X: 5 * Pixel}}}}
		ctx := HoopCarrierContext{Computer: true, HeldHoop: 0}
		baseline, err := StepHoopCarrierAction(before, ctx, KunioProfile(), NewYorkGroundCourt())
		if err != nil {
			t.Fatal(action, err)
		}
		for request, from := range allowed {
			accepted := false
			for _, a := range from {
				if a == action {
					accepted = true
				}
			}
			if accepted {
				continue
			}
			input := before
			input.Player.Request = ActionRequest{Action: Action(request), Pending: true}
			got, err := StepHoopCarrierAction(input, ctx, KunioProfile(), NewYorkGroundCourt())
			want := baseline
			want.State.Player.Request = input.Player.Request
			if err != nil || got != want {
				t.Fatalf("action %x rejected request %x: got %+v want %+v err %v", action, request, got, want, err)
			}
			rejected++
		}
	}
	if rejected != 488 {
		t.Fatal("original rejection count", rejected)
	}
}

func TestHoopCarrierAcceptsRequestBeforeChoosingHandler(t *testing.T) {
	actor := HoopCarrier{Player: Player{Action: ActionHoopAim, Entered: true, Heading: HeadingRight, Request: ActionRequest{Action: ActionHoopThrow, Pending: true}, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}}}
	got, err := StepHoopCarrierAction(actor, HoopCarrierContext{Computer: true, HeldHoop: 0}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.State.Player.Action != ActionHoopThrow || got.State.Player.Request.Pending || !got.Hoop.ChangeAction || got.Hoop.Action != HoopThrown {
		t.Fatal(got, err)
	}
	actor.Player.Request = ActionRequest{Action: 0x5d, Pending: true}
	got, err = StepHoopCarrierAction(actor, HoopCarrierContext{Computer: true, HeldHoop: 0}, KunioProfile(), NewYorkGroundCourt())
	if err == nil || got.State != actor {
		t.Fatal("unsupported accepted action must fail atomically", got, err)
	}
}
