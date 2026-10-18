package tetris

import "fmt"

func applyEvent(s *State, p *Player, kind string, now int64) {
	if s.Result != nil || p.Active == nil {
		return
	}
	switch kind {
	case "left":
		if tryMove(p, -1) {
			resetLock(p, now)
		}
	case "right":
		if tryMove(p, 1) {
			resetLock(p, now)
		}
	case "rotateCW":
		if tryRotate(p, 1) {
			resetLock(p, now)
		}
	case "rotateCCW":
		if tryRotate(p, -1) {
			resetLock(p, now)
		}
	case "softDrop":
		if fits(p.Board, p.Active.Kind, p.Active.Rot, p.Active.X, p.Active.Y+1) {
			p.Active.Y++
			p.Score++
			p.FallAt = now + gravityFor(p.Level)
			resetLock(p, now)
		} else if p.LockAt == 0 {
			p.LockAt = now + LockDelayMS
		}
	case "hardDrop":
		y := dropY(p.Board, p.Active.Kind, p.Active.Rot, p.Active.X, p.Active.Y)
		if dist := y - p.Active.Y; dist > 0 {
			p.Active.Y = y
			p.Score += 2 * dist
		}
		lockPlayer(s, p, now)
	case "hold":
		if p.HoldUsed {
			return
		}
		previous := p.Active.Kind
		if p.Hold == Empty {
			p.Hold = previous
			if !spawn(p, now) {
				eliminatePlayer(s, p, now, "spawn_blocked")
				return
			}
		} else {
			held := p.Hold
			p.Hold = previous
			if !spawnKind(p, held, now) {
				eliminatePlayer(s, p, now, "spawn_blocked")
				return
			}
		}
		p.HoldUsed = true
	}
}

func advancePlayer(s *State, p *Player, now int64) bool {
	if !p.Alive {
		return false
	}
	if p.Active == nil {
		if !spawn(p, now) {
			eliminatePlayer(s, p, now, "spawn_blocked")
			return s.Phase == "finished"
		}
	}
	for step := 0; step < MaxCatchUp; step++ {
		if p.Active == nil {
			return s.Phase == "finished"
		}
		if p.LockAt > 0 && now >= p.LockAt {
			return lockPlayer(s, p, now)
		}
		if now < p.FallAt {
			return false
		}
		if fits(p.Board, p.Active.Kind, p.Active.Rot, p.Active.X, p.Active.Y+1) {
			p.Active.Y++
			p.FallAt += gravityFor(p.Level)
			continue
		}
		if p.LockAt == 0 {
			p.LockAt = now + LockDelayMS
		}
		return false
	}
	p.FallAt = now + gravityFor(p.Level)
	return false
}

func lockPlayer(s *State, p *Player, now int64) bool {
	if s.Result != nil || p.Active == nil {
		return s.Phase == "finished"
	}
	piece := p.Active
	allHidden := true
	for _, pt := range shapeCells(piece.Kind, piece.Rot) {
		x, y := piece.X+pt.X, piece.Y+pt.Y
		if y >= HiddenRows {
			allHidden = false
		}
		setCell(&p.Board, x, y, piece.Kind)
	}
	p.Active = nil
	cleared := clearFullRows(&p.Board)
	count := len(cleared)
	s.emitEvent(Event{Type: "lock", Seat: p.Seat, At: now, Kind: piece.Kind, PieceID: piece.ID})
	if allHidden && count == 0 {
		eliminatePlayer(s, p, now, "top_out")
		return s.Phase == "finished"
	}
	levelBefore := p.Level
	p.Score += scoreForLines(count, levelBefore)
	p.Lines += count
	p.Level = levelForLines(p.Lines)
	if p.Level != levelBefore {
		s.emitEvent(Event{Type: "level", Seat: p.Seat, At: now, Level: p.Level})
	}
	rows := []int{}
	for _, y := range cleared {
		if y >= HiddenRows {
			rows = append(rows, y-HiddenRows)
		}
	}
	if count > 0 {
		ev := Event{Type: "clear", Seat: p.Seat, At: now, Rows: rows, Count: count}
		if s.Options.Mode == ModeSurvival && len(s.Players) > 1 {
			amount := attackFor(count)
			if amount > 0 && p.TargetSeat >= 0 && p.TargetSeat < len(s.Players) && s.Players[p.TargetSeat].Alive {
				target := &s.Players[p.TargetSeat]
				for n := 0; n < amount && len(target.Pending) < MaxPendingGarbage; n++ {
					target.Pending = append(target.Pending, int(nextRandom(&p.Rand)%BoardWidth))
				}
				ev.Target = p.TargetSeat
				s.emitEvent(Event{Type: "garbage", Seat: p.TargetSeat, At: now, Count: amount, Target: p.Seat})
				advanceTarget(s, p)
			}
		}
		s.emitEvent(ev)
	}
	if count > 0 && len(p.Pending) > 0 {
		p.Pending = p.Pending[minInt(count, len(p.Pending)):]
	}
	if len(p.Pending) > 0 {
		pending := append([]int{}, p.Pending...)
		p.Pending = nil
		if insertGarbage(&p.Board, pending) {
			s.emitEvent(Event{Type: "garbage", Seat: p.Seat, At: now, Count: len(pending), Applied: true})
			eliminatePlayer(s, p, now, "garbage_top_out")
			return s.Phase == "finished"
		}
		s.emitEvent(Event{Type: "garbage", Seat: p.Seat, At: now, Count: len(pending), Applied: true})
	}
	if s.Options.Mode == ModeSprint && p.Lines >= SprintTarget {
		finish(s, now, "sprint_finished")
		return true
	}
	p.HoldUsed = false
	if !spawn(p, now) {
		eliminatePlayer(s, p, now, "spawn_blocked")
		return s.Phase == "finished"
	}
	if s.Options.Mode == ModeSurvival && len(s.Players) > 1 && aliveCount(s) <= 1 {
		finish(s, now, "last_standing")
		return true
	}
	return s.Phase == "finished"
}

func eliminatePlayer(s *State, p *Player, now int64, reason string) {
	if !p.Alive {
		return
	}
	p.Alive = false
	p.EliminatedAt = now
	p.Active = nil
	p.Pending = nil
	s.emitEvent(Event{Type: "eliminate", Seat: p.Seat, At: now})
	s.log(fmt.Sprintf("玩家 %d 出局（%s）", p.Seat+1, reason))
	normalizeTargets(s)
	if s.Options.Mode == ModeSurvival {
		switch aliveCount(s) {
		case 1:
			finish(s, now, "last_standing")
		case 0:
			finish(s, now, "all_out")
		}
		return
	}
	if aliveCount(s) == 0 {
		finish(s, now, "all_out")
	}
}
