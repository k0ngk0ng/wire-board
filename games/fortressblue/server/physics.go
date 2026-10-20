package fortressblue

import (
	"math"

	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
)

func clamp(v, lo, hi float64) float64 { return max(lo, min(hi, v)) }

func cellAt(m fbmaps.Map, x, y float64) (int, int) {
	cx := int(math.Floor(x / float64(fbmaps.Cell())))
	cy := int(math.Floor(y / float64(fbmaps.Cell())))
	return cx, cy
}

func solid(m fbmaps.Map, mask []byte, x, y float64) bool {
	if x < 0 || y < 0 || x >= float64(m.Width) || y >= float64(m.Height) {
		return false
	}
	cx, cy := cellAt(m, x, y)
	if cx < 0 || cy < 0 || cx >= m.MaskCols || cy >= m.MaskRows {
		return false
	}
	idx := cy*m.MaskCols + cx
	return mask[idx>>3]&(1<<(7-(idx&7))) != 0
}

func maskFor(m fbmaps.Map, craters []Crater) []byte {
	mask := fbmaps.BaseMask(m)
	for _, c := range craters {
		carveMask(m, mask, c.X, c.Y, c.R)
	}
	return mask
}

func carveMask(m fbmaps.Map, mask []byte, cx, cy, radius float64) bool {
	if radius <= 0 {
		return false
	}
	cell := float64(fbmaps.Cell())
	fromX := max(0, int((cx-radius)/cell))
	toX := min(m.MaskCols-1, int((cx+radius)/cell)+1)
	fromY := max(0, int((cy-radius)/cell))
	toY := min(m.MaskRows-1, int((cy+radius)/cell)+1)
	changed := false
	for gy := fromY; gy <= toY; gy++ {
		for gx := fromX; gx <= toX; gx++ {
			px := float64(gx)*cell + cell/2
			py := float64(gy)*cell + cell/2
			if math.Hypot(px-cx, py-cy) > radius {
				continue
			}
			idx := gy*m.MaskCols + gx
			if mask[idx>>3]&(1<<(7-(idx&7))) != 0 {
				mask[idx>>3] &^= 1 << (7 - (idx & 7))
				changed = true
			}
		}
	}
	return changed
}

// groundY returns the top of the first solid cell at or below y. When no
// ground exists it returns the world bottom so callers can treat the tank as
// falling out of the map.
func groundY(m fbmaps.Map, mask []byte, x, y float64) float64 {
	if x < 0 || x >= float64(m.Width) {
		return -1
	}
	cx := int(x / float64(fbmaps.Cell()))
	if cx < 0 || cx >= m.MaskCols {
		return -1
	}
	start := max(0, int(y/float64(fbmaps.Cell())))
	for cy := start; cy < m.MaskRows; cy++ {
		idx := cy*m.MaskCols + cx
		if mask[idx>>3]&(1<<(7-(idx&7))) != 0 {
			return float64(cy * fbmaps.Cell())
		}
	}
	return float64(m.Height) + 200
}

func tankCenter(t Tank) Point { return Point{X: t.X, Y: t.Y - 18} }

