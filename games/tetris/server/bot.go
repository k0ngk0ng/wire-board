package tetris

import (
	"encoding/json"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// BotAction deliberately reads only the acting seat's board, active piece and
// pending garbage plus the public mode. It never consumes the seed, bag or an
// opponent's queue, and it returns the same inputs batch a human uses.
func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) || ctx.Players != len(s.Players) ||
		s.Phase != "playing" || !s.Players[ctx.Actor].Alive || s.Players[ctx.Actor].Active == nil {
		return nil, nil
	}
	p := &s.Players[ctx.Actor]
	if p.Seq >= MaxSeq {
		return nil, nil
	}
	now := ctx.Now.UnixMilli()
	due := p.Active.SpawnedAt + botDelay(p.Level)
	if now < due && !(p.LockAt > 0 && now >= p.LockAt-100) {
		return nil, nil
	}
	events := planBot(s, p)
	if len(events) == 0 {
		events = []Input{{Kind: "hardDrop"}}
	}
	if len(events) > MaxBatchEvents {
		events = events[:MaxBatchEvents]
	}
	return json.Marshal(Action{Type: "inputs", Epoch: s.Epoch, Seq: p.Seq + 1, Events: events})
}

func botDelay(level int) int64 {
	d := gravityFor(level) / 2
	if d > 250 {
		d = 250
	}
	if d < 50 {
		d = 50
	}
	return d
}

func copyPiece(p *Piece) *Piece {
	if p == nil {
		return nil
	}
	c := *p
	return &c
}

func rotationsFor(kind Kind) []int {
	if kind == O {
		return []int{0}
	}
	return []int{0, 1, 2, 3}
}

type botCandidate struct {
	events []Input
	score  float64
}

func planBot(s *State, p *Player) []Input {
	var best *botCandidate
	for _, rot := range rotationsFor(p.Active.Kind) {
		sim := *p
		sim.Active = copyPiece(p.Active)
		rotations := []Input{}
		reachable := true
		for sim.Active.Rot != rot {
			if !tryRotate(&sim, 1) {
				reachable = false
				break
			}
			rotations = append(rotations, Input{Kind: "rotateCW"})
		}
		if !reachable || len(rotations) > MaxBatchEvents {
			continue
		}
		for x := -2; x <= BoardWidth+1; x++ {
			sim2 := sim
			sim2.Active = copyPiece(sim.Active)
			events := append([]Input{}, rotations...)
			ok := true
			for sim2.Active.X < x {
				if !tryMove(&sim2, 1) {
					ok = false
					break
				}
				events = append(events, Input{Kind: "right"})
			}
			for sim2.Active.X > x {
				if !tryMove(&sim2, -1) {
					ok = false
					break
				}
				events = append(events, Input{Kind: "left"})
			}
			if !ok || len(events)+1 > MaxBatchEvents {
				continue
			}
			y := dropY(sim2.Board, sim2.Active.Kind, sim2.Active.Rot, sim2.Active.X, sim2.Active.Y)
			if !fits(sim2.Board, sim2.Active.Kind, sim2.Active.Rot, sim2.Active.X, y) {
				continue
			}
			board := sim2.Board
			allHidden := true
			for _, pt := range shapeCells(sim2.Active.Kind, sim2.Active.Rot) {
				cx, cy := sim2.Active.X+pt.X, y+pt.Y
				if cy >= HiddenRows {
					allHidden = false
				}
				setCell(&board, cx, cy, sim2.Active.Kind)
			}
			cleared := clearFullRows(&board)
			if allHidden && len(cleared) == 0 {
				continue
			}
			if s.Options.Mode == ModeSurvival && len(s.Players) > 1 && len(sim2.Pending) > 0 {
				pending := sim2.Pending[minInt(len(cleared), len(sim2.Pending)):]
				if insertGarbage(&board, pending) {
					continue
				}
			}
			score := evaluateBoard(board, len(cleared))
			candidate := &botCandidate{events: append(events, Input{Kind: "hardDrop"}), score: score}
			if best == nil || candidate.score > best.score {
				best = candidate
			}
		}
	}
	if best == nil {
		return nil
	}
	return best.events
}

func evaluateBoard(board [BoardRows]string, cleared int) float64 {
	heights := make([]int, BoardWidth)
	holes := 0
	maxHeight := 0
	for x := 0; x < BoardWidth; x++ {
		top := -1
		for y := 0; y < BoardRows; y++ {
			if board[y][x] != '0' {
				top = y
				break
			}
		}
		if top < 0 {
			continue
		}
		h := BoardRows - top
		heights[x] = h
		if h > maxHeight {
			maxHeight = h
		}
		for y := top + 1; y < BoardRows; y++ {
			if board[y][x] == '0' {
				holes++
			}
		}
	}
	aggregate := 0
	bumpiness := 0
	for x := 0; x < BoardWidth; x++ {
		aggregate += heights[x]
		if x > 0 {
			bumpiness += absInt(heights[x] - heights[x-1])
		}
	}
	return 0.76*float64(cleared) - 0.51*float64(aggregate) - 0.36*float64(holes) - 0.18*float64(bumpiness) - 0.05*float64(maxHeight)
}
