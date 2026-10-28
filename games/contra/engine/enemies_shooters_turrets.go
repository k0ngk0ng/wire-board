package contra

import "errors"

func (e *Enemy) pastTurretTrigger(c EnemyStepContext, x, y uint8) bool {
	if c.Vertical {
		return e.Y >= y
	}
	return e.X < x
}
func shooterTurn(current, target, full uint8) int8 {
	delta := (int(target) + int(full) - int(current)) % int(full)
	if delta == 0 {
		return 0
	}
	if delta < int(full)/2 {
		return 1
	}
	return -1
}

// StepTurret is the Japanese rotating-gun state machine. Requests pass through
// the same graphics queue as terrain scrolling; refusal preserves original waits.
func (s *EnemySystem) StepTurret(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.WeaponStrength > 3 {
		return errors.New("invalid turret context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 4 || e.Routine > 10 {
		return errors.New("invalid rotating turret state")
	}
	if e.Routine >= 8 {
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-8))
	}
	if e.Routine != 1 && e.Data[0] >= 12 {
		return errors.New("invalid rotating turret angle")
	}
	switch e.Routine {
	case 1:
		e.scrollOnly(c)
		e.Data[0] = 6
		e.advance()
	case 2:
		e.scrollOnly(c)
		if e.pastTurretTrigger(c, 0xf0, 0x30) {
			e.AnimationDelay = 8
			e.advance()
		}
	case 3:
		e.scrollOnly(c)
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		e.AnimationDelay = 8
		result, err := s.requestTerrain(slot, e.Frame+3, c, true)
		if err != nil {
			return err
		}
		if !result.Accepted {
			return nil
		}
		e.Frame++
		if e.Frame >= 3 {
			e.StateWidth &= 0x7f
			e.AnimationDelay = 8
			e.advance()
		}
	case 4:
		e.scrollOnly(c)
		if e.pastTurretTrigger(c, 0x18, 0xc8) {
			e.shooterRoutine(6)
			return nil
		}
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		e.AnimationDelay = turretRotationDelays[c.WeaponStrength]
		aim, err := shooterAimAtPlayer(c, closestShooterTarget(e, c), e.X, e.Y, 0)
		if err != nil {
			return err
		}
		s.recordGenerationBias(12) // F3DD writes even when this frame does not fire.
		turn := shooterTurn(e.Data[0], aim, 12)
		if turn > 0 {
			e.Data[0] = (e.Data[0] + 1) % 12
		} else if turn < 0 {
			e.Data[0] = (e.Data[0] + 11) % 12
		}
		aligned := e.Data[0] == aim
		code := (e.Data[0]+6)%12 + 5
		result, err := s.requestTerrain(slot, code, c, true)
		if err != nil {
			return err
		}
		if result.Accepted && aligned {
			e.Data[1] = turretBurstCounts[e.Attributes&3]
			e.AnimationDelay = 8
			e.advance()
		}
	case 5:
		e.scrollOnly(c)
		if e.pastTurretTrigger(c, 0x18, 0xc8) {
			e.shooterRoutine(6)
			return nil
		}
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		angle := e.Data[0]
		_, err := s.CreateEnemyShot(c, angle*2, 4, e.X+turretFireOffsets[angle+3], e.Y+turretFireOffsets[angle])
		if err != nil {
			return err
		}
		e.AnimationDelay = 16
		e.Data[1]--
		if e.Data[1] == 0 {
			e.AnimationDelay = turretAttackDelays[c.WeaponStrength]
			e.shooterRoutine(4)
		}
	case 6, 7:
		e.scrollOnly(c)
		code := uint8(3)
		if e.Routine == 7 {
			code = 0x16
		}
		result, err := s.requestTerrain(slot, code, c, false)
		if err != nil {
			return err
		}
		if !result.Accepted {
			return nil
		}
		if e.Routine == 6 {
			e.remove()
		} else {
			e.advance()
		}
	}
	return nil
}

