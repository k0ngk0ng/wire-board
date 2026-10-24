package tabletennis

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"reflect"
	"testing"
	"time"
)

// Failure inventory is in CONTRACT.md before the protocol/physics changes.
func TestPaddleMovementIsPublicAndIndependent(t *testing.T) {
	s := fixture(t, 4)
	m, _ := s.match()
	y := .65
	a := Action{Type: "move", Epoch: s.Epoch, MotionSeq: 1, Ends: s.Ends, RacketX: 1.4, RacketY: &y, RacketZ: 1.4, At: s.ServerAt + 30}
	ctx := gameapi.Context{Players: 4, Actor: 1, Now: time.UnixMilli(a.At)}
	b, _ := json.Marshal(a)
	next, e := Module().Apply(ctx, m, b)
	if e != nil {
		t.Fatal(e)
	}
	n, _ := decode(next)
	if n.Score != s.Score || !reflect.DeepEqual(n.Flight, s.Flight) || n.ServerAt != s.ServerAt || n.Seq[1] != 0 || n.ServeDeadline != s.ServeDeadline {
		t.Fatal("movement changed match/strike clock")
	}
	for _, viewer := range []int{-1, 0, 3} {
		v, _ := Module().View(next, viewer)
		p := v.(View).Paddles[1]
		if p.X != 1.4 || p.Y != y || p.Z != 1.4 || p.MotionSeq != 1 {
			t.Fatal("movement not visible", p)
		}
	}
	a.RacketX = -1
	b, _ = json.Marshal(a)
	duplicate, e := Module().Apply(ctx, next, b)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(duplicate, next) {
		t.Fatal("duplicate movement replaced newer coordinates")
	}
	a.MotionSeq = 2
	a.RacketY = nil
	b, _ = json.Marshal(a)
	if _, e = Module().Apply(ctx, next, b); e == nil {
		t.Fatal("missing height accepted")
	}
	a.RacketY = &y
	a.RacketX = 100
	b, _ = json.Marshal(a)
	if _, e = Module().Apply(ctx, next, b); e == nil {
		t.Fatal("unbounded movement accepted")
	}
}

func TestBotPreparesVisiblePaddleBeforeServing(t *testing.T) {
	s := fixture(t, 2)
	s.Phase = "serve"
	s.PhaseUntil = s.ServerAt
	s.ServeDeadline = s.ServerAt + ServeTimeoutMS
	s.Expected = 0
	m, _ := s.match()
	ctx := gameapi.Context{Players: 2, Actor: 0, Now: time.UnixMilli(s.ServerAt + 100)}
	raw, e := Module().BotAction(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	var a Action
	if json.Unmarshal(raw, &a) != nil || a.Type != "move" {
		t.Fatal("computer has no visible preparation", string(raw))
	}
	next, e := Module().Apply(ctx, m, raw)
	if e != nil {
		t.Fatal(e)
	}
	n, _ := decode(next)
	if n.Paddles[0].MotionSeq != 1 || n.Shot != s.Shot {
		t.Fatal("preparation bypassed normal movement")
	}
}

func TestPaddleHeightControlsContact(t *testing.T) {
	for _, height := range []float64{.82, 1.6} {
		s := fixture(t, 2)
		s.Phase = "rally"
		s.Expected = 0
		s.Receiver = 0
		s.Flight = &Flight{ReceiveAt: s.ServerAt - 50, ContactStart: s.ServerAt - 25, ContactEnd: s.ServerAt + 400, EndAt: s.ServerAt + 700, Segments: []Segment{{At: s.ServerAt, Until: s.ServerAt + 700, P: Vec{.1, .82, 1.1}}}}
		a := Action{Type: "swing", Epoch: s.Epoch, Rally: s.Rally, Shot: s.Shot, Seq: 1, Kind: "drive", Power: .6, RacketX: .1 / .9, RacketY: &height, At: s.ServerAt}
		b, _ := json.Marshal(a)
		m, _ := s.match()
		n, e := Module().Apply(gameapi.Context{Players: 2, Actor: 0, Now: time.UnixMilli(s.ServerAt)}, m, b)
		if e != nil {
			t.Fatal(e)
		}
		after, _ := decode(n)
		if (after.Paddles[0].Result == "miss") != (height > 1) {
			t.Fatal("height ignored", height, after.Paddles[0])
		}
		a.RacketZ = 1e9
		b, _ = json.Marshal(a)
		if _, e = Module().Apply(gameapi.Context{Players: 2, Actor: 0, Now: time.UnixMilli(s.ServerAt)}, m, b); e == nil {
			t.Fatal("unbounded stroke depth accepted")
		}
	}
}

func TestFastStrokeProducesFasterFlightAndLowLiftClearsNet(t *testing.T) {
	launch := func(power, height, z, lift float64) Flight {
		s := fixture(t, 2)
		s.Phase = "rally"
		s.Expected = 0
		s.Flight = &Flight{ContactStart: s.ServerAt - 130, Segments: []Segment{{At: s.ServerAt, Until: s.ServerAt + 500, P: Vec{0, height, z}}}}
		s.launch(Action{Kind: "drive", Power: power, Lift: lift}, 0, s.ServerAt, false)
		return *s.Flight
	}
	slow, fast := launch(.2, 1.05, 1.2, 1), launch(1, 1.05, 1.2, 1)
	speed := func(f Flight) float64 { v := f.Segments[0].V; return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z) }
	t.Logf("same-target slow %.3fm/s in %dms; fast %.3fm/s in %dms", speed(slow), slow.ReceiveAt-slow.StartAt, speed(fast), fast.ReceiveAt-fast.StartAt)
	// Compare forward travel as well as total launch speed: a slow high arc
	// contains vertical velocity without bringing the ball to the receiver sooner.
	if speed(fast) <= speed(slow) || math.Abs(fast.Segments[0].V.Z) < math.Abs(slow.Segments[0].V.Z)*2 {
		t.Fatal("fast stroke is not materially faster", speed(slow), speed(fast))
	}
	if fast.ReceiveAt-fast.StartAt >= (slow.ReceiveAt-slow.StartAt)*3/4 {
		t.Fatal("flight duration barely changed")
	}
	for _, z := range []float64{.5, 1.1, 1.45} {
		f := launch(.7, .79, z, 1)
		if f.ReceiveAt == 0 || f.Reason == "net" || f.Reason == "out" {
			t.Fatal("upward low-ball stroke cannot reach opposite table", z, f.Reason)
		}
	}
}
