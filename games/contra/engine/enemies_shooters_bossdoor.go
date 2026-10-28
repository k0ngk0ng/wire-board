package contra

import "errors"

// StepBossDoor follows the Japanese first-stage type17 table. Signals are
// source progression commands: the enclosing enemy loop applies them to the
// existing World/Campaign fields immediately, never to platform ratings.
func (s *EnemySystem) StepBossDoor(slot int, c EnemyStepContext) (BossDoorSignal, error) {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 0 {
		return BossDoorNoSignal, errors.New("invalid first-stage Boss door context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return BossDoorNoSignal, nil
	}
	if e.Kind != 17 || e.Routine > 7 {
		return BossDoorNoSignal, errors.New("invalid Boss door phase")
	}
	switch e.Routine {
	case 1:
		s.PendingSounds = append(s.PendingSounds, 0x33)
		e.advance()
	case 2:
		e.scrollOnly(c)
	case 3:
		// Original F9D4 stops previous audio before EA50 starts the clear cue.
		// The signal tells the outer tick to discard its earlier sound events.
		s.PendingSounds = nil
		s.PendingSounds = append(s.PendingSounds, 0x77)
		s.falcon(&EnemyCollisionContext{Level: c.Level}, &EnemyCollisionResult{})
		if err := s.StepExplosion(slot, c, ExplosionStart); err != nil {
			return BossDoorNoSignal, err
		}
		return BossDoorDefeated, nil
	case 4:
		return BossDoorNoSignal, s.StepExplosion(slot, c, ExplosionAnimate)
	case 5:
		e.Sprite = 0
		e.advance()
	case 6:
		e.Data[0] = 0
		e.AnimationDelay = 8
		e.advance()
	case 7:
		if c.Terrain == nil || e.Data[0] > 8 {
			return BossDoorNoSignal, errors.New("invalid Boss tunnel state")
		}
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			if e.Data[0] >= 8 {
				return BossDoorNoSignal, errors.New("Boss tunnel past its final offset")
			}
			e.AnimationDelay = 8
			result, err := s.requestTerrain(slot, bossDoorFrames[e.Data[0]], c, true)
			if err != nil {
				return BossDoorNoSignal, err
			}
			if !result.Accepted {
				return BossDoorNoSignal, nil
			}
			if !result.AddressValid {
				return BossDoorNoSignal, errors.New("Boss tunnel request lacks a collision address")
			}
			if err = setBossDoorCollision(c.Terrain, result.Address, bossDoorCollision[e.Data[0]]); err != nil {
				return BossDoorNoSignal, err
			}
			s.recordGenerationBias(0xff)
			x, y := e.X, e.Y
			e.Data[0]++
			if err = s.spawnDoorExplosion(x, y, c.Level); err != nil {
				return BossDoorNoSignal, err
			}
			return BossDoorNoSignal, nil
		}
		if e.AnimationDelay == 1 {
			index := int(e.Data[0]) * 2
			if bossDoorOffsets[index] == 0xff {
				e.remove()
				return BossDoorTunnelFinished, nil
			}
			e.Y += bossDoorOffsets[index]
			e.X += bossDoorOffsets[index+1]
		}
	}
	return BossDoorNoSignal, nil
}

// EA1D uses a type2 effect entity with the common explosion phase. It may fail
// to find space; the tunnel still advances, just as in the original.
func (s *EnemySystem) spawnDoorExplosion(x, y, level uint8) error {
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
	e.StateWidth = 8
	e.X, e.Y = x, y
	return nil
}
