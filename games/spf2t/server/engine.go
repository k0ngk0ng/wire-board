package spf2t

import "fmt"

func rectCells(r powerRect) []Point {
	out := make([]Point, 0, r.area())
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			out = append(out, Point{X: x, Y: y})
		}
	}
	return out
}

func (s *State) emitMerge(p *Player, rects []powerRect, now int64) {
	for _, r := range rects {
		s.emitEvent(Event{Type: "power_merge", Seat: p.Seat, At: now, Count: r.area(), Color: r.Color, Cells: rectCells(r)})
	}
}

func (s *State) removeDiamondAt(p *Player, x, y int) bool {
	ids := map[int]bool{}
	for _, g := range p.Gems {
		if g.Kind == GemDiamond && g.X == x && g.Y == y {
			ids[g.ID] = true
		}
	}
	return len(p.removeGemIDs(ids)) > 0
}

func decrementCounters(p *Player) {
	for i := range p.Gems {
		g := &p.Gems[i]
		if g.Kind != GemCounter {
			continue
		}
		g.Timer--
		if g.Timer <= 0 {
			g.Kind = GemNormal
			g.Timer = 0
		}
	}
}

func (s *State) lockPlayer(p *Player, now int64) {
	if p.Active == nil || p.Resolve != nil || p.Lost {
		return
	}
	pair := *p.Active
	p.Active = nil
	var diamond *Point
	for _, c := range pairCells(pair) {
		g := Gem{Kind: c.Kind, Color: c.Color, X: c.X, Y: c.Y, W: 1, H: 1}
		if c.Kind == GemDiamond {
			pt := Point{X: c.X, Y: c.Y}
			diamond = &pt
		}
		p.addGem(g)
	}
	s.emitEvent(Event{Type: "lock", Seat: p.Seat, At: now, Kind: pair.A.Kind, Cells: []Point{
		{X: pair.X, Y: pair.Y},
	}})
	decrementCounters(p)
	s.emitMerge(p, mergePowerGems(p), now)
	target := ""
	if diamond != nil {
		target = diamondTarget(p, diamond.X, diamond.Y)
	}
	if target != "" {
		p.Resolve = &Resolve{Chain: 1, Step: 1, NextAt: now + ResolveStepMS, Diamond: true, Target: target, DiamondX: diamond.X, DiamondY: diamond.Y}
		s.emitEvent(Event{Type: "diamond", Seat: p.Seat, At: now, Color: target, Cells: []Point{{X: diamond.X, Y: diamond.Y}}})
		return
	}
	if diamond != nil {
		s.removeDiamondAt(p, diamond.X, diamond.Y)
		s.emitEvent(Event{Type: "diamond", Seat: p.Seat, At: now, Cells: []Point{{X: diamond.X, Y: diamond.Y}}})
	}
	if groups := p.detectCrashGroups(); len(groups) > 0 {
		p.Resolve = &Resolve{Chain: 1, Step: 1, NextAt: now + ResolveStepMS}
		return
	}
	s.finishResolution(p, &Resolve{}, now)
}

func (s *State) applyInput(p *Player, kind string, now int64) bool {
	if p.Active == nil || p.Resolve != nil || p.Lost {
		return false
	}
	switch kind {
	case "left":
		if p.tryMove(-1) {
			p.resetLock(now)
		}
	case "right":
		if p.tryMove(1) {
			p.resetLock(now)
		}
	case "rotateCW":
		if p.tryRotate(1) {
			p.resetLock(now)
		}
	case "rotateCCW":
		if p.tryRotate(-1) {
			p.resetLock(now)
		}
	case "softDrop":
		p.softDrop(now)
	case "hardDrop":
		p.hardDrop()
		s.lockPlayer(p, now)
		return true
	}
	return false
}

