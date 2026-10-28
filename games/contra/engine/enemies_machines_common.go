package contra

import "errors"

// spawnMachineRelative follows EABD: allocate in the shared pool, initialize
// using the current stage's properties, then read the source coordinates.
// Reading after allocation also retains the original self-reuse edge case.
func (s *EnemySystem) spawnMachineRelative(slot int, c EnemyStepContext, kind uint8, dx, dy int8) (int, error) {
	child := s.freeSlot()
	if child < 0 {
		return -1, nil
	}
	if err := s.Initialize(child, kind, c.Level); err != nil {
		return -1, err
	}
	s.Slots[child].Y = s.Slots[slot].Y + uint8(dy)
	s.Slots[child].X = s.Slots[slot].X + uint8(dx)
	return child, nil
}

// StepMortarProjectile is the common 0B actor. The first shell divides into
// three actors through the same pool and initialization path as ordinary fire.
func (s *EnemySystem) StepMortarProjectile(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level >= 8 {
		return errors.New("invalid mortar context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 11 || e.Routine > 9 {
		return errors.New("invalid mortar state")
	}
	if e.Routine >= 4 {
		if e.Routine == 7 {
			return s.StepExplosion(slot, c, ExplosionMortarStart)
		}
		phase := e.Routine - 4
		if phase >= 4 {
			phase -= 3
		}
		return s.StepExplosion(slot, c, ExplosionPhase(phase))
	}
	switch e.Routine {
	case 1:
		e.StateWidth = 0x80
		if e.Attributes == 0 {
			e.StateWidth = 0x8a
		}
		e.Sprite, e.SpriteAttributes = 0x20, 6
		index := e.Attributes
		if index == 0 && e.Data[0] != 0 {
			index = e.Data[0] + 3
		}
		// Japanese F17D, fields normalized from fractional/whole source bytes.
		velocities := [...][2]int16{{-1280, 0}, {-512, 0}, {-448, 144}, {-448, -144}, {-1280, -64}, {-1280, -128}, {-1280, -192}, {-1280, -256}}
		if index >= uint8(len(velocities)) {
			return errors.New("unrecovered mortar launch index")
		}
		e.VelocityY, e.VelocityX = velocities[index][0], velocities[index][1]
		e.advance()
	case 2:
		e.VelocityY += 0x10
		e.moveWithScroll(c)
		if e.Attributes == 0 {
			if e.VelocityY >= 0 || e.Y < 48 {
				e.advance()
			}
			return nil
		}
		if e.VelocityY < 0 || e.Y < e.nearestPlayer(c).Y {
			return nil
		}
		if c.Terrain == nil {
			return errors.New("missing mortar collision terrain")
		}
		if e.terrainAt(c, 0, 0) != TerrainEmpty {
			s.PendingSounds = append(s.PendingSounds, 0x3d)
			e.shooterRoutine(7)
		}
	case 3:
		e.moveWithScroll(c)
		for attr := uint8(3); attr != 0; attr-- {
			child := s.freeSlot()
			if child < 0 {
				break
			}
			if err := s.Initialize(child, 11, c.Level); err != nil {
				return err
			}
			s.Slots[child].X = e.X
			s.Slots[child].Y = e.Y
			s.Slots[child].Attributes = attr
		}
		e.advance()
	}
	return nil
}

func (s *EnemySystem) StepTurretMan(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level >= 8 {
		return errors.New("invalid turret soldier context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 14 || e.Routine > 6 {
		return errors.New("invalid turret soldier state")
	}
	if e.Routine >= 4 {
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-4))
	}
	switch e.Routine {
	case 1:
		e.Sprite = 0xbd
		e.AnimationDelay = e.Attributes<<4 + 1
		e.advance()
	case 2:
		e.scrollOnly(c)
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.Sprite++
			e.AnimationDelay = 5
			e.advance()
		}
	case 3:
		e.scrollOnly(c)
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		s.PendingSounds = append(s.PendingSounds, 0x1c)
		if _, err := s.spawnMachineRelative(slot, c, 15, -16, -4); err != nil {
			return err
		}
		e.AnimationDelay = e.Attributes<<4 + 48
		e.Sprite--
		e.shooterRoutine(2)
	}
	return nil
}

func (s *EnemySystem) StepTurretManBullet(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level >= 8 {
		return errors.New("invalid turret bullet context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 15 || e.Routine > 3 {
		return errors.New("invalid turret bullet state")
	}
	switch e.Routine {
	case 1:
		e.VelocityX = -640
		e.Sprite = 0x1f
		e.advance()
	case 2:
		if e.X >= 0xf0 {
			e.advance()
		} else {
			e.moveWithScroll(c)
		}
	case 3:
		e.remove()
	}
	return nil
}

func (s *EnemySystem) StepSpikedDisk(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 5 {
		return errors.New("invalid spiked disk context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 20 || e.Routine > 3 {
		return errors.New("invalid spiked disk state")
	}
	switch e.Routine {
	case 1:
		e.AnimationDelay = 6
		e.Sprite = 0xbb
		e.VelocityX = -768
		boss := 15
		for boss > 0 && s.Slots[boss].Kind != 19 {
			boss--
		}
		if s.Slots[boss].SpriteAttributes&0x40 != 0 {
			e.VelocityX = 768
		}
		e.VelocityY = 512
		e.advance()
	case 2:
		if e.X >= 0xe0 {
			e.advance()
			return nil
		}
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.AnimationDelay = 6
			e.Data[0]++
			e.Sprite = 0xbb + (e.Data[0] & 1)
		} else if e.Y >= 0xaf {
			e.VelocityY = 0
		}
		e.moveWithScroll(c)
	case 3:
		e.remove()
	}
	return nil
}
