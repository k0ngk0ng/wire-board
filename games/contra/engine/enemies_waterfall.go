package contra

import "errors"

// StepWaterfallHazard follows Japanese bank0/1 $9804..$992C. Enemy slots,
// fractions and terrain are the ordinary semantic World state, never ROM RAM.
func (s *EnemySystem) StepWaterfallHazard(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 2 {
		return errors.New("invalid Waterfall hazard context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind < 16 || e.Kind > 19 {
		return errors.New("invalid Waterfall hazard kind")
	}
	limits := [4]uint8{2, 2, 3, 6}
	if e.Routine > limits[e.Kind-16] || (e.Kind == 16 || e.Kind == 17) && e.Routine == 1 && e.Attributes >= 4 || e.Kind == 19 && e.Routine == 3 && c.Terrain == nil {
		return errors.New("invalid Waterfall hazard state")
	}
	switch e.Kind {
	case 16, 17:
		if e.Routine == 1 {
			e.VelocityX = [4]int16{-128, 192, -128, 128}[e.Attributes]
			e.Data[1] = [4]uint8{0x50, 0x70, 0x48, 0x48}[e.Attributes]
			e.Data[0] = [4]uint8{0xb0, 0xc0, 0xb8, 0xb8}[e.Attributes]
			e.scrollOnly(c)
			e.advance()
			return nil
		}
		e.Sprite = 0x48
		if e.Kind == 17 {
			e.Sprite = 0x49
			e.SpriteAttributes = 0
			if c.Frame&8 != 0 {
				e.SpriteAttributes = 0x40
			}
		}
		e.moveWithScroll(c)
		if e.VelocityX < 0 && e.X < e.Data[1] || e.VelocityX >= 0 && e.X >= e.Data[0] {
			e.VelocityX = -e.VelocityX
		}
	case 18:
		switch e.Routine {
		case 1:
			e.scrollOnly(c)
			e.advance()
		case 2:
			e.scrollOnly(c)
			e.AnimationDelay = 8
			e.advance()
		case 3:
			e.moveWithScroll(c)
			e.AnimationDelay--
			if e.AnimationDelay == 0 {
				e.AnimationDelay = 0xe0
				_, err := s.spawnWaterfallRelative(slot, 19, 0, 0, c.Level)
				return err
			}
		}
	case 19:
		switch e.Routine {
		case 1:
			e.AnimationDelay = 0x40
			e.advance()
		case 2:
			e.Sprite = 0x4a
			e.moveWithScroll(c)
			if c.Frame&3 == 0 {
				if c.Frame&4 != 0 {
					e.X++
				} else {
					e.X--
				}
			}
			e.AnimationDelay--
			if e.AnimationDelay == 0 {
				e.StateWidth &= 0x7e
				e.AnimationDelay = 1
				e.advance()
			}
		case 3:
			e.SpriteAttributes = e.SpriteAttributes&0x3f | [4]uint8{0, 0x40, 0xc0, 0x80}[(c.Frame>>2)&3]
			e.Sprite = 0x4a
			// EB99 preserves ADC carry on Y+8 overflow. DFEE's floor carry
			// is set only for terrain code 1, not for solid code $80.
			if e.Y >= e.Data[0] && (uint16(e.Y)+8 >= 256 || e.terrainAt(c, 0, 8) == TerrainPlatform) {
				s.PendingSounds = append(s.PendingSounds, 0x0d)
				e.AnimationDelay = 0x40
				e.Data[0] = uint8(min(int(e.Y)+16, 255))
				e.VelocityY = -320
			}
			e.VelocityY += 0x10
			e.Data[0] = uint8(min(int(e.Data[0])+int(c.FrameScroll), 255))
			e.moveWithScroll(c)
		default:
			return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-4))
		}
	}
	return nil
}

// EAB8/EABD allocate the highest free shared slot, initialize it, then read
// the source coordinates. If removal freed the caller itself, these reads see
// the newly initialized entity; preserving old coordinates would be incorrect.
func (s *EnemySystem) spawnWaterfallRelative(source int, kind, dx, dy, level uint8) (int, error) {
	slot := s.freeSlot()
	if slot < 0 {
		return -1, nil
	}
	if err := s.Initialize(slot, kind, level); err != nil {
		return -1, err
	}
	s.Slots[slot].Y = s.Slots[source].Y + dy
	s.Slots[slot].X = s.Slots[source].X + dx
	return slot, nil
}

// StepScuba follows common fixed-bank $F0AD/$F0B2/$F0E9. Its mortar is kind
// $0B in the same shared enemy pool; this routine does not simulate that child.
func (s *EnemySystem) StepScuba(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level >= 8 {
		return errors.New("invalid scuba context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 12 || e.Routine > 6 {
		return errors.New("invalid scuba state")
	}
	switch e.Routine {
	case 1:
		e.AnimationDelay = 0x80
		e.advance()
	case 2:
		e.Sprite = 0x4b
		e.SpriteAttributes = 8
		if e.AnimationDelay&0x10 != 0 {
			e.SpriteAttributes = 0
		}
		e.scrollOnly(c)
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			if e.Y < 0xb8 {
				e.AnimationDelay = 0x10
				return nil
			}
			e.StateWidth &= 0x7e
			e.AttackDelay = 0x10
			e.AnimationDelay = 0x30
			e.advance()
		}
	case 3:
		e.Sprite = 0x4c
		e.SpriteAttributes = 0
		if e.Data[0] != 0 {
			e.Data[0]--
			e.SpriteAttributes = 8
		}
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.scrollOnly(c)
			e.AnimationDelay = 0xc0
			e.StateWidth |= 0x81
			e.setRoutine(2)
			return nil
		}
		e.AttackDelay--
		if e.AttackDelay == 0 {
			e.Data[0] = 7
			if _, err := s.spawnWaterfallRelative(slot, 11, 5, 0xe8, c.Level); err != nil {
				return err
			}
		}
		e.scrollOnly(c)
	default:
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-4))
	}
	return nil
}
