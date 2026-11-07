package contra

// stepBasePickup is the Japanese nonzero-SceneType branch of 8007/8068/8100.
// Data[0] is the ground projection; Data[1:3] is the independent throw offset.
// A landed item is removed by perspective progress, never by an outdoor floor
// query or by stopping its retained airborne velocity fields.
func (s *EnemySystem) stepBasePickup(slot int, c EnemyStepContext) error {
	e := &s.Slots[slot]
	switch e.Routine {
	case 1:
		e.StateWidth, e.ScoreCollision, e.SpriteAttributes = 0x80, 0x22, 5
		e.Data[0] = e.Y
		e.VelocityX = [...]int16{0xaa, 0x71, 0x38, 0, -0x38, -0x71, -0xaa}[baseLane(e.X, false)]
		e.VelocityY = 0x100
		e.Data[3] = 0x80
		e.AttackDelay = 0xfd
		e.advance()
	case 2:
		e.pickupSprite(c.Frame)
		velocity := uint16(e.AttackDelay)<<8 | uint16(e.Data[3])
		velocity += 0x12
		e.AttackDelay, e.Data[3] = uint8(velocity>>8), uint8(velocity)
		offset := uint16(e.Data[2])<<8 | uint16(e.Data[1])
		offset += velocity
		e.Data[2], e.Data[1] = uint8(offset>>8), uint8(offset)
		ground := uint16(e.Data[0])<<8 | uint16(e.FractionY)
		ground += uint16(e.VelocityY)
		e.Data[0], e.FractionY = uint8(ground>>8), uint8(ground)
		if e.Data[0] >= 0xf0 {
			e.remove()
		} else {
			e.Y = e.Data[0] + e.Data[2]
			e.moveX()
			e.X -= c.FrameScroll
			if e.X < 8 {
				e.remove()
			}
		}
		if int8(e.Data[2]) >= 0 {
			e.Y = 0xa4
			e.advance()
		}
	case 3:
		e.pickupSprite(c.Frame)
		if c.ScreenScroll != 0 {
			e.remove()
		}
	}
	return nil
}
