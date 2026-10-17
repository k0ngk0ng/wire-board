package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestPassLaunchAgainstOriginal(t *testing.T) {
	var rows []struct {
		Scenario                                                        string
		X, Depth, Height, TargetX, TargetDepth, TargetHeight, ArcHeight int32
		VX, VY, VZ, Gravity                                             int32
	}
	b, err := os.ReadFile("testdata/pass-launch.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 21 {
		t.Fatalf("unexpected original pass count %d", len(rows))
	}
	for _, r := range rows {
		position := Position{X: Fixed(r.X) * Pixel, Depth: Fixed(r.Depth) * Pixel, Height: Fixed(r.Height) * Pixel}
		target := Position{X: Fixed(r.TargetX) * Pixel, Depth: Fixed(r.TargetDepth) * Pixel, Height: Fixed(r.TargetHeight) * Pixel}
		got, err := LaunchPass(position, target, Fixed(r.ArcHeight)*Pixel)
		if err != nil {
			t.Fatalf("%s: %v", r.Scenario, err)
		}
		want := Body{Position: position, Velocity: Velocity{X: Fixed(int16(r.VX)), Depth: Fixed(int16(r.VY)), Height: Fixed(int16(r.VZ))}, Gravity: Fixed(r.Gravity)}
		if got != want {
			t.Fatalf("%s: got %+v want %+v", r.Scenario, got, want)
		}
	}
	t.Logf("matched %d original pass launches", len(rows))
}
