package racing

import "math"

func SweepClassicRoad(t Track, before, next ClassicMotionState, vehicle string, height, dt float64) ClassicMotionState {
	return sweepClassicRoadWithProps(t, before, next, vehicle, height, dt, nil)
}

func sweepClassicRoadWithProps(t Track, before, next ClassicMotionState, vehicle string, height, dt float64, props *classicPropCollision) ClassicMotionState {
	steps := int(math.Max(1, math.Ceil(math.Hypot(next.X-before.X, next.Z-before.Z)/.35)))
	steps = min(steps, 80)
	next.X, next.Z = before.X, before.Z
	for i := 0; i < steps; i++ {
		next.X += next.VX * dt / float64(steps)
		next.Z += next.VZ * dt / float64(steps)
		next = collideClassicRoadWithProps(t, next, vehicle, height, props)
	}
	return next
}

func classicCarContact(a, b Car) (Point, Point, float64, float64, float64) {
	abody, bbody := ClassicVehicleBody(a.Vehicle), ClassicVehicleBody(b.Vehicle)
	if a.Height+abody.Height*10 < b.Height || b.Height+bbody.Height*10 < a.Height {
		return Point{}, Point{}, 0, 0, 0
	}
	aa, az, ar := classicCapsule(a.X, a.Y, a.Angle, abody)
	ba, bz, br := classicCapsule(b.X, b.Y, b.Angle, bbody)
	p, q := classicSegmentClosest(aa, az, ba, bz)
	dx, dy := q.X-p.X, q.Y-p.Y
	distance := math.Hypot(dx, dy)
	depth := ar + br - distance
	if depth <= 0 {
		return p, q, 0, 0, 0
	}
	if distance > 1e-7 {
		return p, q, dx / distance, dy / distance, depth
	}
	nx, ny := b.X-a.X, b.Y-a.Y
	if length := math.Hypot(nx, ny); length > 1e-7 {
		nx, ny = nx/length, ny/length
	} else {
		nx, ny = -math.Sin(a.Angle), math.Cos(a.Angle)
	}
	extent := ar + br + math.Abs((az.X-aa.X)*nx+(az.Y-aa.Y)*ny)*.5 + math.Abs((bz.X-ba.X)*nx+(bz.Y-ba.Y)*ny)*.5
	return p, q, nx, ny, math.Max(0, extent-((b.X-a.X)*nx+(b.Y-a.Y)*ny))
}

func classicImpactYaw(c *Car, point Point, dvx, dvy float64) {
	params := ClassicVehicle(c.Vehicle)
	motion := classicMotionFromCar(*c)
	moment := ((point.X-c.X)*dvy - (point.Y-c.Y)*dvx) / 100
	motion.YawRate = clamp(motion.YawRate+moment/(params.Wheelbase*params.Wheelbase*.35), -2.4, 2.4)
	motion.LongAccel = -math.Min(30, math.Hypot(dvx, dvy)/5)
	classicMotionToCar(c, motion)
}

func collideClassicCars(s *State, at int64, previous ...[]Car) {
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
				p, q, nx, ny, depth := classicCarContact(*a, *b)
				remaining := 0.
				if pass == 0 && len(previous) > 0 && len(previous[0]) == len(s.Cars) {
					a0, b0 := previous[0][i], previous[0][j]
					dax, day, dbx, dby := a.X-a0.X, a.Y-a0.Y, b.X-b0.X, b.Y-b0.Y
					travel := math.Hypot(dax-dbx, day-dby)
					if math.Hypot(a0.X-b0.X, a0.Y-b0.Y) > travel+55 {
						continue
					}
					steps := min(80, int(math.Max(1, math.Ceil(travel/4))))
					for k := 0; k <= steps; k++ {
						f := float64(k) / float64(steps)
						ac, bc := *a, *b
						ac.X, ac.Y = a0.X+dax*f, a0.Y+day*f
						bc.X, bc.Y = b0.X+dbx*f, b0.Y+dby*f
						ac.Height = a0.Height + (a.Height-a0.Height)*f
						bc.Height = b0.Height + (b.Height-b0.Height)*f
						p, q, nx, ny, depth = classicCarContact(ac, bc)
						if depth > 0 {
							a.X, a.Y, b.X, b.Y = ac.X, ac.Y, bc.X, bc.Y
							remaining = (1 - f) * float64(stepMS) / 1000
							break
						}
					}
				}
				if depth <= 0 {
					continue
				}
				ia, ib := 1/ClassicVehicle(a.Vehicle).Mass, 1/ClassicVehicle(b.Vehicle).Mass
				a.X -= nx * (depth + .001) * ia / (ia + ib)
				a.Y -= ny * (depth + .001) * ia / (ia + ib)
				b.X += nx * (depth + .001) * ib / (ia + ib)
				b.Y += ny * (depth + .001) * ib / (ia + ib)
				relative := (a.VX-b.VX)*nx + (a.VY-b.VY)*ny
				if relative > 0 {
					impulse := relative * 1.12 / (ia + ib)
					side := -(a.VX-b.VX)*ny + (a.VY-b.VY)*nx
					friction := clamp(side/(ia+ib), -impulse*.3, impulse*.3)
					ix, iy := impulse*nx-friction*ny, impulse*ny+friction*nx
					a.VX -= ix * ia
					a.VY -= iy * ia
					b.VX += ix * ib
					b.VY += iy * ib
					classicImpactYaw(a, p, -ix*ia, -iy*ia)
					classicImpactYaw(b, q, ix*ib, iy*ib)
					if relative > 45 {
						s.emit("collision", i, at, clamp(relative/350, 0, 1))
						s.emit("collision", j, at, clamp(relative/350, 0, 1))
					}
				}
				a.X += a.VX * remaining
				a.Y += a.VY * remaining
				b.X += b.VX * remaining
				b.Y += b.VY * remaining
				classicMotionToCar(a, classicMotionFromCar(*a))
				classicMotionToCar(b, classicMotionFromCar(*b))
				classicPose(a, s.Track)
				classicPose(b, s.Track)
			}
		}
	}
}