func (s *State) resolveStep(p *Player, now int64) {
	r := p.Resolve
	if r == nil {
		return
	}
	if r.Step == 1 && r.Diamond {
		cells, flat, counter, powers := p.diamondClear(r.Target, r.DiamondX, r.DiamondY)
		base := flat + counter
		for _, pw := range powers {
			base += powerBase(pw.area(), pw.W, pw.H)
		}
		attack := stepAttack(r.Chain, base, 1, true)
		r.Accum += attack
		r.DiamondPart += attack
		r.Cleared += len(cells)
		s.emitEvent(Event{Type: "clear", Seat: p.Seat, At: now, Chain: r.Chain, Attack: attack, Kind: GemDiamond, Color: r.Target, Cells: cells})
		p.Resolve = r
		settleGems(p)
		s.emitMerge(p, mergePowerGems(p), now)
		r.Step = 2
		if groups := p.detectCrashGroups(); len(groups) > 0 {
			r.Chain++
			r.NextAt = now + ResolveStepMS
			return
		}
		s.finishResolution(p, r, now)
		return
	}
	groups := p.detectCrashGroups()
	if len(groups) == 0 {
		s.finishResolution(p, r, now)
		return
	}
	cells, flat, crash, counter, powers := p.clearGroups(groups)
	base := flat + crash + counter
	for _, pw := range powers {
		base += powerBase(pw.area(), pw.W, pw.H)
	}
	attack := stepAttack(r.Chain, base, len(groups), false)
	r.Accum += attack
	r.Cleared += len(cells)
	s.emitEvent(Event{Type: "clear", Seat: p.Seat, At: now, Chain: r.Chain, Attack: attack, Cells: cells})
	p.Resolve = r
	settleGems(p)
	s.emitMerge(p, mergePowerGems(p), now)
	if next := p.detectCrashGroups(); len(next) > 0 {
		r.Chain++
		r.Step++
		r.NextAt = now + ResolveStepMS
		return
	}
	s.finishResolution(p, r, now)
}

func (s *State) finishResolution(p *Player, r *Resolve, now int64) {
	if p.Resolve != nil {
		r = p.Resolve
	}
	if r == nil {
		r = &Resolve{}
	}
	attack := r.Accum
	if r.Cleared > 0 && len(p.Gems) == 0 {
		attack += 6
	}
	if r.Cleared > 0 && attack < 1 {
		attack = 1
	}
	if attack < 0 {
		attack = 0
	}
	cancel := attack
	if cancel > len(p.Pending) {
		cancel = len(p.Pending)
	}
	if cancel > 0 {
		p.Pending = p.Pending[cancel:]
		attack -= cancel
	}
	sent := 0
	if attack > 0 {
		opp := &s.Players[1-p.Seat]
		diamondPart := r.DiamondPart
		for i := 0; i < attack; i++ {
			if len(opp.Pending) >= MaxPending {
				s.log("反击宝石超过上限，丢弃剩余部分")
				break
			}
			color, col := patternAt(p.Character, p.PatternIdx)
			timer := 5
			if i < diamondPart {
				timer = 3
			}
			opp.Pending = append(opp.Pending, CounterSpec{Color: color, Column: col, Timer: timer})
			p.PatternIdx = (p.PatternIdx + 1) % 72
			sent++
		}
		if sent > 0 {
			s.emitEvent(Event{Type: "attack", Seat: p.Seat, Target: opp.Seat, At: now, Count: sent, Attack: sent})
		}
	}
	if r.Cleared > 0 {
		p.MaxChain = maxInt(p.MaxChain, r.Chain)
		p.GemsCleared += r.Cleared
	}
	if len(p.Pending) > 0 {
		grid, _ := gemGrid(p.Gems)
		delivered := 0
		for _, spec := range p.Pending {
			y := -1
			for yy := Rows - 1; yy >= 0; yy-- {
				if grid[yy][spec.Column] == 0 {
					y = yy
					break
				}
			}
			if y < 0 {
				p.Lost = true
				break
			}
			id := p.addGem(Gem{Kind: GemCounter, Color: spec.Color, X: spec.Column, Y: y, W: 1, H: 1, Timer: spec.Timer})
			grid[y][spec.Column] = id
			delivered++
		}
		p.Pending = nil
		if delivered > 0 {
			s.emitEvent(Event{Type: "garbage", Seat: p.Seat, At: now, Count: delivered})
		}
	}
	p.Resolve = nil
	p.IdleAt = now + IdleMS
	if p.Lost {
		p.Active = nil
		s.emitEvent(Event{Type: "topout", Seat: p.Seat, At: now})
		return
	}
	if !s.spawn(p, now) {
		return
	}
	s.emitEvent(Event{Type: "resolve_end", Seat: p.Seat, At: now})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *State) advancePlayer(p *Player, now int64) {
	if p.Lost {
		return
	}
	if p.Resolve != nil {
		steps := 0
		for p.Resolve != nil && now >= p.Resolve.NextAt && steps < MaxCatchUp {
			s.resolveStep(p, now)
			steps++
		}
		if p.Resolve != nil && now >= p.Resolve.NextAt {
			p.Resolve.NextAt = now + ResolveStepMS
		}
		return
	}
	if p.Active == nil {
		if !s.spawn(p, now) {
			return
		}
	}
	if p.LockAt > 0 && now >= p.LockAt {
		s.lockPlayer(p, now)
		return
	}
	steps := 0
	for now >= p.FallAt && steps < MaxCatchUp {
		if p.Resolve != nil || p.Active == nil {
			return
		}
		if p.canPlaceAt(p.Active.X, p.Active.Y+1, p.Active.Rot) {
			p.Active.Y++
			p.FallAt += FallMS
			steps++
			continue
		}
		if p.LockAt == 0 {
			p.LockAt = now + LockDelayMS
		}
		return
	}
	if steps == MaxCatchUp {
		p.FallAt = now + FallMS
	}
	if p.LockAt > 0 && now >= p.LockAt {
		s.lockPlayer(p, now)
	}
}

