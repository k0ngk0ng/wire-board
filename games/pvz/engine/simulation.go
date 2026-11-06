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
		motion := captureMotion(s)
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
		s.worldStep()
		s.income()
		s.plantStep()
		if s.Phase == "battle" {
			s.projectileStep()
			s.zombieStep()
			s.mowerStep()
			s.removeDead()
			if s.Phase == "battle" {
				for _, z := range s.Zombies {
					if s.canBreach(z) {
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
		motion.finish(s)
	}
	return nil
}
func (s *State) income() {
	if s.Tick >= s.NextSunTick {
		if !s.Map.Night {
			s.creditSide("plants", 25)
		}
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
func (s *State) damage(z *Zombie, amount int, ice bool) {
	if z.HP <= 0 {
		return
	}
	if ice && CanChillZombie(*z) {
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
			if z.Row == row && z.HP > 0 && z.Allegiance != "plants" && z.Altitude == 0 && !z.Digging && z.Kind != "bungee" && z.Kind != "zomboss" && z.X >= old-350 && z.X <= m.X+350 {
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
		if z.HP > 0 && z.X <= 11000 {
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
