package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalContinuousDodgeActions(t *testing.T) {
	b, err := os.ReadFile("testdata/dodge.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Scenario      string
		Frame, Actor  int
		Before, After struct {
			observedHoopPlayer
			BallAction, BallRequest, DodgeStep, DodgeSpeed uint8
		}
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	count, sequences := 0, 0
	var carrier Carrier
	var memory DodgeMemory
	previousScenario := ""
	previousActor, previousFrame := -1, -1
	for _, r := range rows {
		if r.Before.Action&127 != 46 {
			continue
		}
		if r.Before.Attachment&128 != 0 {
			t.Fatal("original dodge sample has linked actor")
		}
		if r.Scenario != previousScenario || r.Actor != previousActor || r.Frame != previousFrame+3 {
			carrier = Carrier{Player: playerFromObservation(r.Before.observedPlayer), ShotCycle: int8(r.Before.ShotCycle)}
			memory = DodgeMemory{Step: r.Before.DodgeStep, InitialXHigh: int8(r.Before.DodgeSpeed)}
			sequences++
		} else if carrier != (Carrier{Player: playerFromObservation(r.Before.observedPlayer), ShotCycle: int8(r.Before.ShotCycle)}) {
			t.Fatal("original interval is not continuous", r.Frame, carrier, r.Before)
		}
		profile := KunioProfile()
		profile.RunSpeed, err = RunSpeedForGrade(uint8(r.Before.RunGrade))
		if err != nil {
			t.Fatal(err)
		}
		got, err := StepCarrierDodge(carrier, memory, CarrierContext{HasBall: r.Before.Holder == int32(r.Actor), Computer: r.Before.Control&128 != 0}, profile, NewYorkGroundCourt())
		if err != nil {
			t.Fatalf("%s frame %d: %v", r.Scenario, r.Frame, err)
		}
		carrier = got.Carrier
		memory = got.Memory
		if memory != (DodgeMemory{Step: r.After.DodgeStep, InitialXHigh: int8(r.After.DodgeSpeed)}) {
			t.Fatal("dodge phase differs", r.Frame, memory, r.After)
		}
		want := Carrier{Player: playerFromObservation(r.After.observedPlayer), ShotCycle: int8(r.After.ShotCycle)}
		if carrier != want || got.BindBall || got.Ball.Change || got.Ball.ChangeRequest || got.Revisit || r.Before.Holder != r.After.Holder || r.Before.BallAction != r.After.BallAction || r.Before.BallRequest != r.After.BallRequest {
			t.Fatalf("%s frame %d got %+v want %+v", r.Scenario, r.Frame, got, want)
		}
		previousScenario, previousActor, previousFrame = r.Scenario, r.Actor, r.Frame
		count++
	}
	if count != 8 || sequences != 2 {
		t.Fatal(count, sequences)
	}
	t.Log("matched two continuous original dodge sequences, eight action updates")
}

func TestDodgeUsesEntrySpeedSignAndPreservesDepthFraction(t *testing.T) {
	actor := Carrier{Player: Player{Action: Action(46), FacingLeft: false, Heading: HeadingUp, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, Velocity: Velocity{X: -Pixel, Depth: 17}}}}
	first, err := StepCarrierDodge(actor, DodgeMemory{}, CarrierContext{}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || first.Memory.InitialXHigh != -1 || first.Carrier.Player.Body.Velocity.Depth != 4*Pixel+17 || first.Carrier.Player.Animation.NextAction != ActionRun {
		t.Fatal(first, err)
	}
	next, err := StepCarrierDodge(first.Carrier, first.Memory, CarrierContext{}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || next.Carrier.Player.Body.Velocity.X != -2*Pixel || next.Carrier.Player.Pose.Mirrored {
		t.Fatal(next, err)
	}
	for i := 0; i < 2; i++ {
		next, err = StepCarrierDodge(next.Carrier, next.Memory, CarrierContext{}, KunioProfile(), NewYorkGroundCourt())
		if err != nil {
			t.Fatal(err)
		}
	}
	if next.Carrier.Player.Action != ActionRun || next.Carrier.Player.Entered || !next.Carrier.Player.Pose.Mirrored || next.Carrier.Player.Pose.Frame != 7 {
		t.Fatal("final mirrored pose or action lost", next)
	}
}

func TestComputerActorExecutesOriginalDodgeChoice(t *testing.T) {
	actor := ComputerActorState{Carrier: HoopCarrier{Player: Player{Action: ActionDribbleRun, Entered: true, Heading: HeadingRight, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, Velocity: Velocity{X: 5 * Pixel}}}}, Control: AIControlState{Mode: 26, Entered: true, PlanParameter: 0}}
	got, err := AdvanceComputerActor(actor, ComputerActorContext{Seat: 0, HasBall: true, BallHolder: 0, BallAction: BallDribble, CourtID: 1}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.State.Carrier.Player.Action != Action(46) || got.State.Dodge.Step != 1 {
		t.Fatal(got, err)
	}
}

func TestDodgePreservesEveryOriginalRejectedRequest(t *testing.T) {
	// The oracle executes E18B..E1ED; 949F and 86CF do not discard rejected
	// requests. Cold update 467 retains CatchBall while AI directly chooses Dodge.
	b, err := os.ReadFile("testdata/player-request-acceptance.json")
	if err != nil {
		t.Fatal(err)
	}
	var allowed [][]Action
	if err = json.Unmarshal(b, &allowed); err != nil {
		t.Fatal(err)
	}
	before := Carrier{Player: Player{Action: ActionDodge, Heading: HeadingUp, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, Velocity: Velocity{X: 5 * Pixel, Depth: 17}}}}
	baseline, err := StepCarrierDodge(before, DodgeMemory{}, CarrierContext{HasBall: true, Computer: true}, KunioProfile(), NewYorkGroundCourt())
	if err != nil {
		t.Fatal(err)
	}
	rejected := 0
	for action, from := range allowed {
		accepted := false
		for _, a := range from {
			if a == ActionDodge {
				accepted = true
			}
		}
		actor := before
		actor.Player.Request = ActionRequest{Action: Action(action), Pending: true}
		got, err := StepCarrierDodge(actor, DodgeMemory{}, CarrierContext{HasBall: true, Computer: true}, KunioProfile(), NewYorkGroundCourt())
		if accepted {
			if err == nil || got.Carrier != actor {
				t.Fatal("cross-handler transition silently continued dodge", action, got, err)
			}
			continue
		}
		want := baseline
		want.Carrier.Player.Request = actor.Player.Request
		if err != nil || got != want {
			t.Fatalf("rejected request %x must remain pending: got %+v want %+v err %v", action, got, want, err)
		}
		rejected++
	}
	if rejected != 105 {
		t.Fatal("original rejection coverage", rejected)
	}
}
