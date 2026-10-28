package contra

import "errors"

// StepCapsule implements $830B/$835D. The oscillation is original fixed-point
// acceleration toward a saved anchor; it is not a substitute sine animation.
func (s *EnemySystem) StepCapsule(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) {
		return errors.New("invalid capsule slot")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 3 || e.Routine > 3 {
		return errors.New("invalid capsule phase")
	}
	switch e.Routine {
	case 1:
		e.SpriteAttributes = 3
		e.Data[0], e.Data[1] = e.Y, e.X
		if c.Vertical {
			e.X += 0x20
			e.Y = 0xe0
			e.VelocityY, e.VelocityX = -384, 0
		} else {
			e.Y += 0x20
			e.X = 0x10
			e.VelocityY, e.VelocityX = 0, 384
		}
		e.advance()
	case 2:
		e.Sprite = 0x4d
		if c.Vertical {
			e.VelocityX -= int16(int(e.X)-int(e.Data[1])) * 2
		} else {
			if e.X >= 0xf8 {
				e.StateWidth &= 0x7f
			}
			e.VelocityY -= int16(int(e.Y)-int(e.Data[0])) * 2
		}
		e.moveWithScroll(c)
	case 3:
		// Japanese $8385 jumps to the box's post-erasure conversion at $82D4.
		// A flying sprite needs no background erasure or scroll at this phase.
		return s.convertEnemyToPickup(slot, c.Level)
	}
	return nil
}
