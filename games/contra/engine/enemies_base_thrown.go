package contra

import "errors"

func (s *EnemySystem) baseSmallExplosion(slot int, c EnemyStepContext, roller bool) error {
	e := &s.Slots[slot]
	e.scrollOnly(c)
	if e.Routine == 0 {
		return nil
	}
	e.AnimationDelay--
	if e.AnimationDelay != 0 {
		return nil
	}
	e.Frame++
	count := uint8(3)
	if roller {
		count = 2
	}
	if e.Frame >= count {
		e.advance()
		return nil
	}
	if e.Frame+1 >= count {
		e.StateWidth |= 0x81
	}
	e.AnimationDelay = 10
	if roller {
		c.recordThrowWork(0x66) // original E754 roller animation identity.
		e.Sprite = [...]uint8{0x36, 0x37}[e.Frame]
	} else {
		c.recordThrowWork(0x63) // original small Base explosion identity.
		e.Sprite = [...]uint8{0x9d, 0x9e, 0x9f}[e.Frame]
	}
	return nil
}

func (s *EnemySystem) StepBaseThrownWeapon(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || (c.Level != 1 && c.Level != 3) {
		return errors.New("invalid base thrown weapon context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind == 0x11 {
		switch e.Routine {
		case 1:
			e.Y = 0x72
			e.advance()
		case 2:
			appearance := 3
			for appearance > 0 && e.Y < [...]uint8{0xe7, 0x7c, 0x8c, 0x9c}[appearance] {
				appearance--
			}
			e.Sprite = uint8(0x99 + appearance)
			if appearance >= 2 {
				e.ScoreCollision = 0x2e
			}
			e.moveWithScroll(c)
			if e.Y >= 0xbc {
				e.remove()
			} else if e.Y >= 0xac {
				e.StateWidth &= 0x7e
			}
		case 3:
			return s.StepExplosion(slot, c, ExplosionStart)
		case 4:
			return s.baseSmallExplosion(slot, c, true)
		case 5:
			return s.StepExplosion(slot, c, ExplosionRemove)
		default:
			return errors.New("invalid base roller phase")
		}
		return nil
	}
	if e.Kind != 0x12 || e.Routine > 6 {
		return errors.New("invalid base grenade phase")
	}
	switch e.Routine {
	case 1:
		e.Data[0] = e.Y
		e.Data[3] = 0
		e.AttackDelay = 0xfd
		e.advance()
	case 2:
		group := 2
		for group > 0 && e.Data[0] < [...]uint8{0x60, 0x80, 0x90}[group] {
			group--
		}
		if c.Frame&7 == 0 {
			e.Frame++
		}
		count := uint8(8)
		if group == 0 {
			count = 4
		}
		if e.Frame >= count {
			e.Frame = 0
		}
		c.recordThrowWork(count) // 901D: sprite-to-attribute table distance.
		appearances := [][]uint8{{0xa8, 0xa9, 0xa6, 0xa9, 0, 0, 0, 0xc0}, {0xa4, 0xa5, 0xa6, 0xa5, 0xa4, 0xa7, 0xa6, 0xa7, 0, 0, 0, 0xc0, 0xc0, 0, 0, 0xc0}, {0xa0, 0xa1, 0xa2, 0xa1, 0xa0, 0xa3, 0xa2, 0xa3, 0, 0, 0, 0xc0, 0xc0, 0, 0, 0xc0}}
		e.Sprite = appearances[group][e.Frame]
		e.SpriteAttributes = appearances[group][e.Frame+count]
		velocity := uint16(e.AttackDelay)<<8 | uint16(e.Data[3])
		velocity += 12
		e.AttackDelay, e.Data[3] = uint8(velocity>>8), uint8(velocity)
		offset := uint16(e.Data[2])<<8 | uint16(e.Data[1])
		offset += velocity
		e.Data[2], e.Data[1] = uint8(offset>>8), uint8(offset)
		baseline := uint16(e.Data[0])<<8 | uint16(e.FractionY)
		baseline += uint16(e.VelocityY)
		e.Data[0], e.FractionY = uint8(baseline>>8), uint8(baseline)
		if e.Data[0] >= 0xf0 {
			e.remove()
		} else {
			e.Y = e.Data[0] + e.Data[2]
			e.moveX()
			e.X -= c.FrameScroll
			if e.X < 8 {
				e.remove()
			}
		}
		if int8(e.Data[2]) >= 0 {
			e.advance()
		}
	case 3:
		s.PendingSounds = append(s.PendingSounds, 0x3d)
		e.Y = 0xac
		if err := s.StepExplosion(slot, c, ExplosionMortarStart); err != nil {
			return err
		}
		e.advance()
	case 4:
		return s.StepExplosion(slot, c, ExplosionStart)
	case 5:
		return s.StepExplosion(slot, c, ExplosionAnimate)
	case 6:
		return s.StepExplosion(slot, c, ExplosionRemove)
	}
	return nil
}
