package spf2t

import (
	"encoding/json"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s := &State{}
	if err := s.decode(m); err != nil {
		return nil, err
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) {
		return nil, nil
	}
	p := &s.Players[ctx.Actor]
	switch s.Phase {
	case PhaseSelect:
		if p.Selected {
			return nil, nil
		}
		return json.Marshal(Action{Type: "select", Epoch: s.Epoch, Character: botCharacter(s.Epoch, ctx.Actor)})
	case PhasePlaying:
		if p.Lost || p.Resolve != nil || p.Active == nil {
			return nil, nil
		}
		if ctx.Now.UnixMilli() < p.SpawnedAt+300 {
			return nil, nil
		}
		events := planBot(s, ctx.Actor, ctx.Now.UnixMilli())
		if len(events) == 0 {
			return nil, nil
		}
		return json.Marshal(Action{Type: "inputs", Epoch: s.Epoch, Seq: p.Seq + 1, Events: events})
	}
	return nil, nil
}

func cloneState(s *State) *State {
	b, _ := json.Marshal(s)
	var c State
	_ = json.Unmarshal(b, &c)
	c.Result = nil
	return &c
}

func clonePlayer(p Player) Player {
	b, _ := json.Marshal(p)
	var c Player
	_ = json.Unmarshal(b, &c)
	if p.Active != nil {
		a := *p.Active
		c.Active = &a
	}
	if p.Resolve != nil {
		r := *p.Resolve
		c.Resolve = &r
	}
	return c
}

type botChoice struct {
	events []Input
	score  int
}

func planBot(s *State, seat int, now int64) []Input {
	p := s.Players[seat]
	if p.Active == nil {
		return nil
	}
	best := botChoice{score: -1 << 30}
	current := p.Active.Rot
	for targetRot := 0; targetRot < 4; targetRot++ {
		// Rotate a scratch copy exactly as the server would, including kicks.
		scratch := clonePlayer(p)
		cw := (targetRot - current + 4) % 4
		dir := 1
		steps := cw
		if cw > 2 {
			dir = -1
			steps = 4 - cw
		}
		ok := true
		var events []Input
		for i := 0; i < steps; i++ {
			if !scratch.tryRotate(dir) {
				ok = false
				break
			}
			events = append(events, Input{Kind: rotateKind(dir)})
		}
		if !ok || scratch.Active == nil || scratch.Active.Rot != targetRot {
			continue
		}
		for targetX := 0; targetX < Cols; targetX++ {
			sim := clonePlayer(scratch)
			path := append([]Input{}, events...)
			ok = true
			for sim.Active.X != targetX {
				dx := 1
				kind := "right"
				if sim.Active.X > targetX {
					dx = -1
					kind = "left"
				}
				if !sim.tryMove(dx) {
					ok = false
					break
				}
				path = append(path, Input{Kind: kind})
			}
			if !ok || sim.Active == nil || sim.Active.X != targetX {
				continue
			}
			grid, err := gemGrid(sim.Gems)
			if err != nil || !pairFits(grid, *sim.Active, sim.Active.X, sim.Active.Y, sim.Active.Rot) {
				continue
			}
			y := dropYFor(sim.Gems, *sim.Active)
			sim.Active.Y = y
			if len(path)+1 > MaxBatchEvents {
				continue
			}
			score := simulateChoice(s, seat, sim, now)
			if score > best.score {
				best.score = score
				best.events = append(append([]Input{}, path...), Input{Kind: "hardDrop"})
			}
		}
	}
	if len(best.events) == 0 {
		return []Input{{Kind: "hardDrop"}}
	}
	return best.events
}

func rotateKind(dir int) string {
	if dir < 0 {
		return "rotateCCW"
	}
	return "rotateCW"
}

func simulateChoice(s *State, seat int, sim Player, now int64) int {
	c := cloneState(s)
	c.Players[seat] = sim
	if c.Players[seat].Active == nil {
		return -1 << 30
	}
	oppBefore := len(c.Players[1-seat].Pending)
	clearedBefore := c.Players[seat].GemsCleared
	chainBefore := c.Players[seat].MaxChain
	c.lockPlayer(&c.Players[seat], now)
	for steps := 0; c.Players[seat].Resolve != nil && steps < 100; steps++ {
		c.resolveStep(&c.Players[seat], now)
	}
	if c.Players[seat].Lost {
		return -1 << 30
	}
	sent := len(c.Players[1-seat].Pending) - oppBefore
	cleared := c.Players[seat].GemsCleared - clearedBefore
	chain := c.Players[seat].MaxChain - chainBefore
	grid, err := gemGrid(c.Players[seat].Gems)
	if err != nil {
		return -1 << 30
	}
	heights := make([]int, Cols)
	holes := 0
	powers := 0
	for x := 0; x < Cols; x++ {
		top := Rows
		for y := 0; y < Rows; y++ {
			if grid[y][x] != 0 {
				top = y
				break
			}
		}
		heights[x] = Rows - top
		if top == Rows {
			heights[x] = 0
		}
		seen := false
		for y := 0; y < Rows; y++ {
			if grid[y][x] != 0 {
				seen = true
			} else if seen {
				holes++
			}
		}
	}
	for _, g := range c.Players[seat].Gems {
		if g.Kind == GemPower {
			powers++
		}
	}
	bump := 0
	for x := 1; x < Cols; x++ {
		d := heights[x] - heights[x-1]
		if d < 0 {
			d = -d
		}
		bump += d
	}
	maxH := 0
	for _, h := range heights {
		if h > maxH {
			maxH = h
		}
	}
	// The weights favor a live board with useful clears while still valuing
	// immediate pressure. All information here is the acting seat's own board
	// plus the public pending count.
	score := sent*35 + cleared*18 + chain*12 + powers*6
	score -= maxH * 9
	score -= holes * 24
	score -= bump * 2
	score -= len(c.Players[seat].Pending) * 3
	return score
}
