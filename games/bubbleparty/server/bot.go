package bubbleparty

import (
	"encoding/json"
	"math"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type botStep struct {
	x, y int
	dir  string
}

func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) || s.Phase != PhasePlaying || s.Result != nil {
		return nil, nil
	}
	p := &s.Players[ctx.Actor]
	if !p.Alive || p.Seq >= MaxSeq {
		return nil, nil
	}
	move, bomb := botDecision(s, p, ctx.Now.UnixMilli())
	a := Action{Type: "input", Epoch: s.Epoch, Seq: p.Seq + 1, Move: move, Bomb: bomb}
	return json.Marshal(a)
}

func botDecision(s *State, p *Player, now int64) (string, bool) {
	if p.TrappedUntil > 0 {
		return "none", false
	}
	// Consider every placed balloon so escaping starts before the final 900 ms.
	danger := bombDanger(s, now, 0)
	if danger[p.Y*Cols+p.X] {
		if move := escapeMove(s, p, danger); move != "" {
			return move, false
		}
	}
	if move := trappedTargetMove(s, p, danger, true); move != "" {
		return move, false
	}
	if move := trappedTargetMove(s, p, danger, false); move != "" {
		return move, false
	}
	enemyAdjacent := false
	crateAdjacent := false
	for _, d := range directions {
		x, y := p.X+d[0], p.Y+d[1]
		c := s.cell(x, y)
		if c == CellCrate {
			crateAdjacent = true
		}
		for i := range s.Players {
			q := &s.Players[i]
			if q.Alive && q.Team != p.Team && q.X == x && q.Y == y {
				enemyAdjacent = true
			}
		}
	}
	if (crateAdjacent || enemyAdjacent) && s.cell(p.X, p.Y) != CellBomb && bombCount(s, p.Seat) < p.MaxBombs && now >= p.NextBombAt && canEscapePlacedBomb(s, p, now) {
		return "none", true
	}
	if move := pathMove(s, p, danger); move != "" {
		return move, false
	}
	if now >= p.NextBombAt && bombCount(s, p.Seat) < p.MaxBombs && hash64(s.Seed+uint64(now)+uint64(p.Seat), "botbomb")%100 < 18 && canEscapePlacedBomb(s, p, now) {
		return "none", true
	}
	order := [][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}}
	start := int(hash64(s.Seed+uint64(now)+uint64(p.Seat), "botmove") % 4)
	for k := 0; k < 4; k++ {
		d := order[(start+k)%4]
		x, y := p.X+d[0], p.Y+d[1]
		if s.canWalk(p, x, y) && !danger[y*Cols+x] && insideSafe(s, x, y) {
			return directionName(d), false
		}
	}
	return "none", false
}

func directionName(d [2]int) string {
	switch {
	case d[0] == 0 && d[1] == -1:
		return "up"
	case d[0] == 0 && d[1] == 1:
		return "down"
	case d[0] == -1 && d[1] == 0:
		return "left"
	default:
		return "right"
	}
}

func bombCount(s *State, seat int) int {
	n := 0
	for i := range s.Bombs {
		if s.Bombs[i].Seat == seat {
			n++
		}
	}
	return n
}

func insideSafe(s *State, x, y int) bool {
	if !s.Sudden {
		return true
	}
	return x >= s.SafeInset && y >= s.SafeInset && x < Cols-s.SafeInset && y < Rows-s.SafeInset
}

func bombDanger(s *State, now int64, horizon int64) map[int]bool {
	d := map[int]bool{}
	for _, b := range s.Bombs {
		if b.ExplodeAt-now > horizon && horizon > 0 {
			continue
		}
		d[b.Y*Cols+b.X] = true
		for _, dir := range directions {
			for step := 1; step <= b.Range; step++ {
				x, y := b.X+dir[0]*step, b.Y+dir[1]*step
				c := s.cell(x, y)
				if c == CellWall {
					break
				}
				d[y*Cols+x] = true
				if c == CellCrate || c == CellBomb {
					break
				}
			}
		}
	}
	for _, b := range s.Blasts {
		if b.EndAt > now {
			d[b.Y*Cols+b.X] = true
		}
	}
	return d
}

