package engine

import "testing"

func TestOriginalAIUsesCachedPhaseZones(t *testing.T) {
	// A fixed phase-order case: the pre-board zone differs from the current
	// position. It verifies the world boundary, not another natural replay.
	zones := [4]CourtZone{0x70, 0x70, 0x51, 0x00}
	s := AIControlState{Mode: 8, Entered: true, Destination: 0x51}
	v := AIControlView{Actor: 2, Action: ActionRun, ActionEntered: true, Position: Position{X: 100 * Pixel, Depth: 40 * Pixel}, BallPosition: Position{X: 400 * Pixel, Depth: 40 * Pixel}, BallHolder: 6, CachedZones: &zones}
	v.Players[0] = v.Position
	v.Players[1] = v.Position
	got, err := StepOriginalAIControl(s, v)
	if err != nil || got.Control.Mode != 8 || got.Action != ActionIdle || got.Heading != HeadingRight {
		t.Fatal(got, err)
	}
	v.CachedZones = nil
	fallback, err := StepOriginalAIControl(s, v)
	if err != nil || fallback.Control.Mode != 19 {
		t.Fatal("fixture does not distinguish cached and recomputed zones", fallback, err)
	}
	actor := ComputerActorState{Carrier: HoopCarrier{Player: Player{Action: ActionRun, Entered: true, Heading: HeadingRight, Body: Body{Position: v.Position}}}, Control: s}
	step, err := AdvanceComputerActor(actor, ComputerActorContext{Seat: 2, Players: v.Players, BallPosition: v.BallPosition, BallHolder: 6, CachedZones: &zones}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || step.State.Control.Mode != 8 || step.State.Carrier.Player.Action != ActionIdle {
		t.Fatal("actor entry dropped phase cache", step, err)
	}
}

func TestOriginalAIZeroAndInvalidZoneCache(t *testing.T) {
	zones := [4]CourtZone{}
	s := AIControlState{Mode: 8, Entered: true}
	v := AIControlView{Actor: 2, Action: ActionRun, Position: Position{X: 100 * Pixel, Depth: 40 * Pixel}, CachedZones: &zones}
	got, err := StepOriginalAIControl(s, v)
	if err != nil || got.Heading != HeadingUpLeft {
		t.Fatal("zero cache was recomputed", got, err)
	}
	zones[2] = 0x76
	got, err = StepOriginalAIControl(s, v)
	if err == nil || got.Control != s || got.Action != v.Action {
		t.Fatal("invalid zone cache did not fail atomically", got, err)
	}
}
