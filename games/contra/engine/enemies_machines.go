package contra

import "errors"

// StepClaw is stage seven's original four-state mechanism (AEE8..AF8C).
// Its falling/rising length advances only after the shared queue admits the
// original two-column background animation.
func (s *EnemySystem) StepClaw(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 6 {
		return errors.New("invalid claw context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 16 || e.Routine > 4 {
		return errors.New("invalid claw state")
	}
	routine := e.Routine
	e.scrollOnly(c)
	switch routine {
	case 1:
		e.Frame = [...]uint8{0, 32, 64, 96}[(e.Attributes>>2)&3]
		e.Attributes &= 3
		e.AnimationDelay = 32
		e.advance()
	case 2:
		if e.Attributes > 3 {
			return errors.New("invalid claw length")
		}
		if e.Attributes == 3 {
			if e.AnimationDelay != 0 {
				e.AnimationDelay--
				return nil
			}
			if c.Frame >= 0xc0 {
				return nil
			}
			p := c.Players[closestShooterTarget(e, c)]
			d := int(p.X) - int(e.X)
			if d < 0 {
				d = -d
			}
			if p.State != 1 || d >= 16 {
				return nil
			}
		} else if c.Frame&0x7f != e.Frame {
			return nil
		}
		if e.X < 44 {
			return nil
		}
		e.Data[3] = e.Attributes * 2
		e.Data[1] = [...]uint8{4, 3, 8, 3}[e.Attributes]
		e.Data[2] = 0
		e.AnimationDelay = 0
		e.advance()
	case 3:
		accepted, err := s.drawClaw(e, c)
		if err != nil || !accepted {
			return err
		}
		e.Data[1]--
		if e.Data[1] == 0 {
			e.Data[3]++
			e.AnimationDelay = 8
			e.advance()
		} else {
			e.Y += 8
			e.Data[2]++
		}
	case 4:
		accepted, err := s.drawClaw(e, c)
		if err != nil || !accepted {
			return err
		}
		e.Data[2]--
		if e.Data[2]&0x80 == 0 {
			e.Y -= 8
		} else {
			e.AnimationDelay = e.Frame
			e.shooterRoutine(2)
		}
	}
	return nil
}

func (s *EnemySystem) drawClaw(e *Enemy, c EnemyStepContext) (bool, error) {
	if e.AnimationDelay != 0 {
		e.AnimationDelay--
		return false, nil
	}
	if e.Data[3] >= 8 {
		return false, errors.New("invalid claw animation phase")
	}
	length := int(e.Data[2])
	var frames []uint8
	switch e.Data[3] {
	case 0, 2, 6:
		frames = []uint8{0x86, 0x86, 0x86, 0x86}
	case 1, 3, 7:
		frames = []uint8{0x80, 0x80, 0x82, 0x84}
	case 4:
		frames = []uint8{0x86, 0x86, 0x86, 0x86, 0x86, 0x86, 0x86, 0x86}
	case 5:
		frames = []uint8{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x82, 0x84}
	}
	if length >= len(frames) || c.ChangeTiles == nil {
		return false, errors.New("invalid claw drawing context")
	}
	r, err := c.ChangeTiles(EnemyTerrainRequest{X: e.X, Y: e.Y, Code: frames[length]})
	if err != nil || !r.Accepted {
		return false, err
	}
	e.AnimationDelay = 0
	if e.X >= 44 {
		e.AnimationDelay = 2
	}
	return true, nil
}
