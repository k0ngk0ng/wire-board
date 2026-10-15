package xiangqi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"sort"
	"time"
)

const mateValue = 1000000

var pieceValues = [8]int{0, 0, 180, 180, 450, 1000, 500, 100}

func evaluate(board [90]int, color int) int {
	total := 0
	for sq, p := range board {
		if p == 0 {
			continue
		}
		v := pieceValues[abs(p)]
		x, y := sq%9, sq/9
		advance := y
		if p > 0 {
			advance = 9 - y
		}
		central := 4 - abs(x-4)
		switch abs(p) {
		case Pawn:
			v += advance * 10
			if crossed(sq, sign(p)) {
				v += 60 + central*5
			}
		case Horse:
			v += central * 10
		case Rook, Cannon:
			v += central*3 + advance*2
		case King:
			v -= advance * 5
		}
		total += v * sign(p) * color
	}
	return total
}
func ordered(board [90]int, moves []Move) []Move {
	out := append([]Move(nil), moves...)
	sort.SliceStable(out, func(i, j int) bool {
		score := func(m Move) int {
			v := 0
			if board[m.To] != 0 {
				v = 16*pieceValues[abs(board[m.To])] - pieceValues[abs(board[m.From])]
			}
			return v
		}
		return score(out[i]) > score(out[j])
	})
	return out
}
func proposal(s *State, m Move) json.RawMessage {
	v, _ := json.Marshal(Action{Type: "move", Round: s.Round, Ply: s.Ply, From: &m.From, To: &m.To})
	return v
}
func botInput(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	if s.Phase != "playing" || c.Actor != s.Turn {
		return nil, nil
	}
	return json.Marshal(s)
}
func safeRoot(s *State, moves []Move, chosen Move) Move {
	// A searched repetition value is only an estimate. Never knowingly submit a
	// losing long-check/chase move if another legal continuation is available.
	raw, _ := s.envelope()
	c := gameapi.Context{Players: 2, Actor: s.Turn, Now: time.Now()}
	loses := func(m Move) bool {
		v, e := apply(c, raw, proposal(s, m))
		return e != nil || (v.Result != nil && !v.Result.Draw && len(v.Result.Winners) > 0 && v.Result.Winners[0] != s.Turn)
	}
	if loses(chosen) {
		for _, m := range moves {
			if !loses(m) {
				return m
			}
		}
	}
	return chosen
}
func fast(s *State) (Move, bool, error) {
	moves := ordered(s.Board, legal(s.Board, s.color(s.Turn)))
	if len(moves) == 0 {
		return Move{}, false, errors.New("没有合法着法")
	}
	best, bestValue := moves[0], -2*mateValue
	counts := map[string]int{}
	for _, r := range s.History {
		counts[r.Key]++
	}
	for _, m := range moves {
		next := moved(s.Board, m)
		if len(legal(next, -s.color(s.Turn))) == 0 {
			return m, true, nil
		}
		value := evaluate(next, s.color(s.Turn)) - counts[PositionKey(next, 1-s.Turn)]*90
		if value > bestValue {
			best, bestValue = m, value
		}
	}
	return safeRoot(s, moves, best), false, nil
}
func bot(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	if s.Phase != "playing" || c.Actor != s.Turn {
		return nil, nil
	}
	mv, win, e := fast(s)
	if e != nil {
		return nil, e
	}
	if !win && s.Offer >= 0 && s.Offer != s.Turn && evaluate(s.Board, s.color(s.Turn)) <= 0 {
		yes := true
		return json.Marshal(Action{Type: "respond_draw", Round: s.Round, Ply: s.Ply, Accept: &yes})
	}
	return proposal(s, mv), nil
}

type search struct {
	ctx               context.Context
	nodes, limit, red int
	stopped           bool
	counts            map[string]int
}

func (s *search) visit(board [90]int, seat, noCapture, depth, alpha, beta, ply int) int {
	s.nodes++
	if s.nodes >= s.limit || s.ctx.Err() != nil {
		s.stopped = true
		return 0
	}
	color := -1
	if seat == s.red {
		color = 1
	}
	moves := legal(board, color)
	if len(moves) == 0 {
		return -mateValue + ply
	}
	key := PositionKey(board, seat)
	if noCapture >= 100 || s.counts[key] >= 3 {
		return 0
	}
	if depth <= 0 {
		return evaluate(board, color)
	}
	best := -2 * mateValue
	for _, m := range ordered(board, moves) {
		next := moved(board, m)
		half := noCapture + 1
		if board[m.To] != 0 {
			half = 0
		}
		k := PositionKey(next, 1-seat)
		s.counts[k]++
		v := -s.visit(next, 1-seat, half, depth-1, -beta, -alpha, ply+1)
		s.counts[k]--
		if s.stopped {
			return 0
		}
		if v > best {
			best = v
		}
		if v > alpha {
			alpha = v
		}
		if alpha >= beta {
			break
		}
	}
	return best
}
func think(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	s, e := decode(gameapi.Match{Schema: 1, Data: raw})
	if e != nil {
		return nil, e
	}
	if s.Phase != "playing" {
		return nil, nil
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	best, win, e := fast(s)
	if e != nil {
		return nil, e
	}
	if win {
		return proposal(s, best), nil
	}
	budget, limit, depthLimit := 160*time.Millisecond, 8000, 4
	if s.Options.Difficulty == "easy" {
		budget, limit, depthLimit = 30*time.Millisecond, 800, 2
	}
	work, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	engine := search{ctx: work, limit: limit, red: s.Red, counts: map[string]int{}}
	for _, r := range s.History {
		engine.counts[r.Key]++
	}
	moves := ordered(s.Board, legal(s.Board, s.color(s.Turn)))
	draw := s.Offer >= 0 && s.Offer != s.Turn
	chooseDraw := false
	for depth := 1; depth <= depthLimit; depth++ {
		chosen, value, wantsDraw := best, -2*mateValue, false
		if draw {
			value, wantsDraw = 0, true
		}
		for _, m := range moves {
			next := moved(s.Board, m)
			half := s.NoCapture + 1
			if s.Board[m.To] != 0 {
				half = 0
			}
			key := PositionKey(next, 1-s.Turn)
			engine.counts[key]++
			v := -engine.visit(next, 1-s.Turn, half, depth-1, -2*mateValue, -value, 1)
			engine.counts[key]--
			if engine.stopped {
				break
			}
			v -= engine.counts[key] * 8
			if v > value {
				chosen, value, wantsDraw = m, v, false
			}
		}
		if engine.stopped {
			break
		}
		best, chooseDraw = chosen, wantsDraw
		if value > mateValue-100 {
			break
		}
		sort.SliceStable(moves, func(i, j int) bool { return sameMove(moves[i], best) && !sameMove(moves[j], best) })
	}
	if chooseDraw {
		yes := true
		return json.Marshal(Action{Type: "respond_draw", Round: s.Round, Ply: s.Ply, Accept: &yes})
	}
	return proposal(s, safeRoot(s, moves, best)), nil
}
