package racing

import (
	"fmt"
	"math"
)

func stepCar(c *Car, t Track, ms, at int64) {
	dt := float64(ms) / 1000
	_, _, distance, _ := nearest(t, Point{c.X, c.Y})
	c.Offroad = distance > t.RoadWidth/2
	steer, throttle, brake, drift := c.Steer, c.Throttle, c.Brake, c.Drift
	if brake {
		throttle = 0
	}
	if at-c.LastInputAt > lostInputMS || at < c.ResetUntil {
		steer = 0
		throttle = 0
		brake = true
		drift = false
	}
	forward := c.VX*math.Cos(c.Angle) + c.VY*math.Sin(c.Angle)
	yaw := steer * (1.65 + .002*math.Abs(forward)) * math.Min(math.Abs(forward)/65, 1)
	if drift {
		yaw *= 1.18
	}
	c.Angle = angle(c.Angle + yaw*dt)
	cs, sn := math.Cos(c.Angle), math.Sin(c.Angle)
	forward = c.VX*cs + c.VY*sn
	side := -c.VX*sn + c.VY*cs
	drag, grip, limit := .2, 9., MaxSpeed
	if drift {
		grip = 1.35
	}
	if c.Offroad {
		drag = 2.5
		grip = 4
		limit = 110
	}
	forward = (forward + throttle*245*dt) * math.Exp(-drag*dt)
	if drift {
		forward *= math.Exp(-1.2 * dt)
	}
	if brake {
		forward *= math.Exp(-6 * dt)
	}
	side *= math.Exp(-grip * dt)
	forward = clamp(forward, 0, limit)
	c.VX = forward*cs - side*sn
	c.VY = forward*sn + side*cs
	speed := math.Hypot(c.VX, c.VY)
	if speed > limit {
		c.VX *= limit / speed
		c.VY *= limit / speed
	}
	if math.Hypot(c.VX, c.VY) < .05 {
		c.VX = 0
		c.VY = 0
	}
	c.X += c.VX * dt
	c.Y += c.VY * dt
	if c.X < CarRadius || c.X > t.Width-CarRadius {
		c.X = clamp(c.X, CarRadius, t.Width-CarRadius)
		c.VX *= -.3
	}
	if c.Y < CarRadius || c.Y > t.Height-CarRadius {
		c.Y = clamp(c.Y, CarRadius, t.Height-CarRadius)
		c.VY *= -.3
	}
	// Circle obstacles use the visible object's footprint against the car capsule.
	for _, obstacle := range t.Props {
		dx, dy := obstacle.X-c.X, obstacle.Y-c.Y
		if math.Hypot(dx, dy) > obstacle.Radius+CarRadius+10 {
			continue
		}
		along := clamp(dx*cs+dy*sn, -10, 10)
		dx, dy = c.X+cs*along-obstacle.X, c.Y+sn*along-obstacle.Y
		distance := math.Hypot(dx, dy)
		depth := obstacle.Radius + CarRadius - distance
		if depth <= 0 {
			continue
		}
		nx, ny := -cs, -sn
		if distance > 0.00001 {
			nx, ny = dx/distance, dy/distance
		}
		c.X += nx * depth
		c.Y += ny * depth
		inward := c.VX*nx + c.VY*ny
		if inward < 0 {
			c.VX -= inward * nx * 1.1
			c.VY -= inward * ny * 1.1
		}
	}
	c.Speed = math.Hypot(c.VX, c.VY)
}

