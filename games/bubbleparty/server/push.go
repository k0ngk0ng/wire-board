package bubbleparty

import "math"

// The original client moves a 40 px crate by 4 px per 30 ms logic step.
// Collision follows the server envelope; rendering does not unlock a crate.
const PushMoveMS int64 = 300

const maxBoxMotions = 32
const pushContactEpsilon = 0.000001

// canPushBox tests a source crate at (x,y), using the current state clock. It
// includes contact geometry so bots cannot push a remote or diagonally hit box.
func (s *State) canPushBox(p *Player, x, y, dx, dy int) bool {
	return s.canPushBoxAt(p, x, y, dx, dy, s.ServerAt)
}

func (s *State) canPushBoxAt(p *Player, x, y, dx, dy int, at int64) bool {
	if p == nil || !p.Alive || p.TrappedUntil > 0 || mountPassesBoxes(p.Mount) ||
		s.RulesVersion < 3 || s.Phase != PhasePlaying || s.Result != nil {
		return false
	}
	if (dx != -1 && dx != 0 && dx != 1) || (dy != -1 && dy != 0 && dy != 1) ||
		(dx == 0) == (dy == 0) {
		return false
	}
	toX, toY := x+dx, y+dy
	if s.cell(x, y) != CellPushCrate || s.cell(toX, toY) != CellEmpty {
		return false
	}
	// Preserve passable native objects as well: replacing a bush or gate would
	// silently delete its separate terrain semantics.
	if len(s.NativeObjects) == Cols*Rows && s.NativeObjects[toY*Cols+toX] >= 0 {
		return false
	}
	if !s.nativePassage(x, y, dx, dy, false) {
		return false
	}
	px, py := float64(p.X)+p.OffsetX, float64(p.Y)+p.OffsetY
	if math.IsNaN(px) || math.IsNaN(py) || math.IsInf(px, 0) || math.IsInf(py, 0) {
		return false
	}
	// The mover must touch the near face with its center on the same grid line.
	// Reject corners, an approach that has not reached the face, and overlaps.
	along := (float64(x)-px)*float64(dx) + (float64(y)-py)*float64(dy)
	across := (float64(x)-px)*float64(dy) - (float64(y)-py)*float64(dx)
	if math.Abs(along-(.5+PlayerRadius)) > pushContactEpsilon || math.Abs(across) > pushContactEpsilon {
		return false
	}
	active := 0
	for _, m := range s.BoxMotions {
		if m.EndAt <= at {
			continue
		}
		active++
		if (m.X == x && m.Y == y) || (m.FromX == x && m.FromY == y) ||
			(m.X == toX && m.Y == toY) || (m.FromX == toX && m.FromY == toY) {
			return false
		}
	}
	if active >= maxBoxMotions || s.bombAt(toX, toY) != nil {
		return false
	}
	for _, it := range s.Items {
		if it.X == toX && it.Y == toY {
			return false
		}
	}
	// A player can straddle the destination from a neighboring integer cell.
	// Measure the actual circular body against the destination crate rectangle.
	for _, q := range s.Players {
		if !q.Alive {
			continue
		}
		gapX := math.Max(math.Abs(float64(q.X)+q.OffsetX-float64(toX))-.5, 0)
		gapY := math.Max(math.Abs(float64(q.Y)+q.OffsetY-float64(toY))-.5, 0)
		if gapX*gapX+gapY*gapY <= PlayerRadius*PlayerRadius+pushContactEpsilon {
			return false
		}
	}
	return true
}

// tryPushBox changes only authoritative box state. Call it with the mover's
// current continuous position after clipping the intended movement to the box
// face; automatic perpendicular grid alignment must not call this helper.
func (s *State) tryPushBox(p *Player, dx, dy int, at int64) bool {
	if p == nil {
		return false
	}
	px, py := float64(p.X)+p.OffsetX, float64(p.Y)+p.OffsetY
	if math.IsNaN(px) || math.IsNaN(py) || math.IsInf(px, 0) || math.IsInf(py, 0) {
		return false
	}
	x := int(math.Floor(px + float64(dx)*(.5+PlayerRadius) + .5))
	y := int(math.Floor(py + float64(dy)*(.5+PlayerRadius) + .5))
	if !s.canPushBoxAt(p, x, y, dx, dy, at) {
		return false
	}
	toX, toY := x+dx, y+dy
	objectID := -1
	if len(s.NativeObjects) == Cols*Rows {
		objectID = s.NativeObjects[y*Cols+x]
		s.NativeObjects[toY*Cols+toX] = objectID
		s.NativeObjects[y*Cols+x] = -1
	}
	s.setCell(x, y, CellEmpty)
	s.setCell(toX, toY, CellPushCrate)
	kept := s.BoxMotions[:0]
	for _, m := range s.BoxMotions {
		if m.EndAt > at {
			kept = append(kept, m)
		}
	}
	s.BoxMotions = append(kept, BoxMotion{FromX: x, FromY: y, X: toX, Y: toY, At: at, EndAt: at + PushMoveMS, ObjectID: objectID})
	dir := "right"
	if dx < 0 {
		dir = "left"
	} else if dy < 0 {
		dir = "up"
	} else if dy > 0 {
		dir = "down"
	}
	s.emit("push", p.Seat, toX, toY, dir, y*Cols+x, at)
	return true
}
