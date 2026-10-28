package contra

import "errors"

func (s *EnemySystem) StepCartGenerator(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 6 {
		return errors.New("invalid cart generator context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 19 || e.Routine > 2 {
		return errors.New("invalid cart generator state")
	}
	if e.Routine == 1 {
		e.Frame = 0x80
		e.AnimationDelay = 1
		e.advance()
		return nil
	}
	e.scrollOnly(c)
	if e.Frame&0x80 != 0 {
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		e.AnimationDelay++
		child := s.freeSlot()
		if child < 0 {
			return nil
		}
		if err := s.Initialize(child, 20, c.Level); err != nil {
			return err
		}
		cart := &s.Slots[child]
		cart.X = 0xf8
		cart.VelocityX = -128
		cart.Data[3] = 2
		cart.Attributes = 0x80
		cart.Sprite = 0x2a
		cart.Y = 0xc8
		e.Frame = uint8(child)
	} else {
		if e.Frame >= 16 {
			return errors.New("invalid cart generator child")
		}
		if s.Slots[e.Frame].Routine == 0 {
			e.Frame = 0x80
			e.AnimationDelay = 0x80
		}
	}
	return nil
}

func (s *EnemySystem) StepCart(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 6 {
		return errors.New("invalid cart context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if (e.Kind != 20 && e.Kind != 21) || e.Routine > 6 {
		return errors.New("invalid cart state")
	}
	if e.Routine >= 4 {
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-4))
	}
	if e.Kind == 21 {
		if e.Routine == 1 {
			e.VelocityX = int16(uint16(e.VelocityX)&0xff00 | 0xc0)
			e.Sprite = 0x2a
			e.Y = 0xc8
			e.advance()
			return nil
		}
		if e.Routine == 2 {
			if e.Frame != 0 {
				e.advance()
			} else {
				e.scrollOnly(c)
			}
			return nil
		}
	}
	if e.Data[3] != 0 && e.Data[3] != 2 || c.Terrain == nil {
		return errors.New("invalid cart collision context")
	}
	e.Sprite = 0x2a + ((c.Frame >> 2) & 1)
	e.moveWithScroll(c)
	dx := 15
	if e.Data[3] == 2 {
		dx = -15
	}
	x := int(e.X) + dx
	scroll := c.Scroll
	if x < 0 || x >= 256 {
		scroll.NameTable ^= 1
	}
	if enemyTerrainSample(c, uint8(x), e.Y, scroll).Kind != TerrainEmpty {
		if e.Attributes&0x80 != 0 {
			e.shooterRoutine(4)
		} else {
			e.Data[3] ^= 2
			e.VelocityX = -e.VelocityX
		}
	} else if e.terrainAt(c, 0, 9) == TerrainEmpty {
		e.VelocityY += 0x20
	}
	return nil
}

// drawMachineTiles is EB6E's four-pixel coordinate adapter. Clipped requests
// succeed without a queue write, as they do in the original function.
func (s *EnemySystem) drawMachineTiles(e *Enemy, c EnemyStepContext, code uint8) (bool, error) {
	if e.X < 4 || e.Y < 4 {
		return true, nil
	}
	if c.ChangeTiles == nil {
		return false, errors.New("missing machine tile queue")
	}
	r, err := c.ChangeTiles(EnemyTerrainRequest{X: e.X - 4, Y: e.Y - 4, Code: code | 0x80})
	return r.Accepted, err
}

func (s *EnemySystem) drawMortarLauncher(e *Enemy, c EnemyStepContext, frame uint8) (bool, error) {
	if frame > 3 {
		return false, errors.New("invalid mortar launcher frame")
	}
	accepted, err := s.drawMachineTiles(e, c, frame+8)
	if err != nil {
		return false, err
	}
	e.AnimationDelay = 1
	if accepted {
		e.AnimationDelay = 8
	}
	return accepted, nil
}

func (s *EnemySystem) StepMortarLauncher(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 6 {
		return errors.New("invalid mortar launcher context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 23 || e.Routine > 8 {
		return errors.New("invalid mortar launcher state")
	}
	if e.Routine >= 6 {
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-6))
	}
	switch e.Routine {
	case 1:
		e.Y += 4
		e.Data[0] = 4
		e.AnimationDelay = 96
		if e.Attributes&1 != 0 {
			e.AnimationDelay = 16
		}
		e.advance()
	case 2:
		e.scrollOnly(c)
		if c.Scene.BossScrollComplete == nil {
			return errors.New("missing boss scroll state")
		}
		if *c.Scene.BossScrollComplete == 0 {
			return nil
		}
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		accepted, err := s.drawMortarLauncher(e, c, e.Frame)
		if err != nil || !accepted {
			return err
		}
		if e.Frame < 2 {
			e.Frame++
		} else {
			e.StateWidth &= 0x7e
			e.AttackDelay = 16
			e.AnimationDelay = 96
			e.advance()
		}
	case 3:
		e.scrollOnly(c)
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.StateWidth |= 0x81
			e.AnimationDelay = 1
			e.advance()
			return nil
		}
		e.AttackDelay--
		if e.AttackDelay != 0 {
			return nil
		}
		child, err := s.spawnMachineRelative(slot, c, 11, 0, 0)
		if err != nil {
			return err
		}
		if child >= 0 {
			s.Slots[child].Data[0] = e.Data[0]
			e.Data[0]--
			if e.Data[0] == 0 {
				e.Data[0] = 4
			}
		}
	case 4:
		e.scrollOnly(c)
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		accepted, err := s.drawMortarLauncher(e, c, e.Frame)
		if err != nil || !accepted {
			return err
		}
		if e.Frame == 0 {
			e.AnimationDelay = 160
			e.shooterRoutine(2)
		} else {
			e.Frame--
		}
	case 5:
		accepted, err := s.drawMortarLauncher(e, c, 3)
		if err != nil || !accepted {
			return err
		}
		s.BossPartsDestroyed++
		e.advance()
	}
	return nil
}

