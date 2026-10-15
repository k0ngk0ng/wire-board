package xiangqi

import (
	"sort"
	"strings"
)

const (
	King     = 1
	Advisor  = 2
	Elephant = 3
	Horse    = 4
	Rook     = 5
	Cannon   = 6
	Pawn     = 7
)

type Move struct {
	From int `json:"from"`
	To   int `json:"to"`
}
type Threat struct {
	Attacker int  `json:"attacker"`
	Target   int  `json:"target"`
	Exchange bool `json:"exchange,omitempty"`
}
type Record struct {
	Key    string   `json:"key"`
	Seat   int      `json:"seat"`
	Check  bool     `json:"check"`
	Chases []Threat `json:"chases,omitempty"`
	Evaded []int    `json:"evaded,omitempty"`
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func sign(n int) int {
	if n < 0 {
		return -1
	}
	if n > 0 {
		return 1
	}
	return 0
}
func inside(x, y int) bool { return x >= 0 && x < 9 && y >= 0 && y < 10 }
func palace(x, y, color int) bool {
	return x >= 3 && x <= 5 && ((color == 1 && y >= 7 && y <= 9) || (color == -1 && y >= 0 && y <= 2))
}
func crossed(square, color int) bool {
	if color == 1 {
		return square/9 <= 4
	}
	return square/9 >= 5
}
func pseudo(board [90]int, from int) []int {
	pc := board[from]
	if pc == 0 {
		return nil
	}
	color := sign(pc)
	x, y := from%9, from/9
	out := []int{}
	add := func(xx, yy int) {
		if inside(xx, yy) {
			to := yy*9 + xx
			if sign(board[to]) != color {
				out = append(out, to)
			}
		}
	}
	switch abs(pc) {
	case King:
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			xx, yy := x+d[0], y+d[1]
			if palace(xx, yy, color) {
				add(xx, yy)
			}
		}
		// Facing generals attack along an otherwise empty file. Moving to capture
		// the opposing king is excluded by legal(), but this ray defines check.
		for _, dy := range []int{-1, 1} {
			for yy := y + dy; inside(x, yy); yy += dy {
				if v := board[yy*9+x]; v != 0 {
					if v == -color*King {
						out = append(out, yy*9+x)
					}
					break
				}
			}
		}
	case Advisor:
		for _, d := range [][2]int{{1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
			xx, yy := x+d[0], y+d[1]
			if palace(xx, yy, color) {
				add(xx, yy)
			}
		}
	case Elephant:
		for _, d := range [][2]int{{2, 2}, {-2, 2}, {2, -2}, {-2, -2}} {
			xx, yy := x+d[0], y+d[1]
			if inside(xx, yy) && ((color == 1 && yy >= 5) || (color == -1 && yy <= 4)) && board[(y+d[1]/2)*9+x+d[0]/2] == 0 {
				add(xx, yy)
			}
		}
	case Horse:
		for _, d := range [][2]int{{2, 1}, {2, -1}, {-2, 1}, {-2, -1}, {1, 2}, {-1, 2}, {1, -2}, {-1, -2}} {
			xx, yy := x+d[0], y+d[1]
			if !inside(xx, yy) {
				continue
			}
			lx, ly := x, y
			if abs(d[0]) == 2 {
				lx += d[0] / 2
			} else {
				ly += d[1] / 2
			}
			if board[ly*9+lx] == 0 {
				add(xx, yy)
			}
		}
	case Rook, Cannon:
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			screen := false
			for xx, yy := x+d[0], y+d[1]; inside(xx, yy); xx, yy = xx+d[0], yy+d[1] {
				to := yy*9 + xx
				target := board[to]
				if !screen {
					if target == 0 {
						out = append(out, to)
						continue
					}
					if abs(pc) == Rook {
						if sign(target) != color {
							out = append(out, to)
						}
						break
					}
					screen = true
				} else if target != 0 {
					if sign(target) != color {
						out = append(out, to)
					}
					break
				}
			}
		}
	case Pawn:
		add(x, y-color)
		if crossed(from, color) {
			add(x-1, y)
			add(x+1, y)
		}
	}
	return out
}
func checked(board [90]int, color int) bool {
	king := -1
	for i, p := range board {
		if p == color*King {
			king = i
			break
		}
	}
	if king < 0 {
		return true
	}
	for from, p := range board {
		if sign(p) == -color {
			for _, to := range pseudo(board, from) {
				if to == king {
					return true
				}
			}
		}
	}
	return false
}
func moved(board [90]int, m Move) [90]int {
	board[m.To], board[m.From] = board[m.From], 0
	return board
}
func legal(board [90]int, color int) []Move {
	out := []Move{}
	for from, p := range board {
		if sign(p) != color {
			continue
		}
		for _, to := range pseudo(board, from) {
			if abs(board[to]) == King {
				continue
			}
			m := Move{from, to}
			if !checked(moved(board, m), color) {
				out = append(out, m)
			}
		}
	}
	return out
}
func PositionKey(board [90]int, turn int) string {
	var b strings.Builder
	b.Grow(91)
	for _, p := range board {
		b.WriteByte(byte(p + 8))
	}
	b.WriteByte(byte(turn + '0'))
	return b.String()
}
func sameMove(a, b Move) bool { return a.From == b.From && a.To == b.To }
func trueRoot(board [90]int, m Move, color int) bool {
	after := moved(board, m)
	for _, reply := range legal(after, -color) {
		if reply.To == m.To {
			return true
		}
	}
	return false
}

