package contra

import "errors"

func (s *EnemySystem) StepBaseRedBlueSoldier(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 3 {
		return errors.New("invalid red/blue soldier context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 0x1e && e.Kind != 0x1f {
		return errors.New("invalid red/blue soldier kind")
	}
	death := uint8(5)
	if e.Kind == 0x1f {
		death = 4
	}
	if e.Routine >= death {
		phase := e.Routine - death
		if phase > 2 {
			return errors.New("invalid red/blue soldier death phase")
		}
		return s.StepExplosion(slot, c, ExplosionPhase(phase))
	}
	switch e.Routine {
	case 1:
		if e.Attributes > 3 {
			return errors.New("invalid red/blue entry")
		}
		e.Y = [...]uint8{0x9c, 0x9c, 0x61, 0x61}[e.Attributes]
		e.X = [...]uint8{0xf0, 0x10, 0xf0, 0x10}[e.Attributes]
		e.VelocityX = -0x100
		if e.Attributes&1 != 0 {
			e.VelocityX = 0x100
		}
		e.advance()
	case 2:
		if c.Frame&3 == 0 {
			e.Frame++
			if e.Frame >= 3 {
				e.Frame = 0
			}
		}
		e.Sprite = e.Frame + 0x85
		e.SpriteAttributes = 0x47
		if e.Kind == 0x1f {
			e.Sprite = e.Frame + 0x8c
			e.SpriteAttributes = 0x46
		}
		if e.Attributes&1 != 0 {
			e.SpriteAttributes &= 0xbf
		}
		if e.X < 0x24 || e.X >= 0xdc {
			e.SpriteAttributes |= 0x20
		} else {
			e.SpriteAttributes &= 0xdf
		}
		e.moveWithScroll(c)
		if e.X < 0x28 || e.X >= 0xd8 {
			return nil
		}
		actor := closestShooterTarget(e, c)
		distance := int(e.X) - int(c.Players[actor].X)
		if distance < 0 {
			distance = -distance
		}
		if c.Players[actor].State != 1 {
			distance = 254 + actor
		}
		if e.Kind == 0x1e {
			if distance < 16 {
				e.Frame = 0
				e.AnimationDelay = 1
				e.advance()
			}
		} else if e.Data[1] == 0 {
			limit := 16
			if e.Attributes&2 != 0 {
				limit = 48
			}
			if distance < limit {
				e.Sprite = 0x8f
				e.Data[0] = 3
				e.AttackDelay = 16
				e.advance()
			}
		}
	case 3:
		if e.Kind == 0x1f {
			e.AttackDelay--
			if e.AttackDelay != 0 {
				if e.AttackDelay == 0x2c {
					e.SpriteAttributes &= 0xf7
				}
				return nil
			}
			e.Sprite = 0x90
			e.Data[0]--
			if int8(e.Data[0]) < 0 {
				e.Data[1]++
				e.setRoutine(2)
				return nil
			}
			e.AttackDelay = 0x30
			e.SpriteAttributes |= 8
			return s.baseAimShot(e, c, 0, 4)
		}
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		e.AnimationDelay = 8
		e.Sprite = e.Frame + 0x88
		e.Frame++
		if e.Frame < 3 {
			return nil
		}
		e.SpriteAttributes &= 0xdf
		e.StateWidth &= 0x7e
		e.VelocityX = -0x40
		if e.Attributes&1 != 0 {
			e.VelocityX = 0x40
		}
		e.VelocityY = -0x100
		e.AnimationDelay = 16
		e.advance()
	case 4:
		e.Sprite = 0x8b
		if e.AnimationDelay != 0 {
			e.AnimationDelay--
			e.Sprite = 0x8a
		}
		e.VelocityY += 0x10
		e.moveWithScroll(c)
	}
	return nil
}
