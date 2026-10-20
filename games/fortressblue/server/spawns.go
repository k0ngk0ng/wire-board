package fortressblue

import (
	"fmt"
	fbdata "github.com/k0ngk0ng/wire-board/games/fortressblue/data"
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	"math"
)

func spawnBody(spec fbdata.Tank) (float64, float64) {
	// Match the actual movement body. A whole sprite rectangle includes empty
	// corners, weapon effects and slope poses; it is not a collision silhouette.
	_ = spec
	return tankTrackHalfWidth, 28
}

// The old catalog selected the lowest surface and checked only a 20px centre
// column. Keep it as a preferred horizontal location, then choose a supported
// exterior surface with clearance for the same body and muzzle used by movement and ballistics.
func spawnClear(m fbmaps.Map, mask []byte, x, y, half, height float64) bool {
	if x-half < 4 || x+half >= float64(m.Width)-4 || y-height < 0 || y >= float64(m.Height) {
		return false
	}
	for gx := int(math.Floor((x - half) / 4)); gx <= int(math.Floor((x+half)/4)); gx++ {
		for gy := int(math.Floor((y - height) / 4)); gy <= int(math.Floor((y-4)/4)); gy++ {
			if solid(m, mask, float64(gx*4+2), float64(gy*4+2)) {
				return false
			}
		}
	}
	supported := 0
	for dx := -10.0; dx <= 10; dx += 2 {
		if solid(m, mask, x+dx, y+1) {
			supported++
		}
	}
	if supported < 2 {
		return false
	}
	for _, dir := range []float64{-1, 1} {
		g, ok := walkGround(m, mask, x, y, x+dir*1.5)
		if ok && g <= y+8 && g <= float64(m.Height) {
			return true
		}
	}
	return false
}

func spawnHasShot(m fbmaps.Map, mask []byte, tanks []Tank, seat int, spec fbdata.Tank, wind int) bool {
	t := tanks[seat]
	for _, facing := range []int{t.Facing, -t.Facing} {
		for _, angle := range []float64{float64(spec.MinAngle), (float64(spec.MinAngle) + float64(spec.Angle)) / 2, float64(spec.Angle)} {
			for power := 10.0; power <= 100; power += 10 {
				arc := simulateArc(m, mask, tanks, seat, t.X, t.Y, angle, power, facing, float64(spec.Range)/50, wind, MaxPath)
				if arc.OutOfBounds || arc.Hit == seat || math.Hypot(arc.Impact.X-t.X, arc.Impact.Y-(t.Y-18)) < 110 {
					continue
				}
				if arc.Hit >= 0 && tanks[arc.Hit].Team == t.Team {
					continue
				}
				return true
			}
		}
	}
	return false
}

func spawnPlayable(s *State, m fbmaps.Map, mask []byte, seat int, spec fbdata.Tank) bool {
	wind := 0
	for i, turnSeat := range s.Order {
		if turnSeat == seat {
			wind = windFor(s.Seed, i+1, false)
			break
		}
	}
	if !spawnHasShot(m, mask, s.Tanks, seat, spec, 0) {
		return false
	}
	return wind == 0 || spawnHasShot(m, mask, s.Tanks, seat, spec, wind)
}

func chooseSafeSpawn(s *State, m fbmaps.Map, mask []byte, seat int, placed []bool) bool {
	tank := &s.Tanks[seat]
	spec, _ := tankSpec(tank.TankID)
	half, height := spawnBody(spec)
	desired := int(math.Round(tank.X))
	for offset := 0; offset < m.Width; offset += 2 {
		for _, sign := range []int{1, -1} {
			if offset == 0 && sign < 0 {
				continue
			}
			x := float64(desired + sign*offset)
			if tank.Team == TeamRed && x > float64(m.Width)/2-40 || tank.Team == TeamBlue && x < float64(m.Width)/2+40 {
				continue
			}
			if x < half+4 || x > float64(m.Width)-half-4 {
				continue
			}
			y := tankGroundY(m, mask, x, 0)
			if !spawnClear(m, mask, x, y, half, height) {
				continue
			}
			occupied := false
			for other, done := range placed {
				if !done || other == seat {
					continue
				}
				if math.Abs(s.Tanks[other].X-x) < 64 {
					occupied = true
					break
				}
			}
			if occupied {
				continue
			}
			candidate := *s
			candidate.Tanks = append([]Tank(nil), s.Tanks...)
			candidate.Tanks[seat].X, candidate.Tanks[seat].Y = x, y
			if !spawnPlayable(&candidate, m, mask, seat, spec) {
				continue
			}
			tank.X, tank.Y = x, y
			return true
		}
	}
	return false
}

func placeSafeSpawns(s *State, m fbmaps.Map, mask []byte) error {
	placed := make([]bool, len(s.Tanks))
	for seat := range s.Tanks {
		if !chooseSafeSpawn(s, m, mask, seat, placed) {
			return fmt.Errorf("地图 %s 的 %d 号位缺少安全出生点", m.Name, seat+1)
		}
		placed[seat] = true
	}
	// A later tank must not close an earlier tank's only firing lane. Validate
	// the completed formation, including each seat's actual first-turn wind.
	for pass := 0; pass < len(s.Tanks)*2; pass++ {
		stable := true
		for seat, tank := range s.Tanks {
			spec, _ := tankSpec(tank.TankID)
			if spawnPlayable(s, m, mask, seat, spec) {
				continue
			}
			stable = false
			if !chooseSafeSpawn(s, m, mask, seat, placed) {
				return fmt.Errorf("地图 %s 的 %d 号位缺少安全射界", m.Name, seat+1)
			}
		}
		if stable {
			return nil
		}
	}
	return fmt.Errorf("地图 %s 无法布置互不封锁的出生点", m.Name)
}
