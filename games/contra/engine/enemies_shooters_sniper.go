package contra

import "errors"

// StepSniper follows the Japanese type-6 table at $8953. Aim is sampled after
// this enemy's scroll correction; shot allocation remains in the enemy pool.
func (s *EnemySystem) StepSniper(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) {
		return errors.New("invalid sniper slot")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	// Falcon sets attribute bit7 before selecting the fall/death routines.
	// Only live sniper states index the three-entry attribute tables.
	if e.Kind != 6 || e.Routine > 9 || e.Routine < 5 && e.Attributes > 2 {
		return errors.New("invalid sniper state")
	}
	if e.Routine >= 7 {
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-7))
	}
	if e.Routine != 1 && e.Frame >= 7 {
		return errors.New("invalid sniper animation frame")
	}
	switch e.Routine {
	case 1:
		e.AnimationDelay = sniperInitialDelay[e.Attributes]
		e.Frame = sniperInitialFrame[e.Attributes]
		e.snapGround(c)
		if e.Attributes == 1 {
			e.Y += 5
		}
		e.advance()
	case 2:
		e.sniperSprite()
		e.scrollOnly(c)
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		if e.Attributes != 0 {
			e.AnimationDelay = 8
			e.Frame++
			if e.Frame < 3 {
				return nil
			}
			if e.Attributes == 1 {
				e.Frame--
			} else {
				e.Y -= 14
				e.X++
			}
		}
		e.StateWidth &= 0x7e
		e.ScoreCollision = 0x30
		e.AttackDelay = sniperAttackDelay[e.Attributes]
		e.Data[3] = sniperShots[e.Attributes]
		e.advance()
	case 3:
		e.sniperSprite()
		e.scrollOnly(c)
		e.AttackDelay--
		if e.AttackDelay != 0 {
			return nil
		}
		e.Data[3]--
		if e.Data[3]&0x80 != 0 {
			if e.Attributes == 0 {
				e.Data[3] = sniperShots[0]
				e.AttackDelay = 0x80
				return nil
			}
			e.Frame = 3
			if e.Attributes&1 != 0 {
				e.Frame = 2
			}
			e.AnimationDelay = 0x80
			e.advance()
			return nil
		}
		e.AttackDelay = 0x18
		target := closestShooterTarget(e, c)
		e.Data[1] = 0
		if c.Players[target].X >= e.X {
			e.Data[1] = 1
		}
		angle := uint8(0)
		if e.Attributes&1 != 0 {
			if e.Data[1] == 0 {
				angle = 12
			}
		} else {
			y := e.Y
			if e.Attributes == 2 {
				y -= 16
			}
			var err error
			angle, err = shooterAimAtPlayer(c, target, e.X, y, 1)
			if err != nil {
				return err
			}
			s.recordGenerationBias(24) // F3DD: full 24-direction turn range.
		}
		folded := (angle + 6) % 24
		if folded >= 12 {
			folded = 24 - folded
		}
		pose := uint8(0)
		if folded >= 5 {
			pose = 1
		}
		if folded >= 8 {
			pose = 2
		}
		if e.Attributes != 1 {
			e.Frame = sniperFireFrame[pose]
		}
		dx := sniperFireX[pose]
		if e.Data[1]&1 != 0 {
			dx = 0 - dx
		}
		x, y := e.X+dx, e.Y+sniperFireY[pose]
		created, err := s.CreateEnemyShot(c, angle, sniperFireSpeed[e.Attributes], x, y)
		if err != nil {
			return err
		}
		if created {
			e.Data[2] = 6
		}
	case 4:
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.StateWidth |= 0x81
			e.AnimationDelay = 8
			e.Frame--
			if e.Frame == 0 {
				e.AnimationDelay = sniperHideDelay[e.Attributes]
				e.shooterRoutine(2)
			}
			if e.Attributes == 2 && e.Frame == 2 {
				e.Y += 14
				e.X--
			}
		}
		e.sniperSprite()
		e.scrollOnly(c)
	case 5:
		e.Frame = 6
		e.sniperSprite()
		e.StateWidth |= 0x81
		e.VelocityY = -896 // Original $FC80, i.e. -3.5 rather than the reference comment's -4.5.
		e.VelocityX = 96
		if e.X < 16 || e.X >= 240 {
			e.VelocityX = 0
		}
		if e.Data[1] != 0 {
			e.VelocityX = -e.VelocityX
		}
		e.scrollOnly(c)
		e.AnimationDelay = 16
		e.advance()
	case 6:
		e.sniperSprite()
		e.VelocityY += 0x30
		if e.Y >= 8 {
			e.moveWithScroll(c)
			e.AnimationDelay--
			if e.AnimationDelay != 0 {
				return nil
			}
		}
		e.advance()
	}
	return nil
}

func (e *Enemy) sniperSprite() {
	if e.Attributes >= 2 {
		e.Sprite = sniperSprites1[e.Frame]
	} else {
		e.Sprite = sniperSprites0[e.Frame]
	}
	e.SpriteAttributes = 0x40
	if e.Data[1]&1 != 0 {
		e.SpriteAttributes = 0
	}
	if e.Data[2] != 0 {
		e.Data[2]--
		e.SpriteAttributes |= 8
	}
}
func (e *Enemy) shooterRoutine(routine uint8) {
	if e.Routine == 0 {
		e.Sprite = 0
	} else {
		e.Routine = routine
	}
}
