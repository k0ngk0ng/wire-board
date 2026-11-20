package engine

type projectileTrace struct {
	T      int
	Target int
	Life   int
	Wall   int
	Door   bool
}

func (s *State) historyPlayers() []HistoryPlayer {
	players := make([]HistoryPlayer, len(s.Players))
	for i, p := range s.Players {
		players[i] = HistoryPlayer{X: p.X, Y: p.Y, Alive: p.Alive, Life: p.Life, ProtectedUntil: p.ProtectedUntil}
	}
	return players
}

func (s *State) recordHistory() {
	s.History = append(s.History, History{Frame: s.Frame, Players: s.historyPlayers(), Doors: append([]Door{}, s.Doors...), BrokenGlass: append([]int{}, s.BrokenGlass...)})
	if len(s.History) > 8 {
		copy(s.History, s.History[len(s.History)-8:])
		s.History = s.History[:8]
	}
}

func (s *State) historyAt(frame int64) *History {
	for i := len(s.History) - 1; i >= 0; i-- {
		if s.History[i].Frame == frame {
			return &s.History[i]
		}
	}
	return nil
}

func traceProjectile(world Map, doors []Door, broken []int, players []HistoryPlayer, bullet Bullet, frame int64) projectileTrace {
	trace := projectileTrace{T: 65537, Target: -1, Wall: -1}
	tx, ty := bullet.X+bullet.DX, bullet.Y+bullet.DY
	for i, wall := range world.Walls {
		if wall.Kind == "glass" && hasInt(broken, i) {
			continue
		}
		if t, hit := segmentRect(bullet.X, bullet.Y, tx, ty, wall); hit && t < trace.T {
			trace.T = t
			trace.Wall = i
		}
	}
	for _, door := range world.Doors {
		if doorOpen(doors, door.ID) {
			continue
		}
		if t, hit := segmentRect(bullet.X, bullet.Y, tx, ty, Rect{X: door.X, Y: door.Y, W: door.W, H: door.H}); hit && t < trace.T {
			trace.T = t
			trace.Wall = -1
			trace.Door = true
		}
	}
	for seat, player := range players {
		if !player.Alive || seat == bullet.Owner || player.ProtectedUntil > frame {
			continue
		}
		if t, hit := segmentCircle(bullet.X, bullet.Y, tx, ty, player.X, player.Y, Radius); hit && t < trace.T {
			trace.T = t
			trace.Target = seat
			trace.Life = player.Life
		}
	}
	return trace
}

func (s *State) moveProjectile(bullet Bullet, trace projectileTrace) (Bullet, bool) {
	world := s.world()
	bullet.PrevX, bullet.PrevY = bullet.X, bullet.Y
	expired := trace.T <= 65536
	if expired {
		t := max(0, trace.T-64)
		bullet.X += bullet.DX * t / 65536
		bullet.Y += bullet.DY * t / 65536
		if trace.Target < 0 && trace.Wall >= 0 && world.Walls[trace.Wall].Kind == "glass" && !hasInt(s.BrokenGlass, trace.Wall) {
			s.BrokenGlass = append(s.BrokenGlass, trace.Wall)
			s.emit("glass", bullet.Owner, -1, bullet.X, bullet.Y, bullet.Weapon)
		}
		if trace.Target < 0 && !trace.Door {
			s.emit("impact", bullet.Owner, -1, bullet.X, bullet.Y, bullet.Weapon)
		}
	} else {
		bullet.X += bullet.DX
		bullet.Y += bullet.DY
	}
	bullet.Remaining -= int(sqrt(int64(bullet.DX)*int64(bullet.DX) + int64(bullet.DY)*int64(bullet.DY)))
	if bullet.Remaining <= 0 || bullet.X < 0 || bullet.Y < 0 || bullet.X > world.Width || bullet.Y > world.Height {
		expired = true
	}
	return bullet, expired
}

func projectileDamage(bullet Bullet, trace projectileTrace) damage {
	return damage{Target: trace.Target, Owner: bullet.Owner, Life: trace.Life, Weapon: bullet.Weapon, Knock: bullet.Thrown && bullet.Weapon != "knife" && bullet.Weapon != "katana"}
}

func (s *State) compensateProjectile(bullet Bullet, viewFrame int64) (Bullet, bool, []damage) {
	if viewFrame <= 0 || viewFrame >= s.Frame {
		return bullet, true, nil
	}
	start := max(viewFrame, s.Frame-6)
	past := s.historyAt(start)
	if past == nil || bullet.Owner < 0 || bullet.Owner >= len(past.Players) || !past.Players[bullet.Owner].Alive || past.Players[bullet.Owner].Life != bullet.Life {
		return bullet, true, nil
	}
	bullet.X, bullet.Y = past.Players[bullet.Owner].X, past.Players[bullet.Owner].Y
	for frame := start + 1; frame < s.Frame; frame++ {
		past = s.historyAt(frame)
		if past == nil {
			return bullet, false, nil
		}
		trace := traceProjectile(s.world(), past.Doors, past.BrokenGlass, past.Players, bullet, frame)
		var expired bool
		bullet, expired = s.moveProjectile(bullet, trace)
		if trace.Target >= 0 {
			return bullet, false, []damage{projectileDamage(bullet, trace)}
		}
		if expired {
			return bullet, false, nil
		}
	}
	return bullet, true, nil
}
