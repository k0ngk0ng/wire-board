package weiqi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"sort"
	"time"
)

type observation struct {
	State State `json:"state"`
	Actor int   `json:"actor"`
}
type candidate struct {
	pos   int
	value float64
	board []int
}

func proposal(s *State, kind string, pos int) json.RawMessage {
	a := Action{Type: kind, Round: s.Round, Ply: s.Ply, Revision: s.Revision}
	if pos >= 0 {
		a.Pos = &pos
	}
	raw, _ := json.Marshal(a)
	return raw
}
func botInput(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	if c.Actor < 0 || c.Actor > 1 || s.Phase == "finished" || s.Phase == "playing" && c.Actor != s.Turn || s.Phase == "scoring" && s.Confirmed[c.Actor] {
		return nil, nil
	}
	return json.Marshal(observation{*s, c.Actor})
}
func scoreAction(s *State, actor int) json.RawMessage {
	for _, p := range s.Dead {
		if s.Board[p] == s.colour(actor) {
			return proposal(s, "resume", -1)
		}
	}
	return proposal(s, "confirm_score", -1)
}

// trueEye is a strategy filter only; human legal actions always use play.
func trueEye(board []int, p, color, n int) bool {
	for _, q := range neighbours(p, n) {
		if board[q] != color {
			return false
		}
	}
	x, y := p%n, p/n
	bad, off := 0, 0
	for _, d := range [][2]int{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		xx, yy := x+d[0], y+d[1]
		if xx < 0 || yy < 0 || xx >= n || yy >= n {
			off++
		} else if board[yy*n+xx] != color {
			bad++
		}
	}
	if off > 0 {
		return bad == 0
	}
	return bad <= 1
}
func rank(ctx context.Context, s *State, color int) []candidate {
	n := s.Options.Size
	history := keySet(s.Keys)
	groups := make(map[int][]int)
	liberties := make(map[int][]int)
	for p, v := range s.Board {
		if v == 0 {
			continue
		}
		if _, ok := groups[p]; ok {
			continue
		}
		g, l := group(s.Board, p, n)
		for _, q := range g {
			groups[q] = g
			liberties[q] = l
		}
	}
	moves := []candidate{}
	for p, v := range s.Board {
		if ctx.Err() != nil {
			return moves
		}
		if v != 0 || trueEye(s.Board, p, color, n) {
			continue
		}
		next, captured, ok := play(s.Board, p, color, n, history)
		if !ok {
			continue
		}
		own, libs := group(next, p, n)
		if len(libs) == 1 && len(captured) == 0 {
			continue
		}
		value := float64(len(captured)) * 120
		seen := map[int]bool{}
		friends, enemies := 0, 0
		for _, q := range neighbours(p, n) {
			if s.Board[q] == 0 {
				continue
			}
			g := groups[q]
			if seen[g[0]] {
				continue
			}
			seen[g[0]] = true
			if s.Board[q] == color {
				friends++
				if len(liberties[q]) == 1 && len(libs) > 1 {
					value += float64(len(g))*65 + 20
				}
			} else {
				enemies++
				if len(liberties[q]) == 2 {
					value += 18 + float64(len(g))*2
				}
			}
		}
		if len(libs) == 1 {
			value -= float64(len(own)) * 70
		}
		value += float64(min(len(libs), 5)) * 2
		value += float64(enemies) * 3
		if friends > 1 {
			value += 7
		}
		if enemies == 0 && friends >= 3 {
			value -= 20
		}
		x, y := p%n, p/n
		edge := min(x, y, n-1-x, n-1-y)
		if edge == 0 {
			value -= 8
		}
		if edge == 1 {
			value -= 3
		}
		if friends == 0 && enemies == 0 {
			value += 12
			value -= float64(abs(x-n/2)+abs(y-n/2)) * .12
		}
		if value < 0 && len(captured) == 0 {
			continue
		}
		moves = append(moves, candidate{p, value, next})
	}
	sort.Slice(moves, func(i, j int) bool {
		if moves[i].value == moves[j].value {
			return moves[i].pos < moves[j].pos
		}
		return moves[i].value > moves[j].value
	})
	return moves
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func bot(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	raw, e := botInput(c, m)
	if e != nil || raw == nil {
		return nil, e
	}
	var in observation
	_ = json.Unmarshal(raw, &in)
	if in.State.Phase == "scoring" {
		return scoreAction(&in.State, in.Actor), nil
	}
	moves := rank(context.Background(), &in.State, in.State.colour(in.Actor))
	if len(moves) == 0 {
		return proposal(&in.State, "pass", -1), nil
	}
	return proposal(&in.State, "place", moves[0].pos), nil
}
func think(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var in observation
	if json.Unmarshal(raw, &in) != nil || in.Actor < 0 || in.Actor > 1 {
		return nil, errors.New("无效围棋搜索输入")
	}
	s := &in.State
	if s.Phase == "scoring" {
		return scoreAction(s, in.Actor), nil
	}
	color := s.colour(in.Actor)
	moves := rank(ctx, s, color)
	if len(moves) == 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return proposal(s, "pass", -1), nil
	}
	best := moves[0].pos
	if s.Options.Difficulty == "easy" {
		return proposal(s, "place", best), nil
	}
	bestValue := -1e20
	// Bounded two-ply tactical search. Only completed replies affect the choice;
	// cancellation keeps the best already evaluated legal move.
	for _, candidate := range moves[:min(8, len(moves))] {
		if ctx.Err() != nil {
			break
		}
		next := *s
		next.Board = candidate.board
		next.Keys = append(append([]string(nil), s.Keys...), PositionKey(next.Board))
		responses := rank(ctx, &next, 3-color)
		if ctx.Err() != nil {
			break
		}
		value := candidate.value
		if len(responses) > 0 {
			value -= responses[0].value * .8
		}
		if value > bestValue {
			bestValue = value
			best = candidate.pos
		}
	}
	return proposal(s, "place", best), nil
}

var search = &gameapi.BotSearch{Input: botInput, Think: think, Budget: 350 * time.Millisecond}
