package snooker

// Failure inventory is written in docs/snooker-implementation.md before code.
// Physics needs isolated verification: complete games cannot prove no tunnelling,
// time-aligned impacts, deterministic motion or energy dissipation.
import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func testBall(id int, x, y float64) Ball { return Ball{ID: id, Pos: Vec{X: x, Y: y}} }
func saveMotion(t *testing.T, m Motion) {
	t.Helper()
	dir := "../../../.local/snooker/verification"
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	if e := os.WriteFile(filepath.Join(dir, t.Name()+".json"), b, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestPhysicsDeterministicAndNoInputMutation(t *testing.T) {
	balls := InitialBalls()
	before := append([]Ball{}, balls...)
	a, e := Simulate(balls, Strike{Angle: 0.04, Power: 0.8})
	if e != nil {
		t.Fatal(e)
	}
	b, e := Simulate(balls, Strike{Angle: 0.04, Power: 0.8})
	if e != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("non-deterministic simulation", e)
	}
	if !reflect.DeepEqual(balls, before) {
		t.Fatal("mutated input")
	}
	if a.Duration <= 0 || a.Duration > 30 || len(a.Frames) < 2 {
		t.Fatal("invalid timeline")
	}
	for _, ball := range a.Balls {
		if !ball.Down && (ball.Pos.X < -0.12 || ball.Pos.X > TableWidth+0.12 || ball.Pos.Y < -0.12 || ball.Pos.Y > TableHeight+0.12) {
			t.Fatal("escaped table", ball)
		}
	}
	saveMotion(t, a)
}
func TestPhysicsFastCollisionDoesNotTunnel(t *testing.T) {
	bs := []Ball{testBall(0, 0.7, 0.7), testBall(1, 0.8, 0.7)}
	m, e := Simulate(bs, Strike{Power: 1})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.First) != 1 || m.First[0] != 1 {
		t.Fatal("missed first object", m.First)
	}
	maxX := 0.8
	for _, frame := range m.Frames {
		for _, ball := range frame.Balls {
			if ball.ID == 1 {
				maxX = math.Max(maxX, ball.Pos.X)
			}
		}
	}
	if maxX < 1.2 {
		t.Fatal("no momentum transfer before cushion rebound")
	}
	last := -1.0
	for _, ev := range m.Events {
		if ev.Time < last {
			t.Fatal("unordered impacts")
		}
		last = ev.Time
	}
	saveMotion(t, m)
}
func TestPhysicsDrawAndFollowChangeCuePosition(t *testing.T) {
	bs := []Ball{testBall(0, 1.0, 0.65), testBall(1, 1.16, 0.65)}
	draw, e := Simulate(bs, Strike{Power: .42, Top: -.8})
	if e != nil {
		t.Fatal(e)
	}
	follow, e := Simulate(bs, Strike{Power: .42, Top: .8})
	if e != nil {
		t.Fatal(e)
	}
	if draw.Balls[0].Pos.X >= 1.16-2*BallRadius || follow.Balls[0].Pos.X <= draw.Balls[0].Pos.X+.1 {
		t.Fatal("spin did not affect cue motion", draw.Balls[0], follow.Balls[0])
	}
	saveMotion(t, draw)
}
func TestPhysicsSideSpinChangesCushionRebound(t *testing.T) {
	bs := []Ball{testBall(0, 1.0, .25)}
	left, e := Simulate(bs, Strike{Angle: -1.1, Power: .5, Side: -.8})
	if e != nil {
		t.Fatal(e)
	}
	right, e := Simulate(bs, Strike{Angle: -1.1, Power: .5, Side: .8})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(left.Balls[0].Pos.X-right.Balls[0].Pos.X) < .06 {
		t.Fatal("side spin is decorative", left.Balls, right.Balls)
	}
}
func TestPhysicsPocketTimelineAndTouchingEscape(t *testing.T) {
	bs := []Ball{testBall(0, TableWidth/2, .6), testBall(1, TableWidth/2, .23)}
	m, e := Simulate(bs, Strike{Angle: -math.Pi / 2, Power: .32})
	if e != nil {
		t.Fatal(e)
	}
	if !m.Balls[1].Down {
		t.Fatal("middle pocket swallowed no ball", m.Balls)
	}
	found := false
	for _, ev := range m.Events {
		if ev.Kind == "pot" && ev.Ball == 1 {
			found = true
			if ev.Time <= 0 || ev.Time > m.Duration {
				t.Fatal("bad pocket time")
			}
		}
	}
	if !found {
		t.Fatal("no timed pocket event")
	}
	touching := []Ball{testBall(0, 1, .5), testBall(1, 1+2*BallRadius, .5)}
	away, e := Simulate(touching, Strike{Angle: math.Pi, Power: .1})
	if e != nil {
		t.Fatal(e)
	}
	if len(away.First) != 0 {
		t.Fatal("moving away from touching ball counted as strike")
	}
	saveMotion(t, m)
}
func TestPhysicsRejectsMalformedShotsBeforeChangingBalls(t *testing.T) {
	bs := InitialBalls()
	before := append([]Ball{}, bs...)
	for _, shot := range []Strike{{Power: 0}, {Power: 1.1}, {Power: -1}, {Power: .5, Angle: math.NaN()}, {Power: .5, Side: 1}, {Power: .5, Top: .8, Side: .8}, {Power: .5, Angle: math.Inf(1)}} {
		if _, e := Simulate(bs, shot); e == nil {
			t.Fatal("accepted malformed shot", shot)
		}
	}
	if !reflect.DeepEqual(before, bs) {
		t.Fatal("rejection mutated balls")
	}
	overlapping := []Ball{testBall(0, 1, 1), testBall(1, 1, 1)}
	if _, e := Simulate(overlapping, Strike{Power: .5}); e == nil {
		t.Fatal("accepted coincident balls")
	}
}
