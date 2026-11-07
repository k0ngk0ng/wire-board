package contra

import "errors"

// Each row is the original Y displacement, background code and collision
// length/effect displacement. Source tables are Japanese B097 and B10B.
var machineSpikeGrowth = [...][3]uint8{{0xc0, 0x91, 0xd0}, {0xd0, 0x91, 0xe0}, {0xe0, 0x90, 0xf0}, {0xe0, 0x8f, 0xf8}, {0xf0, 0x8e, 0}, {0xf0, 0x8d, 9}, {0xf0, 0x8c, 9}}
var machineSpikeDestruction = [...][3]uint8{{0, 0x84, 8}, {0xe0, 0x8b, 0xf0}, {0xc0, 0x8a, 0xd0}, {0xa0, 0x86, 0xb0}, {0, 0x84, 8}, {0xe0, 0x8b, 0xf0}, {0xc0, 0x86, 0xd0}}

func (s *EnemySystem) StepSpikeWall(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 6 {
		return errors.New("invalid spike wall context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 17 && e.Kind != 18 {
		return errors.New("invalid spike wall kind")
	}
	if e.Kind == 18 {
		switch e.Routine {
		case 1:
			if e.Attributes != 0 && e.Attributes != 2 {
				return errors.New("unrecovered static spike wall attribute")
			}
			e.Data[0] = 0xb8
			e.Data[3] = 4
			e.AnimationDelay = 3
			if e.Attributes == 2 {
				e.Data[3] = 0
				e.AnimationDelay = 4
			}
			e.scrollOnly(c)
			e.Attributes = 0xc0
			e.advance()
			return nil
		case 2:
			e.scrollOnly(c)
			return nil
		case 3:
			return s.startSpikeDestruction(e, c)
		case 4:
			return s.destroySpikeWall(e, c)
		default:
			return errors.New("invalid static spike wall state")
		}
	}
	switch e.Routine {
	case 1:
		trigger := [...][2]uint8{{48, 0}, {80, 15}, {112, 30}, {64, 0}}
		p := trigger[(e.Attributes>>2)&3]
		e.Data[2], e.Data[3] = p[0], p[1]
		e.AttackDelay = [...]uint8{12, 8, 4, 2}[e.Attributes&3]
		e.scrollOnly(c)
		e.Attributes = 0xc0
		e.advance()
	case 2:
		e.scrollOnly(c)
		p := e.nearestPlayer(c)
		distance := 254
		if p.State == 1 {
			distance = int(p.X) - int(e.X)
			if distance < 0 {
				distance = -distance
			}
		}
		if distance < int(e.Data[2]) {
			e.StateWidth &= 0x7e
			e.Data[1] = 6
			e.AnimationDelay = e.Data[3]
			e.advance()
		}
	case 3:
		e.scrollOnly(c)
		if e.AnimationDelay != 0 {
			e.AnimationDelay--
			return nil
		}
		if e.Data[1] >= uint8(len(machineSpikeGrowth)) || c.ChangeTerrain == nil {
			return errors.New("invalid spike growth request")
		}
		row := machineSpikeGrowth[e.Data[1]]
		e.Data[0] = row[2]
		y := uint16(e.Y) + uint16(row[0])
		x := e.X - 14
		if y >= 256 {
			x++
		}
		r, err := c.ChangeTerrain(EnemyTerrainRequest{X: x, Y: uint8(y), Code: row[1]})
		if err != nil || !r.Accepted {
			return err
		}
		if e.Data[1] < 4 {
			if !r.AddressValid {
				return errors.New("missing spike growth collision address")
			}
			if _, err = s.setMachineCollisionColumns(c, r.Address, 0, 15); err != nil {
				return err
			}
		}
		e.AnimationDelay = e.AttackDelay
		e.Data[1]--
		if e.Data[1]&0x80 != 0 {
			e.advance()
		}
	case 4:
		e.scrollOnly(c)
	case 5:
		e.Data[3] = 0
		e.AnimationDelay = 3
		return s.startSpikeDestruction(e, c)
	case 6:
		return s.destroySpikeWall(e, c)
	default:
		return errors.New("invalid rising spike wall state")
	}
	return nil
}

func (s *EnemySystem) startSpikeDestruction(e *Enemy, c EnemyStepContext) error {
	e.scrollOnly(c)
	s.PendingSounds = append(s.PendingSounds, 0x3d)
	e.advance()
	return nil
}

func (s *EnemySystem) destroySpikeWall(e *Enemy, c EnemyStepContext) error {
	e.scrollOnly(c)
	if e.Data[3] >= uint8(len(machineSpikeDestruction)) || c.ChangeTerrain == nil {
		return errors.New("invalid spike destruction request")
	}
	row := machineSpikeDestruction[e.Data[3]]
	y := uint16(e.Y) + uint16(row[0])
	offset := uint8(14)
	if y >= 256 {
		offset = 13
	}
	if e.X < offset {
		e.remove()
		return nil
	}
	r, err := c.ChangeTerrain(EnemyTerrainRequest{X: e.X - offset, Y: uint8(y), Code: row[1]})
	if err != nil || !r.Accepted {
		return err
	}
	if e.Data[3]&3 != 0 {
		if !r.AddressValid {
			return errors.New("missing spike destruction collision address")
		}
		if _, err = s.setMachineCollisionColumns(c, r.Address, 0, 0); err != nil {
			return err
		}
	}
	if err = s.createTerrainExplosion(e.X-4, e.Y+row[2], c.Level, 0x89); err != nil {
		return err
	}
	e.Data[3]++
	e.AnimationDelay--
	if e.AnimationDelay == 0 {
		e.remove()
	}
	return nil
}

func (s *EnemySystem) machineBossControl(c EnemyStepContext, control BossControl) error {
	if c.OnBossControl == nil {
		return errors.New("missing machine Boss progression callback")
	}
	return c.OnBossControl(control)
}

func (s *EnemySystem) beginMachineDefeat(c EnemyStepContext, sound uint8, destroyOthers bool) error {
	if err := s.machineBossControl(c, BossControl{Kind: BossAudioReset}); err != nil {
		return err
	}
	s.PendingSounds = nil
	s.PendingSounds = append(s.PendingSounds, sound)
	if err := s.machineBossControl(c, BossControl{Kind: BossBeginDefeat}); err != nil {
		return err
	}
	if destroyOthers {
		s.falcon(&EnemyCollisionContext{Level: c.Level}, &EnemyCollisionResult{})
	}
	return nil
}

func (s *EnemySystem) StepArmoredDoor(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 6 {
		return errors.New("invalid armored door context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 22 || e.Routine > 7 {
		return errors.New("invalid armored door state")
	}
	switch e.Routine {
	case 1:
		s.PendingSounds = append(s.PendingSounds, 0x33)
		e.advance()
	case 2:
		e.scrollOnly(c)
		if e.HP < 5 {
			s.BossPartsDestroyed = 2
		}
	case 3:
		if e.Data[0] == 0 {
			e.X += 8
			e.Data[0]++
		}
		r, err := s.requestTerrain(slot, 5, c, false)
		if err != nil {
			return err
		}
		if r.Accepted {
			e.advance()
		}
	case 4:
		if err := s.beginMachineDefeat(c, 0x77, true); err != nil {
			return err
		}
		return s.StepExplosion(slot, c, ExplosionStart)
	case 5:
		return s.StepExplosion(slot, c, ExplosionAnimate)
	case 6:
		e.Sprite = 0
		e.advance()
		e.Y += 32
	case 7:
		if e.Data[1] >= 2 {
			return errors.New("invalid armored door breach index")
		}
		r, err := s.requestTerrain(slot, [...]uint8{8, 4}[e.Data[1]], c, false)
		if err != nil || !r.Accepted {
			return err
		}
		if !r.AddressValid {
			return errors.New("missing armored door collision address")
		}
		if _, err = s.setMachineCollisionColumns(c, r.Address, 5, 5); err != nil {
			return err
		}
		if err = s.createTerrainExplosion(e.X, e.Y, c.Level, 0x89); err != nil {
			return err
		}
		e.Y += 32
		e.Data[1]++
		if e.Data[1] >= 2 {
			if err = s.machineBossControl(c, BossControl{Kind: BossExitReady, Delay: 128}); err != nil {
				return err
			}
			e.remove()
		}
	}
	return nil
}
