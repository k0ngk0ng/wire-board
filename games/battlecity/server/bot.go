package battlecity

import (
	"encoding/json"
	"hash/fnv"
	"math"

	"github.com/k0ngk0ng/wire-board/games/battlecity/maps"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// botAction uses only the public match state. It never writes positions,
// damage, scores or results; it submits the same bounded input action as a
// human seat.
func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Tanks) || s.Result != nil {
		return nil, nil
	}
	t := &s.Tanks[ctx.Actor]
	if s.Phase == PhaseSelect {
		if t.Selected {
			return nil, nil
		}
		h := fnv.New64a()
		_, _ = h.Write([]byte(s.Epoch))
		_, _ = h.Write([]byte{byte(ctx.Actor)})
		class := tankOrder[int(h.Sum64()%uint64(len(tankOrder)))]
		return json.Marshal(Action{Type: "select", Epoch: s.Epoch, Tank: class})
	}
	if s.Phase != PhasePlaying || !t.Alive {
		if s.Phase == PhasePlaying && t.Selected && t.Seq < MaxSeq {
			return json.Marshal(Action{Type: "input", Epoch: s.Epoch, Seq: t.Seq + 1, Move: "none", Fire: false})
		}
		return nil, nil
	}
	if t.Seq < 0 || t.Seq >= MaxSeq {
		return nil, nil
	}
	move, fire := chooseBotInput(&s, t)
	return json.Marshal(Action{Type: "input", Epoch: s.Epoch, Seq: t.Seq + 1, Move: move, Fire: fire})
}

func chooseBotInput(s *State, t *Tank) (string, bool) {
	cx, cy := tankCell(t)
	target, isTank := chooseBotGoal(s, t, cx, cy)
	path := astar(s, t, cx, cy, target.X, target.Y)
	dir := t.Dir
	if len(path) > 1 {
		n := path[1]
		if n.X > cx {
			dir = DirRight
		} else if n.X < cx {
			dir = DirLeft
		} else if n.Y > cy {
			dir = DirDown
		} else if n.Y < cy {
			dir = DirUp
		}
	} else {
		dir = toward(cx, cy, target.X, target.Y, t.Dir)
	}
	move := dirName(dir)
	fire := false
	if isTank {
		enemy := &s.Tanks[target.Seat]
		if aligned(cx, cy, enemy) && clearLine(s, t, dir, cx, cy) {
			fire = true
		}
	}
	if len(path) > 1 {
		next := path[1]
		if next.X >= 0 && next.X < Cols && next.Y >= 0 && next.Y < Rows && s.cells[next.Y][next.X] == maps.Brick {
			fire = true
		}
		// Two bots facing each other in a corridor can cancel every bullet.
		// Flank when the next path cell is occupied; this also breaks mirrored
		// bot openings without changing the authoritative movement rules.
		if occupiedByOther(s, t, next.X, next.Y) {
			if side, ok := sideStep(s, t, dir, target.X, target.Y); ok {
				dir = side
				move = dirName(side)
			}
		}
	}
	if brickInFront(s, t, dir) {
		fire = true
	}
	if !isTank {
		bx, by := maps.BaseCell(1 - t.Team)
		if (cx == bx || cy == by) && absInt(cx-bx)+absInt(cy-by) <= 5 {
			fire = true
		}
	}
	if !fire && len(s.Bullets) < MaxBullets/2 && s.Tick%5 == 0 {
		// Keep some pressure without spending every frame on a shot.
		fire = true
	}
	return move, fire
}

type botGoal struct {
	X, Y, Seat int
}

func tankCell(t *Tank) (int, int) {
	x := int((t.X + Cell/2) / Cell)
	y := int((t.Y + Cell/2) / Cell)
	if x < 0 {
		x = 0
	}
	if x >= Cols {
		x = Cols - 1
	}
	if y < 0 {
		y = 0
	}
	if y >= Rows {
		y = Rows - 1
	}
	return x, y
}

func chooseBotGoal(s *State, t *Tank, cx, cy int) (botGoal, bool) {
	best := botGoal{X: 6, Y: 12 - 12*t.Team}
	if t.Team == TeamTop {
		best.Y = 12
	} else {
		best.Y = 0
	}
	best.Seat = -1
	bestDist := 1 << 30
	for i := range s.Tanks {
		o := &s.Tanks[i]
		if o.Team == t.Team || !o.Alive {
			continue
		}
		ox, oy := tankCell(o)
		d := absInt(ox-cx) + absInt(oy-cy)
		if d < bestDist {
			bestDist = d
			best = botGoal{X: ox, Y: oy, Seat: i}
		}
	}
	// Early in the match bots contest nearby tanks; after 30 seconds they
	// commit to the enemy base so a duel in a corridor cannot run to the cap.
	// In 1v1 the top seat is the designated base rusher from the start; this
	// breaks the otherwise perfectly mirrored opening duel.
	rusher := (s.TeamSize == 1 && t.Seat == 0) || s.Tick >= 300
	if !rusher && best.Seat >= 0 && bestDist <= 3 {
		return best, true
	}
	bx, by := maps.BaseCell(1 - t.Team)
	return botGoal{X: bx, Y: by, Seat: -1}, false
}

func dirName(dir int) string {
	switch dir {
	case DirUp:
		return "up"
	case DirRight:
		return "right"
	case DirDown:
		return "down"
	default:
		return "left"
	}
}

