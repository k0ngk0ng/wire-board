package snooker

import (
	"math"
	"slices"
	"sort"
)

type sightLine struct {
	n Vec
	c float64
}
type sightCircle struct {
	p Vec
	r float64
}

// dVisiblePosition returns a legal placement from which at least one on-ball
// has both extreme edges visible. Visibility changes at common tangent lines of
// expanded ball circles. Their arrangement with the D and placement-exclusion
// circles supplies witnesses for narrow regions that a fixed grid would miss.
func dVisiblePosition(balls []Ball, on []int) (Vec, bool) {
	bs := append([]Ball{}, balls...)
	cue := -1
	for i := range bs {
		if bs[i].ID == 0 {
			cue = i
			break
		}
	}
	if cue < 0 || len(on) == 0 {
		return Vec{}, false
	}
	center := Vec{BaulkX, TableHeight / 2}
	probe := func(p Vec) bool {
		if p.X > BaulkX+1e-10 || p.sub(center).length() > DRadius+1e-10 || !clearSpot(p, bs, 0) {
			return false
		}
		bs[cue].Pos = p
		bs[cue].Down = false
		return !IsSnookered(bs, on)
	}
	for _, p := range []Vec{{BaulkX - .15, TableHeight / 2}, {BaulkX - .08, TableHeight/2 - .15}, {BaulkX - .08, TableHeight/2 + .15}, {BaulkX - .25, TableHeight / 2}} {
		if probe(p) {
			return p, true
		}
	}
	lines := []sightLine{{Vec{1, 0}, BaulkX}}
	circles := []sightCircle{{center, DRadius}}
	for _, b := range bs {
		if !b.Down && b.ID != 0 {
			circles = append(circles, sightCircle{b.Pos, 2*BallRadius + .000001})
		}
	}
	for _, id := range on {
		target, ok := findBall(bs, id)
		if !ok || target.Down {
			continue
		}
		for _, block := range bs {
			if block.Down || block.ID == 0 || slices.Contains(on, block.ID) {
				continue
			}
			d := block.Pos.sub(target.Pos)
			length := d.length()
			if length < 1e-12 {
				continue
			}
			along := d.mul(1 / length)
			normal := along.tangent()
			r := 2 * BallRadius
			lines = append(lines, sightLine{normal, normal.dot(target.Pos) + r}, sightLine{normal, normal.dot(target.Pos) - r})
			if length > 2*r {
				a := 2 * r / length
				b := math.Sqrt(1 - a*a)
				for _, sign := range []float64{-1, 1} {
					n := along.mul(a).add(normal.mul(sign * b))
					lines = append(lines, sightLine{n, n.dot(target.Pos) + r})
				}
			}
		}
	}
	witness := func(p Vec) (Vec, bool) {
		if p.X > BaulkX+.00002 || p.sub(center).length() > DRadius+.00002 {
			return Vec{}, false
		}
		if probe(p) {
			return p, true
		}
		angles := []float64{}
		tangent := func(v Vec) {
			a := math.Atan2(v.Y, v.X)
			angles = append(angles, a, math.Remainder(a+math.Pi, 2*math.Pi))
		}
		for _, line := range lines {
			if math.Abs(line.n.dot(p)-line.c) < 1e-7 {
				tangent(line.n.tangent())
			}
		}
		for _, circle := range circles {
			if math.Abs(p.sub(circle.p).length()-circle.r) < 1e-7 {
				tangent(p.sub(circle.p).tangent())
			}
		}
		if len(angles) == 0 {
			angles = []float64{-math.Pi, -math.Pi / 2, 0, math.Pi / 2}
		}
		sort.Float64s(angles)
		for i, a := range angles {
			b := angles[(i+1)%len(angles)]
			if i == len(angles)-1 {
				b += 2 * math.Pi
			}
			mid := (a + b) / 2
			for _, eps := range []float64{.00001, .0000001, .000000001} {
				q := p.add(Vec{math.Cos(mid) * eps, math.Sin(mid) * eps})
				if probe(q) {
					return q, true
				}
			}
		}
		return Vec{}, false
	}
	for i, a := range lines {
		for _, b := range lines[:i] {
			det := a.n.X*b.n.Y - a.n.Y*b.n.X
			if math.Abs(det) < 1e-12 {
				continue
			}
			p := Vec{(a.c*b.n.Y - a.n.Y*b.c) / det, (a.n.X*b.c - a.c*b.n.X) / det}
			if q, ok := witness(p); ok {
				return q, true
			}
		}
		for _, circle := range circles {
			distance := a.c - a.n.dot(circle.p)
			if math.Abs(distance) > circle.r+1e-12 {
				continue
			}
			foot := circle.p.add(a.n.mul(distance))
			half := math.Sqrt(math.Max(0, circle.r*circle.r-distance*distance))
			for _, sign := range []float64{-1, 1} {
				if q, ok := witness(foot.add(a.n.tangent().mul(sign * half))); ok {
					return q, true
				}
			}
		}
	}
	for i, a := range circles {
		for _, offset := range []Vec{{a.r, 0}, {-a.r, 0}, {0, a.r}, {0, -a.r}} {
			if q, ok := witness(a.p.add(offset)); ok {
				return q, true
			}
		}
		for _, b := range circles[:i] {
			d := b.p.sub(a.p)
			length := d.length()
			if length < 1e-12 || length > a.r+b.r+1e-12 || length < math.Abs(a.r-b.r)-1e-12 {
				continue
			}
			along := d.mul(1 / length)
			x := (a.r*a.r - b.r*b.r + length*length) / (2 * length)
			y := math.Sqrt(math.Max(0, a.r*a.r-x*x))
			base := a.p.add(along.mul(x))
			for _, sign := range []float64{-1, 1} {
				if q, ok := witness(base.add(along.tangent().mul(sign * y))); ok {
					return q, true
				}
			}
		}
	}
	return Vec{}, false
}
func freeBallAvailable(bs []Ball, on []int, inHand bool) bool {
	if inHand {
		_, clear := dVisiblePosition(bs, on)
		return !clear
	}
	return IsSnookered(bs, on)
}