// simulateArc is pure and shared by server shot resolution and the bot search.
func simulateArc(m fbmaps.Map, mask []byte, tanks []Tank, shooter int, startX, startY, angle, power float64, facing int, rangeScale float64, wind int, limit int) Arc {
	speed := (120 + clamp(power, 20, 100)*3.2) * clamp(rangeScale, 0.7, 1.3)
	rad := clamp(angle, 0, 90) * math.Pi / 180
	vx := math.Cos(rad) * speed * float64(facing)
	vy := -math.Sin(rad) * speed
	x := startX + float64(facing)*10
	y := startY - 26
	path := make([]Point, 0, min(limit, MaxPath))
	path = append(path, Point{X: x, Y: y})
	dt := 1.0 / 240.0
	windAcc := float64(wind) * WindAcc
	travel := 0.0
	for step := 0; step < 1800 && len(path) < limit; step++ {
		vx += windAcc * dt
		vy += Gravity * dt
		nx := x + vx*dt
		ny := y + vy*dt
		travel += math.Hypot(nx-x, ny-y)
		// Swept sampling prevents fast shells from tunnelling through a 4 px cell.
		sub := max(1, int(math.Ceil(math.Hypot(nx-x, ny-y)/2)))
		for s := 1; s <= sub; s++ {
			t := float64(s) / float64(sub)
			px := x + (nx-x)*t
			py := y + (ny-y)*t
			if px < -60 || px > float64(m.Width)+60 || py > float64(m.Height)+320 || py < -240 {
				path = append(path, Point{X: px, Y: py})
				return Arc{Path: path, Impact: Point{X: clamp(px, -60, float64(m.Width)+60), Y: clamp(py, -80, float64(m.Height)+80)}, Hit: -1}
			}
			for i := range tanks {
				if !tanks[i].Alive || (i == shooter && travel < 24) {
					continue
				}
				c := tankCenter(tanks[i])
				if math.Hypot(px-c.X, py-c.Y) <= 20 {
					path = append(path, Point{X: px, Y: py})
					return Arc{Path: path, Impact: Point{X: px, Y: py}, Hit: i}
				}
			}
			if solid(m, mask, px, py) {
				path = append(path, Point{X: px, Y: py})
				return Arc{Path: path, Impact: Point{X: px, Y: py}, Hit: -1}
			}
		}
		x, y = nx, ny
		if step%4 == 0 {
			path = append(path, Point{X: x, Y: y})
		}
	}
	if len(path) == 0 {
		path = append(path, Point{X: x, Y: y})
	}
	return Arc{Path: path, Impact: path[len(path)-1], Hit: -1}
}

func moveTank(s *State, seat int, dir int, at int64) bool {
	if dir != -1 && dir != 1 {
		return false
	}
	m, ok := fbmaps.ByID(s.MapID)
	if !ok {
		return false
	}
	t := &s.Tanks[seat]
	if !t.Alive || t.Fuel <= 0 {
		return false
	}
	mask := maskFor(m, s.Craters)
	step := math.Min(10, t.Fuel)
	moved := 0.0
	blocked := false
	for moved < step {
		nx := t.X + float64(dir)*1.5
		if nx < 4 || nx > float64(m.Width)-4 {
			break
		}
		ny := groundY(m, mask, nx, t.Y-8)
		if ny < 0 || ny > float64(m.Height) {
			break
		}
		if ny-t.Y < -8 { // too steep to climb
			blocked = true
			break
		}
		for i := range s.Tanks {
			o := &s.Tanks[i]
			if i != seat && o.Alive && math.Abs(o.X-nx) < 26 && math.Abs(o.Y-ny) < 30 {
				blocked = true
				break
			}
		}
		if blocked {
			break
		}
		t.X, t.Y = nx, ny
		moved += 1.5
	}
	if moved <= 0 {
		return false
	}
	t.Facing = dir
	t.Fuel = math.Max(0, t.Fuel-moved)
	s.emit("move", seat, -1, t.X, t.Y, int(moved+0.5), "", at)
	return true
}

func settleTanks(s *State, at int64) {
	m, ok := fbmaps.ByID(s.MapID)
	if !ok {
		return
	}
	mask := maskFor(m, s.Craters)
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if !t.Alive {
			continue
		}
		ground := groundY(m, mask, t.X, t.Y-10)
		if ground < 0 || ground > float64(m.Height) {
			t.HP, t.Alive, t.Deaths = 0, false, t.Deaths+1
			s.emit("death", i, i, t.X, t.Y, 0, "坠入深渊", at)
			continue
		}
		if ground > t.Y {
			fall := ground - t.Y
			t.Y = ground
			if fall > 36 {
				hurt := int((fall - 36) * 0.9)
				if hurt > 0 {
					t.HP -= hurt
					s.DamageTotal[i] += 0 // self-inflicted fall damage does not score
					s.emit("fall", i, i, t.X, t.Y, hurt, "", at)
				}
			}
		}
		if t.HP <= 0 {
			t.HP, t.Alive, t.Deaths = 0, false, t.Deaths+1
			s.emit("death", i, i, t.X, t.Y, 0, "", at)
		}
	}
}

