// Package snooker owns its rules, 2D table geometry and authoritative shot model.
package snooker

import (
	"errors"
	"math"
	"slices"
)

const (
	TableWidth  = 3.569
	TableHeight = 1.778
	BallRadius  = .02625
	BaulkX      = .737
	DRadius     = .292
	physicsStep = 1.0 / 240
)

type Vec struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (a Vec) add(b Vec) Vec     { return Vec{a.X + b.X, a.Y + b.Y} }
func (a Vec) sub(b Vec) Vec     { return Vec{a.X - b.X, a.Y - b.Y} }
func (a Vec) mul(k float64) Vec { return Vec{a.X * k, a.Y * k} }
func (a Vec) dot(b Vec) float64 { return a.X*b.X + a.Y*b.Y }
func (a Vec) length() float64   { return math.Hypot(a.X, a.Y) }
func (a Vec) unit() Vec {
	n := a.length()
	if n < 1e-12 {
		return Vec{1, 0}
	}
	return a.mul(1 / n)
}
func (a Vec) tangent() Vec  { return Vec{-a.Y, a.X} }
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

type Ball struct {
	ID   int  `json:"id"`
	Pos  Vec  `json:"pos"`
	Down bool `json:"down"`
}
type Strike struct {
	Angle    float64 `json:"angle"`
	Power    float64 `json:"power"`
	Top      float64 `json:"top"`
	Side     float64 `json:"side"`
	Nominate int     `json:"nominate,omitempty"`
	FreeBall int     `json:"freeBall,omitempty"`
}
type Impact struct {
	Time  float64 `json:"time"`
	Kind  string  `json:"kind"`
	Ball  int     `json:"ball"`
	Other int     `json:"other,omitempty"`
	Speed float64 `json:"speed"`
	Pos   Vec     `json:"pos"`
}
type MotionFrame struct {
	Time  float64 `json:"time"`
	Balls []Ball  `json:"balls"`
}
type Motion struct {
	Balls    []Ball        `json:"balls"`
	First    []int         `json:"first"`
	Potted   []int         `json:"potted"`
	Off      []int         `json:"off"`
	Events   []Impact      `json:"events"`
	Frames   []MotionFrame `json:"frames"`
	Duration float64       `json:"duration"`
}
type movingBall struct {
	Ball
	v, roll Vec
	side    float64
}
type rail struct{ a, b Vec }

var pockets = []Vec{{0, 0}, {TableWidth / 2, -.026}, {TableWidth, 0}, {0, TableHeight}, {TableWidth / 2, TableHeight + .026}, {TableWidth, TableHeight}}

// Finite cushion faces with round end caps produce jaws instead of swallowing
// everything near the mouth. Capture circles sit inside the actual pocket gaps.
var rails = []rail{
	{Vec{.076, 0}, Vec{TableWidth/2 - .082, 0}},
	{Vec{TableWidth/2 + .082, 0}, Vec{TableWidth - .076, 0}},
	{Vec{.076, TableHeight}, Vec{TableWidth/2 - .082, TableHeight}},
	{Vec{TableWidth/2 + .082, TableHeight}, Vec{TableWidth - .076, TableHeight}},
	{Vec{0, .076}, Vec{0, TableHeight - .076}},
	{Vec{TableWidth, .076}, Vec{TableWidth, TableHeight - .076}},
}