// Search may cross future water lanes, but never an already active water jet.
// The full path must fit before the earliest fuse (including chain reactions).
func escapeMove(s *State, p *Player, danger map[int]bool) string {
	start := p.Y*Cols + p.X
	if !danger[start] {
		return ""
	}
	deadline := s.ServerAt + BombFuseMS
	for _, b := range s.Bombs {
		if b.ExplodeAt < deadline {
			deadline = b.ExplodeAt
		}
	}
	active := map[int]bool{}
	for _, b := range s.Blasts {
		if b.EndAt > s.ServerAt {
			active[b.Y*Cols+b.X] = true
		}
	}
	type node struct {
		x, y, steps int
		dir         string
	}
	seen := make([]bool, Cols*Rows)
	seen[start] = true
	queue := []node{{x: p.X, y: p.Y}}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, d := range directions {
			x, y := n.x+d[0], n.y+d[1]
			if x < 0 || x >= Cols || y < 0 || y >= Rows || !insideSafe(s, x, y) {
				continue
			}
			idx := y*Cols + x
			if seen[idx] || !s.canWalk(p, x, y) || active[idx] {
				continue
			}
			steps := n.steps + 1
			if s.ServerAt+int64(steps*p.SpeedMS)+FrameMS >= deadline {
				continue
			}
			dir := n.dir
			if dir == "" {
				dir = directionName(d)
			}
			if !danger[idx] {
				return dir
			}
			seen[idx] = true
			queue = append(queue, node{x, y, steps, dir})
		}
	}
	return ""
}

func canEscapePlacedBomb(s *State, p *Player, now int64) bool {
	next := *s
	next.Cells = append([]byte(nil), s.Cells...)
	next.Bombs = append([]Bomb(nil), s.Bombs...)
	next.Bombs = append(next.Bombs, Bomb{Seat: p.Seat, Team: p.Team, X: p.X, Y: p.Y, Range: p.Range, At: now, ExplodeAt: now + BombFuseMS})
	next.setCell(p.X, p.Y, CellBomb)
	return escapeMove(&next, p, bombDanger(&next, now, 0)) != ""
}

// Teammate rescue takes precedence over finishing a trapped opponent.
func trappedTargetMove(s *State, p *Player, danger map[int]bool, ally bool) string {
	targets := map[int]int64{}
	for _, q := range s.Players {
		if q.Alive && q.TrappedUntil > s.ServerAt && (q.Team == p.Team) == ally {
			if p.X == q.X && p.Y == q.Y && !danger[p.Y*Cols+p.X] {
				dx, dy := q.OffsetX-p.OffsetX, q.OffsetY-p.OffsetY
				if math.Abs(dx) >= math.Abs(dy) {
					if dx >= 0 {
						return "right"
					}
					return "left"
				}
				if dy >= 0 {
					return "down"
				}
				return "up"
			}
			targets[q.Y*Cols+q.X] = q.TrappedUntil
		}
	}
	if len(targets) == 0 {
		return ""
	}
	type node struct {
		x, y, steps int
		dir         string
	}
	queue := []node{{x: p.X, y: p.Y}}
	seen := map[int]bool{p.Y*Cols + p.X: true}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, d := range directions {
			x, y := n.x+d[0], n.y+d[1]
			if x < 0 || x >= Cols || y < 0 || y >= Rows || !insideSafe(s, x, y) {
				continue
			}
			idx := y*Cols + x
			if seen[idx] || danger[idx] || !s.canWalk(p, x, y) {
				continue
			}
			seen[idx] = true
			dir := n.dir
			if dir == "" {
				dir = directionName(d)
			}
			steps := n.steps + 1
			if until, ok := targets[idx]; ok && s.ServerAt+int64(steps*p.SpeedMS) < until {
				return dir
			}
			queue = append(queue, node{x, y, steps, dir})
		}
	}
	return ""
}

func pathMove(s *State, p *Player, danger map[int]bool) string {
	seen := make([]bool, Cols*Rows)
	type node struct {
		x, y int
		dir  string
	}
	start := node{p.X, p.Y, ""}
	seen[p.Y*Cols+p.X] = true
	queue := []node{start}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if n.dir != "" {
			for _, d := range directions {
				x, y := n.x+d[0], n.y+d[1]
				if s.cell(x, y) == CellCrate {
					return n.dir
				}
				for i := range s.Players {
					q := &s.Players[i]
					if q.Alive && q.Team != p.Team && q.X == x && q.Y == y {
						return n.dir
					}
				}
			}
		}
		order := [][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}}
		for _, d := range order {
			x, y := n.x+d[0], n.y+d[1]
			if x < 0 || x >= Cols || y < 0 || y >= Rows || !insideSafe(s, x, y) {
				continue
			}
			idx := y*Cols + x
			if seen[idx] || danger[idx] || s.cell(x, y) == CellWall || s.cell(x, y) == CellCrate || s.cell(x, y) == CellBomb {
				continue
			}
			dir := n.dir
			if dir == "" {
				dir = directionName(d)
			}
			seen[idx] = true
			queue = append(queue, node{x, y, dir})
		}
	}
	return ""
}
