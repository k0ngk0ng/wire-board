package contra

import "errors"

func giantFacePlayer(e *Enemy, c EnemyStepContext) bool {
	player := 0
	if c.Players[1].GameOver == 0 {
		player = int(c.Random & 1)
		if c.Players[0].GameOver != 0 {
			player = 1
		}
	}
	carry := e.X >= c.Players[player].X
	e.SpriteAttributes = 0
	if !carry {
		e.SpriteAttributes = 0x40
	}
	return carry
}

func giantStayStill(e *Enemy, c EnemyStepContext, carry bool) {
	e.Sprite = 0xb8
	e.VelocityX, e.VelocityY = 0, 0
	e.Data[0] = c.Random
	value := c.Random + c.Frame
	if carry {
		value++
	}
	e.AnimationDelay |= value & 0x80
	e.shooterRoutine(3)
}

func (s *EnemySystem) giantPalette(e *Enemy, c EnemyStepContext) error {
	if c.Frame&7 != 3 || e.HP >= 32 {
		return nil
	}
	if c.Scene.Header == nil || c.Scene.RefreshBackgroundPalette == nil {
		return errors.New("missing giant soldier palette context")
	}
	id := uint8(0x51)
	if e.HP < 16 {
		id = 0x52
	}
	c.Scene.Header.PaletteIndices[7] = id
	r, err := c.Scene.RefreshBackgroundPalette(32)
	if err != nil {
		return err
	}
	if r.GenerationBiasChanged {
		s.recordGenerationBias(r.GenerationBiasValue)
	}
	return nil
}

func (s *EnemySystem) giantExplosion(e *Enemy, c EnemyStepContext, dx, dy uint8, carry bool) error {
	y := uint16(e.Y) + uint16(dy)
	if carry {
		y++
	}
	x := e.X + dx
	if y >= 256 {
		x++
	}
	return s.createTerrainExplosion(x, uint8(y), c.Level, 0x89)
}

