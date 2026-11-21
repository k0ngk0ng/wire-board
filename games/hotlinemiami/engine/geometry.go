package engine

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func sqrt(v int64) int64 {
	if v <= 0 {
		return 0
	}
	x := v
	y := (x + 1) / 2
	for y < x {
		x = y
		y = (x + v/x) / 2
	}
	return x
}

func sine(angle int) int {
	angle = ((angle % 1024) + 1024) % 1024
	sign := 1
	if angle > 512 {
		angle -= 512
		sign = -1
	}
	n := int64(angle * (512 - angle))
	return sign * int(16*n*1024/(5*512*512-4*n))
}

func Direction(aim int) (int, int) {
	return sine(aim + 256), sine(aim)
}

func AimAt(x, y int) int {
	if x == 0 && y == 0 {
		return 0
	}
	ax, ay := abs(x), abs(y)
	a := 0
	if ax >= ay {
		a = ay * 128 / max(1, ax)
	} else {
		a = 256 - ax*128/max(1, ay)
	}
	if x < 0 {
		a = 512 - a
	}
	if y < 0 {
		a = (1024 - a) % 1024
	}
	return a
}

func distance2(x, y, tx, ty int) int64 {
	dx, dy := int64(tx-x), int64(ty-y)
	return dx*dx + dy*dy
}

func circleRect(x, y, r int, wall Rect) bool {
	cx, cy := clamp(x, wall.X, wall.X+wall.W), clamp(y, wall.Y, wall.Y+wall.H)
	return distance2(x, y, cx, cy) < int64(r)*int64(r)
}

func hasInt(values []int, value int) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func doorOpen(doors []Door, id int) bool {
	for _, door := range doors {
		if door.ID == id {
			return door.Open
		}
	}
	return false
}

func blocked(world Map, doors []Door, broken []int, x, y int) bool {
	if x < Radius || y < Radius || x > world.Width-Radius || y > world.Height-Radius {
		return true
	}
	for i, wall := range world.Walls {
		if wall.Kind == "glass" && hasInt(broken, i) {
			continue
		}
		if circleRect(x, y, Radius, wall) {
			return true
		}
	}
	for _, door := range world.Doors {
		if !doorOpen(doors, door.ID) && circleRect(x, y, Radius, Rect{X: door.X, Y: door.Y, W: door.W, H: door.H}) {
			return true
		}
	}
	return false
}

func ceilDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a > 0) == (b > 0) {
		q++
	}
	return q
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a > 0) != (b > 0) {
		q--
	}
	return q
}

func segmentRect(x, y, tx, ty int, wall Rect) (int, bool) {
	lo, hi := int64(0), int64(65536)
	for _, axis := range [][4]int{{x, tx - x, wall.X, wall.X + wall.W}, {y, ty - y, wall.Y, wall.Y + wall.H}} {
		origin, delta, minV, maxV := int64(axis[0]), int64(axis[1]), int64(axis[2]), int64(axis[3])
		if delta == 0 {
			if origin < minV || origin > maxV {
				return 0, false
			}
			continue
		}
		if delta < 0 {
			minV, maxV = maxV, minV
		}
		enter, leave := ceilDiv((minV-origin)*65536, delta), floorDiv((maxV-origin)*65536, delta)
		lo = max(lo, enter)
		hi = min(hi, leave)
		if lo > hi {
			return 0, false
		}
	}
	return int(lo), true
}

func segmentCircle(x, y, tx, ty, cx, cy, r int) (int, bool) {
	dx, dy := int64(tx-x), int64(ty-y)
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return 0, distance2(x, y, cx, cy) <= int64(r)*int64(r)
	}
	t := clamp(int(((int64(cx-x)*dx+int64(cy-y)*dy)*65536)/l2), 0, 65536)
	px, py := x+int(dx*int64(t)/65536), y+int(dy*int64(t)/65536)
	if distance2(px, py, cx, cy) > int64(r)*int64(r) {
		return 0, false
	}
	return t, true
}

func Visible(world Map, doors []Door, broken []int, x, y, tx, ty int) bool {
	for i, wall := range world.Walls {
		if wall.Kind == "glass" || hasInt(broken, i) {
			continue
		}
		if _, hit := segmentRect(x, y, tx, ty, wall); hit {
			return false
		}
	}
	for _, door := range world.Doors {
		if !doorOpen(doors, door.ID) {
			if _, hit := segmentRect(x, y, tx, ty, Rect{X: door.X, Y: door.Y, W: door.W, H: door.H}); hit {
				return false
			}
		}
	}
	return true
}

func PredictMotion(world Map, x, y, aim, moveX, moveY, frames int, doors []Door, broken []int) (int, int, int) {
	frames = clamp(frames, 0, 9)
	moveX = clamp(moveX, -1000, 1000)
	moveY = clamp(moveY, -1000, 1000)
	l2 := int64(moveX*moveX + moveY*moveY)
	length := sqrt(l2)
	if length*length < l2 {
		length++
	}
	denom := max(1000, int(length))
	dx, dy := MoveSpeed*moveX/denom, MoveSpeed*moveY/denom
	for frame := 0; frame < frames; frame++ {
		steps := max(abs(dx), abs(dy))/(Radius/2) + 1
		for step := 1; step <= steps; step++ {
			vx := dx*step/steps - dx*(step-1)/steps
			vy := dy*step/steps - dy*(step-1)/steps
			if !blocked(world, doors, broken, x+vx, y) {
				x += vx
			}
			if !blocked(world, doors, broken, x, y+vy) {
				y += vy
			}
		}
	}
	return x, y, ((aim % 1024) + 1024) % 1024
}

func PredictPlayer(world Map, p Player, in Input, frames int) Player {
	if !p.Alive {
		return p
	}
	p.X, p.Y, p.Aim = PredictMotion(world, p.X, p.Y, in.Aim, in.MoveX, in.MoveY, frames, nil, nil)
	return p
}
