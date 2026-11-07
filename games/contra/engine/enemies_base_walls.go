package contra

import "errors"

func (s *EnemySystem) baseAimShot(e *Enemy, c EnemyStepContext, kind, speed uint8) error {
	angle, err := shooterAimAtPlayer(c, closestShooterTarget(e, c), e.X, e.Y, 1)
	if err != nil {
		return err
	}
	_, err = s.CreateEnemyShot(c, kind|angle, speed, e.X, e.Y)
	return err
}

// requestBaseTiles is EB70; EB0A adds bit7 and retries a refused update next
// frame. It uses a four-pixel origin, distinct from EB1B's supertile origin.
func (s *EnemySystem) requestBaseTiles(slot int, code uint8, c EnemyStepContext, retry bool) (EnemyTerrainResponse, error) {
	e := &s.Slots[slot]
	if retry {
		code |= 0x80
	}
	if e.X < 4 || e.Y < 4 {
		return EnemyTerrainResponse{Accepted: true}, nil
	}
	if c.ChangeTiles == nil {
		return EnemyTerrainResponse{}, errors.New("missing base tile animation queue")
	}
	result, err := c.ChangeTiles(EnemyTerrainRequest{X: e.X - 4, Y: e.Y - 4, Code: code})
	if err == nil && !result.Accepted && retry {
		e.AnimationDelay = 1
	}
	return result, err
}

func (s *EnemySystem) StepBaseWallTurret(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || (c.Level != 1 && c.Level != 3) {
		return errors.New("invalid base turret context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 0x13 || e.Routine > 8 {
		return errors.New("invalid base turret phase")
	}
	switch e.Routine {
	case 1:
		if e.Attributes > 3 {
			return errors.New("invalid base turret delay")
		}
		e.AnimationDelay = [...]uint8{0x50, 0x80, 0xb0, 0xf0}[e.Attributes]
		e.advance()
	case 2:
		if e.Data[0] == 0 {
			result, err := s.requestBaseTiles(slot, 0x84, c, false)
			if err != nil {
				return err
			}
			if result.Accepted {
				e.Data[0]++
			}
		}
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.AnimationDelay = 1
			e.advance()
		}
	case 3:
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		e.AnimationDelay = 8
		if e.Frame >= 3 {
			return errors.New("invalid base turret opening frame")
		}
		result, err := s.requestBaseTiles(slot, [...]uint8{0x85, 0x88, 0x89}[e.Frame], c, true)
		if err != nil || !result.Accepted {
			return err
		}
		e.Frame++
		if e.Frame >= 3 {
			e.StateWidth &= 0x7f
			e.AttackDelay = 0x80
			e.advance()
		}
	case 4:
		e.AttackDelay--
		if e.AttackDelay == 0 {
			e.AttackDelay = 0x50
			return s.baseAimShot(e, c, 0x60, 4)
		}
	case 5:
		result, err := s.requestBaseTiles(slot, 0x83, c, false)
		if err != nil {
			return err
		}
		if result.Accepted {
			e.advance()
		}
	case 6:
		if err := s.StepExplosion(slot, c, ExplosionStart); err != nil {
			return err
		}
		e.Frame = 0
	case 7:
		return s.StepExplosion(slot, c, ExplosionAnimate)
	case 8:
		return s.StepExplosion(slot, c, ExplosionRemove)
	}
	return nil
}

