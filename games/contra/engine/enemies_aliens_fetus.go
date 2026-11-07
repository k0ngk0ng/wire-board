package contra

import "errors"

// StepAlienFetus recovers Japanese B711/B75B; direction is one of twelve
// original table headings, with one-step homing and a shared aim-timer stream.
func (s *EnemySystem) StepAlienFetus(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= 16 || c.Level != 7 {
		return errors.New("invalid alien fetus context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 17 || e.Routine > 5 {
		return errors.New("unsupported alien fetus state")
	}
	switch e.Routine {
	case 1:
		if err := s.alienAimTimer(e); err != nil {
			return err
		}
		e.Data[2] <<= 1
		hp := uint16(c.GameLoop) + 2
		e.HP = uint8(hp)
		e.Sprite = 0xac
		e.AnimationDelay = 6
		if c.Players[1].GameOver == 0 {
			e.Data[3] = (uint8(uint16(c.Random)+uint16(c.Frame)+(hp>>8)) & 31) + 14
			if c.Players[0].GameOver != 0 {
				e.Data[3] = 1
			}
		}
		n := c.Random & 3
		if n == 0 {
			n = 3
		}
		e.Data[0] = n * 2
		if e.Attributes != 0 {
			e.Data[0] = 6
		}
		e.Routine++
		e.setAlienVelocity(int(e.Data[0]) * 2)
		e.Data[3] -= 3
	case 2:
		if e.Data[0] >= 12 {
			return errors.New("invalid fetus heading")
		}
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.AnimationDelay = 6
			n := e.Data[0] + 1
			if n == 12 {
				n = 0
			}
			offset := (n / 3) * 2
			e.Data[1] ^= 1
			e.SpriteAttributes &= 0x3f
			if offset >= 4 {
				e.SpriteAttributes |= 0xc0
				offset -= 4
			}
			e.Sprite = 0xac + offset + e.Data[1]
		}
		e.Data[2]--
		if e.Data[2] == 0 && e.Data[3]&0x3e != 0 {
			if err := s.alienAimTimer(e); err != nil {
				return err
			}
			if _, err := s.turnAlienToward(e, c, e.Data[3]&1, 0); err != nil {
				return err
			}
			e.setAlienVelocity(int(e.Data[0]) * 2)
			e.Data[3] -= 3
		}
		e.moveWithScroll(c)
	default:
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-3))
	}
	return nil
}

func (s *EnemySystem) alienAimTimer(e *Enemy) error {
	if int(s.AlienAimIndex) > len(alienAimTimers) {
		return errors.New("invalid alien aim timer cursor")
	}
	if int(s.AlienAimIndex) == len(alienAimTimers) {
		s.AlienAimIndex = 0
	}
	e.Data[2] = alienAimTimers[s.AlienAimIndex]
	s.AlienAimIndex++
	return nil
}

func (e *Enemy) setAlienVelocity(index int) {
	e.VelocityY = alienVelocities[index]
	e.VelocityX = alienVelocities[index+6]
}

// F386/F380 uses the same discrete aim lookup and chooses decrement for an
// exact half-circle. It reports alignment after applying at most one turn.
func (s *EnemySystem) turnAlienToward(e *Enemy, c EnemyStepContext, actor, precision uint8) (bool, error) {
	angle, err := shooterAimAtPlayer(c, int(actor), e.X, e.Y, precision)
	if err != nil {
		return false, err
	}
	full := uint8(12) * (precision + 1)
	s.recordGenerationBias(full)
	if e.Data[0] == angle {
		return true, nil
	}
	delta := (int(angle) - int(e.Data[0]) + int(full)) % int(full)
	if delta < int(full/2) {
		e.Data[0]++
		if e.Data[0] >= full {
			e.Data[0] = 0
		}
	} else {
		if e.Data[0] == 0 {
			e.Data[0] = full
		}
		e.Data[0]--
	}
	return e.Data[0] == angle, nil
}
