package contra

import "errors"

func addSnowByte(a, b uint8, carry bool) (uint8, bool) {
	v := uint16(a) + uint16(b)
	if carry {
		v++
	}
	return uint8(v), v > 255
}
func subtractSnowByte(a, b uint8, carry bool) (uint8, bool) {
	v := int(a) - int(b)
	if !carry {
		v--
	}
	return uint8(v), v >= 0
}

func (s *EnemySystem) refreshSnowBackground(c *EnemyStepContext) error {
	if c.Scene.Header == nil || c.Scene.Palette == nil || c.Scene.RefreshBackgroundPalette == nil {
		return errors.New("snow enemy requires live palette state")
	}
	result, err := c.Scene.RefreshBackgroundPalette(16)
	if err != nil {
		return err
	}
	if result.GenerationBiasChanged {
		s.recordGenerationBias(result.GenerationBiasValue)
		c.SpawnVariantSeed = result.GenerationBiasValue
	}
	return nil
}
func (s *EnemySystem) snowDraw(c EnemyStepContext, x, y, code uint8) (EnemyTerrainResponse, error) {
	if c.ChangeTerrain == nil {
		return EnemyTerrainResponse{}, errors.New("snow enemy requires graphics queue")
	}
	return c.ChangeTerrain(EnemyTerrainRequest{X: x, Y: y, Code: code})
}
func (s *EnemySystem) snowBossControl(c EnemyStepContext, control BossControl) error {
	if c.OnBossControl == nil {
		return errors.New("snow Boss requires progression callback")
	}
	return c.OnBossControl(control)
}

// Tanks use FractionX as an extended X-page byte, not a subpixel accumulator.
// Their visible art is terrain; current tank scroll is shared with pipe joints
// and the next terrain-scroll phase through the world's unique field.
func (s *EnemySystem) StepSnowTank(slot int, c EnemyStepContext) error {
	e, err := s.snowEnemy(slot, c, 18, 6)
	if err != nil || e.Routine == 0 {
		return err
	}
	if c.TankScroll == nil || c.Scene.Header == nil || c.Scene.Palette == nil || c.Scene.RefreshBackgroundPalette == nil || c.WeaponStrength > 3 {
		return errors.New("incomplete tank scene context")
	}
	if e.Routine == 3 && (e.Data[0] < 10 || e.Data[0] > 12) || e.Routine == 6 && e.Data[0] < 128 && e.Data[0] > 5 {
		return errors.New("invalid tank sequence state")
	}
	switch e.Routine {
	case 1:
		e.X, e.FractionX, e.Y = 0x30, 1, 0x90
		e.Data[0], e.AttackDelay = 12, 6
		c.Scene.Palette.PauseCycle, s.JointScroll = 0x3f, 0x3f
		c.Scene.Header.PaletteIndices[2], c.Scene.Header.PaletteIndices[3] = 0x3f, 0x41
		if err = s.refreshSnowBackground(&c); err != nil {
			return err
		}
		e.advance()
	case 2:
		*c.TankScroll = c.Frame & 1
		e.snowTankPosition(c)
		if e.FractionX == 0 && e.X < 0xa0 {
			*c.TankScroll = 0
			e.Data[3] = snowTankWait[(e.Attributes&1)*4+c.WeaponStrength]
			e.HP, e.AnimationDelay = 0x47, 8
			e.advance()
			return nil
		}
		return s.snowTankMoving(slot, c)
	case 3:
		if e.HP>>4 >= uint8(len(snowTankPalettes)) {
			return errors.New("tank HP exceeds its palette table")
		}
		c.Scene.Header.PaletteIndices[2] = snowTankPalettes[e.HP>>4]
		if err = s.refreshSnowBackground(&c); err != nil {
			return err
		}
		e.snowTankPosition(c)
		if !s.snowTankStillPresent(slot, c) {
			return nil
		}
		if c.Frame&1 == 0 {
			e.Data[3]--
			if e.Data[3] == 0 {
				e.advance()
				return nil
			}
		}
		if e.StateWidth&0x80 != 0 {
			return nil
		}
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		if e.Data[2] == 0 {
			return s.snowTankAim(slot, c)
		}
		index := int(e.Data[0]-10) * 3
		x, carry := subtractSnowByte(e.X, snowTankShots[index], false)
		_, carry = subtractSnowByte(e.FractionX, 0, carry)
		if !carry {
			return nil
		}
		y, _ := subtractSnowByte(e.Y, snowTankShots[index+1], carry)
		if _, err = s.CreateEnemyShot(c, snowTankShots[index+2], e.Attributes, x, y); err != nil {
			return err
		}
		e.Data[2]--
		e.AnimationDelay = 0x20
	case 4:
		*c.TankScroll = c.Frame & 1
		if e.HP>>4 >= uint8(len(snowTankPalettes)) {
			return errors.New("tank HP exceeds its palette table")
		}
		c.Scene.Header.PaletteIndices[2] = snowTankPalettes[e.HP>>4]
		if err = s.refreshSnowBackground(&c); err != nil {
			return err
		}
		e.snowTankPosition(c)
		if !s.snowTankStillPresent(slot, c) {
			return nil
		}
		return s.snowTankMoving(slot, c)
	case 5:
		e.snowTankPosition(c)
		e.StateWidth |= 0x81
		e.Data[0] = 5
		s.PendingSounds = append(s.PendingSounds, 0x75)
		e.AnimationDelay = 0
		e.advance()
	case 6:
		*c.TankScroll = c.Frame & 1
		e.snowTankPosition(c)
		if !s.snowTankStillPresent(slot, c) {
			return nil
		}
		if e.AnimationDelay != 0 {
			e.AnimationDelay--
			return nil
		}
		if e.Data[0]&0x80 != 0 {
			return nil
		}
		index := int(e.Data[0]) * 5
		x, carry := addSnowByte(e.X, snowTankDestruction[index+1], false)
		high, carry := addSnowByte(e.FractionX, snowTankDestruction[index], carry)
		if high != 0 {
			e.Data[0]--
			return nil
		}
		y, _ := addSnowByte(e.Y, snowTankDestruction[index+2], carry)
		result, err := s.snowDraw(c, x, y, 0x9b)
		if err != nil || !result.Accepted {
			return err
		}
		explosionX := e.X + snowTankDestruction[index+3]
		explosionY := e.Y + snowTankDestruction[index+4]
		e.Data[0]--
		e.AnimationDelay = 4
		return s.createTerrainExplosion(explosionX, explosionY, c.Level, 0x89)
	}
	return nil
}

