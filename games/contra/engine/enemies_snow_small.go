package contra

import "errors"

func (s *EnemySystem) snowEnemy(slot int, c EnemyStepContext, kind, phases uint8) (*Enemy, error) {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 4 || c.SceneType != 0 || c.Vertical {
		return nil, errors.New("invalid snow enemy context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return e, nil
	}
	if e.Kind != kind || e.Routine > phases {
		return nil, errors.New("invalid snow enemy phase")
	}
	return e, nil
}

// spawnSnowRelative is EAB8/EABD's source-relative allocation. In particular,
// when a removed source slot is re-used, initialization occurs before reading
// that slot's position; retaining an earlier position copy would change play.
func (s *EnemySystem) spawnSnowRelative(parent int, kind, dx, dy uint8, level uint8) (int, bool, error) {
	slot := s.freeSlot()
	if slot < 0 {
		return 0, false, nil
	}
	if err := s.Initialize(slot, kind, level); err != nil {
		return 0, false, err
	}
	s.Slots[slot].X = s.Slots[parent].X + dx
	s.Slots[slot].Y = s.Slots[parent].Y + dy
	return slot, true, nil
}

func (s *EnemySystem) StepIceGenerator(slot int, c EnemyStepContext) error {
	e, err := s.snowEnemy(slot, c, 16, 3)
	if err != nil || e.Routine == 0 {
		return err
	}
	switch e.Routine {
	case 1:
		e.scrollOnly(c)
		if e.X < 0xc8 {
			e.AnimationDelay = 1
			e.advance()
		}
	case 2:
		e.scrollOnly(c)
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.AnimationDelay = 0x80
			_, _, err = s.spawnSnowRelative(slot, 17, 0, 0, c.Level)
		}
	case 3:
		e.remove()
	}
	return err
}

// Ice grenade sound IDs are Japanese $30/$3D, not the US reference's IDs.
func (s *EnemySystem) StepIceGrenade(slot int, c EnemyStepContext) error {
	e, err := s.snowEnemy(slot, c, 17, 5)
	if err != nil || e.Routine == 0 {
		return err
	}
	switch e.Routine {
	case 1:
		s.PendingSounds = append(s.PendingSounds, 0x30)
		e.SpriteAttributes = 0x20
		e.VelocityX, e.VelocityY = 128, -512
		e.advance()
	case 2:
		if c.Terrain == nil {
			return errors.New("ice grenade requires terrain")
		}
		if c.Frame&7 == 0 {
			e.Frame++
		}
		e.Sprite = [...]uint8{0x74, 0x75, 0x76, 0x77}[e.Frame&3]
		e.moveWithScroll(c)
		e.VelocityY += 10
		if e.VelocityY >= 0 {
			e.SpriteAttributes = 0
			if e.terrainAt(c, 0, 4) != TerrainEmpty {
				s.PendingSounds = append(s.PendingSounds, 0x3d)
				e.advance()
			}
		}
	case 3:
		return s.StepExplosion(slot, c, ExplosionMortarStart)
	case 4:
		return s.StepExplosion(slot, c, ExplosionAnimate)
	case 5:
		return s.StepExplosion(slot, c, ExplosionRemove)
	}
	return nil
}

func (s *EnemySystem) StepIceJoint(slot int, c EnemyStepContext) error {
	e, err := s.snowEnemy(slot, c, 19, 1)
	if err != nil || e.Routine == 0 {
		return err
	}
	e.Sprite = 0xc4
	if s.JointScroll == 0 {
		e.scrollOnly(c)
	} else if c.FrameScroll != 0 {
		e.X--
	}
	return nil
}

func (e *Enemy) snowSaucerSprite() {
	if e.AnimationDelay&3 == 0 {
		e.Sprite++
		if e.Sprite >= 0x7f {
			e.Sprite -= 3
		}
	}
}
func (e *Enemy) snowSaucerAnimation() { e.AnimationDelay--; e.snowSaucerSprite() }

func (s *EnemySystem) StepMiniUFO(slot int, c EnemyStepContext) error {
	e, err := s.snowEnemy(slot, c, 21, 7)
	if err != nil || e.Routine == 0 {
		return err
	}
	switch e.Routine {
	case 1:
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.advance()
		} else {
			e.snowSaucerSprite()
		}
	case 2:
		e.snowSaucerAnimation()
		e.moveX()
		if e.X < 8 {
			e.remove()
		}
		if e.X < 0x20 || e.X >= 0xe0 {
			e.VelocityY = 384
			e.advance()
		}
	case 3:
		e.snowSaucerAnimation()
		e.moveY()
		if e.Y >= 0xe8 {
			e.remove()
		}
		if e.Y >= 0xa9 {
			// Japanese code uses A9/AA; the US A8/A9 pair is different.
			e.Y = 0xaa
			e.VelocityX = 384
			if e.X&0x80 != 0 {
				e.VelocityX = -384
			}
			e.VelocityY = 0
			e.advance()
		}
	case 4:
		e.snowSaucerAnimation()
		e.moveWithScroll(c)
	case 5:
		return s.StepExplosion(slot, c, ExplosionStart)
	case 6:
		return s.StepExplosion(slot, c, ExplosionAnimate)
	case 7:
		return s.StepExplosion(slot, c, ExplosionRemove)
	}
	return nil
}

func (s *EnemySystem) StepSnowBomb(slot int, c EnemyStepContext) error {
	e, err := s.snowEnemy(slot, c, 22, 4)
	if err != nil || e.Routine == 0 {
		return err
	}
	switch e.Routine {
	case 1:
		e.VelocityY += 0x28
		if e.Y < 0xb0 {
			e.moveWithScroll(c)
		} else {
			e.advance()
		}
	case 2:
		return s.StepExplosion(slot, c, ExplosionStart)
	case 3:
		return s.StepExplosion(slot, c, ExplosionAnimate)
	case 4:
		return s.StepExplosion(slot, c, ExplosionRemove)
	}
	return nil
}
