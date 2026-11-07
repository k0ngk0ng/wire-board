package contra

import "errors"

// B9D2 stores the frame index in the low nibble and delay in the high nibble.
// A zero high nibble underflows before the byte is packed again.
func (e *Enemy) animateAlien(base uint8) error {
	index, timer := e.AnimationDelay&15, (e.AnimationDelay>>4)-1
	if timer == 0 {
		if int(index) >= len(alienAnimationFrames) {
			return errors.New("invalid alien animation cursor")
		}
		if alienAnimationFrames[index] == 255 {
			index = 0
		}
		e.Sprite = base + alienAnimationFrames[index]
		index++
		timer = 8
	}
	e.AnimationDelay = timer<<4 + index
	return nil
}

func (s *EnemySystem) StepAlienBlob(slot int, c EnemyStepContext) error {
	e, err := alienState(s, slot, c, 19, 6)
	if err != nil || e.Routine == 0 {
		return err
	}
	switch e.Routine {
	case 1:
		e.Data[1] = (c.Random & 31) + 0x51
		e.AnimationDelay = 0xc0
		if c.Players[1].GameOver == 0 {
			e.Data[2] = (c.Random + c.Frame) & 1
			if c.Players[0].GameOver != 0 {
				e.Data[2] = 1
			}
		}
		e.Sprite = 0xb0
		angle, err := shooterAimAtPlayer(c, int(e.Data[2]), e.X, e.Y, 1)
		if err != nil {
			return err
		}
		s.recordGenerationBias(24)
		e.Data[0] = angle
		e.Routine++
		e.setAlienVelocity(int(angle))
	case 2:
		if e.Data[0] >= 24 {
			return errors.New("invalid alien blob heading")
		}
		if err = e.animateAlien(0xb0); err != nil {
			return err
		}
		e.moveWithScroll(c)
		if e.Data[3] == 0 {
			if e.AnimationDelay>>4 == 8 {
				e.VelocityY = int16(uint16(e.VelocityY)&0xff00 | uint16(uint8(e.VelocityY)+alienBlobFractionAdjust[e.Data[0]]))
				e.VelocityX = int16(uint16(e.VelocityX)&0xff00 | uint16(uint8(e.VelocityX)+alienBlobFractionAdjust[int(e.Data[0])+6]))
			}
			e.Data[1]--
			if e.Data[1] == 0 {
				e.Data[3] = (c.Random & 0x20) + 2
			}
		} else {
			e.VelocityX, e.VelocityY = 0, 0
			e.Data[3]--
			if e.Data[3] == 0 {
				e.Data[3], e.Data[1] = 8, 8
				e.Routine++
				for i := 0; i < 4; i++ {
					if _, err = s.turnAlienToward(e, c, e.Data[2], 1); err != nil {
						return err
					}
				}
			}
		}
	case 3:
		if e.Data[0] >= 24 {
			return errors.New("invalid alien blob heading")
		}
		if err = e.animateAlien(0xb0); err != nil {
			return err
		}
		if e.Data[3] != 0 {
			e.Data[1]--
			if e.Data[1] == 0 {
				e.Data[3] += 2
				e.Data[1] = e.Data[3]
				if _, err = s.turnAlienToward(e, c, e.Data[2], 1); err != nil {
					return err
				}
				e.VelocityY = 3 * alienVelocities[int(e.Data[0])+1]
				e.VelocityX = 3 * alienVelocities[int(e.Data[0])+4]
			}
		}
		e.moveWithScroll(c)
	default:
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-4))
	}
	return nil
}