// contact returns the separating normal and penetration for two radius-13
// capsules with a 20-unit center segment, matching the 26 x 46 painted body.
func contact(a, b Car) (float64, float64, float64) {
	da := Point{math.Cos(a.Angle) * 20, math.Sin(a.Angle) * 20}
	db := Point{math.Cos(b.Angle) * 20, math.Sin(b.Angle) * 20}
	pa := Point{a.X - da.X/2, a.Y - da.Y/2}
	pb := Point{b.X - db.X/2, b.Y - db.Y/2}
	rx, ry := pa.X-pb.X, pa.Y-pb.Y
	aa, bb, ab := 400., 400., da.X*db.X+da.Y*db.Y
	ar, br := da.X*rx+da.Y*ry, db.X*rx+db.Y*ry
	u := 0.
	den := aa*bb - ab*ab
	if den > 1e-8 {
		u = clamp((ab*br-ar*bb)/den, 0, 1)
	}
	v := (ab*u + br) / bb
	if v < 0 {
		v = 0
		u = clamp(-ar/aa, 0, 1)
	} else if v > 1 {
		v = 1
		u = clamp((ab-ar)/aa, 0, 1)
	}
	dx, dy := (pb.X+db.X*v)-(pa.X+da.X*u), (pb.Y+db.Y*v)-(pa.Y+da.Y*u)
	distance := math.Hypot(dx, dy)
	if distance >= CarRadius*2 {
		return 0, 0, 0
	}
	if distance > 1e-6 {
		return dx / distance, dy / distance, CarRadius*2 - distance
	}
	// Intersecting axes have no unique nearest normal. Select a stable normal
	// and separate by their complete projected extents rather than only radii.
	nx, ny := b.X-a.X, b.Y-a.Y
	l := math.Hypot(nx, ny)
	if l > 1e-6 {
		nx /= l
		ny /= l
	} else {
		nx = -math.Sin(a.Angle)
		ny = math.Cos(a.Angle)
	}
	extent := CarRadius*2 + .5*math.Abs(da.X*nx+da.Y*ny) + .5*math.Abs(db.X*nx+db.Y*ny)
	return nx, ny, math.Max(0, extent-((b.X-a.X)*nx+(b.Y-a.Y)*ny))
}
func collide(s *State, at int64) {
	for pass := 0; pass < 3; pass++ {
		for i := range s.Cars {
			a := &s.Cars[i]
			if a.FinishAt > 0 {
				continue
			}
			for j := i + 1; j < len(s.Cars); j++ {
				b := &s.Cars[j]
				if b.FinishAt > 0 {
					continue
				}
				if math.Hypot(a.X-b.X, a.Y-b.Y) > 46 {
					continue
				}
				nx, ny, depth := contact(*a, *b)
				if depth <= 0 {
					continue
				}
				overlap := depth/2 + .001
				a.X -= nx * overlap
				a.Y -= ny * overlap
				b.X += nx * overlap
				b.Y += ny * overlap
				relative := (a.VX-b.VX)*nx + (a.VY-b.VY)*ny
				if relative > 0 {
					impulse := relative * .62
					a.VX -= impulse * nx
					a.VY -= impulse * ny
					b.VX += impulse * nx
					b.VY += impulse * ny
					if relative > 45 {
						s.emit("collision", i, at, clamp(relative/200, 0, 1))
					}
				}
				a.Speed = math.Hypot(a.VX, a.VY)
				b.Speed = math.Hypot(b.VX, b.VY)
			}
		}
	}
}
func updateProgress(s *State, c *Car, before Point, at int64) {
	t := s.Track
	n := len(t.Points)
	if c.FinishAt > 0 {
		return
	}
	arcs, total := pathLengths(t)
	for tries := 0; tries < 3; tries++ {
		i := c.Checkpoint
		p := t.Points[i]
		v := tangent(t, i)
		old := (before.X-p.X)*v.X + (before.Y-p.Y)*v.Y
		now := (c.X-p.X)*v.X + (c.Y-p.Y)*v.Y
		cross := now - old
		if old > 0 || now < 0 || cross <= 1e-7 {
			break
		}
		f := clamp(-old/cross, 0, 1)
		x, y := before.X+(c.X-before.X)*f, before.Y+(c.Y-before.Y)*f
		lateral := math.Abs((x-p.X)*(-v.Y) + (y-p.Y)*v.X)
		if lateral > t.RoadWidth/2+CarRadius {
			break
		}
		if i == 0 && c.Gate == n-1 {
			c.Lap++
			s.emit("lap", c.Seat, at, 1)
			if c.Lap >= s.Options.Laps {
				c.FinishAt = at - stepMS + int64(math.Round(f*float64(stepMS)))
				c.Progress = float64(c.Lap) * total
				c.VX = 0
				c.VY = 0
				c.Speed = 0
				c.Throttle = 0
				c.Brake = true
				s.emit("finish", c.Seat, at, 1)
				s.Log = append(s.Log, fmt.Sprintf("玩家 %d 完成 %d 圈", c.Seat+1, c.Lap))
				if s.FinishDeadline == 0 {
					s.FinishDeadline = c.FinishAt + 30000
					s.Phase = "finishing"
				}
				c.Gate = 0
				c.Checkpoint = 1
				return
			}
		}
		c.Gate = i
		c.Checkpoint = (i + 1) % n
	}
	if c.Gate >= 0 {
		i := c.Gate
		a, b := t.Points[i], t.Points[(i+1)%n]
		dx, dy := b.X-a.X, b.Y-a.Y
		length := math.Hypot(dx, dy)
		local := clamp(((c.X-a.X)*dx+(c.Y-a.Y)*dy)/length, 0, length)
		c.Progress = float64(c.Lap)*total + arcs[i] + local
	}
}
func advance(s *State, until int64) {
	end := until
	if end > s.MaxRaceAt {
		end = s.MaxRaceAt
	}
	for s.ServerAt+stepMS <= end && s.Result == nil {
		at := s.ServerAt + stepMS
		previous := make([]Point, len(s.Cars))
		if s.Phase == "countdown" && at >= s.StartAt {
			s.Phase = "racing"
			s.emit("start", -1, s.StartAt, 1)
			s.Log = append(s.Log, "发车灯熄灭，比赛开始")
		}
		for i := range s.Cars {
			c := &s.Cars[i]
			previous[i] = Point{c.X, c.Y}
			for len(c.Pending) > 0 && c.Pending[0].At <= at {
				c.Applied = c.Pending[0]
				c.Pending = c.Pending[1:]
			}
			if s.Phase == "countdown" || c.FinishAt > 0 {
				continue
			}
			steer, throttle, brake, drift, last := c.Steer, c.Throttle, c.Brake, c.Drift, c.LastInputAt
			c.Steer = c.Applied.Steer
			c.Throttle = c.Applied.Throttle
			c.Brake = c.Applied.Brake
			c.Drift = c.Applied.Drift
			c.LastInputAt = c.Applied.At
			stepCar(c, s.Track, stepMS, at)
			c.Steer, c.Throttle, c.Brake, c.Drift, c.LastInputAt = steer, throttle, brake, drift, last
		}
		if s.Phase != "countdown" {
			collide(s, at)
			for i := range s.Cars {
				updateProgress(s, &s.Cars[i], previous[i], at)
			}
		}
		s.ServerAt = at
		if s.Phase == "finishing" {
			all := true
			for _, c := range s.Cars {
				all = all && c.FinishAt > 0
			}
			if all || at >= s.FinishDeadline {
				s.finish()
			}
		}
	}
	rank(s)
	if s.Result == nil && until >= s.MaxRaceAt {
		s.ServerAt = s.MaxRaceAt
		s.finish()
	}
}
