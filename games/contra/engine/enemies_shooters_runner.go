package contra

import "errors"

// StepRunnerFire is the original runner's state4 ($8812), kept separate from
// travel/edge logic so its bullet allocation shares the recovered shot system.
func (s *EnemySystem) StepRunnerFire(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) {
		return errors.New("invalid firing runner slot")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 5 || e.Routine != 4 || e.Data[1] > 1 {
		return errors.New("invalid firing runner state")
	}
	crouched := e.Attributes&0x0c >= 5
	e.Frame = 6
	if crouched {
		e.Frame = 7
		e.ScoreCollision = 0x1b
	}
	e.AttackDelay--
	if e.AttackDelay == 0 {
		e.Data[2]--
		if e.Data[2]&0x80 != 0 {
			e.ScoreCollision = 0x10
			e.Data[2], e.Frame = 0, 0
			e.runnerSprite()
			e.scrollOnly(c)
			e.shooterRoutine(3)
			return nil
		}
		e.AttackDelay = 16
		y := e.Y - 9
		if crouched {
			y = e.Y + 10
		}
		x := int(e.X) - 16
		angle := uint8(12)
		if e.Data[1] != 0 {
			x = int(e.X) + 16
			angle = 0
		}
		if x >= 8 && x < 256 {
			created, err := s.CreateEnemyShot(c, angle, 6, uint8(x), y)
			if err != nil {
				return err
			}
			if created {
				e.Data[0] = 6
			}
		}
	}
	e.runnerSprite()
	e.scrollOnly(c)
	return nil
}
