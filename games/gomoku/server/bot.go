package gomoku

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type candidate struct{ pos, score int }
type observation struct {
	State State `json:"state"`
	Actor int   `json:"actor"`
}

const mate = 100000000

func moveAction(s *State, pos int) json.RawMessage {
	x, y := pos%Size, pos/Size
	raw, _ := json.Marshal(Action{Type: "place", Round: s.Round, Ply: s.Ply, X: &x, Y: &y})
	return raw
}
func drawResponse(s *State) json.RawMessage {
	// Accept only a late balanced board; otherwise explicitly keep playing.
	accept := s.Ply >= 180
	raw, _ := json.Marshal(Action{Type: "respond_draw", Round: s.Round, Ply: s.Ply, Accept: &accept})
	return raw
}
func botInput(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	if c.Actor < 0 || c.Actor > 1 || s.Phase != "playing" {
		return nil, nil
	}
	if c.Actor != s.Turn && !(s.Offer >= 0 && s.Offer != c.Actor) {
		return nil, nil
	}
	// Gomoku is perfect-information; do not pass the host room or other games'
	// snapshots to the worker. The complete input is this game's public board.
	return json.Marshal(observation{*s, c.Actor})
}
func bot(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	raw, e := botInput(c, m)
	if e != nil || raw == nil {
		return nil, e
	}
	var in observation
	_ = json.Unmarshal(raw, &in)
	if in.State.Offer >= 0 && in.State.Offer != c.Actor {
		return drawResponse(&in.State), nil
	}
	moves := rankMoves(in.State.Board, in.State.colour(c.Actor), in.State.Options.Rules)
	if len(moves) == 0 {
		return nil, errors.New("没有合法落点")
	}
	return moveAction(&in.State, moves[0].pos), nil
}

func threat(board []int, pos, color int, rules string) int {
	board[pos] = color
	defer func() { board[pos] = 0 }()
	x, y := pos%Size, pos/Size
	if winningLine(board, x, y, rules) != nil {
		return mate
	}
	total := 0
	for _, d := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} {
		best := 0
		for offset := -4; offset <= 0; offset++ {
			n, valid := 0, true
			for j := 0; j < 5; j++ {
				xx, yy := x+(offset+j)*d[0], y+(offset+j)*d[1]
				if !inBounds(xx, yy) {
					valid = false
					break
				}
				v := board[yy*Size+xx]
				if v == 3-color {
					valid = false
					break
				}
				if v == color {
					n++
				}
			}
			if valid {
				scores := [6]int{0, 2, 20, 300, 12000, 20000}
				if scores[n] > best {
					best = scores[n]
				}
			}
		}
		// An open straight four is a two-ended immediate threat.
		length, open := 1, 0
		for _, sign := range []int{-1, 1} {
			xx, yy := x+sign*d[0], y+sign*d[1]
			for inBounds(xx, yy) && board[yy*Size+xx] == color {
				length++
				xx += sign * d[0]
				yy += sign * d[1]
			}
			if inBounds(xx, yy) && board[yy*Size+xx] == 0 {
				open++
			}
		}
		if length == 4 && open == 2 {
			best += 200000
		}
		if length == 3 && open == 2 {
			best += 3000
		}
		total += best
	}
	return total
}
func rankMoves(board []int, color int, rules string) []candidate {
	seen := make([]bool, Size*Size)
	stones := 0
	for pos, v := range board {
		if v == 0 {
			continue
		}
		stones++
		x, y := pos%Size, pos/Size
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				xx, yy := x+dx, y+dy
				if inBounds(xx, yy) && board[yy*Size+xx] == 0 {
					seen[yy*Size+xx] = true
				}
			}
		}
	}
	if stones == 0 {
		return []candidate{{Size/2*Size + Size/2, 0}}
	}
	wins, blocks, moves := []candidate{}, []candidate{}, []candidate{}
	for p, ok := range seen {
		if !ok {
			continue
		}
		attack, defence := threat(board, p, color, rules), threat(board, p, 3-color, rules)
		v := candidate{p, attack + defence*9/10 - abs(p%Size-7) - abs(p/Size-7)}
		if attack >= mate {
			wins = append(wins, v)
		}
		if defence >= mate {
			blocks = append(blocks, v)
		}
		moves = append(moves, v)
	}
	if len(wins) > 0 {
		moves = wins
	} else if len(blocks) > 0 {
		moves = blocks
	}
	sort.Slice(moves, func(i, j int) bool {
		if moves[i].score == moves[j].score {
			return moves[i].pos < moves[j].pos
		}
		return moves[i].score > moves[j].score
	})
	return moves
}
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func evaluate(board []int, color int) int {
	total := 0
	weights := [6]int{0, 1, 12, 180, 9000, 20000}
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			for _, d := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} {
				if !inBounds(x+4*d[0], y+4*d[1]) {
					continue
				}
				own, enemy := 0, 0
				for k := 0; k < 5; k++ {
					v := board[(y+k*d[1])*Size+x+k*d[0]]
					if v == color {
						own++
					} else if v == 3-color {
						enemy++
					}
				}
				if enemy == 0 {
					total += weights[own]
				}
				if own == 0 {
					total -= weights[enemy]
				}
			}
		}
	}
	return total
}

