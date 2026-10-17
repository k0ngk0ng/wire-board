package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func readCameraFixture(t *testing.T, name string, target any) {
	t.Helper()
	f, err := os.Open("testdata/" + name + "-stages.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if err := json.NewDecoder(z).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func TestOriginalCompleteCameraStages(t *testing.T) {
	var rows []struct {
		Frame         int
		Before, After CameraState
		Context       CameraContext
	}
	readCameraFixture(t, "camera", &rows)
	if len(rows) != 1997 {
		t.Fatal("missing complete stages", len(rows))
	}
	for _, r := range rows {
		got, err := StepOriginalCamera(r.Before, r.Context)
		if err != nil || got.State != r.After {
			t.Fatalf("frame %d before %+v got %+v want %+v error %v", r.Frame, r.Before, got, r.After, err)
		}
	}
}

func TestOriginalCameraBoundariesAndShake(t *testing.T) {
	c := CameraContext{TrackedObject: 6, BallHolder: 6}
	c.Projected[6].Y = 24
	for _, r := range []struct {
		name                  string
		shown, x, y, target   uint16
		shake                 uint8
		wantX, wantY          uint16
		wantColumn, wantShake uint8
		stream                bool
	}{
		{"left limited", 100, 100, 0x120, 200, 0, 92, 0x120, 12, 0, true},
		{"right limited", 100, 100, 0x120, 260, 0, 108, 0x120, 44, 0, true},
		{"lower equality", 100, 88, 0x120, 212, 0, 88, 0x120, 9, 0, false},
		{"upper equality", 100, 88, 0x120, 244, 0, 88, 0x120, 9, 0, false},
		{"left clamp", 6, 6, 0x120, 100, 0, 6, 0x120, 0, 0, true},
		{"right clamp", 255, 255, 0x120, 500, 0, 255, 0x120, 63, 0, true},
		{"shake subtract", 100, 100, 0x120, 220, 2, 100, 0x11e, 9, 1, false},
		{"shake add", 100, 100, 0x120, 220, 1, 100, 0x122, 9, 0, false},
	} {
		t.Run(r.name, func(t *testing.T) {
			c.Positions[6].X = Fixed(r.target) * Pixel
			got, err := StepOriginalCamera(CameraState{DisplayedX: r.shown, X: r.x, Y: r.y, TileColumn: 9, Shake: r.shake}, c)
			want := CameraState{DisplayedX: r.shown, X: r.wantX, Y: r.wantY, TileColumn: r.wantColumn, Shake: r.wantShake}
			if err != nil || got.State != want || got.StreamColumn != r.stream {
				t.Fatal(got, want, err)
			}
		})
	}
	c.Positions[6].X = 220 * Pixel
	c.BallHolder = 2
	c.Projected[6].Y = 200 // holder projection, not tracked ball projection, chooses Y.
	for _, r := range []struct {
		before uint16
		target int16
		after  uint16
	}{{0xe8, 40, 0x100}, {0x104, 0, 0xe4}, {0x125, 40, 0x129}, {0x31, 0, 0x30}} {
		c.Projected[2].Y = r.target
		got, err := StepOriginalCamera(CameraState{DisplayedX: 100, X: 100, Y: r.before}, c)
		if err != nil || got.State.Y != r.after {
			t.Fatal(r, got, err)
		}
	}
}

func TestOriginalCrowdingStagesAndEdges(t *testing.T) {
	var rows []struct {
		Frame         int
		Before, After uint8
		Projected     [4]ScreenPosition
	}
	readCameraFixture(t, "crowding", &rows)
	if len(rows) != 1997 {
		t.Fatal(len(rows))
	}
	for _, r := range rows {
		if got := OriginalCrowdingFlags(r.Before, r.Projected); got != r.After {
			t.Fatalf("frame %d got %02x want %02x", r.Frame, got, r.After)
		}
	}
	p := [4]ScreenPosition{{X: 0, Y: 100}, {X: 250, Y: 77}, {X: 128, Y: 124}, {X: -1, Y: 100}}
	if got := OriginalCrowdingFlags(0x15, p); got != 0x95 {
		t.Fatal("two later rows inside asymmetric limits", got)
	}
	p[1].Y = 76 // shifted candidate 100 equals lower exclusive bound.
	if got := OriginalCrowdingFlags(0x95, p); got != 0x15 {
		t.Fatal("exclusive lower bound or low bits", got)
	}
}