// rayFirst uses the same finite cushions, jaws and pocket mouths as physics.
// It only screens candidates; a successful reflected ray is NOT itself proof
// of playability. cushionEscape verifies a real stroke before reporting a route.
func rayFirst(balls []Ball, angle float64) int {
	bs := make([]movingBall, len(balls))
	cue := -1
	for i, b := range balls {
		bs[i].Ball = b
		if b.ID == 0 {
			cue = i
		}
	}
	if cue < 0 || bs[cue].Down {
		return -1
	}
	bs[cue].v = Vec{math.Cos(angle), math.Sin(angle)}
	remaining := 18.
	for bounce := 0; bounce <= 4; bounce++ {
		hit := nextCollision(bs, remaining)
		if hit.kind == "" {
			return -1
		}
		bs[cue].Pos = bs[cue].Pos.add(bs[cue].v.mul(hit.t))
		remaining -= hit.t
		switch hit.kind {
		case "ball":
			if hit.a == cue {
				return bs[hit.b].ID
			}
			if hit.b == cue {
				return bs[hit.a].ID
			}
			return -1
		case "pot":
			return -1
		case "cushion":
			v := bs[cue].v
			bs[cue].v = v.sub(hit.n.mul(2 * v.dot(hit.n)))
			bs[cue].Pos = bs[cue].Pos.add(hit.n.mul(1e-7))
		}
	}
	return -1
}
func cushionEscape(balls []Ball, on []int) bool {
	cue, ok := findBall(balls, 0)
	if !ok || cue.Down {
		return false
	}
	seen := map[int]bool{}
	verify := func(angle float64) bool {
		key := int(math.Round(math.Remainder(angle, 2*math.Pi) * 10000))
		if seen[key] {
			return false
		}
		seen[key] = true
		if !slices.Contains(on, rayFirst(balls, angle)) {
			return false
		}
		// Real friction, spin and cushion restitution can change the ideal mirror
		// angle. Probe nearby shots, retaining only actual first legal contacts.
		for _, offset := range []float64{0, -.025, .025, -.065, .065, -.12, .12} {
			for _, power := range []float64{.45, .75} {
				for _, side := range []float64{0, -.4, .4} {
					m, e := Simulate(balls, Strike{Angle: angle + offset, Power: power, Side: side})
					if e != nil || len(m.First) == 0 {
						continue
					}
					valid := true
					for _, id := range m.First {
						if !slices.Contains(on, id) {
							valid = false
						}
					}
					if valid {
						return true
					}
				}
			}
		}
		return false
	}
	reflectPoint := func(p Vec, wall int) Vec {
		switch wall {
		case 0:
			p.X = 2*BallRadius - p.X
		case 1:
			p.X = 2*(TableWidth-BallRadius) - p.X
		case 2:
			p.Y = 2*BallRadius - p.Y
		case 3:
			p.Y = 2*(TableHeight-BallRadius) - p.Y
		}
		return p
	}
	for _, id := range on {
		target, ok := findBall(balls, id)
		if !ok || target.Down {
			continue
		}
		type image struct {
			p    Vec
			wall int
		}
		layer := []image{{target.Pos, -1}}
		for depth := 0; depth < 4; depth++ {
			next := []image{}
			for _, im := range layer {
				for wall := 0; wall < 4; wall++ {
					if wall == im.wall {
						continue
					}
					p := reflectPoint(im.p, wall)
					next = append(next, image{p, wall})
					d := p.sub(cue.Pos)
					angle := math.Atan2(d.Y, d.X)
					spread := math.Asin(math.Min(1, 2*BallRadius/d.length()))
					for _, part := range []float64{0, -.65, .65, -.95, .95} {
						if verify(angle + spread*part) {
							return true
						}
					}
				}
			}
			layer = next
		}
	}
	return false
}

