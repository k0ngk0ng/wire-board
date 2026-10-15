package chess

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	engine "github.com/corentings/chess/v2"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const mateValue = 1000000

var values = map[engine.PieceType]int{engine.King: 0, engine.Queen: 900, engine.Rook: 500, engine.Bishop: 330, engine.Knight: 320, engine.Pawn: 100}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func evaluate(p *engine.Position) int {
	score := 0
	for sq, pc := range p.Board().SquareMap() {
		file, rank := int(sq.File()), int(sq.Rank())
		advance := rank
		if pc.Color() == engine.Black {
			advance = 7 - rank
		}
		central := 14 - abs(2*file-7) - abs(2*rank-7)
		v := values[pc.Type()]
		switch pc.Type() {
		case engine.Pawn:
			v += advance*7 + central*2
		case engine.Knight:
			v += central * 6
		case engine.Bishop:
			v += central * 3
		case engine.Rook:
			v += advance * 2
		case engine.Queen:
			v += central
		case engine.King:
			if p.HalfMoveClock() > 15 {
				v += central * 3
			} else {
				v -= central * 2
			}
		}
		if pc.Color() == p.Turn() {
			score += v
		} else {
			score -= v
		}
	}
	return score
}
func order(p *engine.Position, moves []engine.Move) []engine.Move {
	r := append([]engine.Move(nil), moves...)
	value := func(m engine.Move) int {
		v := 0
		if isCapture(&m) {
			victim := p.Board().Piece(m.S2()).Type()
			if m.HasTag(engine.EnPassant) {
				victim = engine.Pawn
			}
			v += values[victim]*16 - values[p.Board().Piece(m.S1()).Type()]
		}
		if m.Promo() != engine.NoPieceType {
			v += values[m.Promo()] * 12
		}
		if m.HasTag(engine.Check) {
			v += 100
		}
		if m.HasTag(engine.KingSideCastle) || m.HasTag(engine.QueenSideCastle) {
			v += 80
		}
		return v
	}
	sort.SliceStable(r, func(i, j int) bool {
		a, b := value(r[i]), value(r[j])
		if a == b {
			return r[i].String() < r[j].String()
		}
		return a > b
	})
	return r
}
func moveProposal(s *State, m engine.Move) json.RawMessage {
	r, _ := json.Marshal(Action{Type: "move", Round: s.Round, Ply: s.Ply, UCI: m.String()})
	return r
}
func drawProposal(s *State, p *engine.Position) json.RawMessage {
	if s.Offer >= 0 && s.Offer != s.Turn {
		yes := true
		r, _ := json.Marshal(Action{Type: "respond_draw", Round: s.Round, Ply: s.Ply, Accept: &yes})
		return r
	}
	if eligible := claims(s, p, false); len(eligible) > 0 {
		r, _ := json.Marshal(Action{Type: "claim_draw", Round: s.Round, Ply: s.Ply, Reason: eligible[0]})
		return r
	}
	return nil
}
func botInput(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, _, e := decode(m)
	if e != nil {
		return nil, e
	}
	if s.Phase != "playing" || c.Actor != s.Turn {
		return nil, nil
	}
	return json.Marshal(s)
}
func fastChoice(s *State, p *engine.Position) (json.RawMessage, error) {
	moves := order(p, p.ValidMoves())
	if len(moves) == 0 {
		return nil, errors.New("没有合法着法")
	}
	for _, m := range moves {
		if m.HasTag(engine.Check) && p.Update(&m).Status() == engine.Checkmate {
			return moveProposal(s, m), nil
		}
	}
	if draw := drawProposal(s, p); draw != nil && evaluate(p) <= 0 {
		return draw, nil
	}
	best, value := moves[0], -2*mateValue
	for _, m := range moves {
		next := p.Update(&m)
		v := -evaluate(next)
		if m.HasTag(engine.KingSideCastle) || m.HasTag(engine.QueenSideCastle) {
			v += 35
		}
		if v > value {
			best, value = m, v
		}
	}
	return moveProposal(s, best), nil
}
func bot(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, g, e := decode(m)
	if e != nil {
		return nil, e
	}
	if c.Actor != s.Turn || s.Phase != "playing" {
		return nil, nil
	}
	return fastChoice(s, g.Position())
}

