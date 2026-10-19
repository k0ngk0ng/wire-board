package battlecity

import "math"

func (s *State) allSelected() bool {
	for i := range s.Tanks {
		if !s.Tanks[i].Selected {
			return false
		}
	}
	return true
}

func (s *State) autoSelect(at int64) {
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if t.Selected {
			continue
		}
		t.Class = tankOrder[s.randN(len(tankOrder))]
		t.Selected = true
		t.MaxHP = s.maxHP(t)
		t.HP = t.MaxHP
		t.IdleAt = at + IdleMS
		s.emit("select", t.Seat, t.Team, at, t.X, t.Y, 0, t.Class, 0)
	}
}

func (s *State) beginCountdown(now int64) {
	if s.Phase != PhaseSelect {
		return
	}
	if !s.allSelected() {
		s.autoSelect(now)
	}
	s.Phase = PhaseCountdown
	s.SelectUntil = 0
	s.StartAt = now + Countdown
	s.ServerAt = now
	s.emit("countdown", -1, -1, now, 0, 0, 0, "", 0)
	s.log("坦克已选择，3 秒后开战")
}

func (s *State) beginPlaying(at int64) {
	if s.Phase != PhaseCountdown {
		return
	}
	s.Phase = PhasePlaying
	s.StartAt = at
	s.ServerAt = at
	s.MatchEndsAt = at + MatchMS
	s.Tick = 0
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if t.Class == "" {
			t.Class = "balanced"
			t.Selected = true
		}
		t.Alive = false
		t.MaxHP = s.maxHP(t)
		t.HP = t.MaxHP
		t.Tech = 0
		t.ShieldUntil = 0
		t.FreezeUntil = 0
		t.RespawnAt = 0
		t.Pending = nil
		t.Applied = s.neutralInput(at)
		t.LastInputAt = at
		t.IdleAt = at + IdleMS
		if !s.trySpawn(t, at, ShieldMS) {
			t.RespawnAt = at + 250
		}
	}
	s.emit("start", -1, -1, at, 0, 0, 0, "", 0)
	s.log("战斗开始")
}

func (s *State) stepEntities(at int64, dt float64) {
	for i := range s.Tanks {
		t := &s.Tanks[i]
		for len(t.Pending) > 0 && t.Pending[0].At <= at {
			t.Applied = t.Pending[0]
			t.Pending = t.Pending[1:]
		}
		s.stepTank(t, at, dt)
	}
	s.stepBullets(at, dt)
	s.pickupPowerUps(at)
	s.tryRespawns(at)
	s.tickShovel(at)
}

func (s *State) evaluateFrame() {
	if s.Result != nil {
		return
	}
	a, b := s.Bases[0], s.Bases[1]
	if !a.Alive && !b.Alive {
		if a.DestroyTick == b.DestroyTick {
			s.finish(true, -1, "双方大本营同时被摧毁")
			return
		}
		if a.DestroyTick < b.DestroyTick {
			s.finish(false, TeamBottom, "上方大本营被摧毁")
		} else {
			s.finish(false, TeamTop, "下方大本营被摧毁")
		}
		return
	}
	if !a.Alive {
		s.finish(false, TeamBottom, "上方大本营被摧毁")
		return
	}
	if !b.Alive {
		s.finish(false, TeamTop, "下方大本营被摧毁")
	}
}

func (s *State) advance(until int64) {
	if s.Result != nil {
		return
	}
	if s.Phase == PhaseCountdown {
		if until < s.StartAt {
			return
		}
		s.beginPlaying(s.StartAt)
	}
	if s.Phase != PhasePlaying {
		return
	}
	if until > s.MatchEndsAt {
		until = s.MatchEndsAt
	}
	for s.Result == nil && s.ServerAt+FrameMS <= until {
		s.Tick++
		for step := 0; step < 5; step++ {
			at := s.ServerAt + StepMS
			s.stepEntities(at, float64(StepMS)/1000)
			s.ServerAt = at
		}
		s.evaluateFrame()
		if s.Result != nil {
			return
		}
		if s.ServerAt >= s.MatchEndsAt {
			s.finish(true, -1, "时间到，双方大本营均未被摧毁")
			return
		}
	}
	if s.Result == nil && s.ServerAt >= s.MatchEndsAt {
		s.finish(true, -1, "时间到，双方大本营均未被摧毁")
	}
}

func (s *State) onTimer(now int64) {
	if s.Result != nil || s.Phase == PhaseFinished {
		return
	}
	switch s.Phase {
	case PhaseSelect:
		if now >= s.SelectUntil {
			s.autoSelect(now)
			s.beginCountdown(now)
			return
		}
		if now > s.ServerAt {
			s.ServerAt = now
		}
	case PhaseCountdown:
		if now >= s.StartAt {
			s.beginPlaying(s.StartAt)
			s.advance(now)
		}
	case PhasePlaying:
		target := now
		if target > s.MatchEndsAt {
			target = s.MatchEndsAt
		}
		s.advance(target)
		if s.Result == nil && s.ServerAt >= s.MatchEndsAt {
			s.finish(true, -1, "时间到，双方大本营均未被摧毁")
		}
	}
}

func (s *State) neutralInput(at int64) Input {
	return Input{At: at, Move: "none", Fire: false}
}

func (s *State) applyInput(t *Tank, move string, fire bool, at int64) {
	if move == "" {
		move = "none"
	}
	in := Input{At: at, Move: move, Fire: fire}
	if len(t.Pending) > 0 && t.Pending[len(t.Pending)-1].At == at {
		t.Pending[len(t.Pending)-1] = in
	} else {
		t.Pending = append(t.Pending, in)
	}
	if len(t.Pending) > MaxPending {
		t.Pending = t.Pending[len(t.Pending)-MaxPending:]
	}
	t.LastInputAt = at
	t.IdleAt = at + IdleMS
}

func (s *State) validMove(move string) bool {
	if move == "" || move == "none" {
		return true
	}
	_, ok := parseMove(move)
	return ok
}

func (s *State) baseWinner() int {
	a, b := s.Bases[0].Alive, s.Bases[1].Alive
	if a && !b {
		return TeamTop
	}
	if !a && b {
		return TeamBottom
	}
	return -1
}

var _ = math.Abs