func toward(cx, cy, tx, ty, fallback int) int {
	dx, dy := tx-cx, ty-cy
	if absInt(dx) >= absInt(dy) && dx != 0 {
		if dx > 0 {
			return DirRight
		}
		return DirLeft
	}
	if dy != 0 {
		if dy > 0 {
			return DirDown
		}
		return DirUp
	}
	return fallback
}

func aligned(cx, cy int, enemy *Tank) bool {
	ex, ey := tankCell(enemy)
	return cx == ex || cy == ey
}

func brickInFront(s *State, t *Tank, dir int) bool {
	cx, cy := tankCell(t)
	nx, ny := cx+int(dirDX[dir]), cy+int(dirDY[dir])
	if nx < 0 || nx >= Cols || ny < 0 || ny >= Rows {
		return false
	}
	return s.cells[ny][nx] == maps.Brick || s.cells[ny][nx] == maps.Steel
}

func occupiedByOther(s *State, t *Tank, x, y int) bool {
	for i := range s.Tanks {
		o := &s.Tanks[i]
		if i == t.Seat || !o.Alive {
			continue
		}
		ox, oy := tankCell(o)
		if ox == x && oy == y {
			return true
		}
	}
	return false
}

func sideStep(s *State, t *Tank, dir, tx, ty int) (int, bool) {
	cx, cy := tankCell(t)
	dirs := []int{DirLeft, DirRight}
	if dir == DirLeft || dir == DirRight {
		dirs = []int{DirUp, DirDown}
	}
	if (s.Tick+int64(t.Seat))%2 == 0 {
		dirs[0], dirs[1] = dirs[1], dirs[0]
	}
	bestDir, bestDist := dir, absInt(cx-tx)+absInt(cy-ty)
	found := false
	for _, d := range dirs {
		x, y := cx+int(dirDX[d]), cy+int(dirDY[d])
		if x < 0 || x >= Cols || y < 0 || y >= Rows {
			continue
		}
		if s.cells[y][x] == maps.Steel || s.cells[y][x] == maps.Water || s.cells[y][x] == maps.Brick {
			continue
		}
		if occupiedByOther(s, t, x, y) {
			continue
		}
		if !s.tankFree(t.Seat, float64(x*Cell), float64(y*Cell), s.ServerAt) {
			continue
		}
		dst := absInt(x-tx) + absInt(y-ty)
		if !found || dst < bestDist {
			found, bestDir, bestDist = true, d, dst
		}
	}
	return bestDir, found
}

func clearLine(s *State, t *Tank, dir, cx, cy int) bool {
	for step := 1; step <= 12; step++ {
		x, y := cx+int(dirDX[dir])*step, cy+int(dirDY[dir])*step
		if x < 0 || x >= Cols || y < 0 || y >= Rows {
			return false
		}
		if s.cells[y][x] == maps.Steel || s.cells[y][x] == maps.Water {
			return false
		}
	}
	return true
}

// astar returns a 4-neighbour path including the start cell. Bricks are
// passable with a cost because bots can shoot them; steel/water/base walls are
// not. Occupied cells are discouraged but not forbidden, so the bot can keep
// moving when a teammate temporarily blocks a corridor.
func astar(s *State, t *Tank, sx, sy, tx, ty int) []maps.Point {
	if sx == tx && sy == ty {
		return []maps.Point{{X: sx, Y: sy}}
	}
	type node struct {
		x, y, g, f int
	}
	open := []node{{x: sx, y: sy, g: 0, f: absInt(tx-sx) + absInt(ty-sy)}}
	best := map[[2]int]int{{sx, sy}: 0}
	parent := map[[2]int][2]int{}
	found := false
	for len(open) > 0 {
		idx := 0
		for i := 1; i < len(open); i++ {
			if open[i].f < open[idx].f || (open[i].f == open[idx].f && open[i].g < open[idx].g) {
				idx = i
			}
		}
		cur := open[idx]
		open = append(open[:idx], open[idx+1:]...)
		if cur.x == tx && cur.y == ty {
			found = true
			break
		}
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			x, y := cur.x+d[0], cur.y+d[1]
			if x < 0 || x >= Cols || y < 0 || y >= Rows {
				continue
			}
			if s.cells[y][x] == maps.Steel || s.cells[y][x] == maps.Water {
				continue
			}
			// The bot may deliberately drive into the enemy base only as a
			// target; tanks cannot actually enter it, but the path is used to
			// approach the ring.
			cost := 1
			if s.cells[y][x] == maps.Brick {
				cost = 3
			}
			for i := range s.Tanks {
				o := &s.Tanks[i]
				if o.Alive && o.Seat != t.Seat {
					ox, oy := tankCell(o)
					if ox == x && oy == y {
						cost += 50
					}
				}
			}
			ng := cur.g + cost
			key := [2]int{x, y}
			if old, ok := best[key]; ok && old <= ng {
				continue
			}
			best[key] = ng
			parent[key] = [2]int{cur.x, cur.y}
			open = append(open, node{x: x, y: y, g: ng, f: ng + absInt(tx-x) + absInt(ty-y)})
		}
	}
	if !found {
		return []maps.Point{{X: sx, Y: sy}}
	}
	path := []maps.Point{}
	cur := [2]int{tx, ty}
	for {
		path = append([]maps.Point{{X: cur[0], Y: cur[1]}}, path...)
		if cur == [2]int{sx, sy} {
			break
		}
		p, ok := parent[cur]
		if !ok {
			return []maps.Point{{X: sx, Y: sy}}
		}
		cur = p
	}
	return path
}

var _ = math.Abs
