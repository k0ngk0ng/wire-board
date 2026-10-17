package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestRestRollingCompleteOriginalDispatches(t *testing.T) {
	file, err := os.Open("testdata/ball-rest-rolling.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		worldActionFixture
		CameraShake uint8
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 221 {
		t.Fatal("incomplete natural rest/rolling corpus", len(rows))
	}
	counts := [2]int{}
	for i, row := range rows {
		state := row.Before
		ctx := row.context(t)
		ctx.CameraShake = row.CameraShake
		if err := advanceWorldBall(&state, ctx); err != nil {
			t.Fatalf("dispatch %d frame %d: %v", i, row.Frame, err)
		}
		if state != row.After {
			t.Fatalf("dispatch %d frame %d: %s", i, row.Frame, firstWorldLoopDifference(reflect.ValueOf(state), reflect.ValueOf(row.After), "state"))
		}
		if row.Before.Ball.Action == BallRest {
			counts[0]++
		} else {
			counts[1]++
		}
	}
	if counts != ([2]int{192, 29}) {
		t.Fatal(counts)
	}
	t.Log("all 221 original natural dispatches match: 192 resting / 29 rolling, full world action state")
}

func TestRestBallRetainsAnimationAndShakePose(t *testing.T) {
	original := Ball{Action: BallRest, Pose: Pose{Frame: 3, Mirrored: true}, Animation: AnimationState{Cursor: 2, Elapsed: 1}, Body: Body{Position: Position{X: 100 * Pixel, Depth: 40 * Pixel, Height: 50 * Pixel}, Velocity: Velocity{X: 800, Depth: 50, Height: 256}, Gravity: 256}, Airborne: true}
	for _, shake := range []uint8{0, 1} {
		got, err := StepRestingBall(original, RestBallContext{CameraShake: shake}, NewYorkBallCourt())
		if err != nil || got.Entered || got.Animation != original.Animation || got.Body.Velocity != (Velocity{}) || got.Body.Position != original.Body.Position {
			t.Fatal(got, err)
		}
		want := Pose{}
		if shake != 0 {
			want = original.Pose
		}
		if got.Pose != want {
			t.Fatal("rest pose while shaking", shake, got.Pose)
		}
	}
}

func TestRollingBallOriginalFrictionAndStopOrder(t *testing.T) {
	for _, tc := range []struct {
		x, z, wantX, wantZ Fixed
		stop               bool
	}{{768, 50, 704, -14, false}, {-768, -14, -704, 50, false}, {1, 10, 0, -54, false}, {-1, -10, 0, 54, false}, {0, 50, 0, 50, true}} {
		b := Ball{Action: BallRolling, Entered: true, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, Velocity: Velocity{X: tc.x, Depth: tc.z}}}
		got, err := StepRollingBall(b, NewYorkBallCourt())
		if err != nil || got.Ball.Body.Position.X != b.Body.Position.X+tc.x || got.Ball.Body.Velocity.X != tc.wantX || got.Ball.Body.Velocity.Depth != tc.wantZ {
			t.Fatal(tc, got, err)
		}
		if (got.Ball.Action == BallRest) != tc.stop || got.ClearExcludedCatcher {
			t.Fatal("friction-stop phase order", tc, got)
		}
	}
	b := Ball{Action: BallRolling, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, Velocity: Velocity{X: 100}}}
	first, err := StepRollingBall(b, NewYorkBallCourt())
	if err != nil || !first.ClearExcludedCatcher || !first.Ball.Entered {
		t.Fatal(first, err)
	}
	again, err := StepRollingBall(first.Ball, NewYorkBallCourt())
	if err != nil || again.ClearExcludedCatcher {
		t.Fatal(again, err)
	}
}

func TestRestBallRaisedSurfaceReleasesIntoBounce(t *testing.T) {
	b := Ball{Action: BallRest, Body: Body{Position: Position{X: 100 * Pixel, Depth: 72 * Pixel, Height: 64 * Pixel}}}
	got, err := StepRestingBall(b, RestBallContext{}, NewYorkBallCourt())
	if err != nil || got.Action != BallBounce || got.Entered || got.Body.Velocity.Depth != -Pixel/2 {
		t.Fatal(got, err)
	}
}

func TestRestBallLandingThenRaisedReleaseAreSeparateUpdates(t *testing.T) {
	b := Ball{Action: BallRest, Airborne: true, Body: Body{Position: Position{X: 100 * Pixel, Depth: 72 * Pixel, Height: 64 * Pixel}, Velocity: Velocity{Height: -Pixel}}}
	landed, err := StepRestingBall(b, RestBallContext{}, NewYorkBallCourt())
	if err != nil || landed.Action != BallRest || landed.Airborne || landed.Body.Position.Height != 64*Pixel {
		t.Fatal("landing branch must not run equal-height release", landed, err)
	}
	released, err := StepRestingBall(landed, RestBallContext{}, NewYorkBallCourt())
	if err != nil || released.Action != BallBounce || released.Body.Velocity.Depth != -Pixel/2 {
		t.Fatal(released, err)
	}
}