// The effective blocker is the nearest ball not on (all ties count), not just
// a ball whose removal would make the shot clear. Reds with different effective
// blockers have no common effective snookering ball (Section 2 Rule 17).
func effectiveSnookeringBalls(bs []Ball, on []int) []int {
	if !IsSnookered(bs, on) {
		return []int{}
	}
	cue, ok := findBall(bs, 0)
	if !ok || cue.Down {
		return []int{}
	}
	var common []int
	for _, id := range on {
		target, ok := findBall(bs, id)
		if !ok || target.Down {
			continue
		}
		delta := target.Pos.sub(cue.Pos)
		distance := delta.length()
		if distance <= 2*BallRadius {
			return []int{}
		}
		center := math.Atan2(delta.Y, delta.X)
		spread := math.Asin(2 * BallRadius / distance)
		limit := math.Sqrt(distance*distance - 4*BallRadius*BallRadius)
		nearest := math.Inf(1)
		blockers := []int{}
		for _, ball := range bs {
			if ball.Down || ball.ID == 0 || slices.Contains(on, ball.ID) {
				continue
			}
			blocks := false
			for _, angle := range []float64{center - spread, center + spread} {
				dir := Vec{math.Cos(angle), math.Sin(angle)}
				if t, hit := circleTime(cue.Pos.sub(ball.Pos), dir, 2*BallRadius, limit); hit && t < limit-1e-7 {
					blocks = true
				}
			}
			if !blocks {
				continue
			}
			d := ball.Pos.sub(cue.Pos).length()
			if d < nearest-1e-7 {
				nearest = d
				blockers = []int{ball.ID}
			} else if math.Abs(d-nearest) <= 1e-7 {
				blockers = append(blockers, ball.ID)
			}
		}
		if common == nil {
			common = append([]int{}, blockers...)
		} else {
			common = slices.DeleteFunc(common, func(id int) bool { return !slices.Contains(blockers, id) })
		}
		if len(common) == 0 {
			return []int{}
		}
	}
	slices.Sort(common)
	return common
}
func missContact(c RuleContext, shot Strike, allowed []int) (visible, full bool, possible []int) {
	possible = append([]int{}, allowed...)
	if c.Target == "colour" && shot.FreeBall == 0 {
		possible = []int{}
		for _, b := range c.Balls {
			if !b.Down && b.ID >= 16 {
				possible = append(possible, b.ID)
			}
		}
	}
	visible, full = directContact(c.Balls, possible)
	// For reds the full-diameter exception explicitly ignores obstruction by
	// other reds. A mere available centre ray is insufficient if a colour hides
	// an extreme edge. This is distinct from the full-ball colour criterion.
	if c.Target == "red" && shot.FreeBall == 0 {
		full = !IsSnookered(c.Balls, possible)
	}
	return
}