// JungleShooterTarget preserves the observed original red-turret alias through
// named projectile fields. It is deliberately not a generic memory reader.
func JungleShooterTarget(index uint8, players [2]EnemyTarget, weapons *WeaponSystem) (EnemyTarget, error) {
	if index < 2 {
		return players[index], nil
	}
	if index == 0x86 && weapons != nil {
		return EnemyTarget{X: weapons.Projectiles[2].Y, Y: weapons.Projectiles[8].YFraction}, nil
	}
	return EnemyTarget{}, errors.New("unrecovered red-turret target projection")
}

func redTurretTarget(e *Enemy, c EnemyStepContext) int {
	var dx, dy [2]int
	abs := func(n int) int {
		if n < 0 {
			return -n
		}
		return n
	}
	for i, p := range c.Players {
		dx[i] = abs(int(p.X) - int(e.X))
		dy[i] = abs(int(p.Y) - int(e.Y))
		if p.State != 1 {
			dx[i], dy[i] = 254+i, 254+i
		}
	}
	nx, ny := 0, 0
	if dx[1] < dx[0] {
		nx = 1
	}
	if dy[1] < dy[0] {
		ny = 1
	}
	candidate := nx ^ 1
	first := ny ^ 1
	if first == candidate {
		candidate = ny
		first = ny
	}
	if dy[first] < dx[candidate] {
		return first
	}
	return candidate
}
func redTurretInRange(e *Enemy, target EnemyTarget) bool {
	return e.Y+32 >= target.Y && e.X >= target.X
}

func (s *EnemySystem) redTurretDraw(slot int, c EnemyStepContext) (bool, error) {
	e := &s.Slots[slot]
	e.AnimationDelay--
	if e.AnimationDelay != 0 {
		return false, nil
	}
	e.AnimationDelay = 4
	index := e.Frame
	if e.Attributes&1 != 0 {
		index += 4
	}
	if int(index) >= len(redTurretFrames) {
		return false, errors.New("invalid red-turret frame")
	}
	result, err := s.requestTerrain(slot, redTurretFrames[index], c, true)
	return result.Accepted, err
}

func (s *EnemySystem) StepRedTurret(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) {
		return errors.New("invalid red turret slot")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 7 || e.Routine > 9 {
		return errors.New("invalid red turret state")
	}
	if e.Routine >= 7 {
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-7))
	}
	if e.Routine == 4 && (e.Data[0] < 6 || e.Data[0] > 8) {
		return errors.New("invalid red turret angle")
	}
	switch e.Routine {
	case 1:
		e.scrollOnly(c)
		e.Data[0] = 6
		e.advance()
		e.scrollOnly(c)
	case 2:
		if e.pastTurretTrigger(c, 0xf0, 0x40) {
			e.AnimationDelay = 1
			e.advance()
		}
		e.scrollOnly(c)
	case 3:
		accepted, err := s.redTurretDraw(slot, c)
		if err != nil {
			return err
		}
		if accepted {
			e.Frame++
			if e.Frame >= 4 {
				e.Data[1] = 2
				e.AttackDelay = 40
				if c.GameLoop != 0 {
					e.AttackDelay = 8
				}
				e.StateWidth &= 0x7f
				e.AnimationDelay = 16
				e.advance()
			}
		}
		e.scrollOnly(c)
	case 4:
		e.scrollOnly(c)
		if e.pastTurretTrigger(c, 0x30, 0xc0) {
			e.Frame = 2
			e.StateWidth |= 0x81
			e.AnimationDelay = 1
			e.advance()
			return nil
		}
		target := redTurretTarget(e, c)
		if !redTurretInRange(e, c.Players[target]) {
			target ^= 1
		}
		aim, err := shooterAimAtPlayer(c, target, e.X, e.Y, 0)
		if err != nil {
			return err
		}
		s.recordGenerationBias(12)
		turn := shooterTurn(e.Data[0], aim, 12)
		// F3B3 selects precision0 in the shared attribute-lookup byte. A
		// successful graphics request may replace it with a source-table page.
		targetIndex := uint8(0)
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.AnimationDelay = 16
			changed := false
			if turn > 0 && e.Data[0] != 8 {
				e.Data[0]++
				changed = true
			} else if turn < 0 && e.Data[0] != 6 {
				e.Data[0]--
				changed = true
			}
			if changed {
				result, err := s.requestTerrain(slot, redTurretFrames[e.Data[0]+2], c, true)
				if err != nil {
					return err
				}
				if result.AttributeLookupWritten {
					targetIndex = result.AttributeLookup
				}
			}
		}
		e.AttackDelay--
		if e.AttackDelay != 0 {
			return nil
		}
		e.Data[1]--
		e.AttackDelay = 16
		if e.Data[1]&0x80 != 0 {
			e.Data[1] = 2
			e.AttackDelay = 80
		}
		var destination EnemyTarget
		if targetIndex < 2 {
			destination = c.Players[targetIndex]
		} else {
			if c.ShooterTargetAt == nil {
				return errors.New("missing red-turret legacy target projection")
			}
			destination, err = c.ShooterTargetAt(targetIndex)
			if err != nil {
				return err
			}
		}
		if !redTurretInRange(e, destination) {
			return nil
		}
		a := e.Data[0] - 6
		_, err = s.CreateEnemyShot(c, e.Data[0]*2, 5, e.X+redTurretFireX[a], e.Y+redTurretFireY[a])
		return err
	case 5:
		e.scrollOnly(c)
		accepted, err := s.redTurretDraw(slot, c)
		if err != nil {
			return err
		}
		if accepted {
			e.Frame--
			if e.Frame&0x80 != 0 {
				e.remove()
			}
		}
	case 6:
		e.scrollOnly(c)
		code := uint8(0x16)
		if e.Attributes&1 != 0 {
			code = 0x17
		}
		result, err := s.requestTerrain(slot, code, c, false)
		if err != nil {
			return err
		}
		if result.Accepted {
			e.advance()
		}
	}
	return nil
}

