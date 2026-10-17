package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestHeldBallOriginalPassageTerrain(t *testing.T) {
	data, err := os.ReadFile("testdata/held-terrain.json")
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		WorldFrame, BallFrame int
		Carrier               Player
		Before, After         observedBall
	}
	if err = json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	// The ball call crosses a video boundary inside its containing world step.
	if r.WorldFrame != 3652 || r.BallFrame != 3653 || r.Carrier.Body.Position.Height != -256*Pixel {
		t.Fatal("wrong original passage evidence")
	}
	got, err := StepHeldBall(ballFromObservation(r.Before), r.Carrier)
	want := ballFromObservation(r.After)
	if err != nil || got != want {
		t.Fatalf("got %+v want %+v err %v", got, want, err)
	}
}

func TestHeldFallingBallOriginalOffset(t *testing.T) {
	// Fixed $ECF5[$0F] -> index 2 -> $ED8D {6,0,8,1}, shared with catches.
	b := Ball{Action: BallHeld, Entered: true, Body: Body{Position: Position{X: 200*Pixel + 119, Depth: 50*Pixel + 33, Height: 10*Pixel + 11}}}
	p := Player{Action: ActionFalling, Body: Body{Position: Position{X: 100*Pixel + 77, Depth: 40*Pixel + 99, Height: 20*Pixel + 128}}}
	got, err := StepHeldBall(b, p)
	if err != nil || got.Body.Position != (Position{X: 106*Pixel + 119, Depth: 39*Pixel + 33, Height: 28*Pixel + 11}) {
		t.Fatal(got, err)
	}
}

func TestHeldDunkOffsetsAndPoseOverride(t *testing.T) {
	for _, mirrored := range []bool{false, true} {
		for _, pose := range []uint8{29, 30, 59} {
			b := Ball{Action: BallHeld, Entered: true, FacingLeft: true, Body: Body{Position: Position{X: 200*Pixel + 119, Depth: 50*Pixel + 33, Height: 10*Pixel + 11}}}
			p := Player{Action: 0x58, Pose: Pose{Frame: pose, Mirrored: mirrored}, Body: Body{Position: Position{X: 100*Pixel + 77, Depth: 40*Pixel + 99, Height: 20*Pixel + 128}}}
			dx, height, depth := -4*Pixel, 18*Pixel, Pixel
			if pose == 30 {
				dx, height, depth = 4*Pixel, 6*Pixel, -Pixel
			}
			if mirrored {
				dx = -dx
			}
			got, err := StepHeldBall(b, p)
			want := Position{X: 100*Pixel + dx + 119, Depth: 40*Pixel + depth + 33, Height: 20*Pixel + height + 11}
			if err != nil || got.Body.Position != want {
				t.Fatal(pose, mirrored, got, want, err)
			}
			if pose == 59 && got.Pose != (Pose{Frame: 127}) {
				t.Fatal("hidden sprite retained mirror", got.Pose)
			}
		}
	}
}
