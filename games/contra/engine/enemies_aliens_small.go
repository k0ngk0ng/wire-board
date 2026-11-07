package contra

import "errors"

func alienState(s *EnemySystem, slot int, c EnemyStepContext, kind, maxRoutine uint8) (*Enemy, error) {
	if slot < 0 || slot >= 16 || c.Level != 7 {
		return nil, errors.New("invalid alien context")
	}
	e := &s.Slots[slot]
	if e.Routine != 0 && (e.Kind != kind || e.Routine > maxRoutine) {
		return nil, errors.New("unsupported alien state")
	}
	return e, nil
}

func (s *EnemySystem) StepAlienMouth(slot int, c EnemyStepContext) error {
	e, err := alienState(s, slot, c, 18, 6)
	if err != nil || e.Routine == 0 {
		return err
	}
	switch e.Routine {
	case 1:
		e.HP = (c.WeaponStrength + 4) + (c.GameLoop << 1) + (c.GameLoop >> 7)
		e.Data[2] = 32
		if _, err = s.requestTerrain(slot, 1, c, false); err != nil {
			return err
		}
		e.AnimationDelay = 10
		e.advance()
	case 2:
		e.scrollOnly(c)
		if e.X < 32 {
			return nil
		}
		if c.AttackEnabled {
			e.AnimationDelay--
			if e.AnimationDelay == 0 {
				child, err := s.spawnWaterfallRelative(slot, 19, 0, 0, c.Level)
				if err != nil {
					return err
				}
				e.AnimationDelay = (c.Random & 31) + 0xc0
				if child < 0 {
					e.AnimationDelay++
				}
			}
		}
		e.Data[2]--
		if e.Data[2] == 0 {
			r, err := s.requestTerrain(slot, e.Data[3]&1, c, false)
			if err != nil {
				return err
			}
			e.Data[2] = 1
			if r.Accepted {
				e.Data[3]++
				e.Data[2] = 32
			}
		}
	case 3:
		r, err := s.requestTerrain(slot, 2, c, false)
		if err != nil {
			return err
		}
		if r.Accepted {
			e.advance()
		}
	default:
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-4))
	}
	return nil
}

func (s *EnemySystem) StepAlienSpawner(slot int, c EnemyStepContext) error {
	e, err := alienState(s, slot, c, 21, 6)
	if err != nil || e.Routine == 0 {
		return err
	}
	switch e.Routine {
	case 1:
		a := uint16(c.WeaponStrength<<1) + uint16(c.GameLoop<<1) + uint16(c.WeaponStrength>>7)
		hp := uint16(uint8(a)) + 24 + (a >> 8)
		e.HP = uint8(hp)
		r := uint16(c.Random) + uint16(c.Frame) + (hp >> 8)
		e.AnimationDelay = (uint8(r) & 63) + 0xa0 + uint8(r>>8)
		e.Data[0] = 0xa5
		if e.Y < 128 {
			e.Data[0] = 0xa1
		}
		e.Routine++
	case 2:
		e.scrollOnly(c)
		if e.Data[3] == 0 {
			e.AnimationDelay--
			switch e.AnimationDelay {
			case 0:
				return s.alienSpawnSpider(slot, c)
			case 0x30, 0x10:
				return s.animateAlienSpawner(slot, c)
			}
		} else {
			e.AnimationDelay--
			if e.AnimationDelay == 0 {
				e.AnimationDelay = 20
				if err = s.animateAlienSpawner(slot, c); err != nil {
					return err
				}
				e.Data[3]--
				if e.Data[3] == 0 {
					return s.alienSpawnSpider(slot, c)
				}
			}
		}
	case 3:
		code := uint8(0xa8)
		if e.Y < 128 {
			code = 0xa4
		}
		r, err := s.requestTerrain(slot, code, c, false)
		if err != nil {
			return err
		}
		if r.Accepted {
			e.advance()
		}
	default:
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-4))
	}
	return nil
}

func (s *EnemySystem) animateAlienSpawner(slot int, c EnemyStepContext) error {
	e := &s.Slots[slot]
	e.Data[0]++
	e.Data[2]++
	if e.Data[2] >= 2 {
		e.Data[0] -= 2
		e.Data[2] -= 2
	}
	_, err := s.requestTerrain(slot, e.Data[0], c, false)
	return err
}

func (s *EnemySystem) alienSpawnSpider(slot int, c EnemyStepContext) error {
	e := &s.Slots[slot]
	if c.AttackEnabled {
		child, err := s.spawnWaterfallRelative(slot, 20, 0, 0, c.Level)
		if err != nil {
			return err
		}
		if child >= 0 {
			s.Slots[child].Routine = 2
		}
	}
	base := uint8(10) - ((c.PlayerWeapons[0] | c.PlayerWeapons[1]) & 7)
	r := uint16(c.Frame) + uint16(c.Random) + 1
	e.Data[3] = base + (uint8(r) & 3) + uint8(r>>8)
	e.AnimationDelay = 20
	return nil
}