type search struct {
	ctx          context.Context
	nodes, limit int
	counts       map[string]int
	stopped      bool
}

func (s *search) visit(p *engine.Position, depth, qdepth, alpha, beta, ply int) int {
	s.nodes++
	if s.nodes >= s.limit || s.ctx.Err() != nil {
		s.stopped = true
		return 0
	}
	status := p.Status()
	if status == engine.Checkmate {
		return -mateValue + ply
	}
	if status == engine.Stalemate {
		return 0
	}
	if !canMate(p.Board(), engine.White) && !canMate(p.Board(), engine.Black) {
		return 0
	}
	key := positionKey(p)
	if s.counts[key] >= 5 || p.HalfMoveClock() >= 150 {
		return 0
	}
	canClaim := s.counts[key] >= 3 || p.HalfMoveClock() >= 100
	best := -2 * mateValue
	if canClaim {
		best = 0
		if alpha < 0 {
			alpha = 0
		}
	}
	check := checkedSquare(p) >= 0
	if depth <= 0 {
		static := evaluate(p)
		if !check {
			if static >= beta {
				return static
			}
			if static > alpha {
				alpha = static
			}
			if static > best {
				best = static
			}
		}
		if qdepth == 0 {
			if canClaim && static < 0 {
				return 0
			}
			return static
		}
	}
	for _, m := range order(p, p.ValidMoves()) {
		if depth <= 0 && !check && !isCapture(&m) && m.Promo() == engine.NoPieceType {
			continue
		}
		next := p.Update(&m)
		k := positionKey(next)
		s.counts[k]++
		nextQ := qdepth
		if depth <= 0 {
			nextQ--
		}
		v := -s.visit(next, depth-1, nextQ, -beta, -alpha, ply+1)
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
	s, g, e := decode(gameapi.Match{Schema: 1, Data: raw})
	if e != nil {
		return nil, e
	}
	if s.Phase != "playing" {
		return nil, nil
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	p := g.Position()
	fast, e := fastChoice(s, p)
	if e != nil {
		return nil, e
	}
	moves := order(p, p.ValidMoves())
	for _, m := range moves {
		if m.HasTag(engine.Check) && p.Update(&m).Status() == engine.Checkmate {
			return moveProposal(s, m), nil
		}
	}
	budget, limit, maxDepth := 180*time.Millisecond, 16000, 4
	if s.Options.Difficulty == "easy" {
		budget, limit, maxDepth = 35*time.Millisecond, 1800, 2
	}
	work, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	engineSearch := search{ctx: work, limit: limit, counts: map[string]int{}}
	for _, key := range s.Keys {
		engineSearch.counts[key]++
	}
	best := fast
	draw := drawProposal(s, p)
	for depth := 1; depth <= maxDepth; depth++ {
		value, chosen := -2*mateValue, best
		if draw != nil {
			value, chosen = 0, draw
		}
		for _, m := range moves {
			next := p.Update(&m)
			key := positionKey(next)
			engineSearch.counts[key]++
			v := -engineSearch.visit(next, depth-1, 3, -2*mateValue, -value, 1)
			engineSearch.counts[key]--
			if engineSearch.stopped {
				break
			}
			if v > value {
				value, chosen = v, moveProposal(s, m)
			}
		}
		if engineSearch.stopped {
			break
		}
		best = chosen
		var preferred Action
		_ = json.Unmarshal(best, &preferred)
		if preferred.UCI != "" {
			sort.SliceStable(moves, func(i, j int) bool { return moves[i].String() == preferred.UCI && moves[j].String() != preferred.UCI })
		}
		if value > mateValue-100 {
			break
		}
	}
	return best, nil
}