// StepGiantSoldier follows the ten Japanese ABB8..AE57 states, including the
// original eight-bit random arithmetic, palette damage feedback and scripted
// exit opening. Defeat changes campaign progress through the common callback.
func (s *EnemySystem) StepGiantSoldier(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 5 || c.WeaponStrength > 3 {
		return errors.New("invalid giant soldier context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 19 || e.Routine > 10 {
		return errors.New("invalid giant soldier state")
	}
	switch e.Routine {
	case 1:
		e.HP = 64 + c.WeaponStrength*8
		e.Data[0] = c.Random
		e.Sprite = 0xb8
		e.Y = 155
		e.AnimationDelay = 49
		e.advance()
	case 2:
		e.moveWithScroll(c)
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.advance()
		}
	case 3:
		e.moveWithScroll(c)
		switch e.Data[0] & 3 {
		case 0:
			carry := giantFacePlayer(e, c)
			index := c.Random + c.Frame
			if carry {
				index++
			}
			e.VelocityY = -1664
			e.VelocityX = [...]int16{128, 0, 0, -128}[index&3]
			e.Data[3] = 0
			e.Sprite = 0xba
			e.shooterRoutine(5)
		case 1:
			giantFacePlayer(e, c)
			e.Data[3]++
			if e.Data[3] >= 4 {
				giantStayStill(e, c, true)
			} else {
				e.AnimationDelay = 32
				e.advance()
			}
		default:
			giantFacePlayer(e, c)
			e.VelocityX = [...]int16{280, -280}[c.Random&1]
			e.Data[1] = 12
			e.shooterRoutine(6)
		}
	case 4:
		e.moveWithScroll(c)
		carry := e.X >= 8 && e.Y >= 232
		if !c.AttackEnabled {
			giantStayStill(e, c, carry)
			return nil
		}
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			if e.AnimationDelay < 15 {
				e.Sprite = 0xc3
			}
			return s.giantPalette(e, c)
		}
		child, err := s.spawnMachineRelative(slot, c, 20, -16, -24)
		if err != nil {
			return err
		}
		if child >= 0 {
			carry = e.X >= 16
			if e.SpriteAttributes&0x40 != 0 {
				x := uint16(s.Slots[child].X) + 48
				if carry {
					x++
				}
				s.Slots[child].X = uint8(x)
				carry = x >= 256
			}
		}
		giantStayStill(e, c, carry)
	case 5:
		if err := s.giantPalette(e, c); err != nil {
			return err
		}
		e.VelocityY += 0x38
		if e.X < 33 {
			e.X = 32
			e.VelocityX = 0
		} else if e.X >= 192 {
			e.X = 192
			e.VelocityX = 0
		}
		e.moveWithScroll(c)
		if e.Y >= 155 {
			if c.SoundRequestCarry == nil {
				return errors.New("missing giant landing sound return projection")
			}
			carry, err := c.SoundRequestCarry(0x2b, EnemyAudioPrefix{Sounds: append([]uint8(nil), s.PendingSounds...)})
			if err != nil {
				return err
			}
			s.PendingSounds = append(s.PendingSounds, 0x2b)
			e.VelocityX, e.VelocityY = 0, 0
			e.Y = 155
			e.Sprite = 0xb8
			// AC54 consumes the original request's carry even if a global
			// cue vetoed playback. Projection never queues the sound twice.
			giantStayStill(e, c, carry)
		}
	case 6:
		if err := s.giantPalette(e, c); err != nil {
			return err
		}
		e.moveWithScroll(c)
		if e.X < 32 || e.X >= 192 {
			giantStayStill(e, c, e.X >= 192)
			return nil
		}
		e.Data[1]--
		if e.Data[1] == 0 {
			e.Data[3] = 0
			e.Data[1] = 12
			e.Data[2]++
			e.Sprite = 0xb8 + (e.Data[2] & 1)
		}
	case 7:
		if err := s.beginMachineDefeat(c, 0x75, false); err != nil {
			return err
		}
		e.Data = [4]uint8{}
		e.Sprite = 0
		// Sound 75 leaves carry set; ADA6 consumes it in the Y addition.
		if err := s.giantExplosion(e, c, 0, 0, true); err != nil {
			return err
		}
		e.advance()
	case 8:
		e.AnimationDelay = 8
		if e.Data[0] >= 4 {
			e.AnimationDelay = 48
			e.advance()
			return nil
		}
		index := e.Data[0]
		e.Data[0]++
		offsets := [...][2]uint8{{0xf0, 0xf0}, {0x10, 0x10}, {0xf0, 0x10}, {0x10, 0xf0}}
		return s.giantExplosion(e, c, offsets[index][1], offsets[index][0], false)
	case 9:
		e.AnimationDelay--
		e.Data = [4]uint8{}
		e.Data[2] = 8
		e.AnimationDelay = 10
		e.advance()
	case 10:
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		e.AnimationDelay = 10
		if c.ChangeTiles == nil {
			return errors.New("missing giant exit graphics queue")
		}
		if e.Data[0] == 1 {
			if _, err := c.ChangeTiles(EnemyTerrainRequest{X: 208, Y: e.Data[1], Code: 0x8c}); err != nil {
				return err
			}
			e.Data[2]--
			if e.Data[2] == 0 {
				e.Data[0]++
			}
			e.Data[1] -= 8
			return nil
		}
		var y uint8
		var codes []uint8
		switch e.Data[0] {
		case 0:
			y = 0xa0
			codes = []uint8{0x8b, 0x8a}
		case 2:
			y = 0x68
			codes = []uint8{0x8d}
		default:
			return errors.New("invalid giant exit phase")
		}
		for _, code := range codes {
			if _, err := c.ChangeTiles(EnemyTerrainRequest{X: 208, Y: y, Code: code}); err != nil {
				return err
			}
			y += 16
		}
		s.recordGenerationBias(y)
		e.Data[1] = y - 32
		e.Data[0]++
		if e.Data[0] == 3 {
			if err := s.machineBossControl(c, BossControl{Kind: BossExitReady, Delay: 1}); err != nil {
				return err
			}
			e.remove()
		}
	}
	return nil
}
