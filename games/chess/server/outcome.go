package chess

import (
	engine "github.com/corentings/chess/v2"
	"strings"
)

func has(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

// FIDE 9.2: an EP target distinguishes positions only when that capture is
// legal. An adjacent pinned pawn alone is insufficient (the upstream position
// hash intentionally uses pseudo-legal adjacency for speed).
func positionKey(p *engine.Position) string {
	parts := strings.Fields(p.String())
	parts[3] = "-"
	if p.EnPassantSquare() != engine.NoSquare {
		for _, m := range p.ValidMoves() {
			if m.HasTag(engine.EnPassant) {
				parts[3] = p.EnPassantSquare().String()
				break
			}
		}
	}
	return strings.Join(parts[:4], " ")
}
func repetitions(s *State, p *engine.Position) int {
	key := positionKey(p)
	n := 0
	for _, k := range s.Keys {
		if key == k {
			n++
		}
	}
	return n
}
func claims(s *State, p *engine.Position, future bool) []string {
	r := []string{}
	n := repetitions(s, p)
	if future {
		n++
	}
	if n >= 3 {
		r = append(r, "threefold")
	}
	if p.HalfMoveClock() >= 100 {
		r = append(r, "fifty_move")
	}
	return r
}
func checkedSquare(p *engine.Position) int {
	king := engine.NoSquare
	pieces := p.Board().SquareMap()
	for sq, pc := range pieces {
		if pc.Color() == p.Turn() && pc.Type() == engine.King {
			king = sq
			break
		}
	}
	for sq, pc := range pieces {
		if pc.Color() == p.Turn() {
			continue
		}
		for _, target := range p.Board().AttacksFrom(sq) {
			if target == king {
				return int(king)
			}
		}
	}
	return -1
}

// Mating possibility is not forced-mate ability: two knights may mate a bare
// king with cooperation. A bare king cannot mate even when its opponent owns
// many pieces. Extra opposing pieces can assist some minor-piece mates.
func canMate(b *engine.Board, color engine.Color) bool {
	own, knights, bishops := 0, 0, 0
	boardPawns, boardKnights := 0, 0
	bishopColors := [2]bool{}
	opponentHelpers := false
	for sq, p := range b.SquareMap() {
		if p.Type() == engine.Pawn {
			boardPawns++
		}
		if p.Type() == engine.Knight {
			boardKnights++
		}
		if p.Type() == engine.Bishop {
			bishopColors[(int(sq.File())+int(sq.Rank()))%2] = true
		}
		if p.Color() != color {
			if p.Type() != engine.King && p.Type() != engine.Queen {
				opponentHelpers = true
			}
			continue
		}
		if p.Type() == engine.King {
			continue
		}
		own++
		switch p.Type() {
		case engine.Pawn, engine.Rook, engine.Queen:
			return true
		case engine.Knight:
			knights++
		case engine.Bishop:
			bishops++
		}
	}
	if own == 0 {
		return false
	}
	if knights == 1 && own == 1 && !opponentHelpers {
		return false
	}
	if bishops == own && boardPawns == 0 && boardKnights == 0 && !(bishopColors[0] && bishopColors[1]) {
		return false
	}
	return true
}

func settle(s *State, p *engine.Position) {
	switch p.Status() {
	case engine.Checkmate:
		s.end(1-s.Turn, "checkmate")
		s.note("将杀，本局结束")
		return
	case engine.Stalemate:
		s.end(-1, "stalemate")
		s.note("无合法走法且未被将军，逼和")
		return
	}
	if (!canMate(p.Board(), engine.White) && !canMate(p.Board(), engine.Black)) || frozenDead(p) {
		s.end(-1, "dead_position")
		s.note("双方均不可能将杀，死局和棋")
		return
	}
	if repetitions(s, p) >= 5 {
		s.end(-1, "fivefold")
		s.note("同一局面出现五次，自动和棋")
		return
	}
	if p.HalfMoveClock() >= 150 {
		s.end(-1, "seventy_five_move")
		s.note("双方连续 75 回合没有动兵或吃子，自动和棋")
	}
}

// Sound frozen-pawn certificate, not a heuristic score. Pawns must be unable
// to move/capture even if all non-pawn blockers cooperate. Over-approximate
// every remaining piece's reach while keeping those pawns fixed. Reject the
// proof if a pawn can be released or any non-king piece could check a king.
// This covers the locked opposite-colour bishops example in the official FIDE
// Arbiter Manual, without incorrectly drawing a fortress that can be opened.
func frozenDead(p *engine.Position) bool {
	if checkedSquare(p) >= 0 {
		return false
	}
	pieces := p.Board().SquareMap()
	pawns := map[int]engine.Piece{}
	pawnAttacks := [3][64]bool{}
	for sq, pc := range pieces {
		if pc.Type() == engine.Pawn {
			pawns[int(sq)] = pc
			for _, a := range p.Board().AttacksFrom(sq) {
				pawnAttacks[pc.Color()][a] = true
			}
		}
	}
	if len(pawns) == 0 {
		return false
	}
	inside := func(x, y int) bool { return x >= 0 && x < 8 && y >= 0 && y < 8 }
	for sq, pc := range pawns {
		d := 8
		if pc.Color() == engine.Black {
			d = -8
		}
		if pawns[sq+d] == engine.NoPiece {
			return false
		}
		for _, a := range p.Board().AttacksFrom(engine.Square(sq)) {
			if other := pawns[int(a)]; other != engine.NoPiece && other.Color() != pc.Color() {
				return false
			}
		}
	}
	type reach struct {
		piece   engine.Piece
		squares [64]bool
		attacks [64]bool
	}
	all := []reach{}
	reachable := [3][64]bool{}
	kingReach := [3][64]bool{}
	for start, pc := range pieces {
		if pc.Type() == engine.Pawn {
			continue
		}
		r := reach{piece: pc}
		r.squares[start] = true
		queue := []int{int(start)}
		for len(queue) > 0 {
			from := queue[0]
			queue = queue[1:]
			x, y := from%8, from/8
			dirs := [][2]int{}
			slide := false
			switch pc.Type() {
			case engine.Knight:
				dirs = [][2]int{{1, 2}, {2, 1}, {-1, 2}, {-2, 1}, {1, -2}, {2, -1}, {-1, -2}, {-2, -1}}
			case engine.King:
				dirs = [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
			case engine.Bishop:
				dirs = [][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
				slide = true
			case engine.Rook:
				dirs = [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
				slide = true
			case engine.Queen:
				dirs = [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
				slide = true
			}
			for _, d := range dirs {
				for step := 1; step <= 7; step++ {
					xx, yy := x+d[0]*step, y+d[1]*step
					if !inside(xx, yy) {
						break
					}
					to := yy*8 + xx
					if pc.Type() == engine.King && pawnAttacks[pc.Color().Other()][to] {
						break
					}
					r.attacks[to] = true
					if pawn := pawns[to]; pawn != engine.NoPiece {
						if pawn.Color() != pc.Color() {
							return false
						}
						break
					}
					if !r.squares[to] {
						r.squares[to] = true
						queue = append(queue, to)
					}
					if !slide {
						break
					}
				}
			}
		}
		for sq, yes := range r.squares {
			if yes {
				reachable[pc.Color()][sq] = true
				if pc.Type() == engine.King {
					kingReach[pc.Color()][sq] = true
				}
			}
		}
		all = append(all, r)
	}
	for _, color := range []engine.Color{engine.White, engine.Black} {
		for sq, yes := range pawnAttacks[color] {
			if yes && reachable[color.Other()][sq] {
				return false
			}
		}
	}
	for _, r := range all {
		if r.piece.Type() == engine.King {
			continue
		}
		for sq, yes := range r.attacks {
			if yes && kingReach[r.piece.Color().Other()][sq] {
				return false
			}
		}
	}
	return true
}
