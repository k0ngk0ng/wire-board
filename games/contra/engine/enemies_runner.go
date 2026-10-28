package contra

import "errors"

var runnerSprites = [...]uint8{0x3b, 0x3c, 0x3d, 0x3f, 0x3c, 0x3e, 0x40, 0x26, 0x73, 0x18, 0x28, 0x27}

// StepRunner advances the original running soldier's recovered travel phases.
// Shooting and destruction are separate recovery obligations and are rejected
// until their original transitions and interaction events are implemented.
func (s *EnemySystem) StepRunner(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Terrain == nil {
		return errors.New("invalid runner context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	usesRunnerSprite := e.Routine == 3 || e.Routine == 6 || e.Routine == 11
	if e.Kind != 5 || e.Data[1] > 1 || usesRunnerSprite && e.Frame >= uint8(len(runnerSprites)) {
		return errors.New("invalid runner state")
	}
	switch e.Routine {
	case 1:
		e.scrollOnly(c)
		e.snapGround(c)
		e.AnimationDelay = [...]uint8{1, 0x10, 0x20, 0x30}[(e.Attributes>>4)&3]
		e.advance()
	case 2:
		if c.Vertical {
			e.scrollOnly(c)
		} else if c.FrameScroll != 0 {
			if e.Attributes&1 != 0 {
				if c.Frame&1 == 0 {
					return nil
				}
			} else {
				e.AnimationDelay--
				if e.AnimationDelay == 0 {
					return e.activateRunner(c)
				}
			}
		}
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			return e.activateRunner(c)
		}
	case 3:
		if e.Data[2] != 0 {
			e.Frame = 10
			if e.VelocityY >= 0 {
				tile := e.terrainAt(c, 0, 16)
				if tile == TerrainSolid || tile == TerrainPlatform {
					e.Data[2], e.Frame = 0, 0
					e.snapGround(c)
					e.setRunnerVelocity(c)
					e.VelocityY = 0
					e.finishRunnerGround(c)
					return nil
				}
				if tile == TerrainWater {
					e.Routine = 10
				}
			}
			e.VelocityY += 0x10
			e.finishRunnerGround(c)
			return nil
		}
		if e.Attributes&0x0c != 0 && c.AttackEnabled {
			e.AnimationDelay--
			if e.AnimationDelay == 0 {
				e.AnimationDelay, e.AttackDelay = 0x80, 8
				index := (c.WeaponStrength&2)<<1 | c.Random&3
				e.Data[2] = [...]uint8{1, 1, 2, 1, 2, 1, 2, 2}[index]
				e.advance()
				e.runnerSprite()
				e.moveWithScroll(c)
				return nil
			}
		}
		e.VariableA++
		if e.VariableA&7 == 0 {
			e.Frame++
			if e.Frame >= 6 {
				e.Frame = 0
			}
		}
		floor := e.terrainAt(c, int8(e.VelocityX>>8), 16)
		if floor != TerrainSolid && floor != TerrainPlatform {
			if e.Data[3] < 2 && e.Attributes&2 != 0 {
				e.turnRunner(c)
			} else {
				e.Data[2]++
				target := e.nearestPlayer(c)
				base := uint8(0)
				if target.Y >= e.Y && target.Y-e.Y >= 16 {
					base = 4
				}
				choice := [...]uint8{0, 0, 4, 0, 4, 0, 4, 4}[(c.Random&3)+base]
				if choice == 0 {
					e.VelocityY, e.VelocityX = -512, -184
				} else {
					e.VelocityY, e.VelocityX = -256, -160
				}
				if e.Data[1] != 0 {
					e.VelocityX = -e.VelocityX
				}
				e.runnerSprite()
				e.moveWithScroll(c)
				return nil
			}
		}
		e.finishRunnerGround(c)
	case 4:
		return s.StepRunnerFire(slot, c)
	case 5:
		e.Frame = 11
		e.runnerSprite()
		e.StateWidth |= 0x81
		e.VelocityY, e.VelocityX = -896, 96 // Japanese $FC80 and $0060.
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
		e.runnerSprite()
		e.VelocityY += 0x30
		if e.Y < 8 {
			e.advance()
			return nil
		}
		e.moveWithScroll(c)
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.advance()
		}
	case 7, 8, 9:
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-7))
	case 10:
		e.Frame = 8
		e.Y += 16
		e.runnerSprite()
		e.scrollOnly(c)
		// The Japanese routine explicitly invokes this pair twice.
		e.runnerSprite()
		e.scrollOnly(c)
		e.AnimationDelay = 8
		e.advance()
	case 11:
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.AnimationDelay = 8
			e.Frame++
			if e.Frame >= 10 {
				e.remove()
				return nil
			}
			e.Y += 8
		}
		e.runnerSprite()
		e.scrollOnly(c)
	default:
		return errors.New("runner phase has not been recovered")
	}
	return nil
}

func (e *Enemy) activateRunner(c EnemyStepContext) error {
	if e.terrainAt(c, 0, 16) == TerrainEmpty {
		e.remove()
		return nil
	}
	e.StateWidth &= 0x7e
	e.Data[1] = e.Attributes & 1
	if e.Data[1] != 0 {
		e.X = 10
	}
	e.setRunnerVelocity(c)
	e.VelocityY = 0
	e.AnimationDelay = 16
	e.advance()
	return nil
}
func (e *Enemy) setRunnerVelocity(c EnemyStepContext) {
	if e.Data[1] == 0 {
		e.VelocityX = -256
	} else if c.Vertical {
		e.VelocityX = 256
	} else {
		e.VelocityX = 320
	}
}
func (e *Enemy) turnRunner(c EnemyStepContext) {
	e.Data[3]++
	e.Data[1] ^= 1
	e.setRunnerVelocity(c)
}
func (e *Enemy) snapGround(c EnemyStepContext) {
	base := c.Scroll.Y&15 | 0xf0
	e.Y = ((e.Y + base) & 0xf0) - base + 4
}
func (e *Enemy) terrainAt(c EnemyStepContext, dx int8, dy uint8) TerrainKind {
	y := uint16(e.Y) + uint16(dy)
	if y >= 256 {
		// EB9B records the horizontal probe before the vertical carry aborts
		// lookup. The previous low-byte result survives this early return.
		if c.AlienBoss.CollisionAddress != nil {
			*c.AlienBoss.CollisionAddress = *c.AlienBoss.CollisionAddress&255 | uint16(e.X+uint8(dx))<<8
		}
		return TerrainEmpty
	}
	return enemyTerrainSample(c, e.X+uint8(dx), uint8(y), c.Scroll).Kind
}
func (e *Enemy) nearestPlayer(c EnemyStepContext) EnemyTarget {
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
		return c.Players[1]
	}
	return c.Players[0]
}
func (e *Enemy) runnerSprite() {
	e.Sprite = runnerSprites[e.Frame]
	e.SpriteAttributes = 0x40
	if e.Data[1] != 0 {
		e.SpriteAttributes = 0
	}
	if e.Data[0] != 0 {
		e.Data[0]--
		e.SpriteAttributes |= 8
	}
}
func (e *Enemy) finishRunnerGround(c EnemyStepContext) {
	if e.solidBackground(c) {
		e.Routine = 7
		return
	}
	if e.Data[3] < 2 {
		dx := uint8(0xf8)
		if e.Data[1] != 0 {
			dx = 8
		}
		x := e.X + dx
		if x >= 16 && x < 240 && enemySolidAt(c, x, e.Y) {
			e.turnRunner(c)
		}
	}
	e.runnerSprite()
	e.moveWithScroll(c)
}
