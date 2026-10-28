package contra

import "errors"

// StepSnowCarrier follows the Japanese multi-slot UFO sequence. Scene effects
// are applied to the existing world owners at their original boundaries.
func (s *EnemySystem) StepSnowCarrier(slot int, c EnemyStepContext) error {
	e, err := s.snowEnemy(slot, c, 20, 12)
	if err != nil || e.Routine == 0 {
		return err
	}
	if c.Scene.Header == nil || c.Scene.Palette == nil || c.Scene.RefreshBackgroundPalette == nil {
		return errors.New("carrier requires live scene palette")
	}
	if (e.Routine == 3 || e.Routine == 8) && e.AnimationDelay > 3 || (e.Routine == 4 || e.Routine == 6) && e.Data[0] > 3 || e.Routine == 11 && e.Data[0] < 128 && e.Data[0] > 4 || e.Routine == 12 && e.Data[0] < 128 && e.Data[0] > 2 {
		return errors.New("invalid carrier sequence state")
	}
	switch e.Routine {
	case 1:
		e.Y = 16
		c.Scene.Palette.FadeTimer = 16
		if err = s.refreshSnowBackground(&c); err != nil {
			return err
		}
		e.advance()
	case 2:
		e.X = snowCarrierX[c.Random&3]
		e.Y += 32
		if e.Y >= 0x71 {
			e.Y = 0x30
		}
		e.AnimationDelay = 3
		e.advance()
	case 3:
		return s.snowCarrierDrawPart(slot, c, snowCarrierParts[e.AnimationDelay], e.AnimationDelay)
	case 4:
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return s.snowCarrierThrusters(slot, c)
		}
		e.StateWidth &= 0x7e
		if err = s.snowCarrierTop(slot, c); err != nil {
			return err
		}
		e.Data[0]--
		if e.Data[0]&0x80 != 0 {
			e.AnimationDelay = 0
			e.advance()
		}
	case 5:
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.Data[0] = 1
			e.AnimationDelay = 8
			e.advance()
			return nil
		}
		if e.AnimationDelay&15 != 0 {
			return s.snowCarrierThrusters(slot, c)
		}
		index := (e.AnimationDelay >> 2) & 12
		child, ok, err := s.spawnSnowRelative(slot, snowCarrierChildren[index], snowCarrierChildren[index+1], 0xf4, c.Level)
		if err != nil || !ok {
			return err
		}
		p := &s.Slots[child]
		p.VelocityX = int16(int8(snowCarrierChildren[index+2])) << 8
		if index&4 == 0 {
			p.VelocityX |= 0x80
		}
		p.AnimationDelay, p.ScoreCollision, p.Sprite = 16, 2, snowCarrierChildren[index+3]
	case 6:
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return s.snowCarrierThrusters(slot, c)
		}
		if err = s.snowCarrierTop(slot, c); err != nil {
			return err
		}
		e.Data[0]++
		if e.Data[0] == 4 {
			e.StateWidth |= 0x81
			e.AnimationDelay = 32
			e.advance()
		}
	case 7:
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return s.snowCarrierThrusters(slot, c)
		}
		c.Scene.Palette.FadeTimer = 24
		if err = s.refreshSnowBackground(&c); err != nil {
			return err
		}
		e.AnimationDelay = 3
		e.advance()
	case 8:
		return s.snowCarrierDrawPart(slot, c, 0x9b, e.AnimationDelay)
	case 9:
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.shooterRoutine(2)
		}
	case 10:
		if err = s.snowBossControl(c, BossControl{Kind: BossAudioReset}); err != nil {
			return err
		}
		s.PendingSounds = nil
		s.PendingSounds = append(s.PendingSounds, 0x75)
		e.StateWidth |= 0x81
		e.Data[0] = 4
		s.falcon(&EnemyCollisionContext{Level: c.Level}, &EnemyCollisionResult{})
		e.AnimationDelay = 16
		e.advance()
	case 11:
		if e.AnimationDelay != 0 {
			e.AnimationDelay--
			return nil
		}
		if e.Data[0]&0x80 != 0 {
			e.Data[0] = 2
			e.AnimationDelay = 4
			e.advance()
			return nil
		}
		index := int(e.Data[0]) * 2
		oldX, oldY := e.X, e.Y
		e.X += snowCarrierExplosions[index]
		e.Y += snowCarrierExplosions[index+1]
		result, err := s.requestTerrain(slot, 0x9b, c, false)
		if err != nil {
			return err
		}
		if !result.Accepted {
			e.X, e.Y = oldX, oldY
			return nil
		}
		return s.snowCarrierExplosion(slot, c)
	case 12:
		if e.AnimationDelay != 0 {
			e.AnimationDelay--
			return nil
		}
		if e.Data[0]&0x80 != 0 {
			if err = s.snowBossControl(c, BossControl{Kind: BossBeginDefeat}); err != nil {
				return err
			}
			if err = s.snowBossControl(c, BossControl{Kind: BossExitReady, Delay: 48}); err != nil {
				return err
			}
			e.remove()
			return nil
		}
		index := int(e.Data[0]) * 3
		e.X, e.Y = snowCarrierDoor[index], snowCarrierDoor[index+1]
		result, err := s.requestTerrain(slot, snowCarrierDoor[index+2], c, false)
		if err != nil || !result.Accepted {
			return err
		}
		return s.snowCarrierExplosion(slot, c)
	}
	return nil
}

func (s *EnemySystem) snowCarrierDrawPart(slot int, c EnemyStepContext, code, part uint8) error {
	if part > 3 {
		return errors.New("invalid carrier art part")
	}
	e := &s.Slots[slot]
	_, err := s.snowDraw(c, e.X+snowCarrierOffsets[part*2], e.Y+snowCarrierOffsets[part*2+1], code)
	if err != nil {
		return err
	}
	// Original A711 decrements even if the graphics request was refused.
	e.AnimationDelay--
	if e.AnimationDelay&0x80 != 0 {
		e.Data[0] = 2
		e.AnimationDelay = 0x60
		e.advance()
	}
	return nil
}
func (s *EnemySystem) snowCarrierTop(slot int, c EnemyStepContext) error {
	e := &s.Slots[slot]
	index := int(e.Data[0]) * 2
	e.AnimationDelay = 10
	if err := s.snowCarrierDrawPart(slot, c, snowCarrierTop[index], 0); err != nil {
		return err
	}
	return s.snowCarrierDrawPart(slot, c, snowCarrierTop[index+1], 1)
}
func (s *EnemySystem) snowCarrierThrusters(slot int, c EnemyStepContext) error {
	e := &s.Slots[slot]
	if e.AnimationDelay&7 != 3 {
		return nil
	}
	saved := e.AnimationDelay
	s.recordGenerationBias(saved)
	index := 0
	if saved&8 == 0 {
		index = 2
	}
	e.AnimationDelay = 2
	if err := s.snowCarrierDrawPart(slot, c, snowCarrierThrusters[index], 2); err != nil {
		return err
	}
	if err := s.snowCarrierDrawPart(slot, c, snowCarrierThrusters[index+1], 3); err != nil {
		return err
	}
	e.AnimationDelay = saved
	return nil
}
func (s *EnemySystem) snowCarrierExplosion(slot int, c EnemyStepContext) error {
	e := &s.Slots[slot]
	x, y := e.X, e.Y
	e.Data[0]--
	e.AnimationDelay = 8
	return s.createTerrainExplosion(x, y, c.Level, 0x89)
}
