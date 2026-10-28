package contra

import "errors"

// EnemyAimQuadrant follows Japanese $F4C4: each 32-pixel band selects a nibble
// from the original lookup. It deliberately does not use a floating-point angle.
func EnemyAimQuadrant(sourceX, sourceY, targetX, targetY, precision uint8) (uint8, uint8, error) {
	if precision > 1 {
		return 0, 0, errors.New("unsupported enemy aim precision")
	}
	dx, dy := int(targetX)-int(sourceX), int(targetY)-int(sourceY)
	quadrant := uint8(0)
	if dy < 0 {
		dy = -dy
		quadrant |= 1
	}
	if dx < 0 {
		dx = -dx
		quadrant |= 2
	}
	index := (dy>>5)*4 + (dx >> 6)
	value := aimQuarter12[index]
	if precision == 1 {
		value = aimQuarter24[index]
	}
	if dx&32 == 0 {
		value >>= 4
	}
	return value & 15, quadrant, nil
}

// EnemyAimAngle returns a clockwise direction from right, in 12 or 24 steps.
func EnemyAimAngle(sourceX, sourceY, targetX, targetY, precision uint8) (uint8, error) {
	angle, quadrant, err := EnemyAimQuadrant(sourceX, sourceY, targetX, targetY, precision)
	if err != nil {
		return 0, err
	}
	half := uint8(6) * (precision + 1)
	full := half * 2
	if quadrant&2 != 0 {
		angle = half - angle
	}
	if quadrant&1 != 0 {
		angle = full - angle
		if angle >= full {
			angle = 0
		}
	}
	return angle, nil
}

// EnemyShotVelocity follows $F29A/$F30B. The inner adjustment indexes speed's
// low three bits; CreateEnemyShot separately applies the original clamp to 7.
func EnemyShotVelocity(angle, speed uint8) (int16, int16, error) {
	if angle >= 24 {
		return 0, 0, errors.New("invalid enemy shot angle")
	}
	offset := shotDirectionIndex[angle]
	scale := func(v uint8) int16 {
		n := int16(v)
		switch speed & 7 {
		case 0:
			return n >> 1
		case 1:
			return (n >> 1) + (n >> 2)
		case 2:
			return n
		case 3:
			return n + (n >> 2)
		case 4:
			return n + (n >> 1)
		case 5:
			return n + (n >> 1) + (n >> 3)
		case 6:
			return n + (n >> 1) + (n >> 2)
		default:
			return n + (n >> 1) + (n >> 2) + (n >> 3)
		}
	}
	vy, vx := scale(shotComponents[offset]), scale(shotComponents[offset+1])
	if angle >= 13 {
		vy = -vy
	}
	if angle >= 7 && angle < 18 {
		vx = -vx
	}
	return vx, vy, nil
}

// CreateEnemyShot follows $F225. It allocates only; the original initialization
// and movement phases occur when the world later reaches this enemy slot.
func (s *EnemySystem) CreateEnemyShot(c EnemyStepContext, kindAngle, speed, x, y uint8) (bool, error) {
	if c.Level >= 8 || kindAngle&31 >= 24 || kindAngle>>5 > 5 {
		return false, errors.New("unsupported enemy shot request")
	}
	// Original F225 writes the requested speed before checking attack/space.
	s.recordGenerationBias(speed)
	c.recordThrowWork(kindAngle) // F227 precedes attack/space checks.
	if kindAngle&0xe0 != 0x20 && !c.AttackEnabled {
		return false, nil
	}
	slot := s.freeSlot()
	if slot < 0 {
		return false, nil
	}
	vx, vy, err := EnemyShotVelocity(kindAngle&31, min(speed, 7))
	if err != nil {
		return false, err
	}
	if err = s.Initialize(slot, 1, c.Level); err != nil {
		return false, err
	}
	// A successful allocation then stores the original clamped speed at F269.
	s.recordGenerationBias(min(speed, 7))
	e := &s.Slots[slot]
	e.X, e.Y = x, y
	e.Data[0] = kindAngle >> 5
	e.VelocityX, e.VelocityY = vx, vy
	c.recordThrowWork(uint8(vx)) // F2AD/F2DC: signed X velocity fraction.
	return true, nil
}

func closestShooterTarget(e *Enemy, c EnemyStepContext) int {
	distance := func(x uint8) int {
		n := int(x) - int(e.X)
		if n < 0 {
			return -n
		}
		return n
	}
	a, b := distance(c.Players[0].X), distance(c.Players[1].X)
	if c.Players[0].State != 1 {
		a = 254
	}
	if c.Players[1].State != 1 {
		b = 255
	}
	if b < a {
		return 1
	}
	return 0
}
func shooterAimAtPlayer(c EnemyStepContext, actor int, x, y, precision uint8) (uint8, error) {
	actor &= 1
	if c.Players[actor].State != 1 {
		actor ^= 1
	}
	if c.Players[actor].State != 1 {
		return EnemyAimAngle(x, y, 128, 255, precision)
	}
	target := c.Players[actor]
	if c.SceneType&1 != 0 {
		target.Y = 0xb0
	}
	return EnemyAimAngle(x, y, target.X, target.Y, precision)
}