func blastRadius(weapon Weapon) float64 {
	r := 36 + weapon.Blast
	return clamp(r, 28, 90)
}

func applyBlast(s *State, seat int, weapon Weapon, center Point, kind string, at int64, direct int) Blast {
	mapSpec, _ := fbmaps.ByID(s.MapID)
	center.X = clamp(center.X, 0, float64(mapSpec.Width))
	center.Y = clamp(center.Y, 0, float64(mapSpec.Height)+40)
	radius := blastRadius(weapon)
	blast := Blast{X: center.X, Y: center.Y, R: radius, Seat: seat, Kind: kind}
	shooter := &s.Tanks[seat]
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if !t.Alive || (t.Team == shooter.Team && i != seat) {
			continue
		}
		c := tankCenter(*t)
		dist := math.Hypot(center.X-c.X, center.Y-c.Y)
		if dist > radius {
			continue
		}
		damage := int(math.Round(weapon.Damage * (1 - 0.5*dist/radius) * 100 / (100 + float64(tankDefense(*t))*0.6)))
		if kind == "lightning" && i == direct {
			damage = int(float64(damage) * 1.3)
		}
		if s.Sudden {
			damage *= 2
		}
		damage = shieldedDamage(t, damage)
		t.HP -= damage
		t.Damage += damage
		shooter.Damage += damage
		s.DamageTotal[seat] += damage
		blast.Victim, blast.Damage = i, damage
		s.emit("hit", seat, i, t.X, t.Y, damage, kind, at)
		if kind == "poison" {
			addStatus(t, Status{Kind: "poison", Turns: 3, Damage: 25, Source: seat})
		} else if kind == "fire" {
			addStatus(t, Status{Kind: "burn", Turns: 3, Damage: 18, Source: seat})
		}
		if t.HP <= 0 && t.Alive {
			t.HP, t.Alive, t.Deaths = 0, false, t.Deaths+1
			if i != seat {
				shooter.Kills++
			}
			s.emit("death", seat, i, t.X, t.Y, 0, "", at)
		}
	}
	s.Craters = append(s.Craters, Crater{X: center.X, Y: center.Y, R: radius, Seat: seat})
	if len(s.Craters) > MaxCraters {
		s.Craters = append([]Crater(nil), s.Craters[len(s.Craters)-MaxCraters:]...)
	}
	blast.Terrain = true
	return blast
}

func shieldedDamage(t *Tank, damage int) int {
	if hasStatus(t, "shield") {
		damage = max(1, int(math.Round(float64(damage)*0.5)))
	}
	return max(1, damage)
}

func addStatus(t *Tank, st Status) {
	for i := range t.Statuses {
		if t.Statuses[i].Kind == st.Kind {
			t.Statuses[i] = st
			return
		}
	}
	if len(t.Statuses) < MaxStatuses {
		t.Statuses = append(t.Statuses, st)
	}
}

func tickStatuses(s *State, at int64) {
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if !t.Alive || len(t.Statuses) == 0 {
			continue
		}
		next := t.Statuses[:0]
		for _, st := range t.Statuses {
			if st.Turns <= 0 {
				continue
			}
			if st.Damage > 0 {
				t.HP -= st.Damage
				if st.Source >= 0 && st.Source < len(s.Tanks) {
					s.DamageTotal[st.Source] += st.Damage
					s.Tanks[st.Source].Damage += st.Damage
				}
				s.emit("status", st.Source, i, t.X, t.Y, st.Damage, st.Kind, at)
			}
			st.Turns--
			if st.Turns > 0 {
				next = append(next, st)
			}
		}
		t.Statuses = next
		if t.HP <= 0 && t.Alive {
			t.HP, t.Alive, t.Deaths = 0, false, t.Deaths+1
			s.emit("death", -1, i, t.X, t.Y, 0, "状态伤害", at)
		}
	}
}

func tankDefense(t Tank) int {
	if spec, ok := tankSpec(t.TankID); ok {
		return spec.Defense
	}
	return 100
}
