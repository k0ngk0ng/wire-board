package engine

type damage struct {
	Target int
	Owner  int
	Life   int
	Weapon string
	Knock  bool
}

func (s *State) attack(seat int, viewFrame int64) []damage {
	p := &s.Players[seat]
	p.ProtectedUntil = 0
	if s.Frame < p.NextAttackFrame {
		return nil
	}
	weapon, ok := WeaponByID(p.Weapon)
	if !ok {
		weapon = Weapon{ID: "", Cooldown: 22, Range: 25 * Unit, Melee: true}
	}
	if !weapon.Melee && p.Ammo <= 0 {
		return nil
	}
	p.NextAttackFrame = s.Frame + weapon.Cooldown
	s.emit("fire", seat, -1, p.X, p.Y, p.Weapon)
	if weapon.Melee {
		hits := []damage{}
		dx, dy := Direction(p.Aim)
		for _, other := range s.Players {
			if !other.Alive || other.Seat == seat || other.ProtectedUntil > s.Frame {
				continue
			}
			vx, vy := other.X-p.X, other.Y-p.Y
			dot := int64(dx)*int64(vx) + int64(dy)*int64(vy)
			cross := int64(dx)*int64(vy) - int64(dy)*int64(vx)
			if cross < 0 {
				cross = -cross
			}
			if dot <= 0 || cross > dot || distance2(p.X, p.Y, other.X, other.Y) > int64(weapon.Range+Radius)*int64(weapon.Range+Radius) || !Visible(s.world(), s.Doors, s.BrokenGlass, p.X, p.Y, other.X, other.Y) {
				continue
			}
			hits = append(hits, damage{Target: other.Seat, Owner: seat, Life: other.Life, Weapon: p.Weapon, Knock: p.Weapon == ""})
		}
		return hits
	}
	p.Ammo--
	hits := []damage{}
	for pellet := 0; pellet < weapon.Pellets; pellet++ {
		spread := 0
		if weapon.Spread > 0 {
			spread = int(s.random()%uint64(weapon.Spread*2+1)) - weapon.Spread
		}
		dx, dy := Direction((p.Aim + spread + 1024) % 1024)
		if len(s.Bullets) < MaxBullets {
			bullet := Bullet{ID: s.nextID(), Owner: seat, Life: p.Life, X: p.X, Y: p.Y, DX: dx * weapon.Speed / 1024, DY: dy * weapon.Speed / 1024, Remaining: weapon.Range, Weapon: weapon.ID}
			bullet, active, compensated := s.compensateProjectile(bullet, viewFrame)
			hits = append(hits, compensated...)
			if active {
				s.Bullets = append(s.Bullets, bullet)
			}
		}
	}
	return hits
}

func (s *State) throw(seat int) {
	p := &s.Players[seat]
	p.ProtectedUntil = 0
	if p.Weapon == "" || s.Frame < p.NextAttackFrame || len(s.Bullets) >= MaxBullets {
		return
	}
	dx, dy := Direction(p.Aim)
	s.Bullets = append(s.Bullets, Bullet{ID: s.nextID(), Owner: seat, Life: p.Life, X: p.X, Y: p.Y, DX: dx * 10 * Unit / 1024, DY: dy * 10 * Unit / 1024, Remaining: 190 * Unit, Weapon: p.Weapon, Ammo: p.Ammo, Thrown: true})
	s.emit("throw", seat, -1, p.X, p.Y, p.Weapon)
	p.Weapon = ""
	p.Ammo = 0
	p.NextAttackFrame = s.Frame + 12
}

func (s *State) pickup(seat int) {
	p := &s.Players[seat]
	best := -1
	distance := int64(42*Unit) * int64(42*Unit)
	for i, drop := range s.Drops {
		d := distance2(p.X, p.Y, drop.X, drop.Y)
		if d <= distance && Visible(s.world(), s.Doors, s.BrokenGlass, p.X, p.Y, drop.X, drop.Y) {
			best = i
			distance = d
		}
	}
	if best < 0 {
		return
	}
	drop := s.Drops[best]
	s.Drops = append(s.Drops[:best], s.Drops[best+1:]...)
	if drop.SpawnID > 0 && drop.SpawnID <= len(s.WeaponTimers) {
		s.WeaponTimers[drop.SpawnID-1] = s.Frame + 600
	}
	old := Drop{X: p.X, Y: p.Y, Weapon: p.Weapon, Ammo: p.Ammo}
	p.Weapon, p.Ammo = drop.Weapon, drop.Ammo
	s.addDrop(old)
	s.emit("pickup", seat, -1, p.X, p.Y, p.Weapon)
}

