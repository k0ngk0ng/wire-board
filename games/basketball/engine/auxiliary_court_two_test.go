package engine

import "testing"

func TestAuxiliaryCourtTwoUsesOriginalMotionBinding(t *testing.T) {
	// Court2's $06DB is switchTicks[1], not a New York passage marker.
	s := AuxiliaryState{Action: 0, Entered: true, Owner: -1, NewYorkPassage: 7, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, Velocity: Velocity{X: Pixel}}}
	ctx := AuxiliaryContext{Object: 4, Court: 2, SpecialHolder: -1}
	if got, err := StepAuxiliaryRest(s, ctx); err == nil || got.State != s {
		t.Fatal("Court2 requires original motion, not NY fallback", got, err)
	}
	f := CourtFloorState{}
	f.Objects[4].Kind = 12
	f.CourtFlags[4] = 7
	stored := false
	court := FlatCourt{originalMotion: &courtMotionBinding{object: 4, court: 2, load: func() CourtFloorState { return f }, store: func(next CourtFloorState, selected uint8) {
		stored = true
		f = next
		if selected != 4 {
			t.Fatal(selected)
		}
	}}}
	got, err := stepAuxiliaryRestOnCourt(s, ctx, court)
	if err != nil || !stored || got.State.Body.Position != (Position{X: 201 * Pixel, Depth: 40 * Pixel}) || got.State.NewYorkPassage != 7 {
		t.Fatal(got, stored, err)
	}
}
