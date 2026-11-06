package engine

import "errors"

func Step(s *State, ticks int) error {
	if ticks < 0 || ticks > 24000 {
		return errors.New("仿真步数超出范围")
	}
	for range ticks {
		if s.Phase == "finished" {
			break
		}
		s.Tick++
		switch s.Phase {
		case "intermission":
			if s.Tick >= s.PhaseEndTick {
				s.Round = 2
				s.beginRound()
			}
			continue
		case "prepare":
			if s.Tick >= s.PhaseEndTick {
				s.Phase = "battle"
				s.BattleStartTick = s.Tick
				s.PhaseEndTick = s.Tick + int64(s.Options.RoundSeconds*1000/TickMS)
				s.NextBrainTick = s.Tick + 40
				s.emit("battle", 0, 0, -1, "", 0, 0)
			}
		}
		s.income()
		s.plantStep()
		if s.Phase == "battle" {
			s.projectileStep()
			s.zombieStep()
			s.mowerStep()
			s.removeDead()
			if s.Phase == "battle" {
				for _, z := range s.Zombies {
					if z.X < 0 && s.Mowers[z.Row].State != "ready" {
						s.endRound(true)
						break
					}
				}
				// The last legal tick includes impacts and house crossings. A
				// defense timeout is considered only after that tick resolves.
				if s.Phase == "battle" && s.Tick >= s.PhaseEndTick {
					s.endRound(false)
				}
			}
		}
	}
	return nil
}
func (s *State) income() {
	if s.Tick >= s.NextSunTick {
		s.creditSide("plants", 25)
		s.NextSunTick += 100
	}
	if s.Phase == "battle" && s.Tick >= s.NextBrainTick {
		age := (s.Tick - s.BattleStartTick) * TickMS / 60000
		amount := 25 + min(int(age), 4)*5
		s.creditSide("zombies", amount)
		s.NextBrainTick += 40
	}
}
func (s *State) creditSide(side string, amount int) {
	ids := []int{}
	for p := range s.Players {
		if s.Players[p].Side == side {
			ids = append(ids, p)
		}
	}
	if len(ids) == 0 {
		return
	}
	team := s.Players[ids[0]].Team
	for i, p := range ids {
		n := amount / len(ids)
		if (i-s.IncomeCursor[team]+len(ids))%len(ids) < amount%len(ids) {
			n++
		}
		s.Players[p].Money = min(MaxMoney, s.Players[p].Money+n)
	}
	s.IncomeCursor[team] = (s.IncomeCursor[team] + amount%len(ids)) % len(ids)
}
func (s *State) plantStep() {
	for i := range s.Plants {
		p := &s.Plants[i]
		if p.HP <= 0 {
			continue
		}
		x := PlantX(*p)
		if p.Kind == "sunflower" && s.Tick >= p.ReadyAt {
			s.Players[p.Owner].Money = min(MaxMoney, s.Players[p.Owner].Money+25)
			p.ReadyMS += int64((2350 + s.random(151)) * 10)
			p.ReadyAt = counterTick(p.ReadyMS)
			s.emit("sun", p.Row, x, p.Owner, p.Kind, p.ID, 25)
		}
		if s.Phase != "battle" {
			continue
		}
		switch p.Kind {
		case "cherrybomb":
			if s.Tick >= p.ReadyAt {
				row := p.Row
				owner := p.Owner
				kind := p.Kind
				id := p.ID
				p.HP = 0
				s.blast(row, x, 1450, 1, 1800, owner, kind, id)
			}
		case "potatomine":
			if s.Tick >= p.ReadyAt {
				for _, z := range s.Zombies {
					if z.HP > 0 && z.Row == p.Row && abs(z.X-x) < 450 {
						row := p.Row
						owner := p.Owner
						id := p.ID
						p.HP = 0
						s.blast(row, x, 600, 0, 1800, owner, "potatomine", id)
						break
					}
				}
			}
		case "peashooter", "snowpea", "repeater":
			if p.BurstAt > 0 && s.Tick >= p.BurstAt {
				s.shoot(*p)
				p.BurstAt = 0
			}
			if s.Tick >= p.AttackAt {
				target := false
				for _, z := range s.Zombies {
					if z.HP > 0 && z.Row == p.Row && z.X >= x-200 {
						target = true
						break
					}
				}
				if target {
					s.shoot(*p)
					base := p.AttackMS
					if base < s.Tick*TickMS-TickMS {
						base = s.Tick * TickMS
					}
					p.AttackMS = base + int64((136+s.random(15))*10)
					p.AttackAt = counterTick(p.AttackMS)
					if p.Kind == "repeater" {
						p.BurstAt = s.Tick + 5
					}
				}
			}
		}
	}
	s.removeDead()
}
func (s *State) shoot(p Plant) {
	if len(s.Projectiles) >= MaxProjectiles {
		return
	}
	kind := "pea"
	if p.Kind == "snowpea" {
		kind = "icepea"
	}
	x := PlantX(p) + 200
	s.Projectiles = append(s.Projectiles, Projectile{ID: s.id(), Owner: p.Owner, Row: p.Row, X: x, Kind: kind, Damage: 20})
	s.emit("shoot", p.Row, x, p.Owner, kind, p.ID, 0)
}
func (s *State) projectileStep() {
	live := s.Projectiles[:0]
	for _, p := range s.Projectiles {
		old := p.X
		p.X += 250
		hit := -1
		nearest := 100000
		for i, z := range s.Zombies {
			if z.HP <= 0 || z.Row != p.Row || z.State == "vaulting" {
				continue
			}
			if z.X+180 >= old && z.X-180 <= p.X && z.X < nearest {
				hit = i
				nearest = z.X
			}
		}
		if hit >= 0 {
			z := &s.Zombies[hit]
			s.damage(z, p.Damage, p.Kind == "icepea")
			s.emit("hit", p.Row, z.X, p.Owner, p.Kind, z.ID, p.Damage)
		} else if p.X < 10000 {
			live = append(live, p)
		}
	}
	s.Projectiles = live
}
func (s *State) damage(z *Zombie, amount int, ice bool) {
	if z.HP <= 0 {
		return
	}
	if ice {
		z.SlowUntil = max(z.SlowUntil, s.Tick+200)
	}
	if z.Armor > 0 {
		armorDamage := min(amount, z.Armor)
		z.Armor -= armorDamage
		amount -= armorDamage
		if z.Armor == 0 {
			s.emit("armor", z.Row, z.X, z.Owner, z.Kind, z.ID, 0)
			if z.Kind == "newspaper" {
				z.State = "angry"
			}
		}
	}
	z.HP -= amount
}
func (s *State) blast(row, x, radius, rows, damage, owner int, kind string, target int) {
	for i := range s.Zombies {
		z := &s.Zombies[i]
		if abs(z.Row-row) <= rows && abs(z.X-x) <= radius {
			s.damage(z, damage, false)
		}
	}
	s.emit("explosion", row, x, owner, kind, target, damage)
}
func (s *State) zombieStep() {
	for i := range s.Zombies {
		z := &s.Zombies[i]
		if z.HP <= 0 {
			continue
		}
		if z.State == "vaulting" {
			if s.Tick >= z.VaultUntil {
				z.X = z.VaultX
				z.State = "walking"
			}
			continue
		}
		moveMilli := z.SpeedMilli
		if z.Kind == "polevaulter" && !z.Vaulted || z.Kind == "newspaper" && z.Armor == 0 {
			moveMilli = z.SpecialSpeedMilli
		}
		slow := z.SlowUntil > s.Tick
		if slow {
			moveMilli /= 2
		}
		z.MoveRemainder += moveMilli
		speed := z.MoveRemainder / 1000
		z.MoveRemainder %= 1000
		next := z.X - speed
		at := -1
		closest := -10000
		for j, p := range s.Plants {
			if p.HP <= 0 || p.Row != z.Row {
				continue
			}
			x := PlantX(p)
			if z.X >= x-250 && next <= x+450 && x > closest {
				at = j
				closest = x
			}
		}
		if at >= 0 {
			p := &s.Plants[at]
			if z.Kind == "polevaulter" && !z.Vaulted {
				z.Vaulted = true
				if p.Kind != "tallnut" {
					z.State = "vaulting"
					z.VaultUntil = s.Tick + 12
					z.VaultX = PlantX(*p) - 650
					s.emit("vault", z.Row, z.X, z.Owner, z.Kind, z.ID, 0)
					continue
				}
			}
			z.X = max(z.X, PlantX(*p)+400)
			z.State = "eating"
			// The PC game applies 4 HP each 40 ms (80 ms when chilled).
			// Carry milliseconds across our 50 ms steps instead of lumpy 50 HP bites.
			interval := 40
			if slow {
				interval = 80
			}
			z.BiteCarryMS += TickMS
			damage := (z.BiteCarryMS / interval) * 4
			z.BiteCarryMS %= interval
			z.BiteAt = s.Tick + counterTick(int64(interval-z.BiteCarryMS))
			if damage > 0 {
				p.HP -= damage
				s.emit("bite", z.Row, z.X, z.Owner, z.Kind, p.ID, damage)
			}
		} else {
			z.X = next
			z.State = "walking"
			if z.Kind == "newspaper" && z.Armor == 0 {
				z.State = "angry"
			}
		}
		if z.X <= 150 && s.Mowers[z.Row].State == "ready" {
			s.Mowers[z.Row].State = "moving"
			s.Mowers[z.Row].X = -150
			s.emit("mower", z.Row, -150, -1, "", 0, 0)
		}
	}
}
func (s *State) mowerStep() {
	for row := range s.Mowers {
		m := &s.Mowers[row]
		if m.State != "moving" {
			continue
		}
		old := m.X
		m.X += 300
		for i := range s.Zombies {
			z := &s.Zombies[i]
			if z.Row == row && z.HP > 0 && z.X >= old-350 && z.X <= m.X+350 {
				z.HP = 0
			}
		}
		if m.X > 10200 {
			m.State = "spent"
		}
	}
}
func (s *State) removeDead() {
	plants := s.Plants[:0]
	for _, p := range s.Plants {
		if p.HP > 0 {
			plants = append(plants, p)
		} else {
			s.emit("plant-death", p.Row, PlantX(p), p.Owner, p.Kind, p.ID, 0)
		}
	}
	s.Plants = plants
	zombies := s.Zombies[:0]
	for _, z := range s.Zombies {
		if z.HP > 0 {
			zombies = append(zombies, z)
		} else {
			s.emit("zombie-death", z.Row, z.X, z.Owner, z.Kind, z.ID, 0)
		}
	}
	s.Zombies = zombies
}
func (s *State) endRound(breached bool) {
	s.Rounds = append(s.Rounds, RoundResult{AttackTeam: 1 - (s.Round-1)%2, Breached: breached, ElapsedMS: (s.Tick - s.BattleStartTick) * TickMS})
	s.emit("round-end", 0, 0, -1, "", 0, 0)
	if s.Round == 1 {
		s.Phase = "intermission"
		s.PhaseEndTick = s.Tick + 100
		return
	}
	s.Phase = "finished"
	s.PhaseEndTick = 0
	s.WinnerTeam = -1
	left, right := s.Rounds[0], s.Rounds[1]
	if left.Breached != right.Breached {
		if left.Breached {
			s.WinnerTeam = left.AttackTeam
		} else {
			s.WinnerTeam = right.AttackTeam
		}
	} else if left.Breached && left.ElapsedMS != right.ElapsedMS {
		if left.ElapsedMS < right.ElapsedMS {
			s.WinnerTeam = left.AttackTeam
		} else {
			s.WinnerTeam = right.AttackTeam
		}
	}
	s.emit("finished", 0, 0, -1, "", 0, s.WinnerTeam)
}
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