func (s *State) finish(seat int) []damage {
	p := &s.Players[seat]
	if s.Frame < p.NextAttackFrame {
		return nil
	}
	for _, other := range s.Players {
		if other.Seat == seat || !other.Alive || other.DownUntil <= s.Frame || other.ProtectedUntil > s.Frame || distance2(p.X, p.Y, other.X, other.Y) > int64(40*Unit)*int64(40*Unit) || !Visible(s.world(), s.Doors, s.BrokenGlass, p.X, p.Y, other.X, other.Y) {
			continue
		}
		p.NextAttackFrame = s.Frame + 24
		p.BusyUntil = s.Frame + 24
		p.ProtectedUntil = 0
		s.emit("finish", seat, other.Seat, other.X, other.Y, p.Weapon)
		return []damage{{Target: other.Seat, Owner: seat, Life: other.Life, Weapon: "finish"}}
	}
	return nil
}

func (s *State) interact(seat int) {
	p := s.Players[seat]
	for i := range s.Doors {
		for _, definition := range s.world().Doors {
			if definition.ID != s.Doors[i].ID || distance2(p.X, p.Y, definition.X+definition.W/2, definition.Y+definition.H/2) > int64(56*Unit)*int64(56*Unit) {
				continue
			}
			if s.Doors[i].Open {
				occupied := false
				for _, other := range s.Players {
					if other.Alive && circleRect(other.X, other.Y, Radius, Rect{X: definition.X, Y: definition.Y, W: definition.W, H: definition.H}) {
						occupied = true
						break
					}
				}
				if occupied {
					continue
				}
			}
			s.Doors[i].Open = !s.Doors[i].Open
			s.emit("door", seat, -1, definition.X, definition.Y, "")
			return
		}
	}
}

func (s *State) advanceBullets() []damage {
	hits := []damage{}
	remaining := make([]Bullet, 0, len(s.Bullets))
	players := s.historyPlayers()
	for _, bullet := range s.Bullets {
		trace := traceProjectile(s.world(), s.Doors, s.BrokenGlass, players, bullet, s.Frame)
		var expired bool
		bullet, expired = s.moveProjectile(bullet, trace)
		if trace.Target >= 0 {
			hits = append(hits, projectileDamage(bullet, trace))
		}
		if expired {
			if bullet.Thrown {
				s.addDrop(Drop{X: bullet.X, Y: bullet.Y, Weapon: bullet.Weapon, Ammo: bullet.Ammo})
			}
		} else {
			remaining = append(remaining, bullet)
		}
	}
	s.Bullets = remaining
	return hits
}

func (s *State) settle(hits []damage) {
	dead := make(map[int]damage)
	for _, hit := range hits {
		if hit.Target < 0 || hit.Target >= len(s.Players) {
			continue
		}
		p := &s.Players[hit.Target]
		if !p.Alive || p.Life != hit.Life || p.ProtectedUntil > s.Frame {
			continue
		}
		if hit.Knock {
			p.DownUntil = max(p.DownUntil, s.Frame+54)
			s.emit("down", hit.Owner, hit.Target, p.X, p.Y, hit.Weapon)
		} else if _, found := dead[hit.Target]; !found {
			dead[hit.Target] = hit
		}
	}
	for seat := range s.Players {
		hit, found := dead[seat]
		if !found {
			continue
		}
		p := &s.Players[seat]
		s.addDrop(Drop{X: p.X, Y: p.Y, Weapon: p.Weapon, Ammo: p.Ammo})
		s.Corpses = append(s.Corpses, Corpse{X: p.X, Y: p.Y, Aim: p.Aim, Character: p.Character, Frame: s.Frame})
		if len(s.Corpses) > MaxCorpses {
			s.Corpses = s.Corpses[len(s.Corpses)-MaxCorpses:]
		}
		p.Alive = false
		p.Deaths++
		p.RespawnFrame = s.Frame + 120
		p.DownUntil = 0
		p.ProtectedUntil = 0
		p.Weapon = ""
		p.Ammo = 0
		ClearInput(s, seat)
		if hit.Owner == seat {
			p.Score--
		} else if hit.Owner >= 0 && hit.Owner < len(s.Players) {
			s.Players[hit.Owner].Score++
			s.Players[hit.Owner].Kills++
		}
		s.emit("kill", hit.Owner, seat, p.X, p.Y, hit.Weapon)
	}
}
