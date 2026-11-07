package contra

import "errors"

type EnemyStepContext struct {
	ThrowWork *BaseThrowWork
	// A Base roller copies the latest named frame-work producer. Isolated callers supply their observed entry;
	// Tick replaces it in real slot/collision order, never from a RAM reader.
	InheritedThrowAttribute *uint8
	// E989's bounded adjacent-data effect: Guardian collision packing can
	// clear bits in the first five encoded graphics bytes. World owns them.
	ClearQueuedGraphicsPrefix func(offset uint8, andMask uint8) error
	SoundRequestCarry         func(uint8, EnemyAudioPrefix) (bool, error)
	AlienBoss                 AlienBossContext
	ScreenScroll              uint8
	PlayerWeapons             [2]uint8
	Scene                     EnemySceneContext
	OnBossControl             func(BossControl) error
	ScreenCleared             *uint8
	TankScroll                *uint8
	ChangeTiles               func(EnemyTerrainRequest) (EnemyTerrainResponse, error)
	SharedRandom              *RandomState
	Screen, SpawnVariantSeed  uint8
	OnBossDoorSignal          func(BossDoorSignal) error
	// Only source-verified non-player aliases may be resolved here. World should
	// provide semantic projectile projections, never a generic RAM reader.
	ShooterTargetAt           func(uint8) (EnemyTarget, error)
	WeaponStrength, GameLoop  uint8
	Level, Frame, FrameScroll uint8
	SceneType                 uint8
	Random                    uint8
	AttackEnabled             bool
	Players                   [2]EnemyTarget
	Vertical                  bool
	// Header $59 bit7: ordinary hostile bullets hit solid terrain.
	SolidBulletBackground bool
	SolidPickupBackground bool
	Terrain               *CollisionWindow
	Scroll                CollisionScroll
	ChangeTerrain         func(EnemyTerrainRequest) (EnemyTerrainResponse, error)
}

type EnemyTarget struct{ State, X, Y, GameOver uint8 }

// StepBullet follows bank0/1 $814F/$8161/$81E4 and the common remove state.
// Collision with a player is a later world phase, not a decision made here.
func (s *EnemySystem) StepBullet(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level >= 8 {
		return errors.New("invalid enemy bullet step")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	variant := e.Data[0]
	if e.Kind != 1 || e.Routine > 4 || variant > 5 || c.SolidBulletBackground && c.Terrain == nil {
		return errors.New("unsupported enemy bullet state")
	}
	switch e.Routine {
	case 1:
		e.ScoreCollision = [...]uint8{1, 5, 5, 1, 2, 0}[variant]
		e.advance()
	case 2:
		appearance := variant
		if variant == 0 && c.Level == 4 {
			appearance = 5
		}
		e.Sprite = [...]uint8{0x1e, 0x21, 0x21, 0x1e, 0x79, 7}[appearance]
		e.SpriteAttributes = [...]uint8{1, 2, 2, 1, 1, 2}[appearance]
		e.moveWithScroll(c)
		if variant == 0 && c.SolidBulletBackground && e.solidBackground(c) {
			e.remove()
			return nil
		}
		switch variant {
		case 1:
			e.VelocityY += 0x14
			if e.Y >= 0xd0 {
				e.Frame, e.AnimationDelay = 0, 1
				e.advance()
			}
		case 3:
			if e.Y >= 0xb4 || e.X < 0x20 || e.X >= 0xe0 {
				e.remove()
			}
		case 4:
			// Japanese $81A6 rotates both flip bits while retaining palette 1.
			e.SpriteAttributes = [...]uint8{1, 0x41, 0xc1, 0x81}[(c.Frame>>2)&3]
		}
	case 3:
		e.scrollOnly(c)
		e.AnimationDelay--
		if e.AnimationDelay == 0 {
			e.AnimationDelay = 8
			e.Frame++
			if e.Frame >= 3 {
				e.advance()
			} else {
				e.Sprite = [...]uint8{0x37, 0x36, 0x37}[e.Frame]
			}
		}
	case 4:
		e.remove()
	}
	return nil
}

func (e *Enemy) remove() { e.Routine, e.Sprite = 0, 0 }
func (e *Enemy) advance() {
	if e.Routine == 0 {
		e.Sprite = 0
	} else {
		e.Routine++
	}
}

// Coordinate updates retain fractions and byte wrap, including the original
// early exit ordering when the first moved axis leaves the visible screen.
func (e *Enemy) moveWithScroll(c EnemyStepContext) {
	if c.Vertical {
		e.moveY()
		e.Y += c.FrameScroll
		if e.Y >= 0xe8 {
			e.remove()
			return
		}
		e.moveX()
		if e.X < 8 {
			e.remove()
		}
		return
	}
	e.moveX()
	e.X -= c.FrameScroll
	if e.X < 8 {
		e.remove()
		return
	}
	e.moveY()
	if e.Y >= 0xe8 {
		e.remove()
	}
}
func (e *Enemy) moveX() {
	position := uint16(e.X)<<8 | uint16(e.FractionX)
	position += uint16(e.VelocityX)
	e.X, e.FractionX = uint8(position>>8), uint8(position)
}
func (e *Enemy) moveY() {
	position := uint16(e.Y)<<8 | uint16(e.FractionY)
	position += uint16(e.VelocityY)
	e.Y, e.FractionY = uint8(position>>8), uint8(position)
}
func (e *Enemy) scrollOnly(c EnemyStepContext) {
	if c.Vertical {
		e.Y += c.FrameScroll
		if e.Y >= 0xe8 {
			e.remove()
		}
	} else {
		e.X -= c.FrameScroll
		if e.X < 8 {
			e.remove()
		}
	}
}
func (e *Enemy) solidBackground(c EnemyStepContext) bool {
	return enemySolidAt(c, e.X, e.Y)
}
func enemySolidAt(c EnemyStepContext, x, y uint8) bool {
	sample := enemyTerrainSample(c, x, y, c.Scroll)
	if sample.Kind == TerrainSolid {
		return true
	}
	if sample.Kind != TerrainPlatform {
		return false
	}
	below := sample.SourceIndex&0xc0 | (sample.SourceIndex+4)&0x3f
	return (c.Terrain[below]>>(6-2*sample.Column))&3 == 3
}

// The original terrain lookup leaves its column/index pair available to a
// later Guardian collision edit when that actor's graphics request is refused.
// Publish the named lookup result, including clipped high-Y lookups; no raw
// memory reads or instruction state are introduced into the independent rules.
func enemyTerrainSample(c EnemyStepContext, x, y uint8, scroll CollisionScroll) TerrainSample {
	sample := c.Terrain.At(x, y, scroll)
	if c.AlienBoss.CollisionAddress != nil {
		*c.AlienBoss.CollisionAddress = uint16(sample.Column) | uint16(sample.SourceIndex)<<8
	}
	return sample
}