func (s *EnemySystem) StepBaseWallCore(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || (c.Level != 1 && c.Level != 3) {
		return errors.New("invalid base core context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 0x14 || e.Routine > 10 {
		return errors.New("invalid base core phase")
	}
	switch e.Routine {
	case 1:
		kind := (e.Attributes >> 2) & 3
		e.ScoreCollision = 0x25
		if kind&1 != 0 {
			e.VariableA = 4
			e.ScoreCollision = 0x22
		}
		e.HP = [...]uint8{8, 5, 16, 5}[kind]
		e.Data[1] = [...]uint8{0, 3, 0, 3}[kind]
		delay := uint8(0)
		if e.Attributes&4 == 0 {
			delay = e.Attributes & 3
		}
		e.AnimationDelay = [...]uint8{0x20, 0x80, 0xb0, 0xf0}[delay]
		e.advance()
	case 2:
		if e.Attributes&8 == 0 && e.Data[0] == 0 {
			code := uint8(0x84)
			if e.Attributes&4 != 0 {
				code = 0x80
			}
			result, err := s.requestBaseTiles(slot, code, c, false)
			if err != nil {
				return err
			}
			if result.Accepted {
				e.Data[0]++
			}
		}
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			if e.Attributes&4 != 0 {
				e.StateWidth &= 0x7f
				e.advance()
			}
			e.AttackDelay = 1
			e.AnimationDelay = 1
			e.advance()
		}
	case 3:
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		if e.Attributes&8 == 0 {
			e.AnimationDelay = 8
			if e.Frame >= 3 {
				return errors.New("invalid base core opening frame")
			}
			result, err := s.requestBaseTiles(slot, [...]uint8{0x85, 0x86, 0x87}[e.Frame], c, true)
			if err != nil || !result.Accepted {
				return err
			}
			e.Frame++
			if e.Frame < 3 {
				return nil
			}
		}
		e.StateWidth &= 0x7f
		e.advance()
	case 4:
		if s.Base.AttackRounds < 7 || e.Data[1] != 0 || e.Y >= 0x70 {
			return nil
		}
		e.AttackDelay--
		if e.AttackDelay == 0 {
			e.AttackDelay = 0x28
			return s.baseAimShot(e, c, 0x60, 5)
		}
	case 5:
		index := e.Data[1]
		if e.Attributes&8 != 0 {
			index += 4
		}
		if index >= 8 {
			return errors.New("invalid base core damage frame")
		}
		result, err := s.requestBaseTiles(slot, [...]uint8{0x83, 0x87, 0x82, 0x81, 0x83, 0x8a, 0x82, 0x81}[index], c, false)
		if err != nil || !result.Accepted {
			return err
		}
		e.Data[1]--
		if int8(e.Data[1]) < 0 {
			e.advance()
		} else {
			e.HP = 5
			if e.Data[1] == 0 {
				e.VariableA = 0
				e.ScoreCollision = 0x25
				e.HP = 8
			}
			e.setRoutine(4)
		}
	case 6:
		if err := s.StepExplosion(slot, c, ExplosionStart); err != nil {
			return err
		}
		e.Frame = 0
	case 7:
		return s.StepExplosion(slot, c, ExplosionAnimate)
	case 8:
		s.Base.CoresRemaining--
		if s.Base.CoresRemaining != 0 {
			e.remove()
			return nil
		}
		e.Sprite = 0
		s.falcon(&EnemyCollisionContext{Level: c.Level}, &EnemyCollisionResult{})
		e.Data[2] = 3
		e.AnimationDelay = 4
		e.advance()
	case 9:
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			if e.AnimationDelay == 1 {
				s.PendingSounds = append(s.PendingSounds, 0x3e)
			}
			return nil
		}
		if e.Data[2] > 3 {
			return errors.New("invalid base opening section")
		}
		i := e.Data[2]
		e.Y = [...]uint8{0x78, 0x78, 0x58, 0x58}[i]
		e.X = [...]uint8{0x70, 0x90, 0x90, 0x70}[i]
		result, err := s.requestTerrain(slot, [...]uint8{2, 3, 1, 0}[i], c, false)
		if err != nil {
			return err
		}
		if result.Accepted {
			c.recordThrowWork(0x89) // EA13, before the explosion allocation.
			child := s.freeSlot()
			if child >= 0 {
				if err = s.Initialize(child, 2, c.Level); err != nil {
					return err
				}
				s.Slots[child].Routine = 9
				s.Slots[child].Sprite = 1
				s.Slots[child].StateWidth = 0x89
				s.Slots[child].X = e.X
				s.Slots[child].Y = e.Y - 4
				if i&2 != 0 {
					s.Slots[child].Y -= 8
				}
			}
			e.Data[2]--
			if int8(e.Data[2]) < 0 {
				e.AnimationDelay = 16
				e.advance()
				return nil
			}
		}
		e.AnimationDelay = 1
	case 10:
		if c.ScreenCleared == nil {
			return errors.New("missing base room-clear state")
		}
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			*c.ScreenCleared = 1
			e.remove()
		}
	}
	return nil
}
