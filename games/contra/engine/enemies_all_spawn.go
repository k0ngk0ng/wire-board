package contra

import "errors"

// AA11 consumes a complete indoor room once. E97C only clears Sprite/Routine;
// inactive slot payloads and the prior core count survive an empty FF stream.
func (s *EnemySystem) spawnIndoorScreen(c EnemySpawnContext, script []uint8) error {
	if s.ScreenReadOffset != 0 {
		return nil
	}
	if len(script) == 0 {
		return errors.New("empty indoor enemy stream")
	}
	if script[0] == 0 && c.ScreenCleared == nil {
		return errors.New("missing authoritative screen-cleared state")
	}
	s.ScreenReadOffset = 1
	s.Base.PlatingDestroyed, s.Base.AttackRounds, s.Base.RedSoldierCreated, s.Base.GrenadeLauncher = 0, 0, 0, 0
	for i := range s.Slots {
		s.Slots[i].remove()
	}
	if script[0] == 0xff {
		return nil
	}
	s.Base.CoresRemaining = script[0]
	if script[0] == 0 {
		*c.ScreenCleared = 1
	}
	for offset, slot := 1, 15; slot >= 0; offset, slot = offset+3, slot-1 {
		if offset >= len(script) {
			return errors.New("unterminated indoor enemy stream")
		}
		if script[offset] == 0xff {
			return nil
		}
		if offset+2 >= len(script) {
			return errors.New("truncated indoor enemy entry")
		}
		position, encoded := script[offset], script[offset+1]
		if err := s.Initialize(slot, encoded&0x3f, c.Level); err != nil {
			return err
		}
		e := &s.Slots[slot]
		e.Y = position & 0xf0
		if encoded&0x80 != 0 {
			e.Y += 8
		}
		e.X = position << 4
		if encoded&0x40 != 0 {
			e.X += 8
		}
		e.Attributes = script[offset+2]
	}
	return nil
}
