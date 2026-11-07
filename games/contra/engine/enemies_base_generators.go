package contra

import "errors"

// StepBaseGenerator executes the original room wave cursors. Allocation is
// descending and a full pool consumes the scheduled entry just as the FC does.
func (s *EnemySystem) StepBaseGenerator(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || (c.Level != 1 && c.Level != 3) {
		return errors.New("invalid base generator context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if (e.Kind != 0x19 && e.Kind != 0x1a && e.Kind != 0x20) || e.Routine > 3 {
		return errors.New("invalid base generator state")
	}
	if e.Routine == 3 {
		e.remove()
		return nil
	}
	if e.Routine == 1 {
		e.AnimationDelay = 0x40
		if e.Kind == 0x1a {
			e.AnimationDelay = 0x60
		}
		if e.Kind == 0x20 {
			e.AnimationDelay = 0x80
		}
		e.advance()
		return nil
	}
	switch e.Kind {
	case 0x19:
		if c.Frame&1 == 0 || s.Base.GrenadeLauncher != 0 {
			return nil
		}
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		if e.Attributes >= 2 || int(c.Screen) >= len(baseSoldierWaves[e.Attributes]) {
			return errors.New("invalid indoor soldier wave room")
		}
		wave := baseSoldierWaves[e.Attributes][c.Screen]
		at := int(e.Data[0])
		if at+1 >= len(wave) {
			return errors.New("invalid indoor soldier wave cursor")
		}
		encoded, delay := wave[at], wave[at+1]
		c.recordThrowWork(encoded & 0x3f) // original 8D68; even a full pool keeps it.
		next := uint8(at + 2)
		if delay&0x80 != 0 {
			next = 0
			s.Base.AttackRounds++
			if s.Base.AttackRounds >= 7 {
				e.remove()
				return nil
			}
		}
		e.AnimationDelay = delay & 0x7f
		e.Data[0] = next
		group := encoded >> 6
		kind := [...]uint8{0x15, 0x16, 0x18, 0x17}[group]
		count := 1
		if group == 2 {
			count = 4
		}
		for i := count - 1; i >= 0; i-- {
			child := s.freeSlot()
			if child < 0 {
				return nil
			}
			if err := s.Initialize(child, kind, c.Level); err != nil {
				return err
			}
			s.Slots[child].Attributes = encoded & 0x3f
			if group == 2 {
				s.Slots[child].Data[0] = uint8(i)
			}
		}
	case 0x1a:
		if s.Base.AttackRounds >= 7 {
			e.remove()
			return nil
		}
		if c.Frame&1 == 0 {
			return nil
		}
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		program := e.Attributes & 7
		if int(program) >= len(baseRollerWaves) {
			return errors.New("unrecovered roller wave program")
		}
		wave := baseRollerWaves[program]
		for n := 0; n < len(wave); n++ {
			at := int(e.Data[0])
			if at >= len(wave) {
				return errors.New("invalid roller wave cursor")
			}
			if wave[at] == 0xff {
				at = 0
			}
			if at+1 >= len(wave) {
				return errors.New("truncated roller wave")
			}
			encoded := wave[at]
			c.recordThrowWork(encoded & 15) // original 9606, before allocation.
			e.AnimationDelay = wave[at+1]
			e.Data[0] = uint8(at + 2)
			direction := encoded >> 4
			if direction >= 7 {
				return errors.New("invalid roller wave direction")
			}
			if c.AttackEnabled {
				child := s.freeSlot()
				if child >= 0 {
					if err := s.Initialize(child, 0x11, c.Level); err != nil {
						return err
					}
					r := &s.Slots[child]
					r.X = [...]uint8{0x98, 0x90, 0x88, 0x80, 0x78, 0x70, 0x68}[direction]
					r.Y = 0x70
					r.Attributes = encoded & 15
					r.VelocityX = [...]int16{0x55, 0x38, 0x1c, 0, -0x1c, -0x38, -0x55}[direction]
					r.VelocityY = 0x80
				}
			}
			if e.AnimationDelay != 0 {
				return nil
			}
		}
		return errors.New("unterminated roller wave")
	case 0x20:
		if s.Base.PlatingDestroyed >= 3 {
			e.remove()
			return nil
		}
		if c.Frame&1 != 0 {
			return nil
		}
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		for n := 0; n < len(baseRedBlueWaves); n++ {
			at := int(e.Data[0])
			if at >= len(baseRedBlueWaves) {
				return errors.New("invalid red/blue wave cursor")
			}
			encoded := baseRedBlueWaves[at]
			e.Data[0]++
			if encoded == 0xff {
				e.Data[0] = 0
				continue
			}
			if encoded&0x80 != 0 {
				e.AnimationDelay = encoded << 1
				return nil
			}
			child := s.freeSlot()
			if child < 0 {
				continue
			}
			kind := uint8(0x1f)
			if encoded&4 != 0 {
				kind = 0x1e
			}
			if err := s.Initialize(child, kind, c.Level); err != nil {
				return err
			}
			s.Slots[child].Attributes = encoded & 3
		}
		return errors.New("unterminated red/blue wave")
	}
	return nil
}