// setMachineCollisionColumns is E98C, whose two inputs describe the left and
// right 16-pixel columns independently. It returns its original final write
// cursor because EB34 reuses that value as its second collision-update flag.
func (s *EnemySystem) setMachineCollisionColumns(c EnemyStepContext, address uint16, left, right uint8) (uint8, error) {
	index := int((address>>8)&7)<<4 | int((address>>4)&12) | int((address>>3)&3)
	column := int(address>>1) & 3
	if c.Terrain == nil || index+4+(column+1)/4 >= len(c.Terrain) {
		return 0, errors.New("machine collision update outside terrain")
	}
	for row := 0; row < 2; row++ {
		for col, value := range []uint8{left, right} {
			cell := column + col
			at := index + row*4 + cell/4
			shift := 6 - 2*(cell&3)
			c.Terrain[at] = c.Terrain[at]&^(uint8(3)<<shift) | ((value>>(row*2))&3)<<shift
		}
	}
	s.recordGenerationBias(0xff)
	return uint8(index + 4 + (column+2)/4), nil
}

func (s *EnemySystem) drawHangarGenerator(slot int, c EnemyStepContext) (bool, error) {
	e := &s.Slots[slot]
	e.scrollOnly(c)
	e.AnimationDelay--
	if e.AnimationDelay != 0 {
		return false, nil
	}
	if e.Frame >= 3 {
		return false, errors.New("invalid soldier generator frame")
	}
	codes := [...][2]uint8{{3, 18}, {19, 20}, {7, 21}}
	r, err := s.requestTerrain(slot, codes[e.Frame][0], c, false)
	if err != nil {
		return false, err
	}
	finish := func(accepted bool) (bool, error) {
		e.AnimationDelay = 1
		if accepted {
			e.AnimationDelay = 8
		}
		return accepted, nil
	}
	if !r.Accepted {
		return finish(false)
	}
	if !r.AddressValid {
		return false, errors.New("clipped generator collision address is not recovered")
	}
	flag, err := s.setMachineCollisionColumns(c, r.Address, 0, 15)
	if err != nil {
		return false, err
	}
	if uint16(e.Y)+20 >= 256 || e.X < 12 {
		return finish(true)
	}
	if c.ChangeTerrain == nil {
		return false, errors.New("missing generator terrain queue")
	}
	r, err = c.ChangeTerrain(EnemyTerrainRequest{X: e.X - 12, Y: e.Y + 20, Code: codes[e.Frame][1]})
	if err != nil {
		return false, err
	}
	if !r.Accepted {
		return finish(false)
	}
	if flag == 0 {
		if !r.AddressValid {
			return false, errors.New("missing lower generator collision address")
		}
		if _, err = s.setMachineCollisionColumns(c, r.Address, 1, 1); err != nil {
			return false, err
		}
	}
	return finish(true)
}

func (s *EnemySystem) StepHangarSoldierGenerator(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 6 {
		return errors.New("invalid hangar soldier generator context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 24 || e.Routine > 5 {
		return errors.New("invalid hangar soldier generator state")
	}
	switch e.Routine {
	case 1:
		e.AnimationDelay = 160
		e.advance()
	case 2:
		if e.Data[2] >= 30 || s.BossPartsDestroyed >= 2 {
			e.AnimationDelay = 240
		}
		accepted, err := s.drawHangarGenerator(slot, c)
		if err != nil || !accepted {
			return err
		}
		if e.Frame < 2 {
			e.Frame++
			return nil
		}
		e.Data[2]++
		e.Data[1] = 0
		if c.Players[0].X >= 160 || c.Players[1].X >= 160 {
			e.Data[1] = 1
		}
		e.Data[0] = [...]uint8{3, 4, 2, 4}[c.Random&3]
		e.AttackDelay = 16
		e.AnimationDelay = 128
		e.advance()
	case 3:
		e.scrollOnly(c)
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.AnimationDelay = 1
			e.advance()
			return nil
		}
		e.AttackDelay--
		if e.AttackDelay != 0 {
			return nil
		}
		child, err := s.spawnMachineRelative(slot, c, 5, -8, 0)
		if err != nil {
			return err
		}
		// EABD leaves Y at the parent on allocation failure; B3E1 still
		// writes its attribute. The original does not guard this write.
		if child < 0 {
			child = slot
		}
		attr := e.Data[1]
		if attr == 0 && (e.Data[2] >= 20 || s.BossPartsDestroyed != 0) {
			attr = (c.Random >> 1) & 1
		}
		s.Slots[child].Attributes = attr
		e.Data[0]--
		e.AttackDelay = 16
		if e.Data[0] == 0 {
			e.AttackDelay = 255
		}
	case 4:
		accepted, err := s.drawHangarGenerator(slot, c)
		if err != nil || !accepted {
			return err
		}
		if e.Frame != 0 {
			e.Frame--
		} else {
			e.AnimationDelay = [...]uint8{240, 128, 160, 192}[(c.Random>>3)&3]
			e.shooterRoutine(2)
		}
	case 5:
		e.remove()
	}
	return nil
}
