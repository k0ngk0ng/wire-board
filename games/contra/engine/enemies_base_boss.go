package contra

import "errors"

// StepBaseBoss follows the four Japanese Base Boss type tables. Defeat signals
// change source campaign timing only; the platform match remains in progress.
func (s *EnemySystem) StepBaseBoss(slot int, c EnemyStepContext) (BossDoorSignal, error) {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 1 && c.Level != 3 || c.WeaponStrength > 3 {
		return BossDoorNoSignal, errors.New("invalid Base Boss context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return BossDoorNoSignal, nil
	}
	switch e.Kind {
	case 0x10:
		return s.stepBaseEye(slot, c)
	case 0x1b, 0x1d:
		return BossDoorNoSignal, s.stepBaseBossProjectile(slot, c)
	case 0x1c:
		return s.stepBaseGemini(slot, c)
	default:
		return BossDoorNoSignal, errors.New("unsupported Base Boss kind")
	}
}

// EAB8 creates a child at the parent's current position. A full pool is an
// ordinary missed attack; it does not defer the already-reset attack timer.
func (s *EnemySystem) baseBossChild(slot int, kind uint8, c EnemyStepContext) error {
	child := s.freeSlot()
	if child < 0 {
		return nil
	}
	if err := s.Initialize(child, kind, c.Level); err != nil {
		return err
	}
	// EACD initializes first, then reads the parent. If movement just removed
	// the parent and allocation reuses its slot, those coordinates are zero.
	s.Slots[child].X, s.Slots[child].Y = s.Slots[slot].X, s.Slots[slot].Y
	return nil
}

func (s *EnemySystem) baseBossDefeated(slot int, c EnemyStepContext) (BossDoorSignal, error) {
	s.PendingSounds = nil
	s.PendingSounds = append(s.PendingSounds, 0x77)
	s.falcon(&EnemyCollisionContext{Level: c.Level}, &EnemyCollisionResult{})
	err := s.StepExplosion(slot, c, ExplosionStart)
	return BossDoorDefeated, err
}

func (s *EnemySystem) stepBaseEye(slot int, c EnemyStepContext) (BossDoorSignal, error) {
	e := &s.Slots[slot]
	switch e.Routine {
	case 1:
		e.AnimationDelay = 64
		e.advance()
	case 2:
		if s.Base.PlatingDestroyed < 4 {
			break
		}
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			break
		}
		e.VelocityX = 0x140
		e.Data[0] = 16
		e.StateWidth &= 0x7e
		e.AttackDelay = 32
		e.AnimationDelay = 192
		e.advance()
	case 3:
		pose := uint8(0)
		if e.AnimationDelay != 0 {
			e.AnimationDelay--
			if c.Frame&4 != 0 {
				pose = 4
			}
		}
		e.Frame++
		e.Sprite = baseEyeSprites[((e.Frame>>3)&3)+pose]
		e.moveWithScroll(c)
		if e.VelocityX >= 0 && e.X >= 0xb0 || e.VelocityX < 0 && e.X < 0x50 {
			e.VelocityX = -e.VelocityX
		}
		if c.AttackEnabled {
			e.AttackDelay--
			if e.AttackDelay == 0 {
				e.AttackDelay = baseEyeAttackDelays[c.WeaponStrength]
				if err := s.baseBossChild(slot, 0x1b, c); err != nil {
					return BossDoorNoSignal, err
				}
			}
		}
	case 4:
		e.Data[0]--
		if e.Data[0] == 0 {
			e.advance()
			break
		}
		if e.Data[0] == 1 {
			e.ScoreCollision = 0x52
		}
		s.PendingSounds = append(s.PendingSounds, 0x2c)
		e.HP = 1
		e.AnimationDelay = 32
		e.shooterRoutine(3)
	case 5:
		return s.baseBossDefeated(slot, c)
	case 6:
		return BossDoorNoSignal, s.StepExplosion(slot, c, ExplosionAnimate)
	case 7:
		e.remove()
		return BossBaseFinished, nil
	default:
		return BossDoorNoSignal, errors.New("invalid Base Eye phase")
	}
	return BossDoorNoSignal, nil
}

