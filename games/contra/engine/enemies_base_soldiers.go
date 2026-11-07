package contra

import "errors"

func (e *Enemy) baseSoldierInitialize(speed int16) {
	e.VelocityX = speed
	e.X = 0xa8
	if e.Attributes&1 != 0 {
		e.VelocityX = -e.VelocityX
		e.X = 0x58
	}
	e.Y = 0x6d
}
func (e *Enemy) baseSoldierAnimate(c EnemyStepContext) {
	if c.Frame&3 == 0 {
		e.Frame++
		if e.Frame >= 3 {
			e.Frame = 0
		}
	}
	e.Sprite = e.Frame + 0x93
	if e.VelocityX < 0 {
		e.SpriteAttributes |= 0x40
	} else {
		e.SpriteAttributes &= 0xbf
	}
}
func (e *Enemy) baseSoldierMove() {
	e.moveX()
	if (e.VelocityX < 0 && e.X < 0x50) || (e.VelocityX >= 0 && e.X >= 0xb0) {
		e.remove()
		return
	}
	if e.X < 0x60 || e.X >= 0xa0 {
		e.SpriteAttributes |= 0x20
	} else {
		e.SpriteAttributes &= 0xdf
	}
	if e.X < 0x62 || e.X >= 0x9e {
		e.StateWidth |= 0x80
	} else {
		e.StateWidth &= 0x7f
	}
}
func baseLane(x uint8, player bool) uint8 {
	table := [7]uint8{0xff, 0x94, 0x8c, 0x84, 0x7c, 0x74, 0x6c}
	if player {
		table = [7]uint8{0xff, 0xbc, 0xa4, 0x8c, 0x74, 0x5c, 0x44}
	}
	for i := 6; i >= 0; i-- {
		if x < table[i] {
			return uint8(i)
		}
	}
	return 0
}
func (s *EnemySystem) baseSoldierThrow(e *Enemy, c EnemyStepContext, kind uint8, dy uint8) error {
	x, y := e.X, e.Y+dy
	lane := baseLane(x, false)
	if kind == 1 && (x < 0x60 || x >= 0xa0) {
		return nil
	}
	if !c.AttackEnabled {
		return nil
	}
	slot := s.freeSlot()
	if slot < 0 {
		return nil
	}
	attribute := c.InheritedThrowAttribute
	if c.ThrowWork != nil && c.ThrowWork.Known {
		attribute = &c.ThrowWork.Value
	}
	if kind == 0x11 && attribute == nil {
		return errors.New("missing inherited Base throw attribute")
	}
	if err := s.Initialize(slot, kind, c.Level); err != nil {
		return err
	}
	shot := &s.Slots[slot]
	shot.X, shot.Y = x, y
	shot.VelocityX = [...]int16{0x55, 0x38, 0x1c, 0, -0x1c, -0x38, -0x55}[lane]
	shot.VelocityY = 0x80
	if kind == 1 {
		shot.Data[0] = 3
		shot.VelocityX = [...]int16{0xd4, 0x8d, 0x46, 0, -0x46, -0x8d, -0xd4}[lane]
		shot.VelocityY = 0x140
	}
	// 971B/971E preserves the actual inherited value, including the last box
	// height from an earlier slot. No expectation or output normalization.
	if kind == 0x11 {
		shot.Attributes = *attribute
	}
	return nil
}
func (s *EnemySystem) baseSoldierDeath(slot int, c EnemyStepContext, phase uint8) error {
	e := &s.Slots[slot]
	switch phase {
	case 0:
		e.StateWidth |= 0x81
		e.Sprite = 0x96
		e.VelocityY = -0x280
		e.VelocityX = 0
		e.AnimationDelay = 16
		e.advance()
	case 1:
		e.moveWithScroll(c)
		e.VelocityY += 0x38
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.advance()
		}
	case 2:
		return s.StepExplosion(slot, c, ExplosionStart)
	case 3:
		return s.baseSmallExplosion(slot, c, false)
	case 4:
		return s.StepExplosion(slot, c, ExplosionRemove)
	}
	return nil
}
func (s *EnemySystem) StepBaseIndoorSoldier(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || (c.Level != 1 && c.Level != 3) {
		return errors.New("invalid indoor soldier context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind < 0x15 || e.Kind > 0x18 {
		return errors.New("invalid indoor soldier kind")
	}
	deathStart := uint8(3)
	if e.Kind == 0x18 {
		deathStart = 4
	}
	if e.Routine >= deathStart {
		phase := e.Routine - deathStart
		if e.Kind == 0x16 && phase >= 2 {
			if phase == 2 {
				if e.Attributes&2 != 0 && e.Attributes&0x80 == 0 {
					e.Attributes >>= 2
					s.PendingSounds = append(s.PendingSounds, 0x2f)
					c.recordThrowWork(0x89) // EA13 publishes before attempting allocation.
					if err := s.createBoxExplosion(e.X, e.Y, c.Level); err != nil {
						return err
					}
					e.Attributes &= 7
					e.clearVisualMotion()
					e.Kind = 0
					e.Routine = 1
				} else {
					e.advance()
				}
				return nil
			}
			phase--
		}
		if phase > 4 {
			return errors.New("invalid indoor soldier death phase")
		}
		err := s.baseSoldierDeath(slot, c, phase)
		if e.Kind == 0x17 && phase == 4 {
			s.Base.GrenadeLauncher = 0
		}
		return err
	}
	switch e.Kind {
	case 0x15:
		if e.Routine == 1 {
			e.baseSoldierInitialize(-0xe0)
			e.AttackDelay = 8
			e.advance()
			return nil
		}
		e.baseSoldierAnimate(c)
		e.baseSoldierMove()
		e.AttackDelay--
		if e.AttackDelay != 0 {
			return nil
		}
		e.AttackDelay = 16
		if e.X < 0x68 || e.X >= 0x98 {
			return nil
		}
		switch (e.Attributes >> 1) & 3 {
		case 0:
			return s.baseSoldierThrow(e, c, 1, 0)
		case 1:
			e.Data[0]++
			if e.Data[0]&1 != 0 {
				return s.baseSoldierThrow(e, c, 0x12, 0)
			}
		default:
			return s.baseSoldierThrow(e, c, 0x11, 8)
		}
	case 0x16:
		if e.Routine == 1 {
			if e.Attributes&2 != 0 {
				if s.Base.RedSoldierCreated == 0 && s.Base.AttackRounds != 0 {
					s.Base.RedSoldierCreated = 1
				} else {
					e.Attributes &= 0xfd
				}
			}
			e.baseSoldierInitialize(-0xc0)
			e.advance()
			return nil
		}
		e.Sprite = 0x97
		if e.AnimationDelay != 0 {
			e.Sprite = 0x93
			if e.AnimationDelay >= 4 {
				e.Sprite = 0x98
			}
		}
		if e.VelocityX < 0 {
			e.SpriteAttributes |= 0x40
		} else {
			e.SpriteAttributes &= 0xbf
		}
		e.SpriteAttributes &= 0xf8
		if e.Attributes&2 != 0 {
			e.SpriteAttributes |= 5
		}
		if e.AnimationDelay != 0 {
			e.AnimationDelay--
			if e.Attributes&2 == 0 && e.AnimationDelay == 8 {
				return s.baseAimShot(e, c, 0x60, 4)
			}
			return nil
		}
		e.baseSoldierMove()
		if e.Data[0] >= 20 {
			return errors.New("invalid indoor jumper arc")
		}
		e.Y += [...]uint8{253, 253, 254, 254, 254, 255, 255, 255, 0, 0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3}[e.Data[0]]
		e.Data[0]++
		if e.Data[0] >= 20 {
			e.Data[0] = 0
			e.AnimationDelay = 16
		}
	case 0x17:
		target := func() int {
			actor := closestShooterTarget(e, c)
			if c.Players[actor].State != 1 {
				actor ^= 1
			}
			e.Data[1] = uint8(actor)
			return actor
		}
		if e.Routine == 1 {
			s.Base.GrenadeLauncher = 1
			target()
			e.baseSoldierInitialize(-0xc0)
			e.AnimationDelay = 32
			e.advance()
			return nil
		}
		if e.Data[2] != 0 {
			e.Sprite = 0x96
			e.AnimationDelay--
			if e.AnimationDelay == 0 {
				e.AnimationDelay = 8
				e.Data[2] = 0
				own := baseLane(e.X, false)
				c.recordThrowWork(own) // 949C: lane retained across target selection.
				actor := target()
				desiredNegative := baseLane(c.Players[actor].X, true) >= own
				if desiredNegative != (e.VelocityX < 0) {
					e.VelocityX = -e.VelocityX
				}
				return nil
			}
			if e.Data[0] != 0 {
				e.AttackDelay--
				if e.AttackDelay == 0 {
					e.AttackDelay = 20
					e.Data[0]--
					return s.baseSoldierThrow(e, c, 0x12, 0)
				}
			}
			return nil
		}
		e.baseSoldierAnimate(c)
		edge := (e.VelocityX >= 0 && e.X >= 0xa0) || (e.VelocityX < 0 && e.X < 0x60)
		if !edge {
			e.baseSoldierMove()
			e.AnimationDelay--
			if e.AnimationDelay != 0 {
				return nil
			}
		}
		if e.Data[1] > 1 {
			return errors.New("invalid indoor launcher player")
		}
		own := baseLane(e.X, false)
		c.recordThrowWork(own) // 94EE.
		aligned := baseLane(c.Players[e.Data[1]].X, true) == own
		e.AnimationDelay = 0x18
		if aligned {
			e.AnimationDelay = 0x38
		}
		e.Data[2]++
		e.AttackDelay = 4
		e.Data[0] = 0
		if aligned {
			e.Data[0] = (e.Attributes >> 1) & 3
		}
	case 0x18:
		index := func() (int, error) {
			i := int(e.Data[1])*4 + int(e.Data[0])
			if i >= 12 {
				return 0, errors.New("invalid indoor group pattern")
			}
			return i, nil
		}
		waiting := [...]uint8{1, 7, 13, 19, 24, 24, 24, 24, 16, 24, 24, 16}
		moving := [...]uint8{63, 57, 51, 45, 24, 16, 16, 24, 255, 255, 255, 255}
		if e.Routine == 1 {
			e.baseSoldierInitialize(-0xc0)
			i, err := index()
			if err != nil {
				return err
			}
			e.AnimationDelay = waiting[i]
			e.advance()
			return nil
		}
		if e.Routine == 2 {
			e.AnimationDelay--
			if e.AnimationDelay == 0 {
				if e.Data[1] == 1 && e.Data[0] >= 2 {
					e.VelocityX = -e.VelocityX
				}
				i, err := index()
				if err != nil {
					return err
				}
				e.AnimationDelay = moving[i]
				e.advance()
			} else if e.AnimationDelay == 4 {
				return s.baseSoldierThrow(e, c, 1, 0)
			}
			return nil
		}
		e.baseSoldierAnimate(c)
		e.baseSoldierMove()
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.Sprite = 0x96
			e.Data[1]++
			i, err := index()
			if err != nil {
				return err
			}
			e.AnimationDelay = waiting[i]
			e.setRoutine(2)
		}
	}
	return nil
}
