package contra

import "errors"

// AlienBossContext borrows named World presentation state. CollisionAddress is
// the source-derived value consumed by this phase's nametable collision writer.
// The World must project preceding terrain-query results as well as graphics
// locator results; refusal preserves that supplied value. This is not a RAM bus.
type AlienBossContext struct {
	AutoScroll, PresentationPace *uint8
	CollisionAddress             *uint16
}

func (s *EnemySystem) StepAlienGuardian(slot int, c EnemyStepContext, b AlienBossContext) error {
	if slot < 0 || slot >= 16 || c.Level != 7 || b.AutoScroll == nil || b.CollisionAddress == nil {
		return errors.New("invalid Guardian context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 16 || e.Routine > 13 || e.Routine == 5 && e.Data[0] >= 12 || (e.Routine == 10 || e.Routine == 11) && c.Terrain == nil || e.Routine == 12 && c.OnBossControl == nil {
		return errors.New("invalid Guardian state")
	}
	switch e.Routine {
	case 1:
		e.HP = alienBossHP(c)
		e.Data[3] = 0x20
		e.Data[0] = 0x90
		*b.AutoScroll = 0x40
		e.AnimationDelay = 3
		e.advance()
	case 2:
		if e.X < 0x50 {
			e.scrollOnly(c)
			return nil
		}
		if e.Data[1] != 0 {
			if err := s.alienGuardianTop(slot, c, b); err != nil {
				return err
			}
		}
		if e.Data[2] != 0 {
			if err := s.alienGuardianLower(slot, c, b); err != nil {
				return err
			}
		}
		e.Data[3]--
		if e.Data[3] == 0 {
			e.Data[3] = 0x20
			if e.Data[0] == 0x90 {
				e.Data[0] = 0x92
				e.AnimationDelay--
			} else {
				e.Data[0] = 0x90
			}
			if err := s.alienGuardianTop(slot, c, b); err != nil {
				return err
			}
			if err := s.alienGuardianLower(slot, c, b); err != nil {
				return err
			}
		}
		e.scrollOnly(c)
		if e.AnimationDelay == 0 {
			if err := s.alienGuardianLower(slot, c, b); err != nil {
				return err
			}
			e.AnimationDelay = 0x20
			e.advance()
		}
	case 3:
		e.scrollOnly(c)
		if c.AttackEnabled {
			e.AnimationDelay--
			if e.AnimationDelay > 2 || e.AnimationDelay == 2 && c.WeaponStrength < 3 {
				return nil
			}
			child := s.freeSlot()
			if child >= 0 {
				if err := s.Initialize(child, 17, c.Level); err != nil {
					return err
				}
				s.Slots[child].X = e.X
				s.Slots[child].Y = e.Y
			}
			if e.AnimationDelay != 0 {
				return nil
			}
		}
		e.AnimationDelay = 3
		e.setRoutine(2)
	case 4:
		s.PendingSounds = append(s.PendingSounds, 0x75)
		return s.alienBossExplosionStart(slot, c)
	case 5:
		return s.alienBossExplosionStep(slot, c)
	case 6:
		return s.alienGuardianBlank(slot, c, b)
	case 7:
		e.scrollOnly(c)
		if e.Data[1] != 0 {
			flag, err := s.alienBossDraw(0x83, e.X+0x10, e.Y+0x20, c, b)
			e.Data[1] = flag
			return err
		}
		e.Y -= 0x20
		err := s.alienGuardianBlank(slot, c, b)
		e.Y += 0x20
		return err
	case 8:
		e.scrollOnly(c)
		if e.Data[1] != 0 {
			flag, err := s.alienBossPattern(slot, 1, c, b)
			e.Data[1] = flag
			return err
		}
		flag, err := s.alienBossPattern(slot, 2, c, b)
		if err != nil {
			return err
		}
		if flag == 0 {
			e.Data[1]++
			e.advance()
		}
	case 9:
		e.scrollOnly(c)
		if _, err := s.alienBossDraw(0x83, e.X+0x30, e.Y, c, b); err != nil {
			return err
		}
		flag, err := s.alienBossDraw(0x83, e.X-0x10, e.Y-0x40, c, b)
		if err != nil {
			return err
		}
		if flag == 0 {
			e.Data[1]++
			e.advance()
		}
	case 10:
		e.scrollOnly(c)
		second := e.Data[1] == 0
		index := uint8(9)
		if second {
			index = 10
		}
		flag, err := s.alienBossPattern(slot, index, c, b)
		if err != nil {
			return err
		}
		if err = s.alienBossClearWall(c, b); err != nil {
			return err
		}
		e.Data[1] = flag
		if second && flag == 0 {
			e.Data[1]++
			e.advance()
		}
	case 11:
		e.scrollOnly(c)
		if e.Data[1] == 0 {
			e.advance()
			return nil
		}
		flag, err := s.alienBossPattern(slot, 11, c, b)
		if err != nil {
			return err
		}
		*b.CollisionAddress -= 0x20
		if err = s.alienBossClearWall(c, b); err != nil {
			return err
		}
		if flag == 0 {
			e.advance()
		}
	case 12:
		return s.alienBossClearScreen(slot, c)
	case 13:
		e.remove()
	}
	return nil
}

func (s *EnemySystem) StepAlienHeart(slot int, c EnemyStepContext, b AlienBossContext) error {
	if slot < 0 || slot >= 16 || c.Level != 7 || b.CollisionAddress == nil {
		return errors.New("invalid Heart context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 22 || e.Routine > 8 || e.Routine == 5 && e.Data[0] >= 12 || e.Routine == 4 && (c.Scene.Header == nil || b.PresentationPace == nil || c.OnBossControl == nil) || e.Routine == 8 && c.OnBossControl == nil {
		return errors.New("invalid Heart state")
	}
	switch e.Routine {
	case 1:
		e.HP = alienBossHP(c)
		e.AnimationDelay = 10
		e.advance()
	case 2:
		e.scrollOnly(c)
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.Data[1]++
			e.AnimationDelay = max(e.HP>>1, 1)
			e.advance()
		}
	case 3:
		e.scrollOnly(c)
		if e.Data[1] != 0 {
			e.Data[3]++
			flag, err := s.alienBossPattern(slot, 3+(e.Data[3]&1)*2, c, b)
			e.Data[1] = flag
			return err
		}
		flag, err := s.alienBossPattern(slot, 4+(e.Data[3]&1)*2, c, b)
		if err != nil {
			return err
		}
		e.Data[1] = flag
		if flag == 0 {
			e.Routine = 2
		}
	case 4:
		c.Scene.Header.CycleIndices[1] = 0x4b
		c.Scene.Header.CycleIndices[3] = 0x4b
		*b.PresentationPace = 3
		if err := c.OnBossControl(BossControl{Kind: BossAudioReset}); err != nil {
			return err
		}
		s.PendingSounds = []uint8{0x7b}
		if err := c.OnBossControl(BossControl{Kind: BossBeginDefeat}); err != nil {
			return err
		}
		return s.alienBossExplosionStart(slot, c)
	case 5:
		return s.alienBossExplosionStep(slot, c)
	case 6, 7:
		flag, err := s.alienBossPattern(slot, e.Routine+1, c, b)
		if err != nil {
			return err
		}
		if flag == 0 {
			e.advance()
		}
	case 8:
		return s.alienBossClearScreen(slot, c)
	}
	return nil
}

func alienBossHP(c EnemyStepContext) uint8 {
	return uint8(min(int(uint8(c.WeaponStrength<<4))+0x37+int(uint8(c.GameLoop<<4)), 0xa0))
}

func (s *EnemySystem) alienGuardianTop(slot int, c EnemyStepContext, b AlienBossContext) error {
	e := &s.Slots[slot]
	flag, err := s.alienBossPair(e.Data[0], e.Data[0]+1, e.X+0x10, e.Y, c, b)
	e.Data[1] = flag
	return err
}
func (s *EnemySystem) alienGuardianLower(slot int, c EnemyStepContext, b AlienBossContext) error {
	e := &s.Slots[slot]
	code := uint8(0x83)
	if e.Data[0] == 0x92 {
		code = 0x94
	}
	flag, err := s.alienBossDraw(code, e.X+0x11, e.Y+0x20, c, b)
	e.Data[2] = flag
	return err
}
func (s *EnemySystem) alienGuardianBlank(slot int, c EnemyStepContext, b AlienBossContext) error {
	e := &s.Slots[slot]
	e.scrollOnly(c)
	flag, err := s.alienBossPattern(slot, 0, c, b)
	if err != nil {
		return err
	}
	if flag == 0 {
		e.Data[1] = 1
		e.advance()
	}
	return nil
}
func (s *EnemySystem) alienBossPattern(slot int, index uint8, c EnemyStepContext, b AlienBossContext) (uint8, error) {
	if index >= 12 {
		return 1, errors.New("invalid alien Boss graphic pattern")
	}
	e := &s.Slots[slot]
	v := alienBossPatterns[index]
	return s.alienBossPair(v[0], v[1], e.X+v[3], e.Y+v[2], c, b)
}
func (s *EnemySystem) alienBossPair(right, left, x, y uint8, c EnemyStepContext, b AlienBossContext) (uint8, error) {
	s.recordGenerationBias(x)
	if _, err := s.alienBossDraw(right, x, y, c, b); err != nil {
		return 1, err
	}
	return s.alienBossDraw(left, x-0x20, y, c, b)
}
func (s *EnemySystem) alienBossDraw(code, x, y uint8, c EnemyStepContext, b AlienBossContext) (uint8, error) {
	if c.ChangeTerrain == nil {
		return 1, errors.New("missing alien Boss graphics callback")
	}
	v, err := c.ChangeTerrain(EnemyTerrainRequest{X: x - 14, Y: y - 16, Code: code})
	if err != nil {
		return 1, err
	}
	if !v.Accepted {
		return 1, nil
	}
	if !v.AddressValid {
		return 1, errors.New("alien Boss draw has no nametable address")
	}
	*b.CollisionAddress = v.Address
	return 0, nil
}

// E989 mutates the locator's low byte while deriving packed collision indices.
// B686 saves that low byte for the neighboring supertile, then leaves the second
// locator's shifted low byte in place. Refusal does not skip these original writes.
func (s *EnemySystem) alienBossClearWall(c EnemyStepContext, b AlienBossContext) error {
	address := *b.CollisionAddress
	if err := clearAlienCollisionBlock(c, b, address); err != nil {
		return err
	}
	address += 4
	if err := clearAlienCollisionBlock(c, b, address); err != nil {
		return err
	}
	s.recordGenerationBias(255)
	return nil
}

// E989 masks the retained work value rather than validating it as a PPU
// address. Its two rows/two cells can reach indices128..132: those are exactly
// the first five bytes of the shared graphics queue, not another terrain page.
// The low-byte shift precedes cell writes, including the named prefix callback.
func clearAlienCollisionBlock(c EnemyStepContext, b AlienBossContext, address uint16) error {
	if c.Terrain == nil || b.CollisionAddress == nil {
		return errors.New("missing Guardian collision state")
	}
	index := int((address>>8)&7)<<4 | int((address>>4)&12) | int((address>>3)&3)
	column := int(address>>1) & 3
	*b.CollisionAddress = address&0xff00 | uint16(uint8(address)<<5)
	for row := 0; row < 2; row++ {
		for col := 0; col < 2; col++ {
			cell := column + col
			at := index + row*4 + cell/4
			andMask := ^(uint8(3) << uint(6-2*(cell&3)))
			if at < len(c.Terrain) {
				c.Terrain[at] &= andMask
				continue
			}
			offset := at - len(c.Terrain)
			if offset >= 5 {
				return errors.New("Guardian collision prefix exceeds recovered bound")
			}
			if c.ClearQueuedGraphicsPrefix == nil {
				return errors.New("missing Guardian graphics-prefix clear callback")
			}
			if err := c.ClearQueuedGraphicsPrefix(uint8(offset), andMask); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *EnemySystem) alienBossExplosionStart(slot int, c EnemyStepContext) error {
	e := &s.Slots[slot]
	e.scrollOnly(c)
	e.Data = [4]uint8{}
	if err := s.spawnAlienBossExplosion(e.X, e.Y, c.Level); err != nil {
		return err
	}
	e.AnimationDelay = 5
	e.advance()
	return nil
}
func (s *EnemySystem) alienBossExplosionStep(slot int, c EnemyStepContext) error {
	e := &s.Slots[slot]
	e.scrollOnly(c)
	e.AnimationDelay--
	if e.AnimationDelay != 0 {
		return nil
	}
	e.AnimationDelay = 5
	e.Data[0]++
	if e.Data[0] == 12 {
		e.advance()
		return nil
	}
	offset := alienBossExplosionOffsets[e.Data[0]]
	return s.spawnAlienBossExplosion(e.X+offset[1], e.Y+offset[0], c.Level)
}
func (s *EnemySystem) spawnAlienBossExplosion(x, y, level uint8) error {
	slot := s.freeSlot()
	if slot < 0 {
		return nil
	}
	if err := s.Initialize(slot, 2, level); err != nil {
		return err
	}
	e := &s.Slots[slot]
	e.Routine = 6
	e.Sprite = 1
	e.StateWidth = 0x89
	e.X = x
	e.Y = y
	return nil
}
func (s *EnemySystem) alienBossClearScreen(slot int, c EnemyStepContext) error {
	for i := 15; i > 0; i-- {
		switch s.Slots[i].Kind {
		case 17, 18, 21:
			s.Slots[i].Routine = 3
		case 19:
			s.Slots[i].Routine = 4
		case 20:
			s.Slots[i].Routine = 6
		}
	}
	if err := c.OnBossControl(BossControl{Kind: BossExitReady, Delay: 0xa0}); err != nil {
		return err
	}
	s.Slots[slot].remove()
	return nil
}

// Japanese bank0/1 B5CD..B5FC and BD59..BD70. Each row retains original byte
// arithmetic for wrapping relative offsets and separate right/left draw codes.
var alienBossPatterns = [12][4]uint8{{0x83, 0x83, 0, 0x10}, {0x95, 0x96, 0xc0, 0x30}, {0x97, 0x83, 0xe0, 0x50}, {0x84, 0x85, 0xf0, 0x10}, {0x86, 0x87, 0x10, 0x10}, {0x88, 0x89, 0xf0, 0x10}, {0x8a, 0x8b, 0x10, 0x10}, {0x8c, 0x8d, 0xf0, 0x10}, {0x8e, 0x8f, 0x10, 0x10}, {0x83, 0x83, 0x20, 0xf0}, {0x83, 0x83, 0x40, 0xf0}, {0x29, 0x29, 0x60, 0xf0}}
var alienBossExplosionOffsets = [12][2]uint8{{0x10, 0x10}, {0xf0, 0x10}, {0xf0, 0xf0}, {0x10, 0xf0}, {0x20, 0x20}, {0xe0, 0x20}, {0xe0, 0xe0}, {0x20, 0xe0}, {0x40, 0x40}, {0xc0, 0x40}, {0xc0, 0xc0}, {0x50, 0}}