func (s *EnemySystem) stepBaseGemini(slot int, c EnemyStepContext) (BossDoorSignal, error) {
	e := &s.Slots[slot]
	switch e.Routine {
	case 1:
		e.Data[3] = 10
		e.Data[0] = e.X
		e.VelocityX = 0x80
		e.AttackDelay = 128
		e.AnimationDelay = 64
		e.advance()
	case 2:
		if s.Base.PlatingDestroyed < 3 {
			break
		}
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			break
		}
		e.AttackDelay = 160
		e.StateWidth &= 0x7f
		e.AnimationDelay = 32
		e.advance()
	case 3:
		if c.Frame&7 == 0 {
			e.Frame++
			if e.Frame >= 3 {
				e.Frame = 0
			}
		}
		if e.Frame >= 3 {
			return BossDoorNoSignal, errors.New("invalid Gemini animation frame")
		}
		pose := e.Data[2]
		if e.Data[1] != 0 {
			e.Data[1]--
			if e.Data[1]&1 != 0 {
				pose ^= 1
			}
		}
		e.Sprite = baseGeminiSprites[e.Frame+3*(pose&1)]
		if c.AttackEnabled {
			e.AttackDelay--
			if e.AttackDelay == 0 {
				if c.SharedRandom == nil {
					return BossDoorNoSignal, errors.New("Gemini requires shared random state")
				}
				c.SharedRandom.Value += c.Frame
				e.AttackDelay = baseGeminiAttackDelays[(c.SharedRandom.Value>>1)&3] - (c.WeaponStrength << 3)
				if err := s.baseBossChild(slot, 0x1d, c); err != nil {
					return BossDoorNoSignal, err
				}
			}
		}
		moving := e.AnimationDelay == 0
		if !moving {
			e.AnimationDelay--
			if e.AnimationDelay == 0 {
				e.StateWidth |= 0x81
				moving = true
			}
		}
		if moving {
			// The original stores its signed 8.8 separation in the Y-velocity
			// fields and adds X velocity; no world Y movement happens here.
			e.VelocityY += e.VelocityX
			offset := uint8(uint16(e.VelocityY) >> 8)
			if e.VelocityX >= 0 {
				if offset >= 0x30 {
					e.AnimationDelay = 32
					e.VelocityX = -e.VelocityX
				}
			} else if offset&0x80 != 0 {
				e.VelocityY = 0
				e.StateWidth &= 0x7f
				e.AnimationDelay = 48
				e.VelocityX = -e.VelocityX
			}
		}
		offset := uint8(uint16(e.VelocityY) >> 8)
		if c.Frame&1 == 0 {
			e.X = e.Data[0] + offset
		} else {
			e.X = e.Data[0] - offset
		}
	case 4:
		if e.AnimationDelay != 0 {
			e.AnimationDelay--
		}
		e.Data[3]--
		if e.Data[3] == 0 {
			e.advance()
			break
		}
		if e.Data[3] < 7 {
			if e.Data[3] == 1 {
				e.ScoreCollision = 0x52
			}
			e.Data[2] = 1
		}
		e.HP = 1
		e.Data[1] = 16
		s.PendingSounds = append(s.PendingSounds, 0x2c)
		e.shooterRoutine(3)
	case 5:
		s.Base.CoresRemaining--
		if s.Base.CoresRemaining == 0 {
			return s.baseBossDefeated(slot, c)
		}
		return BossDoorNoSignal, s.StepExplosion(slot, c, ExplosionStart)
	case 6:
		return BossDoorNoSignal, s.StepExplosion(slot, c, ExplosionAnimate)
	case 7:
		e.remove()
		if s.Base.CoresRemaining == 0 {
			return BossBaseFinished, nil
		}
	default:
		return BossDoorNoSignal, errors.New("invalid Base Gemini phase")
	}
	return BossDoorNoSignal, nil
}

func (s *EnemySystem) stepBaseBossProjectile(slot int, c EnemyStepContext) error {
	e := &s.Slots[slot]
	if e.Routine >= 3 && e.Routine <= 5 {
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-3))
	}
	if e.Routine > 5 {
		return errors.New("invalid Base Boss projectile phase")
	}
	if e.Routine == 1 {
		actor := closestShooterTarget(e, c)
		speed := uint8(6)
		if e.Kind == 0x1d {
			e.Data[1] = uint8(actor)
			e.Attributes = c.Frame & 3
			speed = baseBubbleInitialSpeeds[e.Attributes]
		}
		// 8F58/A08D publish the speed in shared $06 before aiming. The
		// spinning bubble subsequently replaces it with 24 at F3DD; the
		// Eye projectile keeps 6 for the following original frame phases.
		s.recordGenerationBias(speed)
		angle, err := shooterAimAtPlayer(c, actor, e.X, e.Y, 1)
		if err != nil {
			return err
		}
		e.VelocityX, e.VelocityY, err = EnemyShotVelocity(angle, speed)
		if err != nil {
			return err
		}
		if e.Kind == 0x1d {
			s.recordGenerationBias(24)
			e.Data[0] = angle
			e.AttackDelay = 32
		}
		e.advance()
		return nil
	}
	if e.Kind == 0x1b {
		e.Sprite = 0x63
		if e.Y >= 0x48 {
			e.StateWidth &= 0x7e
			e.Sprite = 0x64
		}
		e.SpriteAttributes = e.SpriteAttributes&0x3f | baseEyeProjectileFlips[(c.Frame>>2)&3]
		e.moveWithScroll(c)
		return nil
	}
	if e.Attributes > 3 || e.Data[0] >= 24 {
		return errors.New("invalid spinning bubble state")
	}
	e.AnimationDelay++
	if e.AnimationDelay >= baseBubbleSpinDelays[e.Attributes] {
		e.AnimationDelay = 0
		e.Frame++
		if e.Frame >= 6 {
			e.Frame = 0
		}
	}
	e.Sprite = e.Frame + 0x6d
	e.moveWithScroll(c)
	if e.Data[2] >= 20 {
		return nil
	}
	e.AttackDelay--
	if e.AttackDelay != 0 {
		return nil
	}
	e.AttackDelay = 8
	e.Data[2]++
	aim, err := shooterAimAtPlayer(c, int(e.Data[1]), e.X, e.Y, 1)
	if err != nil {
		return err
	}
	s.recordGenerationBias(24)
	turn := shooterTurn(e.Data[0], aim, 24)
	if turn > 0 {
		e.Data[0] = (e.Data[0] + 1) % 24
	} else if turn < 0 {
		e.Data[0] = (e.Data[0] + 23) % 24
	}
	if e.Data[0] == aim {
		return nil
	}
	e.Attributes |= 3
	e.VelocityY = baseBubbleVelocities[e.Data[0]]
	e.VelocityX = baseBubbleVelocities[e.Data[0]+6]
	return nil
}