// WXF 20.3/6/7/9: track physical target identity, not merely an attacked
// square. Pawns/generals and uncrossed enemy pawns are exempt; true roots and
// reciprocal exchanges are exempt except a horse/cannon chasing a rook.
func chaseThreats(board [90]int, ids [90]int, color int) []Threat {
	out := []Threat{}
	for _, m := range legal(board, color) {
		attacker, target := abs(board[m.From]), abs(board[m.To])
		if target == 0 || target == King || attacker == King || attacker == Pawn {
			continue
		}
		if target == Pawn && !crossed(m.To, -color) {
			continue
		}
		reciprocal := false
		if attacker == target {
			for _, back := range legal(board, -color) {
				if back.From == m.To && back.To == m.From {
					reciprocal = true
					break
				}
			}
		}
		if !((attacker == Horse || attacker == Cannon) && target == Rook) && trueRoot(board, m, color) {
			continue
		}
		out = append(out, Threat{ids[m.From], ids[m.To], reciprocal})
	}
	return out
}
func newChases(before, after [90]int, oldIDs, ids [90]int, color, movedID int) []Threat {
	old := chaseThreats(before, oldIDs, color)
	next := chaseThreats(after, ids, color)
	out := []Threat{}
	for _, t := range next {
		exists := false
		for _, o := range old {
			if t == o {
				exists = true
				break
			}
		}
		if !exists || t.Attacker == movedID {
			out = append(out, t)
		}
	}
	return out
}

// A chase also requires the opponent to escape or directly neutralize the
// preceding attack. This distinguishes actual pursuit from a target left
// under an unchanged attack while unrelated pieces shuffle elsewhere.
func evaded(previous Record, board [90]int, ids [90]int, attackerColor int) []int {
	if len(previous.Chases) == 0 {
		return nil
	}
	remaining := chaseThreats(board, ids, attackerColor)
	resolved := map[int]bool{}
	for _, threat := range previous.Chases {
		resolved[threat.Target] = true
	}
	for _, threat := range previous.Chases {
		for _, current := range remaining {
			if threat.Attacker == current.Attacker && threat.Target == current.Target {
				resolved[threat.Target] = false
			}
		}
	}
	out := []int{}
	for target, yes := range resolved {
		if yes {
			out = append(out, target)
		}
	}
	sort.Ints(out)
	return out
}

func includes(values []int, value int) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func repetition(s *State) (int, [2]int) {
	key := PositionKey(s.Board, s.Turn)
	indices := []int{}
	for i, h := range s.History {
		if h.Key == key {
			indices = append(indices, i)
		}
	}
	severity := [2]int{}
	if len(indices) < 3 {
		return len(indices), severity
	}
	begin := indices[len(indices)-3] + 1
	for seat := 0; seat < 2; seat++ {
		everyCheck, everyChase := true, true
		common := map[int]bool{}
		strictChase := map[int]bool{}
		first := true
		for i, r := range s.History[begin:] {
			if r.Seat != seat {
				continue
			}
			everyCheck = everyCheck && r.Check
			targets := map[int]bool{}
			if !r.Check {
				next := begin + i + 1
				if next == len(s.History) {
					next = begin // The loop has returned to the same position.
				}
				for _, t := range r.Chases {
					if !includes(s.History[next].Evaded, t.Target) {
						continue
					}
					targets[t.Target] = true
					if !t.Exchange {
						strictChase[t.Target] = true
					}
				}
			}
			if len(targets) == 0 {
				everyChase = false
			}
			if first {
				common = targets
				first = false
			} else {
				for id := range common {
					if !targets[id] {
						delete(common, id)
					}
				}
			}
		}
		for id := range common {
			if !strictChase[id] {
				delete(common, id)
			}
		}
		if everyCheck && !first {
			severity[seat] = 2
		} else if everyChase && len(common) > 0 {
			severity[seat] = 1
		}
	}
	return len(indices), severity
}
func settle(s *State) {
	if len(legal(s.Board, s.color(s.Turn))) == 0 {
		reason := "stalemate_loss"
		if checked(s.Board, s.color(s.Turn)) {
			reason = "checkmate"
		}
		s.end(1-s.Turn, reason)
		return
	}
	attackers := false
	for _, p := range s.Board {
		if abs(p) >= Horse {
			attackers = true
			break
		}
	}
	if !attackers {
		s.end(-1, "dead_position")
		return
	}
	count, severity := repetition(s)
	s.Warning = ""
	if count >= 3 && severity[0] != severity[1] {
		loser := 0
		if severity[1] > severity[0] {
			loser = 1
		}
		kind := "长捉"
		if severity[loser] == 2 {
			kind = "长将"
		}
		s.Warning = fmtWarning(loser, kind)
		if count >= 4 {
			s.end(1-loser, map[bool]string{true: "perpetual_check", false: "perpetual_chase"}[severity[loser] == 2])
			return
		}
	}
	if count >= 4 {
		s.end(-1, "repetition")
		return
	}
	if s.NoCapture >= 100 {
		s.end(-1, "fifty_move")
		return
	}
}
func fmtWarning(seat int, kind string) string {
	return "玩家 " + string(rune('1'+seat)) + " 须改变" + kind + "循环；再次重复将判负"
}
