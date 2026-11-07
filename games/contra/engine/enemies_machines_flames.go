package contra

import "errors"

// StepFlame retains all three original directional state machines, including
// the asymmetric trigger clocks and the queue-limited collision length.
func (s *EnemySystem) StepFlame(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 5 {
		return errors.New("invalid flame context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind < 16 || e.Kind > 18 || e.Routine > 4 {
		return errors.New("invalid flame state")
	}
	switch e.Routine {
	case 1:
		switch e.Kind {
		case 16:
			e.Frame = 4
			e.Attributes |= 0x80
		case 17:
			e.Attributes |= 0x40
		case 18:
			e.SpriteAttributes = 0x40
		}
		e.Y += 8
		e.VariableA = [...]uint8{0, 32, 64, 96}[(e.Attributes>>2)&3]
		e.AnimationDelay = e.VariableA
		e.advance()
	case 2:
		e.AttackDelay--
		if e.AttackDelay&7 == 0 {
			index := (e.AttackDelay>>3)&3 | e.Frame
			if index >= 8 {
				return errors.New("invalid flame idle frame")
			}
			e.Sprite = [...]uint8{1, 0xbf, 0xc0, 0xbf, 1, 0xc1, 0xc2, 0xc1}[index]
		}
		e.scrollOnly(c)
		switch e.Kind {
		case 16:
			if e.AnimationDelay != 0 {
				e.AnimationDelay--
				return nil
			}
			p := c.Players[closestShooterTarget(e, c)]
			d := int(p.X) - int(e.X)
			if d < 0 {
				d = -d
			}
			if p.State != 1 || d >= 32 {
				return nil
			}
		case 17:
			if c.Frame&0x7f != e.VariableA {
				return nil
			}
		case 18:
			e.AnimationDelay--
			if e.AnimationDelay != 0 {
				return nil
			}
			e.VariableA = c.Random & 0x3f
		}
		if e.X < 48 {
			return nil
		}
		s.PendingSounds = append(s.PendingSounds, 0x19)
		e.StateWidth &= 0xfe
		e.Data[1] = [...]uint8{5, 9, 13, 15}[e.Attributes&3]
		e.Sprite = 1
		e.Data[0], e.Data[2], e.Data[3] = 0, 0, 0
		e.AnimationDelay = 0
		e.advance()
	case 3:
		e.scrollOnly(c)
		accepted, err := s.drawFlame(e, c, false)
		if err != nil || !accepted {
			return err
		}
		e.Data[1]--
		if e.Data[1] == 0 {
			e.AnimationDelay = 16
			e.advance()
		} else {
			switch e.Kind {
			case 16:
				e.Data[3] += 8
			case 17:
				e.Data[2] -= 8
			case 18:
				e.Data[2] += 8
			}
		}
	case 4:
		e.scrollOnly(c)
		accepted, err := s.drawFlame(e, c, true)
		if err != nil || !accepted {
			return err
		}
		done := false
		switch e.Kind {
		case 16:
			e.Data[3] -= 8
			done = e.Data[3]&0x80 != 0
		case 17:
			e.Data[2] += 8
			done = e.Data[2] != 0 && e.Data[2]&0x80 == 0
		case 18:
			e.Data[2] -= 8
			done = e.Data[2]&0x80 != 0
		}
		if done {
			e.AnimationDelay = e.VariableA
			e.StateWidth |= 0x81
			e.shooterRoutine(2)
		}
	}
	return nil
}

func (s *EnemySystem) drawFlame(e *Enemy, c EnemyStepContext, retract bool) (bool, error) {
	if e.AnimationDelay != 0 {
		e.AnimationDelay--
		return false, nil
	}
	index := int(e.Kind-16) * 4
	if retract {
		index += 2
	}
	if e.Data[2]|e.Data[3] != 0 {
		index++
	}
	code := [...]uint8{0x87, 0x88, 0x80, 0x89, 0x84, 0x85, 0x80, 0x86, 0x81, 0x82, 0x80, 0x83}[index]
	base := uint8(e.X - 7)
	sum := uint16(base) + uint16(e.Data[2])
	if e.Data[2]&0x80 == 0 && sum >= 256 || e.Data[2]&0x80 != 0 && sum < 256 {
		return true, nil
	}
	if c.ChangeTiles == nil {
		return false, errors.New("missing flame graphics callback")
	}
	r, err := c.ChangeTiles(EnemyTerrainRequest{X: uint8(sum), Y: e.Y + e.Data[3] - 1, Code: code})
	if err != nil || !r.Accepted {
		return false, err
	}
	e.AnimationDelay = 0
	if e.X >= 48 {
		e.AnimationDelay = 1
	}
	e.Data[0] = e.Data[2] | e.Data[3]
	return true, nil
}