func BallValue(id int) int {
	if id == 0 {
		return 0
	}
	if id <= 15 {
		return 1
	}
	return id - 14
}
func ColourSpot(value int) Vec {
	switch value {
	case 2:
		return Vec{BaulkX, TableHeight/2 + DRadius}
	case 3:
		return Vec{BaulkX, TableHeight/2 - DRadius}
	case 4:
		return Vec{BaulkX, TableHeight / 2}
	case 5:
		return Vec{TableWidth / 2, TableHeight / 2}
	case 6:
		return Vec{TableWidth * .75, TableHeight / 2}
	case 7:
		return Vec{TableWidth - .324, TableHeight / 2}
	}
	return Vec{}
}
func InitialBalls() []Ball {
	balls := []Ball{{ID: 0, Pos: Vec{BaulkX - .16, TableHeight/2 + .09}}}
	id := 1
	apex := ColourSpot(6).X + 2*BallRadius + .003
	for row := 0; row < 5; row++ {
		for col := 0; col <= row; col++ {
			balls = append(balls, Ball{ID: id, Pos: Vec{apex + float64(row)*math.Sqrt(3)*(BallRadius+.00002), TableHeight/2 + float64(2*col-row)*(BallRadius+.00002)}})
			id++
		}
	}
	for value := 2; value <= 7; value++ {
		balls = append(balls, Ball{ID: value + 14, Pos: ColourSpot(value)})
	}
	return balls
}
func validStrike(s Strike) bool {
	return finite(s.Angle) && finite(s.Power) && finite(s.Top) && finite(s.Side) && s.Power >= .02 && s.Power <= 1 && s.Top*s.Top+s.Side*s.Side <= .81+1e-12
}
func validateBalls(balls []Ball) error {
	if len(balls) == 0 || len(balls) > 22 {
		return errors.New("无效球位")
	}
	ids := map[int]bool{}
	cue := false
	for i, b := range balls {
		if b.ID < 0 || b.ID > 21 || ids[b.ID] || !finite(b.Pos.X) || !finite(b.Pos.Y) {
			return errors.New("无效球位")
		}
		ids[b.ID] = true
		if b.ID == 0 && !b.Down {
			cue = true
		}
		if b.Down {
			continue
		}
		if b.Pos.X < 0 || b.Pos.X > TableWidth || b.Pos.Y < 0 || b.Pos.Y > TableHeight {
			return errors.New("球位不在台面内")
		}
		for _, other := range balls[:i] {
			if !other.Down && b.Pos.sub(other.Pos).length() < 2*BallRadius-1e-5 {
				return errors.New("球位重叠")
			}
		}
	}
	if !cue {
		return errors.New("请先在 D 区摆好白球")
	}
	return nil
}

// Entry time for a moving point into a circle. At an existing contact only
// approaching motion counts, so a touching cue ball can legally be played away.
func circleTime(p, v Vec, r, limit float64) (float64, bool) {
	a := v.dot(v)
	if a < 1e-16 {
		return 0, false
	}
	b := p.dot(v)
	c := p.dot(p) - r*r
	if c <= 1e-11 {
		if b < -1e-10 {
			return 0, true
		}
		return 0, false
	}
	if b >= 0 {
		return 0, false
	}
	d := b*b - a*c
	if d < 0 {
		return 0, false
	}
	t := (-b - math.Sqrt(d)) / a
	return math.Max(0, t), t >= -1e-10 && t <= limit+1e-10
}

type collision struct {
	t    float64
	kind string
	a, b int
	n    Vec
}