func (s *EnemySystem) bossCannonDraw(slot int, index uint8, c EnemyStepContext) (bool, error) {
	e := &s.Slots[slot]
	if e.Attributes&1 != 0 {
		index++
	}
	if int(index) >= len(bossCannonFrames) {
		return false, errors.New("invalid boss-cannon graphic")
	}
	if e.Attributes&1 != 0 {
		e.X -= 8
	}
	result, err := s.requestTerrain(slot, bossCannonFrames[index], c, true)
	// Native restoration tests the whole attribute, unlike the bit0 test
	// before drawing. Falcon's bit7 therefore moves the left cannon by +8;
	// preserve this observed path when the door destroys surviving cannons.
	if e.Attributes != 0 {
		e.X += 8
	}
	return result.Accepted, err
}
func (s *EnemySystem) StepBossCannon(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) {
		return errors.New("invalid boss cannon slot")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if c.Level != 0 || e.Kind != 16 || e.Routine > 6 || e.Attributes&0x7e != 0 {
		return errors.New("invalid first-stage boss cannon state")
	}
	if e.Routine >= 4 {
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-4))
	}
	switch e.Routine {
	case 1:
		e.AttackDelay = 32
		e.advance()
	case 2:
		if e.Data[0] != 0 && e.Data[0] != 2 {
			return errors.New("invalid boss cannon firing phase")
		}
		e.scrollOnly(c)
		e.AttackDelay--
		if e.AttackDelay != 0 {
			return nil
		}
		accepted, err := s.bossCannonDraw(slot, e.Data[0], c)
		if err != nil {
			return err
		}
		if !accepted {
			return nil
		}
		e.AttackDelay = 40
		if e.Data[0] != 0 {
			e.AttackDelay = 8
		}
		e.Data[0] ^= 2
		if e.Data[0] != 0 {
			_, err = s.CreateEnemyShot(c, 0x2e, bossCannonSpeed[c.Random&3], e.X-8, e.Y)
			return err
		}
	case 3:
		accepted, err := s.bossCannonDraw(slot, 4, c)
		if err != nil {
			return err
		}
		if accepted {
			e.advance()
		}
	}
	return nil
}
