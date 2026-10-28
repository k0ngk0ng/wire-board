package contra

import "errors"

// StepPickup advances an original weapon item. Acquisition is handled
// by the later player/enemy collision phase, including its original immunity
// ordering; merely approaching or rendering an item never grants a weapon.
func (s *EnemySystem) StepPickup(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.SceneType == 0 && c.Terrain == nil || c.SceneType != 0 && c.Level != 1 && c.Level != 3 {
		return errors.New("unsupported pickup context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 0 || e.Routine > 3 || e.Attributes&7 > 6 {
		return errors.New("unsupported pickup state")
	}
	if c.SceneType != 0 {
		return s.stepBasePickup(slot, c)
	}
	switch e.Routine {
	case 1:
		e.StateWidth, e.ScoreCollision, e.SpriteAttributes = 0x80, 0x22, 5
		e.VelocityY = -768
		e.VelocityX = 128
		if c.Vertical {
			e.VelocityX = 64
			if e.X >= 128 {
				e.VelocityX = -64
			}
		}
		e.advance()
	case 2:
		e.pickupSprite(c.Frame)
		e.movePickup(c)
		if e.Y >= 0x20 && e.pickupGround(c) {
			base := c.Scroll.Y&15 | 0xf0
			e.Y = ((e.Y + base) & 0xf0) - base + 10
			e.VelocityX, e.VelocityY = 0, 0
			e.advance()
			return nil
		}
		bounce := false
		if e.VelocityX >= 0 {
			bounce = e.X >= 0xe8 || e.pickupWall(c, 10)
		} else {
			bounce = e.X < 0x18 || e.pickupWall(c, -10)
		}
		if bounce {
			e.VelocityX = -e.VelocityX
		}
		e.VelocityY += 0x10
	case 3:
		e.pickupSprite(c.Frame)
		e.movePickup(c)
		if !e.pickupGround(c) {
			e.setRoutine(2)
		}
	}
	return nil
}
func (e *Enemy) pickupSprite(frame uint8) {
	e.Sprite = 0
	if e.Frame != 0 {
		return
	}
	kind := e.Attributes & 7
	if kind == 6 {
		e.SpriteAttributes = (frame>>3)&3 | 4
	}
	e.Sprite = [...]uint8{0x33, 0x34, 0x31, 0x2f, 0x32, 0x30, 0x4e}[kind]
}
func (e *Enemy) movePickup(c EnemyStepContext) {
	if !c.Vertical {
		e.moveX()
		e.X -= c.FrameScroll
		if e.X < 8 {
			e.remove()
			return
		}
	}
	velocity := e.VelocityY
	if c.Vertical {
		velocity += int16(c.FrameScroll) << 8
	}
	position := int32(uint32(e.Frame)<<16 | uint32(e.Y)<<8 | uint32(e.FractionY))
	position = (position + int32(velocity)) & 0xffffff
	e.Frame, e.Y, e.FractionY = uint8(position>>16), uint8(position>>8), uint8(position)
	if e.Frame == 1 {
		e.remove()
		return
	}
	if c.Vertical {
		e.moveX()
		if e.X < 8 {
			e.remove()
		}
	}
}
func (e *Enemy) pickupGround(c EnemyStepContext) bool {
	if e.Frame&0x80 != 0 || e.VelocityY < 0 {
		return false
	}
	return e.terrainAt(c, 0, 8) != TerrainEmpty
}
func (e *Enemy) pickupWall(c EnemyStepContext, dx int8) bool {
	if !c.SolidPickupBackground {
		return false
	}
	y := e.Y
	if e.Frame != 0 || y < 16 {
		y = 16
	}
	return enemySolidAt(c, e.X+uint8(dx), y)
}
