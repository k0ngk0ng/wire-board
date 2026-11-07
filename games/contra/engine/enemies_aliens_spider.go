package contra

import "errors"

func (e *Enemy) alienSpiderAppearance(c EnemyStepContext) {
	a := uint16(c.WeaponStrength) + uint16(c.GameLoop) + 1
	e.HP = uint8(a) + 1 + uint8(a>>8)
	e.AnimationDelay = 0x60
	e.SpriteAttributes = 0
	if e.Y < 128 {
		e.SpriteAttributes = 0x80
	}
}

func (e *Enemy) alienSpiderGround(c EnemyStepContext, carry uint8) {
	e.Data = [4]uint8{}
	e.Sprite = 0xb3
	e.VelocityX = -384
	e.VelocityY = 0
	if c.Players[1].GameOver == 0 {
		e.Data[0] = (c.Random + c.Frame + carry) & 1
	}
	e.Data[1] = ((c.Random >> 1) + c.Frame + (c.Random & 1)) & 2
	e.Routine = 4
}

func (s *EnemySystem) StepAlienSpider(slot int, c EnemyStepContext) error {
	e, err := alienState(s, slot, c, 20, 8)
	if err != nil || e.Routine == 0 {
		return err
	}
	switch e.Routine {
	case 1:
		e.alienSpiderAppearance(c)
		carry := uint8(0)
		if e.Y >= 128 {
			carry = 1
		}
		e.alienSpiderGround(c, carry)
	case 2:
		e.ScoreCollision = 0x33
		e.alienSpiderAppearance(c)
		e.Sprite = 0xb6
		e.VelocityX = -80
		e.Data[2], e.Data[3] = 0xfc, 0
		e.advance()
	case 3:
		v := uint16(e.Data[2])<<8 | uint16(e.Data[3])
		v += 0x28
		e.Data[2], e.Data[3] = uint8(v>>8), uint8(v)
		if e.Y >= 128 {
			e.VelocityY = int16(v)
		} else {
			e.VelocityY = int16(uint16(e.Data[2]^255)<<8 | uint16(-e.Data[3]))
		}
		e.moveWithScroll(c)
		if e.Y >= 0xc1 || e.Y < 0x30 {
			carry := uint8(0)
			if e.Y >= 0xc1 {
				carry = 1
			}
			e.alienSpiderGround(c, carry)
		}
	case 4:
		if err = e.animateAlien(0xb3); err != nil {
			return err
		}
		jump := false
		if e.Data[2] == 0 && e.Data[1] != 0 {
			if e.Data[0] >= 2 {
				return errors.New("invalid spider target")
			}
			p := c.Players[e.Data[0]]
			dx := e.X - p.X
			if p.Y >= 32 && dx < 48 {
				if e.Y < p.Y {
					e.VelocityY = int16(0x240 + uint16(c.WeaponStrength<<4) + uint16((c.WeaponStrength>>4)&1))
					jump = true
				} else if dx < 32 {
					e.VelocityY = -256
					jump = true
				}
			}
		}
		if jump {
			e.Sprite = 0xb3
			e.SpriteAttributes &= 0x3f
			if e.VelocityY >= 0 {
				e.VelocityX = -128
			}
			e.Data[2]++
			e.Routine++
			e.moveWithScroll(c)
		} else {
			if e.Y > 184 {
				e.Y = 184
			} else if e.Y <= 56 {
				e.Y = 56
			}
			e.moveWithScroll(c)
			e.VelocityY = 0
		}
	case 5:
		if c.Terrain == nil {
			return errors.New("missing spider collision terrain")
		}
		if e.terrainAt(c, 0, 0) == TerrainPlatform {
			e.VelocityY = 0
			e.VelocityX = -384
			e.Routine = 4
		}
		e.moveWithScroll(c)
	default:
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-6))
	}
	return nil
}
