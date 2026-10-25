package tabletennis

// Boundary fixtures specified BEFORE implementation; full natural matches are
// separate in ../tests. Failure inventory: ../CONTRACT.md items 1,3–9.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"reflect"
	"testing"
	"time"
)

func fixture(t *testing.T, n int) State {
	t.Helper()
	m, e := Module().Start(gameapi.Context{Players: n, Now: time.Unix(100000, 0)}, nil)
	if e != nil {
		t.Fatal(e)
	}
	s, e := decode(m)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestServiceOrderAndDeuce(t *testing.T) {
	s := fixture(t, 4)
	s.Order = []int{0, 1, 2, 3}
	s.InitialOrder = append([]int{}, s.Order...)
	for _, c := range []struct {
		score [2]int
		want  [2]int
	}{{[2]int{0, 0}, [2]int{0, 1}}, {[2]int{1, 0}, [2]int{0, 1}}, {[2]int{2, 0}, [2]int{1, 2}}, {[2]int{3, 1}, [2]int{2, 3}}, {[2]int{4, 2}, [2]int{3, 0}}, {[2]int{10, 10}, [2]int{2, 3}}, {[2]int{11, 10}, [2]int{3, 0}}, {[2]int{11, 11}, [2]int{0, 1}}} {
		s.Score = c.score
		a, b := s.service()
		if [2]int{a, b} != c.want {
			t.Fatalf("%v: %d,%d want %v", c.score, a, b, c.want)
		}
	}
	s.Score = [2]int{10, 10}
	s.Options.BestOf = 1
	s.PointWinner = 0
	s.award(0, "test-boundary", s.ServerAt)
	if s.Result != nil || s.Score != [2]int{11, 10} {
		t.Fatal("11:10 must continue")
	}
	s.award(0, "test-boundary", s.ServerAt)
	if s.Result == nil || len(s.Result.Winners) != 2 {
		t.Fatal("12:10 must finish for a team")
	}
}
func TestDoublesReturnOrderAndNewGame(t *testing.T) {
	s := fixture(t, 4)
	s.Order = []int{0, 1, 2, 3}
	for seat, want := range map[int]int{0: 1, 1: 2, 2: 3, 3: 0} {
		if s.nextSeat(seat) != want {
			t.Fatal("not four-seat alternation")
		}
	}
	s.InitialOrder = []int{0, 1, 2, 3}
	s.Options.BestOf = 3
	s.Score = [2]int{10, 4}
	s.award(0, "test-boundary", s.ServerAt)
	if s.Phase != "game_break" || s.GamesWon != [2]int{1, 0} {
		t.Fatal("missing game break")
	}
	s.newGame(s.ServerAt + 3000)
	if !reflect.DeepEqual(s.Order, []int{1, 0, 3, 2}) {
		t.Fatalf("new game serving/receiving relationship: %v", s.Order)
	}
	s = fixture(t, 4)
	s.Order = []int{0, 1, 2, 3}
	s.Options.BestOf = 1
	s.Score = [2]int{4, 0}
	s.award(0, "test-boundary", s.ServerAt)
	if !s.EndsSwapped || s.Ends != 1 {
		t.Fatal("decider ends did not swap")
	}
	a, b := s.service()
	if a != 2 || b != 1 {
		t.Fatalf("receiver must swap after fifth point: %d,%d", a, b)
	}
	order := append([]int{}, s.Order...)
	s.award(1, "test-boundary", s.ServerAt)
	if !reflect.DeepEqual(order, s.Order) {
		t.Fatal("repeated end swap")
	}
}
func TestBallisticNetTableAndSpin(t *testing.T) {
	for _, spin := range []float64{-1, 0, 1} {
		f := traceFlight(Vec{0, 1.05, 1.1}, Vec{0, 1.6, -3.8}, spin, 1000, 0, 1, false, false)
		if len(f.Segments) < 2 || f.ReceiveAt == 0 || f.ContactEnd <= f.ContactStart {
			t.Fatalf("no playable bounce: %+v", f)
		}
		for _, seg := range f.Segments {
			for _, n := range []float64{seg.P.X, seg.P.Y, seg.P.Z, seg.V.X, seg.V.Y, seg.V.Z} {
				if math.IsNaN(n) || math.IsInf(n, 0) {
					t.Fatal("nonfinite trajectory")
				}
			}
		}
	}
	top := traceFlight(Vec{0, 1.05, 1.1}, Vec{0, 1.6, -3.8}, 1, 1000, 0, 1, false, false)
	cut := traceFlight(Vec{0, 1.05, 1.1}, Vec{0, 1.6, -3.8}, -1, 1000, 0, 1, false, false)
	bounceSpeed := func(f Flight) float64 {
		for _, segment := range f.Segments {
			if segment.At == f.ReceiveAt {
				return math.Abs(segment.V.Z)
			}
		}
		t.Fatal("missing post-bounce segment")
		return 0
	}
	if bounceSpeed(top) <= bounceSpeed(cut) {
		t.Fatal("topspin must rebound faster than backspin")
	}
	net := traceFlight(Vec{0, .83, .2}, Vec{0, 0, -10}, 0, 1000, 0, 1, false, false)
	if net.Reason != "net" || net.ReceiveAt != 0 {
		t.Fatal("fast ball passed through net")
	}
	wrong := traceFlight(Vec{0, 1, .5}, Vec{0, 0, .2}, 0, 1000, 0, 1, false, false)
	if wrong.Reason != "wrong_side" {
		t.Fatalf("own-side bounce: %s", wrong.Reason)
	}
	out := traceFlight(Vec{1.2, 1, 1}, Vec{0, 1, -3}, 0, 1000, 0, 1, false, false)
	if out.Reason != "out" || out.ReceiveAt != 0 {
		t.Fatal("off-table flight awarded bounce")
	}
}
func TestInputBoundaryFixtures(t *testing.T) {
	s := fixture(t, 2)
	s.Phase = "serve"
	s.PhaseUntil = s.ServerAt
	s.Expected, s.Receiver = s.service()
	m, _ := s.match()
	ctx := gameapi.Context{Players: 2, Actor: s.Expected, Now: time.UnixMilli(s.ServerAt)}
	a := Action{Type: "swing", Epoch: s.Epoch, Rally: s.Rally, Shot: s.Shot, Seq: 1, Kind: "drive", Power: .5, At: s.ServerAt}
	for _, mutate := range []func(*Action){func(a *Action) { a.Seq = 0 }, func(a *Action) { a.Power = 2 }, func(a *Action) { a.Aim = 2 }, func(a *Action) { a.Epoch = "old" }, func(a *Action) { a.Rally-- }, func(a *Action) { a.At -= 5000 }, func(a *Action) { a.At += 1000 }, func(a *Action) { a.Kind = "winner" }} {
		bad := a
		mutate(&bad)
		raw, _ := json.Marshal(bad)
		if _, e := Module().Apply(ctx, m, raw); e == nil {
			t.Fatalf("accepted invalid input: %s", raw)
		}
	}
	raw, _ := json.Marshal(a)
	ctx.Actor = -1
	if _, e := Module().Apply(ctx, m, raw); e == nil {
		t.Fatal("spectator input accepted")
	}
	ctx.Actor = 1 - s.Expected
	if _, e := Module().Apply(ctx, m, raw); e == nil {
		t.Fatal("wrong server accepted")
	}
	ctx.Actor = s.Expected
	next, e := Module().Apply(ctx, m, raw)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Module().Apply(ctx, next, raw); e == nil {
		t.Fatal("duplicate stroke accepted")
	}
	if !json.Valid(next.Data) {
		t.Fatal("invalid saved state")
	}
}

// A swipe is expressed in the acting seat's view. The far side must not turn
// a right swipe into a left placement, including after the deciding-game swap.
func TestAimUsesActingSeatPerspective(t *testing.T) {
	for _, ends := range []int{0, 1} {
		for seat := 0; seat < 2; seat++ {
			s := fixture(t, 2)
			s.Ends = ends
			from := s.side(s.Options.Teams[seat])
			dir := sign(from)
			s.Flight = &Flight{Spin: 0, ContactStart: 1000, Segments: []Segment{{At: 1000, Until: 2000, P: Vec{0, 1.08, dir * .9}, V: Vec{}, A: Vec{}}}}
			s.launch(Action{Kind: "drive", Aim: .65, Power: .6}, seat, 1130, false)
			if s.Flight.ReceiveAt == 0 {
				t.Fatal("ordinary placement did not land")
			}
			p, _ := s.Flight.at(s.Flight.ReceiveAt)
			if p.X*dir < .3 {
				t.Fatalf("seat %d ends %d right swipe landed left: %+v", seat, ends, p)
			}
		}
	}
}

// Real TCP acceptance measured a legitimate same-ball input age of 253ms at
// 250ms RTT. Keep a bounded allowance, without admitting old balls or seconds-old input.
func TestNetworkDelayAllowance(t *testing.T) {
	s := fixture(t, 2)
	s.Phase = "serve"
	s.PhaseUntil = s.ServerAt
	s.Expected, s.Receiver = s.service()
	s.ServeDeadline = s.ServerAt + ServeTimeoutMS
	m, _ := s.match()
	a := Action{Type: "swing", Epoch: s.Epoch, Rally: s.Rally, Shot: s.Shot, Seq: 1, Kind: "drive", Power: .5, At: s.ServerAt}
	ctx := gameapi.Context{Players: 2, Actor: s.Expected, Now: time.UnixMilli(s.ServerAt + 253)}
	b, _ := json.Marshal(a)
	if _, e := Module().Apply(ctx, m, b); e != nil {
		t.Fatalf("legitimate 250ms transport rejected: %v", e)
	}
	ctx.Now = time.UnixMilli(s.ServerAt + 1000)
	if _, e := Module().Apply(ctx, m, b); e == nil {
		t.Fatal("unbounded stale input accepted")
	}
	a.Rally--
	ctx.Now = time.UnixMilli(s.ServerAt + 253)
	b, _ = json.Marshal(a)
	if _, e := Module().Apply(ctx, m, b); e == nil {
		t.Fatal("old rally accepted under lag allowance")
	}
}

func TestTerminalImpactWaitsForLagAllowance(t *testing.T) {
	s := fixture(t, 2)
	s.Events = nil
	s.EventID = 0
	s.FlightCursor = 0
	s.Flight = &Flight{ReceiveAt: 1000, EndAt: 1300, Events: []FlightEvent{{Type: "bounce", At: 1000}, {Type: "bounce", At: 1300}}}
	emitFlight(&s, 1350)
	if len(s.Events) != 1 {
		t.Fatal("a terminal bounce was published before a legal late return could arrive")
	}
	emitFlight(&s, 1300+LateGraceMS)
	if len(s.Events) != 2 {
		t.Fatal("confirmed terminal bounce was lost")
	}
}

// First-person, body-free play must end the strike window before the ball
// travels behind the observer. Late transport may not extend physical reach.
func TestStrikeWindowStaysInFrontOfPlayer(t *testing.T) {
	for _, direction := range []float64{-1, 1} {
		from, to := 0, 1
		if direction > 0 {
			from, to = 1, 0
		}
		f := traceFlight(Vec{0, 1.05, -direction * 1.1}, Vec{0, 1.6, direction * 3.8}, .9, 1000, from, to, false, false)
		if f.ReceiveAt == 0 || f.ContactEnd < f.ContactStart {
			t.Fatal("ordinary shot has no strike window")
		}
		p, _ := f.at(f.ContactEnd)
		if p.Z*direction > ReachDepth+.01 {
			t.Fatalf("strike window extends behind the player: %+v", p)
		}
	}
}

// The immediate visual serve predicts from the bounded input timestamp. Network
// delay must not start a second, later toss and pull the ball backwards on ACK.
func TestServeUsesBoundedInputTime(t *testing.T) {
	s := fixture(t, 2)
	s.Phase = "serve"
	s.PhaseUntil = s.ServerAt
	s.Expected, s.Receiver = s.service()
	s.ServeDeadline = s.ServerAt + ServeTimeoutMS
	m, _ := s.match()
	a := Action{Type: "swing", Epoch: s.Epoch, Rally: s.Rally, Shot: s.Shot, Seq: 1, Kind: "drive", Power: .5, At: s.ServerAt}
	ctx := gameapi.Context{Players: 2, Actor: s.Expected, Now: time.UnixMilli(s.ServerAt + 253)}
	b, _ := json.Marshal(a)
	next, e := Module().Apply(ctx, m, b)
	if e != nil {
		t.Fatal(e)
	}
	after, e := decode(next)
	if e != nil {
		t.Fatal(e)
	}
	if after.Flight == nil || after.Flight.StartAt != a.At+180 || after.Phase != "rally" {
		t.Fatal("late acknowledgement restarted the serve toss")
	}
	a.At = s.PhaseUntil - 1
	b, _ = json.Marshal(a)
	if _, e = Module().Apply(ctx, m, b); e == nil {
		t.Fatal("pre-countdown input accepted as a serve")
	}
}

func TestLateCancellationCannotUndoAConfirmedStroke(t *testing.T) {
	s := fixture(t, 2)
	s.Phase = "serve"
	s.PhaseUntil = s.ServerAt
	s.Expected, s.Receiver = s.service()
	s.ServeDeadline = s.ServerAt + ServeTimeoutMS
	m, _ := s.match()
	actor := s.Expected
	a := Action{Type: "swing", Epoch: s.Epoch, Rally: s.Rally, Shot: s.Shot, Seq: 1, Kind: "drive", Power: .5, At: s.ServerAt}
	ctx := gameapi.Context{Players: 2, Actor: actor, Now: time.UnixMilli(s.ServerAt + 253)}
	b, _ := json.Marshal(a)
	m, e := Module().Apply(ctx, m, b)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := decode(m)
	cancel := Action{Type: "cancel", Epoch: s.Epoch, Rally: s.Rally, Shot: s.Shot, Seq: 2}
	b, _ = json.Marshal(cancel)
	next, e := Module().Apply(ctx, m, b)
	if e != nil {
		t.Fatal("late cancellation should acknowledge harmlessly", e)
	}
	after, _ := decode(next)
	if !reflect.DeepEqual(before.Flight, after.Flight) || before.Score != after.Score || after.Shot != before.Shot || after.Seq[actor] != 2 {
		t.Fatal("late cancellation changed confirmed play")
	}
	repeated, e := Module().Apply(ctx, next, b)
	if e != nil || !reflect.DeepEqual(repeated, next) {
		t.Fatal("duplicate cancellation was not idempotent")
	}
}

func TestCancellationDoesNotStrandACommittedServe(t *testing.T) {
	s := fixture(t, 2)
	s.Phase = "serve"
	s.PhaseUntil = s.ServerAt
	s.Expected, s.Receiver = s.service()
	s.ServeDeadline = s.ServerAt + ServeTimeoutMS
	m, _ := s.match()
	ctx := gameapi.Context{Players: 2, Actor: s.Expected, Now: time.UnixMilli(s.ServerAt)}
	a := Action{Type: "swing", Epoch: s.Epoch, Rally: s.Rally, Shot: s.Shot, Seq: 1, Kind: "drive", Power: .5, At: s.ServerAt}
	b, _ := json.Marshal(a)
	m, e := Module().Apply(ctx, m, b)
	if e != nil {
		t.Fatal(e)
	}
	a.Type = "cancel"
	a.Seq = 2
	b, _ = json.Marshal(a)
	m, e = Module().Apply(ctx, m, b)
	if e != nil {
		t.Fatal(e)
	}
	ctx.Actor = -1
	ctx.Now = ctx.Now.Add(250 * time.Millisecond)
	m, e = Module().OnTimer(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	after, _ := decode(m)
	if after.Phase != "rally" || after.Shot != 1 {
		t.Fatal("cancellation stranded serve animation")
	}
}

// Reclaim neutralizes old controls; a game-owned serve clock is never extended
// by toggling autoplay or by the platform's realtime startup migration.
func TestReclaimPreservesServeClock(t *testing.T) {
	s := fixture(t, 2)
	s.Phase = "serve"
	s.PhaseUntil = s.ServerAt
	s.Expected, s.Receiver = s.service()
	s.ServeDeadline = s.ServerAt + ServeTimeoutMS
	m, _ := s.match()
	ctx := gameapi.Context{Players: 2, Actor: s.Expected, Now: time.UnixMilli(s.ServeDeadline + 200)}
	next, e := Module().Reclaim(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	after, _ := decode(next)
	if after.ServeDeadline != s.ServeDeadline || after.PhaseUntil != s.PhaseUntil {
		t.Fatal("reclaim extended game-owned serve budget")
	}
}

func TestPaddlePositionDeterminesContact(t *testing.T) {
	for _, ends := range []int{0, 1} {
		for seat := 0; seat < 2; seat++ {
			s := fixture(t, 2)
			s.Ends = ends
			s.Phase = "rally"
			s.Expected = seat
			s.Receiver = seat
			dir := sign(s.side(s.Options.Teams[seat]))
			at := s.ServerAt
			s.Flight = &Flight{Spin: 0, ReceiveAt: at - 25, ContactStart: at, ContactEnd: at + 500, EndAt: at + 1000, Segments: []Segment{{At: at, Until: at + 1000, P: Vec{dir * .6, 1.08, dir * .9}, V: Vec{}, A: Vec{}}}}
			m, _ := s.match()
			ctx := gameapi.Context{Players: 2, Actor: seat, Now: time.UnixMilli(at + 130)}
			a := Action{Type: "swing", Epoch: s.Epoch, Rally: s.Rally, Shot: s.Shot, Seq: 1, Kind: "drive", Power: .5, At: at + 130, RacketX: -.7}
			b, _ := json.Marshal(a)
			miss, e := Module().Apply(ctx, m, b)
			if e != nil {
				t.Fatal("a whiff is an ordinary stroke, not an RPC error", e)
			}
			mv, _ := decode(miss)
			if mv.Shot != s.Shot || mv.Score != s.Score || mv.Paddles[seat].Result != "miss" || mv.Seq[seat] != 1 {
				t.Fatal("wrong-position swing hit the ball or changed score")
			}
			visible, _ := project(miss, seat)
			if visible.(View).Controls.RetrySeq != 1 {
				t.Fatal("whiff cannot be retried")
			}
			a.RacketX = .45
			a.Seq = 2
			b, _ = json.Marshal(a)
			hit, e := Module().Apply(ctx, miss, b)
			if e != nil {
				t.Fatal(e)
			}
			hv, _ := decode(hit)
			if hv.Shot != s.Shot+1 || hv.Paddles[seat].Result != "hit" {
				t.Fatal("light assist failed to connect near the ball")
			}
			bad := a
			bad.RacketX = 2
			b, _ = json.Marshal(bad)
			if _, e = Module().Apply(ctx, m, b); e == nil {
				t.Fatal("unbounded racket reach accepted")
			}
		}
	}
}
