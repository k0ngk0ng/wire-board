package contra

// BulletStep is the world context at original $8584. Terrain is the game's
// solid-background probe (including its platform/support interpretation).
// Enemy hits are resolved by the world collision pass, not by client rendering.
type BulletStep struct {
	ThrowWork                     *BaseThrowWork
	StageType, ScrollType, Scroll uint8
	SolidCollision, ScreenCleared bool
	Terrain                       func(x, y uint8) TerrainKind
}

// Hit records the original six-tick impact phase. The world calls this only
// after the authoritative enemy/background collision has actually occurred.
func (p *Projectile) Hit() { p.Routine = 2; p.Timer = 6 }

func (s *WeaponSystem) Step(c BulletStep) {
	indoor := c.StageType != 0 && c.StageType&0x80 == 0
	for i := range s.Projectiles {
		p := &s.Projectiles[i]
		if p.Kind == 0 {
			continue
		}
		weapon := p.Kind & 15
		if weapon < 1 || weapon > 5 || p.Routine > 2 {
			panic("invalid Contra projectile state")
		}
		family := 0
		if indoor {
			family = 1
		}
		c.ThrowWork.record(projectileFamilyIdentity[family][weapon-1])
		if p.Routine == 2 {
			if c.ScreenCleared {
				p.clear()
				continue
			}
			p.Sprite = 0x47
			p.scroll(c)
			p.Timer--
			if p.Timer == 0 {
				p.clear()
			}
			continue
		}
		if p.Routine == 0 {
			if weapon != 5 {
				p.Routine++
				continue
			}
			if !indoor {
				if c.ScrollType == 0 {
					p.X -= c.Scroll
				} else {
					p.Y += c.Scroll
				}
			}
			p.Timer--
			if p.Timer != 0 {
				continue
			}
			p.Routine++
			if indoor {
				p.Timer = 21
				p.indoorLaserSprite()
			} else {
				p.Sprite, p.Attributes = laserPoses[p.Aim][0], laserPoses[p.Aim][1]
			}
			continue
		}
		if indoor {
			switch weapon {
			case 1, 2, 5:
				p.advance(c)
			case 3:
				p.advanceCenter()
				p.indoorFlame(c)
			case 4:
				p.Sprite = 31
				if p.Timer < 26 {
					p.Sprite = 32
				}
				if p.Timer < 10 {
					p.Sprite = 33
				}
				p.indoorSpread()
			}
			p.Timer--
			if p.Timer == 0 {
				p.Hit()
			}
			continue
		}
		switch weapon {
		case 3:
			if p.hitSolid(c) {
				continue
			}
			if c.Scroll != 0 {
				if c.ScrollType == 0 {
					p.CenterX--
				} else {
					p.CenterY += c.Scroll
				}
			}
			p.advanceCenter()
			phase := p.Timer & 15
			p.X, p.Y = p.CenterX+flameXOffsets[phase], p.CenterY+flameYOffsets[phase]
			if p.Aim == 10 || p.Aim < 5 {
				p.Timer++
			} else {
				p.Timer--
			}
		default:
			if weapon == 4 {
				p.DistanceOrSpread++
				p.Sprite = 31
				if p.DistanceOrSpread >= 16 {
					p.Sprite = 32
				}
				if p.DistanceOrSpread >= 32 {
					p.Sprite = 33
				}
			}
			p.advance(c)
			p.scroll(c)
		}
		if p.X < 5 || p.X >= 251 || p.Y < 5 || p.Y >= 232 {
			p.clear()
		}
	}
}

func advanceProjectileAxis(position, fraction *uint8, velocity int16) {
	value := (uint16(*position)<<8 | uint16(*fraction)) + uint16(velocity)
	*position, *fraction = uint8(value>>8), uint8(value)
}

func (p *Projectile) hitSolid(c BulletStep) bool {
	if !c.SolidCollision {
		return false
	}
	if c.Terrain == nil {
		panic("missing Contra projectile terrain")
	}
	if c.Terrain(p.X, p.Y) != TerrainSolid {
		return false
	}
	p.Hit()
	return true
}

func (p *Projectile) advance(c BulletStep) {
	if p.hitSolid(c) {
		return
	}
	advanceProjectileAxis(&p.X, &p.XFraction, p.VX)
	advanceProjectileAxis(&p.Y, &p.YFraction, p.VY)
}

func (p *Projectile) advanceCenter() {
	advanceProjectileAxis(&p.CenterY, &p.CenterYFraction, p.VY)
	advanceProjectileAxis(&p.CenterX, &p.CenterXFraction, p.VX)
}

func (p *Projectile) scroll(c BulletStep) {
	if c.Scroll == 0 {
		return
	}
	// The shared horizontal path decrements once, unlike delayed lasers which
	// subtract the complete scroll byte. This distinction is present in the ROM.
	if c.ScrollType == 0 {
		p.X--
	} else {
		p.Y += c.Scroll
	}
}

func (p *Projectile) indoorFlame(c BulletStep) {
	if p.Timer < 2 {
		p.X, p.Y = p.CenterX, p.CenterY
		return
	}
	index := 0
	if p.RapidOrSpread != 0 {
		index = 2
	}
	phase := 2
	for phase > 0 && p.Timer < indoorFlameThresholds[index] {
		index++
		phase--
	}
	c.ThrowWork.record(indoorFlameIdentity[phase])
	offset := p.DistanceOrSpread & 15
	p.X, p.Y = p.CenterX+indoorFlameOffsets[phase][0][offset], p.CenterY+indoorFlameOffsets[phase][1][offset]
	p.DistanceOrSpread += p.RapidOrSpread + 1
}

func (p *Projectile) indoorSpread() {
	advanceProjectileAxis(&p.CenterX, &p.CenterXFraction, p.VX)
	advanceProjectileAxis(&p.Y, &p.YFraction, p.VY)
	p.XFraction = p.CenterXFraction + p.DistanceOrSpread
	p.X = p.CenterX + p.RapidOrSpread
	index := p.CenterYFraction
	if p.CenterY&1 != 0 {
		index += 5
	}
	adjustment := (uint16(p.RapidOrSpread)<<8 | uint16(p.DistanceOrSpread)) + uint16(indoorSpreadSteps[index])
	p.RapidOrSpread, p.DistanceOrSpread = uint8(adjustment>>8), uint8(adjustment)
}

func (p *Projectile) indoorLaserSprite() {
	index := 8
	for index > 0 && p.X < indoorLaserBounds[index] {
		index--
	}
	p.Sprite = indoorLaserSprites[index]
	p.Attributes = 0
	if index < 4 {
		p.Attributes = 0x40
	}
}