func nextCollision(bs []movingBall, limit float64) collision {
	best := collision{t: limit + 1, kind: ""}
	take := func(c collision) {
		if c.t < best.t-1e-10 {
			best = c
		}
	}
	for i, a := range bs {
		if a.Down {
			continue
		}
		for j := i + 1; j < len(bs); j++ {
			b := bs[j]
			if b.Down {
				continue
			}
			if t, ok := circleTime(a.Pos.sub(b.Pos), a.v.sub(b.v), 2*BallRadius, limit); ok {
				take(collision{t: t, kind: "ball", a: i, b: j})
			}
		}
		for k, p := range pockets {
			radius := .051
			if k == 1 || k == 4 {
				radius = .049
			}
			if t, ok := circleTime(a.Pos.sub(p), a.v, radius, limit); ok {
				take(collision{t: t, kind: "pot", a: i, b: k})
			}
		}
		for _, r := range rails {
			tangent := r.b.sub(r.a).unit()
			normal := tangent.tangent()
			for _, sign := range []float64{-1, 1} {
				n := normal.mul(sign)
				vn := a.v.dot(n)
				distance := a.Pos.sub(r.a).dot(n)
				if vn >= -1e-10 || distance < -1e-8 {
					continue
				}
				t := (BallRadius - distance) / vn
				if t >= -1e-8 && t <= limit+1e-10 {
					p := a.Pos.add(a.v.mul(math.Max(0, t)))
					along := p.sub(r.a).dot(tangent)
					if along >= 0 && along <= r.b.sub(r.a).length() {
						take(collision{t: math.Max(0, t), kind: "cushion", a: i, n: n})
					}
				}
			}
			for _, end := range []Vec{r.a, r.b} {
				if t, ok := circleTime(a.Pos.sub(end), a.v, BallRadius, limit); ok {
					n := a.Pos.add(a.v.mul(t)).sub(end).unit()
					take(collision{t: t, kind: "cushion", a: i, n: n})
				}
			}
		}
	}
	return best
}
func cloth(b *movingBall, dt float64) {
	if b.Down {
		return
	}
	slip := b.v.sub(b.roll)
	n := slip.length()
	if n > .00001 {
		// For a solid sphere I=2mr²/5. Sliding friction moves translational and
		// rotational surface speeds toward one another without adding energy.
		amount := math.Min(.20*9.81*dt, n/3.5)
		change := slip.unit().mul(-amount)
		b.v = b.v.add(change)
		b.roll = b.roll.sub(change.mul(2.5))
	} else {
		speed := b.v.length()
		if speed > 0 {
			b.v = b.v.mul(math.Max(0, speed-.012*9.81*dt) / speed)
		}
		b.roll = b.v
	}
	b.side = math.Copysign(math.Max(0, math.Abs(b.side)-3*dt), b.side)
	if b.v.length() < .004 && b.roll.length() < .004 {
		b.v = Vec{}
		b.roll = Vec{}
		b.side = 0
	}
}
func snapshot(bs []movingBall) []Ball {
	out := make([]Ball, len(bs))
	for i, b := range bs {
		out[i] = b.Ball
	}
	return out
}