func (e *Enemy) snowTankPosition(c EnemyStepContext) {
	scroll := c.FrameScroll + *c.TankScroll
	old := e.X
	e.X -= scroll
	if old < scroll {
		e.FractionX--
		if e.HP != 0 {
			e.StateWidth ^= 0x81
		}
	}
}
func (s *EnemySystem) snowTankStillPresent(slot int, c EnemyStepContext) bool {
	e := &s.Slots[slot]
	if e.FractionX&0x80 == 0 || e.X >= 0xd0 {
		return true
	}
	e.remove()
	*c.TankScroll = 0
	c.Scene.Palette.PauseCycle = 0
	s.JointScroll = 0
	return false
}
func (s *EnemySystem) snowTankMoving(slot int, c EnemyStepContext) error {
	e := &s.Slots[slot]
	if e.HP != 0 {
		e.AttackDelay--
		if e.AttackDelay == 0 {
			s.PendingSounds = append(s.PendingSounds, 0x36)
			e.AttackDelay = 6
		}
	}
	var x, high uint8
	if c.Frame&1 == 0 {
		var carry bool
		x, carry = subtractSnowByte(e.X, 12, true)
		high, _ = subtractSnowByte(e.FractionX, 0, carry)
	} else {
		var carry bool
		x, carry = addSnowByte(e.X, 20, false)
		high, _ = addSnowByte(e.FractionX, 0, carry)
	}
	if high != 0 {
		return nil
	}
	_, err := s.snowDraw(c, x, e.Y, snowTankWheels[c.Frame&3])
	return err
}
func (s *EnemySystem) snowTankAim(slot int, c EnemyStepContext) error {
	e := &s.Slots[slot]
	previous := e.Data[0]
	target, err := shooterAimAtPlayer(c, closestShooterTarget(e, c), e.X, e.Y-12, 1)
	if err != nil {
		return err
	}
	s.recordGenerationBias(24)
	turn := shooterTurn(e.Data[0], target, 24)
	if turn > 0 {
		e.Data[0] = (e.Data[0] + 1) % 24
	} else if turn < 0 {
		e.Data[0] = (e.Data[0] + 23) % 24
	}
	if e.Data[0] < 10 || e.Data[0] >= 13 {
		e.Data[0], target = previous, previous
	}
	if e.Data[0] == target {
		e.Data[2] = 3
	}
	index := e.Data[0] - 10
	y, carry := subtractSnowByte(e.Y, 28, true)
	x, carry := subtractSnowByte(e.X, 44, carry)
	high, _ := subtractSnowByte(e.FractionX, 0, carry)
	if high != 0 {
		return nil
	}
	result, err := s.snowDraw(c, x, y, snowTankTurrets[index])
	if err != nil {
		return err
	}
	e.AnimationDelay = 1
	if result.Accepted {
		e.AnimationDelay = 0x30
	}
	return nil
}