type search struct {
	ctx                 context.Context
	rules               string
	nodes, limit, width int
	stopped             bool
}

func (s *search) visit(board []int, color, depth, alpha, beta int) int {
	s.nodes++
	if s.nodes >= s.limit || s.ctx.Err() != nil {
		s.stopped = true
		return 0
	}
	if depth == 0 {
		return evaluate(board, color)
	}
	moves := rankMoves(board, color, s.rules)
	if len(moves) == 0 {
		return 0
	}
	best := -mate
	for _, m := range moves[:min(len(moves), s.width)] {
		board[m.pos] = color
		value := mate + depth
		if winningLine(board, m.pos%Size, m.pos/Size, s.rules) == nil {
			value = -s.visit(board, 3-color, depth-1, -beta, -alpha)
		}
		board[m.pos] = 0
		if s.stopped {
			return 0
		}
		if value > best {
			best = value
		}
		if value > alpha {
			alpha = value
		}
		if alpha >= beta {
			break
		}
	}
	return best
}
func think(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var in observation
	if json.Unmarshal(raw, &in) != nil || len(in.State.Board) != Size*Size || in.Actor < 0 || in.Actor > 1 {
		return nil, errors.New("无效电脑局面")
	}
	s := &in.State
	if s.Phase != "playing" {
		return nil, nil
	}
	if s.Offer >= 0 && s.Offer != in.Actor {
		return drawResponse(s), nil
	}
	if s.Turn != in.Actor {
		return nil, nil
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	color := s.colour(in.Actor)
	moves := rankMoves(s.Board, color, s.Options.Rules)
	if len(moves) == 0 {
		return nil, errors.New("没有合法落点")
	}
	best := moves[0].pos
	if len(moves) == 1 || moves[0].score >= mate {
		return moveAction(s, best), nil
	}
	budget, limit, width, maxDepth := 200*time.Millisecond, 6000, 10, 4
	if s.Options.Difficulty == "easy" {
		budget, limit, width, maxDepth = 40*time.Millisecond, 600, 6, 2
	}
	work, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	engine := search{ctx: work, rules: s.Options.Rules, limit: limit, width: width}
	for depth := 1; depth <= maxDepth; depth++ {
		value, chosen := -2*mate, best
		for _, m := range moves[:min(len(moves), width)] {
			s.Board[m.pos] = color
			v := -engine.visit(s.Board, 3-color, depth-1, -2*mate, 2*mate)
			s.Board[m.pos] = 0
			if engine.stopped {
				break
			}
			if v > value {
				value, chosen = v, m.pos
			}
		}
		if engine.stopped {
			break
		}
		best = chosen
	}
	return moveAction(s, best), nil
}
