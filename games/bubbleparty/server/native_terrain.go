package bubbleparty

import "math"

func nativeDirection(dx, dy int) int {
	switch {
	case dx == -1 && dy == 0:
		return 0
	case dx == 0 && dy == -1:
		return 1
	case dx == 1 && dy == 0:
		return 2
	case dx == 0 && dy == 1:
		return 3
	}
	return -1
}
func (s *State) nativeObjectFields(x, y int) []int {
	if !nativeInBounds(x, y) || len(s.NativeObjects) != Cols*Rows {
		return nil
	}
	if t := classicTileIndex[s.NativeObjects[y*Cols+x]]; t != nil {
		return t.Info.Fields
	}
	return nil
}
func (s *State) nativePassage(fromX, fromY, dx, dy int, water bool) bool {
	if classicMapIndex[s.MapID] == nil {
		return true
	}
	direction := nativeDirection(dx, dy)
	toX, toY := fromX+dx, fromY+dy
	if direction < 0 || !nativeInBounds(fromX, fromY) || !nativeInBounds(toX, toY) {
		return false
	}
	permit := func(f []int, d int) bool { return len(f) < 6 || f[0] != 3 || (water && f[5] == 0) || f[d+1] != 0 }
	return permit(s.nativeObjectFields(fromX, fromY), direction) && permit(s.nativeObjectFields(toX, toY), (direction+2)%4)
}
func (s *State) nativeGroundFields(x, y int) []int {
	m := classicMapIndex[s.MapID]
	if m == nil {
		return nil
	}
	ox, oy := classicOffset(*m)
	x -= ox
	y -= oy
	if x < 0 || y < 0 || x >= m.Width || y >= m.Height {
		return nil
	}
	if t := classicTileIndex[m.Ground[y][x]]; t != nil {
		return t.Info.Fields
	}
	return nil
}
func (s *State) clearNativeObject(x, y int) {
	if nativeInBounds(x, y) && len(s.NativeObjects) == Cols*Rows {
		s.NativeObjects[y*Cols+x] = -1
	}
}
func (s *State) moveMS(p *Player) float64 {
	f := s.nativeGroundFields(p.X, p.Y)
	if p.Mount == "railcar" && len(f) > 8 && f[8] != 0 {
		return 120
	}
	return effectiveMoveMS(p)
}
func (s *State) nativeConveyorDirection(x, y int) string {
	f := s.nativeGroundFields(x, y)
	if len(f) < 6 || f[3] != 2 || f[5] < 0 || f[5] > 3 {
		return "none"
	}
	return []string{"left", "up", "right", "down"}[f[5]]
}

// nativeConveyorMotion mirrors 0x449850: first pull toward the tile anchor
// according to the direction mask, then use the exit direction when centered.
// The native 2px/30ms motion is scaled to the platform substep, not retuned.
func (s *State) nativeConveyorMotion(p *Player) (dx, dy int, distance float64) {
	if p == nil || !p.Alive || p.TrappedUntil > 0 || mountPassesBoxes(p.Mount) {
		return 0, 0, 0
	}
	f := s.nativeGroundFields(p.X, p.Y)
	if len(f) < 6 || f[3] != 2 {
		return 0, 0, 0
	}
	budget := float64(StepMS) / 30 * float64(f[3]) / 40
	corrections := []struct {
		bit, dx, dy int
		delta       float64
	}{{1, -1, 0, p.OffsetX}, {2, 0, -1, p.OffsetY}, {4, 1, 0, -p.OffsetX}, {8, 0, 1, -p.OffsetY}}
	for _, c := range corrections {
		if f[4]&c.bit != 0 && c.delta > 0 {
			return c.dx, c.dy, math.Min(c.delta, budget)
		}
	}
	if f[5] < 0 || f[5] > 3 {
		return 0, 0, 0
	}
	d := [][2]int{{-1, 0}, {0, -1}, {1, 0}, {0, 1}}[f[5]]
	return d[0], d[1], budget
}
func (s *State) nativeTransport(p *Player, at int64) {
	m := classicMapIndex[s.MapID]
	if m == nil || p == nil || !p.Alive || p.TrappedUntil > 0 {
		return
	}
	ox, oy := classicOffset(*m)
	target := [2]int{}
	found := false
	for _, pair := range m.Metadata.Pairs {
		a, b := [2]int{pair[0] + ox, pair[1] + oy}, [2]int{pair[2] + ox, pair[3] + oy}
		if p.X == a[0] && p.Y == a[1] {
			target, found = b, true
			break
		}
		if p.X == b[0] && p.Y == b[1] {
			target, found = a, true
			break
		}
	}
	if !found {
		if at >= p.TeleportUntil {
			p.TeleportUntil = 0
		}
		return
	}
	// Project adaptation: 1-second cooldown plus leaving the paired pads before
	// rearming. Persisting the nonzero latch prevents reconnect/standing loops.
	if p.TeleportUntil != 0 || math.Abs(p.OffsetX) > .15 || math.Abs(p.OffsetY) > .15 {
		return
	}
	if !nativeInBounds(target[0], target[1]) || len(s.Cells) != Cols*Rows || s.Cells[target[1]*Cols+target[0]] != CellEmpty {
		return
	}
	for _, b := range s.Bombs {
		if b.X == target[0] && b.Y == target[1] {
			return
		}
	}
	for i := range s.Players {
		other := &s.Players[i]
		if other != p && other.Alive && other.X == target[0] && other.Y == target[1] {
			return
		}
	}
	p.X, p.Y = target[0], target[1]
	p.OffsetX, p.OffsetY = 0, 0
	p.TeleportUntil = at + 1000
	s.emit("teleport", p.Seat, p.X, p.Y, "", 0, at)
}
