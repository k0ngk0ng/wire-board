package bubbleparty

import (
	"encoding/json"

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
	danger := bombDanger(s, now, 900)
	if danger[p.Y*Cols+p.X] {
		if move := escapeMove(s, p, danger); move != "" {
			return move, false
		}
	}
	if move := escapeMove(s, p, danger); move != "" {
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
	if (crateAdjacent || enemyAdjacent) && s.cell(p.X, p.Y) != CellBomb && bombCount(s, p.Seat) < p.MaxBombs && now >= p.NextBombAt {
		return "none", true
	}
	if move := pathMove(s, p, danger); move != "" {
		return move, false
	}
	if now >= p.NextBombAt && bombCount(s, p.Seat) < p.MaxBombs && hash64(s.Seed+uint64(now)+uint64(p.Seat), "botbomb")%100 < 18 {
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

func escapeMove(s *State, p *Player, danger map[int]bool) string {
	start := p.Y*Cols + p.X
	if !danger[start] {
		return ""
	}
	seen := make([]bool, Cols*Rows)
	seen[start] = true
	type node struct {
		x, y int
		dir  string
	}
	queue := []node{{p.X, p.Y, ""}}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, d := range directions {
			x, y := n.x+d[0], n.y+d[1]
			if x < 1 || x >= Cols-1 || y < 1 || y >= Rows-1 || !insideSafe(s, x, y) {
				continue
			}
			idx := y*Cols + x
			if seen[idx] || s.cell(x, y) == CellWall || s.cell(x, y) == CellCrate {
				continue
			}
			if s.cell(x, y) == CellBomb && !(x == p.X && y == p.Y) {
				continue
			}
			if danger[idx] {
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
			queue = append(queue, node{x, y, dir})
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
			if x < 1 || x >= Cols-1 || y < 1 || y >= Rows-1 || !insideSafe(s, x, y) {
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
