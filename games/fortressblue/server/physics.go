package fortressblue

import (
	"math"
)

func clamp(v, lo, hi float64) float64 {
	return max(lo, min(hi, v))
}

func colAt(x float64) int {
	c := int(x / ColumnWidth)
	if c < 0 {
		return 0
	}
	if c >= Columns {
		return Columns - 1
	}
	return c
}

// groundAt returns the y coordinate of the terrain surface at x. Heights are
// measured upwards from the bottom of the field.
func groundAt(heights []float64, x float64) float64 {
	if len(heights) == 0 {
		return FieldHeight
	}
	c := colAt(x)
	if c >= len(heights) {
		c = len(heights) - 1
	}
	return FieldHeight - clamp(heights[c], 0, FieldHeight)
}

func groundHeight(heights []float64, x float64) float64 {
	return FieldHeight - groundAt(heights, x)
}

func generateHeights(seed uint64, mapID string) []float64 {
	h := make([]float64, Columns)
	jitter := func(i int, mul float64) float64 {
		v := hashString(seedString(seed), mapID, string(rune(i)))
		return (float64(v%1000)/1000 - 0.5) * mul
	}
	for i := 0; i < Columns/2; i++ {
		x := float64(i)
		var value float64
		switch mapID {
		case "canyon":
			value = 82 - 48*math.Exp(-math.Pow((x-80)/16, 2)) + 6*math.Sin(x/7)
		case "islands":
			value = 34 + 17*math.Sin(x/11) + 11*math.Sin(x/3.2+0.4)
			if i > 54 && i < 106 {
				value -= 14
			}
		case "snow":
			value = 70 + 17*math.Sin(x/19) + 5*math.Sin(x/5+0.8)
		case "volcano":
			value = 42 + 66*math.Exp(-math.Pow((x-80)/22, 2)) + 8*math.Sin(x/6)
		default: // hills
			value = 55 + 18*math.Sin(x/13) + 8*math.Sin(x/4.5+0.7)
		}
		value += jitter(i, 1.8)
		h[i] = clamp(value, 8, 150)
		h[Columns-1-i] = h[i]
	}
	return h
}

func seedString(seed uint64) string {
	const digits = "0123456789"
	if seed == 0 {
		return "0"
	}
	var buf [24]byte
	i := len(buf)
	for seed > 0 {
		i--
		buf[i] = digits[seed%10]
		seed /= 10
	}
	return string(buf[i:])
}

func spawnX(teamSize, team, index int) float64 {
	center := 70.0
	if team == TeamBlue {
		center = FieldWidth - 70
	}
	spacing := 30.0
	switch teamSize {
	case 1:
		spacing = 0
	case 2:
		spacing = 36
	case 3:
		spacing = 32
	case 4:
		spacing = 30
	case 5:
		spacing = 28
	}
	return center + (float64(index)-float64(teamSize-1)/2)*spacing
}

func spawnYs(heights []float64, players int) [][2]float64 {
	t := players / 2
	out := make([][2]float64, players)
	for team := 0; team < 2; team++ {
		for i := 0; i < t; i++ {
			x := spawnX(t, team, i)
			out[team*t+i] = [2]float64{x, groundAt(heights, x)}
		}
	}
	return out
}

func flattenSpawns(h []float64, players int) {
	t := players / 2
	for team := 0; team < 2; team++ {
		for i := 0; i < t; i++ {
			x := spawnX(t, team, i)
			c := colAt(x)
			value := h[c]
			if value < 36 {
				value = 36
			}
			for d := -6; d <= 6; d++ {
				j := c + d
				if j >= 0 && j < Columns {
					h[j] = value
					h[Columns-1-j] = value
				}
			}
		}
	}
}

func heightsValid(h []float64) bool {
	if len(h) != Columns {
		return false
	}
	for _, v := range h {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > FieldHeight {
			return false
		}
	}
	return true
}

func heightsSymmetric(h []float64) bool {
	if len(h) != Columns {
		return false
	}
	for i, v := range h {
		if v != h[Columns-1-i] {
			return false
		}
	}
	return true
}

