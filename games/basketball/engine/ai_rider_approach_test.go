package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestAIRiderApproachIsolatedOriginalMatrix(t *testing.T) {
	f, err := os.Open("testdata/ai-rider-approach-function.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		Actor, Action, Zone, Grade, Holder uint8
		Entered, Air                       bool
		Phase, Speed                       int8
		Positions                          [7]struct{ X, Depth uint16 }
		After                              struct {
			Mode, Action, Heading, Facing, Destination, Target, Phase uint8
			VX, VZ                                                    uint16
		}
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 9176 {
		t.Fatal("incomplete original AA2C isolated matrix", len(rows))
	}
	for i, r := range rows {
		target := r.Actor ^ 1
		if !r.Entered {
			target = r.Actor ^ 2
		}
		control := AIControlState{Mode: 29, Entered: r.Entered, Plan: 8, Counter: 7, PlanParameter: 6, Destination: 7, TrackedObject: int8(target)}
		speed, _ := JumpVelocityForGrade(r.Grade)
		zones := [4]CourtZone{}
		zones[r.Actor] = CourtZone(r.Zone)
		v := AIControlView{Actor: r.Actor, Action: Action(r.Action), ActionEntered: true, Heading: HeadingUpRight, FacingLeft: r.Actor&1 != 0, Airborne: r.Air, BallHolder: int8(r.Holder), CachedZones: &zones, Random: 0x12345678, ShotContext: AIShotContext{JumpVelocity: speed, JumpPhase: r.Phase, VerticalSpeed: r.Speed}}
		for j, p := range r.Positions {
			position := Position{X: Fixed(p.X) * Pixel, Depth: Fixed(p.Depth) * Pixel}
			if j < 4 {
				v.Players[j] = position
			}
			if j == 6 {
				v.BallPosition = position
			}
		}
		v.Position = v.Players[r.Actor]
		got, err := StepOriginalAIControl(control, v)
		if err != nil {
			t.Fatal(i, err)
		}
		want := AIControlDecision{Control: control, Action: Action(r.After.Action & 127), ActionEntered: r.After.Action&128 != 0, Heading: HeadingNone, FacingLeft: r.After.Facing&128 != 0, BindHoop: -1, Random: v.Random}
		want.Control.Mode, want.Control.Entered = AIMode(r.After.Mode&127), r.After.Mode&128 != 0
		want.Control.Destination = CourtZone(r.After.Destination)
		want.Control.TrackedObject = int8(r.After.Target)
		if r.After.Heading != 255 {
			want.Heading = Heading(r.After.Heading / 32)
		}
		if r.After.Phase != uint8(r.Phase) {
			want.ShotPlan.ChangeJumpPhase = true
			want.ShotPlan.JumpPhase = int8(r.After.Phase)
		}
		if r.After.VX == 0 && r.After.VZ == 0 {
			want.Engagement.StopPlanar = true
		}
		if got != want {
			t.Fatalf("isolated AA2C case%d %+v\ngot %+v\nwant %+v", i, r, got, want)
		}
		original := ComputerActorState{JumpPhase: r.Phase, Carrier: HoopCarrier{Player: Player{Body: Body{Velocity: Velocity{X: 1234, Depth: -345, Height: 77}}}}}
		applied := ApplyOriginalAIActorDecision(original, got)
		if applied.Carrier.Player.Body.Velocity != (Velocity{X: Fixed(int16(r.After.VX)), Depth: Fixed(int16(r.After.VZ)), Height: 77}) || applied.JumpPhase != int8(r.After.Phase) {
			t.Fatal("original rider commands applied incorrectly", i, applied)
		}
	}
	t.Log("matched 9176 isolated original AA2C/helper cases; no natural-call or full-match claim")
}