func (s *State) onTimer(now int64) error {
	if s.Result != nil {
		return nil
	}
	s.ServerAt = now
	switch s.Phase {
	case PhaseSelect:
		if s.allSelected() {
			s.moveToCountdown(now)
			return nil
		}
		if now >= s.SelectUntil {
			s.autoSelect(now)
			s.moveToCountdown(now)
			return nil
		}
	case PhaseCountdown:
		if now >= s.StartAt {
			s.beginMatch(now)
		}
	case PhasePlaying:
		if now >= s.MatchEndsAt {
			return s.finish(now, "time_limit")
		}
		for i := range s.Players {
			s.advancePlayer(&s.Players[i], now)
		}
		lost := [2]bool{s.Players[0].Lost, s.Players[1].Lost}
		if lost[0] || lost[1] {
			if lost[0] && lost[1] {
				s.emitEvent(Event{Type: "round_end", Seat: -1, Target: -1, At: now, Count: s.Round})
				s.log(fmt.Sprintf("第 %d 回合双方同时顶出，重赛", s.Round))
			} else {
				winner := 0
				if lost[0] {
					winner = 1
				}
				s.Players[winner].RoundWins++
				s.emitEvent(Event{Type: "round_end", Seat: -1, Target: winner, At: now, Count: s.Round})
				s.log(fmt.Sprintf("第 %d 回合玩家 %d 获胜", s.Round, winner+1))
			}
			s.Phase = PhaseRoundBreak
			s.BreakUntil = now + BreakMS
			for i := range s.Players {
				s.Players[i].Active = nil
				s.Players[i].Resolve = nil
			}
			return nil
		}
		if now >= s.MatchEndsAt {
			return s.finish(now, "time_limit")
		}
	case PhaseRoundBreak:
		if now >= s.BreakUntil {
			for i := range s.Players {
				if s.Players[i].RoundWins >= roundsToWin(s.Options.Format) {
					return s.finish(now, "match_won")
				}
			}
			if now >= s.MatchEndsAt {
				return s.finish(now, "time_limit")
			}
			s.Phase = PhasePlaying
			s.resetRound(now)
		}
	case PhaseFinished:
		return nil
	}
	s.ServerAt = now
	return nil
}