// simulateArc is pure: it never mutates the state and is used by both the
// server shot resolution and the bot's candidate search.
func simulateArc(heights []float64, tanks []Tank, shooter int, startX, startY, angle, power float64, facing int, wind int, limit int) Arc {
	speed := 80 + clamp(power, 20, 100)*1.6
	rad := clamp(angle, 5, 85) * math.Pi / 180
	vx := math.Cos(rad) * speed * float64(facing)
	vy := -math.Sin(rad) * speed
	x, y := startX+float64(facing)*9, startY-9
	path := make([]Point, 0, min(limit, MaxPath))
	path = append(path, Point{X: x, Y: y})
	dt := 1.0 / 120.0
	windAcc := float64(wind) * WindAcc
	travel := 0.0
	lastX, lastY := x, y
	for step := 0; step < 1500 && len(path) < limit; step++ {
		lastX, lastY = x, y
		vx += windAcc * dt
		vy += Gravity * dt
		x += vx * dt
		y += vy * dt
		travel += math.Hypot(vx*dt, vy*dt)
		if step%2 == 0 || x < -20 || x > FieldWidth+20 {
			path = append(path, Point{X: x, Y: y})
		}
		if x < -30 || x > FieldWidth+30 || y > FieldHeight+30 || y < -240 {
			return Arc{Path: path, Impact: Point{X: clamp(x, -30, FieldWidth+30), Y: clamp(y, -40, FieldHeight+30)}, Hit: -1}
		}
		for i := range tanks {
			t := &tanks[i]
			if !t.Alive {
				continue
			}
			if i == shooter && travel < 15 {
				continue
			}
			if math.Hypot(x-t.X, y-(t.Y-7)) <= 9.5 {
				path = append(path, Point{X: x, Y: y})
				return Arc{Path: path, Impact: Point{X: x, Y: y}, Hit: i}
			}
		}
		if y >= groundAt(heights, x) {
			gy := groundAt(heights, x)
			path = append(path, Point{X: x, Y: gy})
			return Arc{Path: path, Impact: Point{X: x, Y: gy}, Hit: -1}
		}
	}
	if len(path) == 0 {
		path = append(path, Point{X: lastX, Y: lastY})
	}
	return Arc{Path: path, Impact: path[len(path)-1], Hit: -1}
}

func carveTerrain(heights []float64, cx, cy, radius float64) bool {
	changed := false
	from := colAt(cx - radius)
	to := colAt(cx + radius)
	for c := from; c <= to; c++ {
		dx := math.Abs(float64(c)*ColumnWidth + ColumnWidth/2 - cx)
		if dx > radius {
			continue
		}
		bottom := cy + math.Sqrt(max(0, radius*radius-dx*dx))
		surface := groundAt(heights, float64(c)*ColumnWidth+ColumnWidth/2)
		if bottom <= surface {
			continue
		}
		newHeight := clamp(FieldHeight-bottom, 0, FieldHeight)
		if newHeight < heights[c] {
			heights[c] = newHeight
			changed = true
		}
	}
	return changed
}

func moveTank(s *State, seat int, dir int, at int64) bool {
	t := &s.Tanks[seat]
	if t.Fuel <= 0 || !t.Alive {
		return false
	}
	step := math.Min(9, t.Fuel)
	startX := t.X
	moved := 0.0
	for moved < step {
		next := t.X + float64(dir)*0.7
		if next < 7 || next > FieldWidth-7 {
			break
		}
		nextY := groundAt(s.Heights, next)
		if nextY-t.Y < -4.5 {
			break // too steep to climb
		}
		blocked := false
		for i := range s.Tanks {
			o := &s.Tanks[i]
			if i != seat && o.Alive && math.Abs(o.X-next) < 15 && math.Abs(o.Y-nextY) < 20 {
				blocked = true
				break
			}
		}
		if blocked {
			break
		}
		t.X, t.Y = next, nextY
		moved += 0.7
	}
	t.Fuel = math.Max(0, t.Fuel-moved)
	if moved > 0.01 {
		t.Facing = dir
		s.emit("move", seat, -1, t.X, t.Y, int(moved+0.5), "", at)
		return true
	}
	_ = startX
	return false
}

func settleTanks(s *State, killer *Tank, at int64) {
	water := 0.0
	if spec, ok := mapByID(s.MapID); ok {
		water = spec.Water
	}
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if !t.Alive {
			continue
		}
		ground := groundAt(s.Heights, t.X)
		if ground > t.Y {
			fall := ground - t.Y
			t.Y = ground
			if fall > 18 {
				hurt := int((fall - 18) * 0.9)
				if hurt > 0 {
					t.HP -= hurt
					t.Damage += 0
					s.emit("fall", i, i, t.X, t.Y, hurt, "", at)
					if killer != nil {
						killer.Damage += hurt
					}
				}
			}
		}
		if groundHeight(s.Heights, t.X) < water {
			t.HP = 0
		}
		if t.HP <= 0 && t.Alive {
			t.HP, t.Alive, t.Deaths = 0, false, t.Deaths+1
			s.emit("death", i, i, t.X, t.Y, 0, "沉没", at)
		}
	}
}
