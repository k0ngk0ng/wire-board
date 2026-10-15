package racing

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
)

// The driver uses only the public road and car states. It submits exactly the
// same bounded steering/throttle/brake commands as a human, never a position.
func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Cars) || s.Result != nil {
		return nil, nil
	}
	c := s.Cars[ctx.Actor]
	if c.FinishAt > 0 || c.Seq < 0 || c.Seq >= maxInputSequence {
		return nil, nil
	}
	t := s.Track
	n := len(t.Points)
	_, near, off, f := nearest(t, Point{c.X, c.Y})
	look := clamp(35+c.Speed*.2, 45, 90)
	start := near
	distance := 0.
	target := t.Points[(near+1)%n]
	for k := 0; k < 12; k++ {
		i := (start + k) % n
		segment := dist(t.Points[i], t.Points[(i+1)%n])
		if k == 0 {
			segment *= 1 - f
		}
		distance += segment
		target = t.Points[(i+1)%n]
		if distance >= look {
			break
		}
	}
	lane := float64(ctx.Actor%2*2-1) * 11.
	tv := tangent(t, (near+2)%n)
	target.X -= tv.Y * lane
	target.Y += tv.X * lane
	// An unpassed gate is a hard route obligation. After a collision/off-road
	// excursion aim back before it, rather than skipping ahead or teleporting.
	gate := t.Points[c.Checkpoint]
	gv := tangent(t, c.Checkpoint)
	ahead := (c.X-gate.X)*gv.X + (c.Y-gate.Y)*gv.Y
	if (near-c.Checkpoint+n)%n > 3 && (c.Checkpoint-near+n)%n > 3 || ahead > 15 {
		target = Point{gate.X - gv.X*75, gate.Y - gv.Y*75}
	}
	diff := angle(math.Atan2(target.Y-c.Y, target.X-c.X) - c.Angle)
	steer := clamp(diff*2.1, -1, 1)
	targetSpeed := 250.
	for k := 1; k <= 5; k++ {
		a := tangent(t, (near+k)%n)
		b := tangent(t, (near+k+1)%n)
		bend := math.Abs(angle(math.Atan2(b.Y, b.X) - math.Atan2(a.Y, a.X)))
		curve := bend / 45
		if curve > .0001 {
			limit := math.Sqrt(95 / curve)
			targetSpeed = math.Min(targetSpeed, limit+float64(k-1)*7)
		}
	}
	if math.Abs(diff) > .65 {
		targetSpeed = math.Min(targetSpeed, 100)
	}
	if math.Abs(diff) > 1.15 {
		targetSpeed = 65
	}
	if off > t.RoadWidth*.4 {
		targetSpeed = math.Min(targetSpeed, 105)
	}
	// Yield or choose the open side when approaching a slower nearby car.
	for _, other := range s.Cars {
		if other.Seat == c.Seat || other.FinishAt > 0 {
			continue
		}
		dx, dy := other.X-c.X, other.Y-c.Y
		along := dx*math.Cos(c.Angle) + dy*math.Sin(c.Angle)
		lateral := -dx*math.Sin(c.Angle) + dy*math.Cos(c.Angle)
		if along > 0 && along < 90 && math.Abs(lateral) < 32 && other.Speed < c.Speed+20 {
			avoid := -.24
			if lateral < 0 {
				avoid = .24
			}
			steer = clamp(steer+avoid, -1, 1)
			if along < 38 {
				targetSpeed = math.Min(targetSpeed, math.Max(45, other.Speed-15))
			}
		}
	}
	throttle := clamp((targetSpeed-c.Speed)/65+.32, 0, 1)
	brake := c.Speed > targetSpeed+12
	drift := math.Abs(diff) > .28 && math.Abs(diff) < .65 && c.Speed > 145 && off < t.RoadWidth*.3
	a := Action{Type: "drive", Epoch: s.Epoch, Seq: c.Seq + 1, Steer: steer, Throttle: throttle, Brake: brake, Drift: drift}
	return json.Marshal(a)
}
