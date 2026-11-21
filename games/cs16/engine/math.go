package engine

var cordicAngles = [...]int{8192, 4836, 2555, 1297, 651, 326, 163, 81, 41, 20, 10, 5, 3, 1, 1}

func SinCos(angle int) (int, int) {
	angle = ((angle+32768)%65536+65536)%65536 - 32768
	switch angle {
	case 0:
		return 0, NormalUnit
	case 16384:
		return NormalUnit, 0
	case -16384:
		return -NormalUnit, 0
	case -32768:
		return 0, -NormalUnit
	}
	sign := 1
	if angle > 16384 {
		angle -= 32768
		sign = -1
	}
	if angle < -16384 {
		angle += 32768
		sign = -1
	}
	x, y := 636751, 0
	for shift, step := range cordicAngles {
		if angle >= 0 {
			x, y = x-(y>>shift), y+(x>>shift)
			angle -= step
		} else {
			x, y = x+(y>>shift), y-(x>>shift)
			angle += step
		}
	}
	return y * sign, x * sign
}

func Direction(yaw, pitch int) Vec3 {
	sy, cy := SinCos(yaw)
	sp, cp := SinCos(pitch)
	return Vec3{int(int64(cy) * int64(cp) / NormalUnit), int(int64(sy) * int64(cp) / NormalUnit), -sp}
}

func AngleTo(dx, dy int) int {
	if dx == 0 && dy == 0 {
		return 0
	}
	angle := 0
	if dx < 0 {
		dx, dy = -dx, -dy
		angle = 32768
	}
	for shift, step := range cordicAngles {
		if dy > 0 {
			dx, dy = dx+(dy>>shift), dy-(dx>>shift)
			angle += step
		} else if dy < 0 {
			dx, dy = dx-(dy>>shift), dy+(dx>>shift)
			angle -= step
		}
	}
	return ((angle % 65536) + 65536) % 65536
}

func Sqrt(value int64) int {
	if value <= 0 {
		return 0
	}
	x := value
	y := (x + 1) / 2
	for y < x {
		x = y
		y = (x + value/x) / 2
	}
	return int(x)
}

func Distance(a, b Vec3) int {
	x, y, z := int64(a.X-b.X), int64(a.Y-b.Y), int64(a.Z-b.Z)
	return Sqrt(x*x + y*y + z*z)
}

func Length2D(x, y int) int { return Sqrt(int64(x)*int64(x) + int64(y)*int64(y)) }
func Abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
func Clamp(value, low, high int) int { return min(high, max(low, value)) }
func Add(a, b Vec3) Vec3             { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func Sub(a, b Vec3) Vec3             { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func Scale(a Vec3, numerator, denominator int) Vec3 {
	return Vec3{int(int64(a.X) * int64(numerator) / int64(denominator)), int(int64(a.Y) * int64(numerator) / int64(denominator)), int(int64(a.Z) * int64(numerator) / int64(denominator))}
}
func Dot(a, b Vec3) int {
	return int((int64(a.X)*int64(b.X) + int64(a.Y)*int64(b.Y) + int64(a.Z)*int64(b.Z)) / NormalUnit)
}
func Contains(bounds Bounds, p Vec3) bool {
	return p.X >= bounds.Min.X && p.Y >= bounds.Min.Y && p.Z >= bounds.Min.Z && p.X <= bounds.Max.X && p.Y <= bounds.Max.Y && p.Z <= bounds.Max.Z
}
func Position(player Player) Vec3 { return Vec3{player.X, player.Y, player.Z} }
func Eye(player Player) Vec3 {
	z := 17 * Unit
	if player.Crouch {
		z = 12 * Unit
	}
	return Vec3{player.X, player.Y, player.Z + z}
}

func Emit(state *State, event Event) {
	state.EventID++
	event.ID, event.Frame = state.EventID, state.Frame
	state.Events = append(state.Events, event)
	if len(state.Events) > MaxEvents {
		state.Events = append([]Event(nil), state.Events[len(state.Events)-MaxEvents:]...)
	}
}
