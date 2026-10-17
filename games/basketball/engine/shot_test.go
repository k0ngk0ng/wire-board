package engine

// Original shots and bounces were recorded before implementation. The failure
// matrix in VERIFICATION.md separates targeting, flight and contact effects.
import (
	"encoding/json"
	"os"
	"testing"
)

type observedBall struct {
	Action, Request, Pose, Facing, Clock, Cursor, RepeatCount, NextAction, Air, Attachment, Bounces, Holder, Hoop, Deflection int32
	X, Depth, Height, VX, VY, VZ, Gravity                                                                                     int32
	Shooter, ShooterHeight, Character, Team, SpecialFlags, Equipment                                                          int32
	RimFlags                                                                                                                  [6]uint8
	TargetX, TargetDepth, TargetHeight, ArcHeight                                                                             int32
}
type ballRow struct {
	Scenario      string
	Frame         int
	Before, After observedBall
}

func ballRows(t *testing.T) []ballRow {
	t.Helper()
	b, err := os.ReadFile("testdata/ball.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []ballRow
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1194 {
		t.Fatalf("unexpected original ball observation count %d", len(rows))
	}
	return rows
}

func ballFromObservation(r observedBall) Ball {
	next := ActionIdle
	if r.NextAction < 128 {
		next = Action(r.NextAction)
	}
	return Ball{Body: Body{Position: Position{X: Fixed(r.X << 8 >> 8), Depth: Fixed(r.Depth), Height: Fixed(r.Height << 8 >> 8)}, Velocity: Velocity{X: Fixed(int16(r.VX)), Depth: Fixed(int16(r.VY)), Height: Fixed(int16(r.VZ))}, Gravity: Fixed(int16(r.Gravity))}, Action: BallAction(r.Action & 127), Entered: r.Action&128 != 0, FacingLeft: r.Facing&128 != 0, Pose: Pose{Frame: uint8(r.Pose & 127), Mirrored: r.Pose&128 != 0}, Animation: AnimationState{Cursor: uint8(r.Cursor), Elapsed: uint8(r.Clock), Loops: uint8(r.RepeatCount), NextAction: next, HasNextAction: r.NextAction < 128}, Airborne: r.Air&128 != 0, BouncesRemaining: uint8(r.Bounces), Request: BallRequest{Action: BallAction(r.Request & 127), Pending: r.Request&128 == 0}, Deflection: DeflectionAxes(r.Deflection)}
}

func TestOrdinaryShotTargetsAgainstOriginal(t *testing.T) {
	count := 0
	for _, r := range ballRows(t) {
		if r.Before.Action != 4 || r.Before.SpecialFlags != 0 {
			continue
		}
		plan, err := ChooseOrdinaryShot(ballFromObservation(r.Before).Body.Position, Fixed(r.Before.ShooterHeight), r.Before.Facing&128 != 0, r.Before.RimFlags)
		if err != nil {
			t.Fatalf("%s frame %d: %v", r.Scenario, r.Frame, err)
		}
		want := ShotPlan{Target: Position{X: Fixed(r.After.TargetX) * Pixel, Depth: Fixed(r.After.TargetDepth) * Pixel, Height: Fixed(r.After.TargetHeight) * Pixel}, ArcHeight: Fixed(r.After.ArcHeight) * Pixel, Hoop: uint8(r.After.Hoop - 7)}
		if plan != want {
			t.Fatalf("%s frame %d: got %+v want %+v", r.Scenario, r.Frame, plan, want)
		}
		count++
	}
	if count < 5 {
		t.Fatal("missing ordinary shot launches", count)
	}
	t.Logf("matched %d original ordinary shot targets", count)
}

func TestShotTierBoundaries(t *testing.T) {
	// Thresholds and heights are from the original fixed-bank target tables.
	for _, c := range []struct {
		ball, shooter Fixed
		tier          uint8
		height        Fixed
	}{{95, 55, 0, 88}, {96, 56, 1, 184}, {191, 151, 1, 184}, {192, 152, 2, 280}} {
		for _, left := range []bool{false, true} {
			rims := [6]uint8{3, 3, 3, 3, 3, 3}
			got, err := ChooseOrdinaryShot(Position{Height: c.ball * Pixel}, c.shooter*Pixel, left, rims)
			index := c.tier*2 + 1
			x := Fixed(460) * Pixel
			if left {
				index--
				x = 68 * Pixel
			}
			if err != nil || got.Target != (Position{X: x, Depth: 47 * Pixel, Height: c.height * Pixel}) || got.Hoop != index || got.ArcHeight != 48*Pixel {
				t.Fatal(c, left, got, err)
			}
		}
	}
}

func TestShotLaunchAgainstOriginal(t *testing.T) {
	var rows []struct {
		Scenario                                                                             string
		X, Depth, Height, TargetX, TargetDepth, TargetHeight, ArcHeight, VX, VY, VZ, Gravity int32
	}
	b, err := os.ReadFile("testdata/shot-launch.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 21 {
		t.Fatal("unexpected shot launch count", len(rows))
	}
	for _, r := range rows {
		pos := Position{X: Fixed(r.X) * Pixel, Depth: Fixed(r.Depth) * Pixel, Height: Fixed(r.Height) * Pixel}
		plan := ShotPlan{Target: Position{X: Fixed(r.TargetX) * Pixel, Depth: Fixed(r.TargetDepth) * Pixel, Height: Fixed(r.TargetHeight) * Pixel}, ArcHeight: Fixed(r.ArcHeight) * Pixel}
		got, err := LaunchShot(pos, plan)
		want := Body{Position: pos, Velocity: Velocity{X: Fixed(int16(r.VX)), Depth: Fixed(int16(r.VY)), Height: Fixed(int16(r.VZ))}, Gravity: Fixed(r.Gravity)}
		if err != nil || got != want {
			t.Fatalf("%s: got %+v want %+v err %v", r.Scenario, got, want, err)
		}
	}
	t.Logf("matched %d original shot launch calculations", len(rows))
}

func TestShotFlightAgainstOriginal(t *testing.T) {
	count := 0
	for _, r := range ballRows(t) {
		if r.Before.Action&127 != 4 || r.After.Action&127 != 4 {
			continue
		}
		plan := ShotPlan{Target: Position{X: Fixed(r.After.TargetX) * Pixel, Depth: Fixed(r.After.TargetDepth) * Pixel, Height: Fixed(r.After.TargetHeight) * Pixel}, ArcHeight: Fixed(r.After.ArcHeight) * Pixel, Hoop: uint8(r.After.Hoop)}
		got, err := StepShotBall(ballFromObservation(r.Before), plan, NewYorkBallCourt())
		if err != nil {
			t.Fatalf("%s frame %d: %v", r.Scenario, r.Frame, err)
		}
		if want := ballFromObservation(r.After); got != want {
			t.Fatalf("%s frame %d\nbefore %+v\ngot %+v\nwant %+v", r.Scenario, r.Frame, ballFromObservation(r.Before), got, want)
		}
		count++
	}
	if count != 288 {
		t.Fatal("unexpected shot flight count", count)
	}
	t.Logf("matched %d original shot flight calls, excluding basket-contact transitions", count)
}

func TestBallBouncesAgainstOriginal(t *testing.T) {
	count := 0
	for _, r := range ballRows(t) {
		if r.Before.Action&127 != 6 || r.After.Action&127 != 6 {
			continue
		}
		got, err := StepBouncingBall(ballFromObservation(r.Before), NewYorkBallCourt())
		if err != nil {
			t.Fatalf("%s frame %d: %v", r.Scenario, r.Frame, err)
		}
		if want := ballFromObservation(r.After); got != want {
			t.Fatalf("%s frame %d\nbefore %+v\ngot %+v\nwant %+v", r.Scenario, r.Frame, ballFromObservation(r.Before), got, want)
		}
		count++
	}
	if count != 165 {
		t.Fatal("unexpected bounce count", count)
	}
	t.Logf("matched %d original bounce calls, excluding external-contact transitions", count)
}

func TestBallFloorContactKeepsHorizontalAdvance(t *testing.T) {
	b := Ball{Action: BallShot, Entered: true, Airborne: true, Body: Body{Position: Position{X: 200*Pixel + 119, Depth: 40*Pixel + 33, Height: Pixel}, Velocity: Velocity{X: Pixel, Height: -2 * Pixel}, Gravity: 128}}
	got, err := StepShotBall(b, ShotPlan{}, NewYorkBallCourt())
	if err != nil {
		t.Fatal(err)
	}
	if got.Body.Position != (Position{X: 201*Pixel + 119, Depth: 40*Pixel + 33, Height: 0}) || got.Body.Velocity.Height != 0 || got.Body.Gravity != 0 || got.Airborne || got.Action != BallBounce || got.Entered {
		t.Fatal("incorrect first floor contact", got)
	}
}

func TestBallFloorFractionClampsBeforeNextLandingUpdate(t *testing.T) {
	b := Ball{Action: BallShot, Entered: true, Airborne: true, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel, Height: 200}, Gravity: 128}}
	got, err := StepShotBall(b, ShotPlan{}, NewYorkBallCourt())
	if err != nil {
		t.Fatal(err)
	}
	if got.Body.Position.Height != 0 || got.Body.Velocity.Height != -128 || !got.Airborne || got.Action != BallShot {
		t.Fatal("floor equality must retain the original one-update airborne phase", got)
	}
}
