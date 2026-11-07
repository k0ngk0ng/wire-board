package contra

import "errors"

// StepBaseCommonWall recovers fixed-bank type08 cannon and type0A plating.
// Their geometry uses the Boss supertile set; requests share the frame queue.
func (s *EnemySystem) StepBaseCommonWall(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || (c.Level != 1 && c.Level != 3) {
		return errors.New("invalid base common wall context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind == 8 {
		if e.Routine > 8 {
			return errors.New("invalid base wall cannon phase")
		}
		if e.Routine >= 6 {
			return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-6))
		}
		draw := func() (bool, error) {
			e.AnimationDelay = 6
			result, err := s.requestTerrain(slot, e.Frame, c, true)
			return result.Accepted, err
		}
		switch e.Routine {
		case 1:
			e.Data[0] = 8
			e.AnimationDelay = 0x50
			e.advance()
		case 2:
			if !c.AttackEnabled {
				return nil
			}
			e.AnimationDelay--
			if e.AnimationDelay != 0 {
				return nil
			}
			ok, err := draw()
			if err != nil || !ok {
				return err
			}
			if e.Frame < 2 {
				e.Frame++
			} else {
				e.HP = e.Data[0]
				e.AttackDelay = 4
				e.AnimationDelay = 0x40
				e.advance()
			}
		case 3:
			if e.AttackDelay != 0 {
				e.AttackDelay--
				if e.AttackDelay == 0 {
					for i := 2; i >= 0; i-- {
						_, err := s.CreateEnemyShot(c, [...]uint8{0x48, 0x46, 0x44}[i], 7, e.X+[...]uint8{0xf8, 0, 8}[i], e.Y+8)
						if err != nil {
							return err
						}
					}
				}
			}
			e.AnimationDelay--
			if e.AnimationDelay == 0 {
				e.Data[0] = e.HP
				e.HP = 0xf1
				e.AnimationDelay = 6
				e.advance()
			}
		case 4:
			e.AnimationDelay--
			if e.AnimationDelay != 0 {
				return nil
			}
			ok, err := draw()
			if err != nil || !ok {
				return err
			}
			if e.Frame != 0 {
				e.Frame--
			} else {
				e.AnimationDelay = 0xc0
				if c.WeaponStrength >= 2 {
					e.AnimationDelay = 0x60
				}
				e.setRoutine(2)
			}
		case 5:
			result, err := s.requestTerrain(slot, 5, c, false)
			if err != nil {
				return err
			}
			if result.Accepted {
				e.advance()
			}
		}
		return nil
	}
	if e.Kind != 10 || e.Routine > 7 {
		return errors.New("invalid base wall plating phase")
	}
	if e.Routine >= 5 {
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-5))
	}
	switch e.Routine {
	case 1:
		e.AnimationDelay = 0x80
		e.advance()
	case 2:
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		e.AnimationDelay = 4
		result, err := s.requestTerrain(slot, e.Frame+3, c, true)
		if err != nil || !result.Accepted {
			return err
		}
		e.Frame++
		if e.Frame >= 2 {
			e.HP = 10
			e.advance()
		}
	case 3:
		return nil
	case 4:
		result, err := s.requestTerrain(slot, 5, c, false)
		if err != nil {
			return err
		}
		if result.Accepted {
			s.Base.PlatingDestroyed++
			e.advance()
		}
	}
	return nil
}