// Simulate resolves one complete stroke. It is deterministic, bounded and reads
// no global clock/randomness. Rules consume the resulting timed event stream.
func Simulate(balls []Ball, strike Strike) (Motion, error) {
	m := Motion{First: []int{}, Potted: []int{}, Off: []int{}, Events: []Impact{}, Frames: []MotionFrame{}}
	if !validStrike(strike) {
		return m, errors.New("请使用有效的击球方向、力度和击点")
	}
	if e := validateBalls(balls); e != nil {
		return m, e
	}
	bs := make([]movingBall, len(balls))
	for i, b := range balls {
		bs[i].Ball = b
		if b.ID == 0 {
			speed := 4.2 * strike.Power
			bs[i].v = Vec{math.Cos(strike.Angle) * speed, math.Sin(strike.Angle) * speed}
			bs[i].roll = bs[i].v.mul(strike.Top)
			bs[i].side = strike.Side * speed / BallRadius
		}
	}
	m.Frames = append(m.Frames, MotionFrame{Balls: snapshot(bs)})
	now := 0.
	nextFrame := 1. / 30
	firstTime := -1.
	for step := 0; step < 7200; step++ {
		remain := physicsStep
		for contacts := 0; remain > 1e-9; contacts++ {
			if contacts > 128 {
				return Motion{}, errors.New("球位碰撞未收敛，请重新瞄准")
			}
			hit := nextCollision(bs, remain)
			advance := remain
			if hit.kind != "" {
				advance = math.Min(remain, hit.t)
			}
			for i := range bs {
				if !bs[i].Down {
					bs[i].Pos = bs[i].Pos.add(bs[i].v.mul(advance))
				}
			}
			now += advance
			remain -= advance
			if hit.kind == "" {
				break
			}
			a := &bs[hit.a]
			ev := Impact{Time: now, Kind: hit.kind, Ball: a.ID, Speed: a.v.length(), Pos: a.Pos}
			switch hit.kind {
			case "pot":
				a.Down = true
				a.v = Vec{}
				a.roll = Vec{}
				a.side = 0
				m.Potted = append(m.Potted, a.ID)
			case "ball":
				b := &bs[hit.b]
				n := b.Pos.sub(a.Pos).unit()
				closing := a.v.sub(b.v).dot(n)
				if closing > 0 {
					impulse := (1 + .97) * closing / 2
					a.v = a.v.sub(n.mul(impulse))
					b.v = b.v.add(n.mul(impulse))
					tangent := n.tangent()
					slip := a.v.sub(b.v).dot(tangent) + (a.side+b.side)*BallRadius
					jt := math.Max(-.04*impulse, math.Min(.04*impulse, -slip/7))
					a.v = a.v.add(tangent.mul(jt))
					b.v = b.v.sub(tangent.mul(jt))
					a.side += 2.5 * jt / BallRadius
					b.side += 2.5 * jt / BallRadius
				}
				overlap := 2*BallRadius - a.Pos.sub(b.Pos).length()
				if overlap > 0 {
					a.Pos = a.Pos.sub(n.mul(overlap/2 + 1e-9))
					b.Pos = b.Pos.add(n.mul(overlap/2 + 1e-9))
				}
				ev.Other = b.ID
				ev.Speed = closing
				other := -1
				if a.ID == 0 {
					other = b.ID
				} else if b.ID == 0 {
					other = a.ID
				}
				if other >= 0 {
					if firstTime < 0 {
						firstTime = now
					}
					if now-firstTime < .00002 && !slices.Contains(m.First, other) {
						m.First = append(m.First, other)
					}
				}
			case "cushion":
				n := hit.n
				tangent := n.tangent()
				j := -(1 + .80) * a.v.dot(n)
				if j > 0 {
					a.v = a.v.add(n.mul(j))
					slip := a.v.dot(tangent) - a.side*BallRadius
					jt := math.Max(-.18*j, math.Min(.18*j, -slip/3.5))
					a.v = a.v.add(tangent.mul(jt))
					a.side -= 2.5 * jt / BallRadius
				}
				a.Pos = a.Pos.add(n.mul(1e-9))
			}
			m.Events = append(m.Events, ev)
			recordFrame(&m, now, bs)
			if len(m.Events) > 4096 {
				return Motion{}, errors.New("本杆碰撞过多，请重新瞄准")
			}
		}
		active := false
		for i := range bs {
			b := &bs[i]
			if b.Down {
				continue
			}
			if b.Pos.X < -.12 || b.Pos.X > TableWidth+.12 || b.Pos.Y < -.12 || b.Pos.Y > TableHeight+.12 {
				b.Down = true
				m.Off = append(m.Off, b.ID)
				m.Events = append(m.Events, Impact{Time: now, Kind: "off", Ball: b.ID, Pos: b.Pos})
				continue
			}
			cloth(b, physicsStep)
			if b.v.length() > 0 || b.roll.length() > 0 {
				active = true
			}
		}
		if now+1e-9 >= nextFrame || !active {
			recordFrame(&m, now, bs)
			nextFrame += 1. / 30
		}
		if !active {
			m.Balls = snapshot(bs)
			m.Frames = compactFrames(m.Frames)
			m.Duration = now
			slices.Sort(m.First)
			return m, nil
		}
	}
	return Motion{}, errors.New("本杆运动时间超限，请重新瞄准")
}

// Impact samples keep visible contact, sound and direction changes on the same
// timestamp. Simultaneous contacts share one complete position sample.
func recordFrame(m *Motion, at float64, bs []movingBall) {
	frame := MotionFrame{Time: at, Balls: snapshot(bs)}
	if len(m.Frames) > 0 && math.Abs(m.Frames[len(m.Frames)-1].Time-at) < 1e-9 {
		m.Frames[len(m.Frames)-1] = frame
	} else {
		m.Frames = append(m.Frames, frame)
	}
}
