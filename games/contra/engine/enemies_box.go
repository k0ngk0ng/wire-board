package contra

import "errors"

// StepWeaponBox retains the original visible-tile/HP handshake. Rejected
// requests retry later; a box is not opened or erased before admission.
func (s *EnemySystem) StepWeaponBox(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level >= 8 {
		return errors.New("invalid weapon box context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 2 {
		return errors.New("wrong weapon box kind")
	}
	switch e.Routine {
	case 1:
		e.Frame = 1
		e.AnimationDelay = 0x20
		e.advance()
	case 2:
		e.scrollOnly(c)
		if !e.boxBoundary(c, 0xf0, 0x30) {
			return nil
		}
		if e.boxBoundary(c, 0x18, 0xc8) {
			e.setRoutine(4)
			return nil
		}
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.AnimationDelay = 8
			e.advance()
		}
	case 3:
		e.scrollOnly(c)
		if e.boxBoundary(c, 0x18, 0xc8) {
			e.setRoutine(4)
			return nil
		}
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		e.AnimationDelay = 8
		if e.Frame > 2 {
			return errors.New("invalid weapon box animation")
		}
		result, err := s.requestTerrain(slot, e.Frame, c, true)
		if err != nil {
			return err
		}
		if !result.Accepted {
			return nil
		}
		if e.Data[1] == 0 {
			if e.Frame < 2 {
				e.Frame++
				return nil
			}
			e.Frame--
			e.HP = 1
			e.Data[1] = 1
		} else {
			if e.Frame != 0 {
				e.Frame--
				return nil
			}
			e.Frame++
			e.HP = 0xf0
			e.Data[1] = 0
		}
		e.AnimationDelay = 0x40
		e.setRoutine(2)
	case 4:
		e.scrollOnly(c)
		r, err := s.requestTerrain(slot, 0, c, false)
		if err != nil {
			return err
		}
		if r.Accepted {
			e.remove()
		}
	case 5:
		e.scrollOnly(c)
		index := c.Level * 2
		if e.Attributes&8 != 0 {
			index++
		}
		// Japanese bank0/1 $82F5; the final pair precedes the capsule table.
		code := [...]uint8{0x16, 0x16, 0x16, 0x16, 0x16, 0x16, 0x16, 0x16, 0x19, 0x1a, 3, 4, 9, 9, 0x16, 0x16}[index]
		r, err := s.requestTerrain(slot, code, c, false)
		if err != nil {
			return err
		}
		if !r.Accepted {
			return nil
		}
		return s.convertEnemyToPickup(slot, c.Level)
	case 6, 7, 8:
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-6))
	case 9, 10, 11:
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-9))
	default:
		return errors.New("unknown weapon box routine")
	}
	return nil
}

func (s *EnemySystem) convertEnemyToPickup(slot int, level uint8) error {
	e := &s.Slots[slot]
	s.PendingSounds = append(s.PendingSounds, 0x2f)
	if err := s.createBoxExplosion(e.X, e.Y, level); err != nil {
		return err
	}
	e.Attributes &= 7
	e.clearVisualMotion()
	e.Routine, e.Kind = 1, 0
	return nil
}

func (e *Enemy) boxBoundary(c EnemyStepContext, x, y uint8) bool {
	if c.Vertical {
		return e.Y >= y
	}
	return e.X < x
}
func (e *Enemy) setRoutine(routine uint8) {
	if e.Routine == 0 {
		e.Sprite = 0
	} else {
		e.Routine = routine
	}
}

// ED57 clears the visible/motion portion, deliberately retaining position,
// fractions, attributes, HP and current kind until its caller changes them.
func (e *Enemy) clearVisualMotion() {
	e.Sprite, e.SpriteAttributes = 0, 0
	e.VelocityX, e.VelocityY = 0, 0
	e.AnimationDelay, e.VariableA, e.AttackDelay, e.Frame = 0, 0, 0, 0
	e.StateWidth, e.ScoreCollision = 0, 0
	e.Data = [4]uint8{}
}
func (s *EnemySystem) createBoxExplosion(x, y, level uint8) error {
	return s.createTerrainExplosion(x, y, level, 0x89)
}
func (s *EnemySystem) createTerrainExplosion(x, y, level, width uint8) error {
	slot := s.freeSlot()
	if slot < 0 {
		return nil
	}
	if err := s.Initialize(slot, 2, level); err != nil {
		return err
	}
	e := &s.Slots[slot]
	e.Routine, e.Sprite, e.StateWidth = 6, 1, width
	e.X, e.Y = x, y
	return nil
}
