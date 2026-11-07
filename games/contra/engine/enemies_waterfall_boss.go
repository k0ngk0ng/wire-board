package contra

import "errors"

// WaterfallBossContext borrows the World's live arm count. Progress callbacks
// update the existing campaign fields; no independent copy of Boss state lives
// in the enemy system. OnDefeated includes the original audio reset.
type WaterfallBossContext struct {
	ScrollComplete, PaletteTimer uint8
	DestroyedArms                *uint8
	OnDefeated                   func() error
	OnClearDelay                 func(uint8) error
}

func (s *EnemySystem) StepDragonMouth(slot int, c EnemyStepContext, b WaterfallBossContext) error {
	if slot < 0 || slot >= 16 || c.Level != 2 {
		return errors.New("invalid Dragon mouth context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 20 || e.Routine > 9 || b.DestroyedArms == nil || (e.Routine == 3 || e.Routine == 5) && e.Frame > 2 || e.Routine == 9 && e.Data[1] >= 14 || e.Routine == 6 && b.OnDefeated == nil || e.Routine == 9 && b.OnClearDelay == nil {
		return errors.New("invalid Dragon mouth state")
	}
	switch e.Routine {
	case 1:
		e.Data[0] = 0x20
		e.Data[2] = 2
		e.Frame = 1
		e.AnimationDelay = 255
		e.advance()
	case 2:
		e.scrollOnly(c)
		if b.ScrollComplete != 0 && b.PaletteTimer == 0 {
			e.AnimationDelay--
			if e.AnimationDelay == 0 {
				e.advance()
			}
		}
	case 3:
		accepted, err := s.drawDragonMouth(slot, c)
		if err != nil || !accepted {
			return err
		}
		if e.Frame < 2 {
			e.Frame++
		} else {
			e.StateWidth &= 0x7f
			e.HP = e.Data[0]
			e.AttackDelay = 6
			e.AnimationDelay = 0x70
			e.advance()
		}
	case 4:
		e.scrollOnly(c)
		if e.AttackDelay != 0 {
			e.AttackDelay--
			if e.AttackDelay == 0 {
				for _, angle := range [3]uint8{0x84, 0x86, 0x88} {
					speed := uint8(6)
					if *b.DestroyedArms >= 2 {
						speed = 7
					}
					if _, err := s.CreateEnemyShot(c, angle, speed, e.X, e.Y+8); err != nil {
						return err
					}
				}
			}
		}
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.StateWidth |= 0x80
			e.Data[0] = e.HP
			e.HP = 0xf1
			e.AnimationDelay = 6
			e.advance()
		}
	case 5:
		accepted, err := s.drawDragonMouth(slot, c)
		if err != nil || !accepted {
			return err
		}
		if e.Frame != 0 {
			e.Frame--
		} else {
			e.AnimationDelay = [3]uint8{0xc0, 0x70, 0x20}[min(*b.DestroyedArms, 2)]
			e.setRoutine(3)
		}
	case 6:
		if err := b.OnDefeated(); err != nil {
			return err
		}
		s.PendingSounds = []uint8{0x77}
		s.falcon(&EnemyCollisionContext{Level: c.Level}, &EnemyCollisionResult{})
		return s.StepExplosion(slot, c, ExplosionStart)
	case 7:
		return s.StepExplosion(slot, c, ExplosionAnimate)
	case 8:
		e.Sprite = 0
		e.advance()
	case 9:
		e.Data[2]--
		if e.Data[2] != 0 {
			return nil
		}
		e.Data[2] = 1
		index := e.Data[1]
		e.X = dragonMouthEndX[index]
		e.Y = dragonMouthEndY[index]
		result, err := s.requestTerrain(slot, dragonMouthEndTile[index], c, false)
		if err != nil || !result.Accepted {
			return err
		}
		child := s.freeSlot()
		if child >= 0 {
			if err = s.Initialize(child, 2, c.Level); err != nil {
				return err
			}
			spawn := &s.Slots[child]
			spawn.Routine = 6
			spawn.Sprite = 1
			spawn.StateWidth = 0x89
			spawn.X = dragonMouthEndX[index]
			spawn.Y = dragonMouthEndY[index]
		}
		e.Data[1]++
		if e.Data[1] >= 14 {
			if err = b.OnClearDelay(0x60); err != nil {
				return err
			}
			e.remove()
		}
	}
	return nil
}

// 9991/EB34 can accept the top tile and reject the bottom tile. The next call
// retries both; no atomic visual replacement is invented for the original.
func (s *EnemySystem) drawDragonMouth(slot int, c EnemyStepContext) (bool, error) {
	e := &s.Slots[slot]
	e.scrollOnly(c)
	e.AnimationDelay--
	if e.AnimationDelay != 0 {
		return false, nil
	}
	first, err := s.requestTerrain(slot, 0xa0+2*e.Frame, c, false)
	if err != nil {
		return false, err
	}
	accepted := first.Accepted
	if accepted && e.X >= 12 && uint16(e.Y)+20 < 256 {
		if c.ChangeTerrain == nil {
			return false, errors.New("missing Dragon mouth graphics callback")
		}
		second, e2 := c.ChangeTerrain(EnemyTerrainRequest{X: e.X - 12, Y: e.Y + 20, Code: 0xa1 + 2*e.Frame})
		if e2 != nil {
			return false, e2
		}
		accepted = second.Accepted
	}
	e.AnimationDelay = 6
	if !accepted {
		e.AnimationDelay = 1
	}
	return accepted, nil
}

// Japanese ROM bank0/1 9A7D/9A8B/9A99. The source binding is exported with
// the REA evidence, separately from the executable state machines.
var dragonMouthEndY = [14]uint8{0x20, 0x20, 0x20, 0x20, 0x40, 0x40, 0x60, 0x60, 0x80, 0x80, 0xa0, 0xa0, 0xc0, 0xc0}
var dragonMouthEndX = [14]uint8{0x50, 0xb0, 0x70, 0x90, 0x70, 0x90, 0x70, 0x90, 0x70, 0x90, 0x70, 0x90, 0x70, 0x90}
var dragonMouthEndTile = [14]uint8{0x19, 0x19, 0x19, 0x19, 0x1a, 0x1b, 0x29, 0x2a, 0x1c, 0x1d, 0x1e, 0x1f, 0x26, 0x27}
