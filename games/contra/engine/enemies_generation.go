package contra

import "errors"

// SoldierGenerator retains the original timer/scan/commit phases. Generated is
// the original screen counter, not a count of all enemies ever created.
type SoldierGenerator struct{ Routine, Timer, X, Y, Screen, Generated uint8 }
type SoldierGenerationResult struct {
	NextVariantSeed uint8
	Spawned         []int
}

// SpawnSoldiers reproduces bank4/5 AA85. Random is read from the shared original
// accumulator when supplied; this method never creates a substitute PRNG.
// SpawnVariantSeed is a source-verified cross-phase work value, not entropy.
func (s *EnemySystem) SpawnSoldiers(c EnemyStepContext) (SoldierGenerationResult, error) {
	r := SoldierGenerationResult{NextVariantSeed: c.SpawnVariantSeed}
	if c.Level >= 8 || s.Generator.Routine > 2 {
		return r, errors.New("invalid soldier generator state")
	}
	if c.SharedRandom != nil {
		c.Random = c.SharedRandom.Value
	}
	g := &s.Generator
	if c.Screen != g.Screen {
		g.Screen = c.Screen
		g.Generated = 0
	}
	switch g.Routine {
	case 0:
		if soldierLevelTimers[c.Level] != 0 {
			g.resetTimer(c, false)
			g.Routine++
		}
	case 1:
		if c.Terrain == nil {
			return r, errors.New("soldier search requires terrain")
		}
		carry := false
		if c.Frame&1 != 0 && c.FrameScroll != 0 {
			g.Timer--
			if g.Timer != 0 {
				return r, nil
			}
			carry = true
		} else {
			old := g.Timer
			g.Timer -= 2
			if old >= 2 {
				return r, nil
			}
		}
		carry = g.resetTimer(c, carry)
		sum := uint16(c.Frame) + uint16(c.Random)
		if carry {
			sum++
		}
		y := c.Players[uint8(sum)&1].Y
		// The Japanese zero-Y branch does not transfer its toggled index back
		// to Y, so it continues using the same player's coordinate.
		sideSum := uint16(c.Frame) + uint16(c.Random)
		if sum > 255 {
			sideSum++
		}
		x := soldierSidePositions[uint8(sideSum)&15]
		if c.GameLoop == 0 && c.Level == 0 && g.Generated < 30 {
			x = 0xfc
		}
		step := uint8(0xf0)
		switch c.Frame & 3 {
		case 0:
			y = 0
			step = 16
		case 1:
			y = 0xf0
		}
		g.probe(c, x, y) // The initial result is retained, but does not stop the scan.
		for i := 0; i < 15; i++ {
			y += step
			if g.probe(c, x, y) {
				break
			}
		}
	case 2:
		defer func() { g.Routine = 0 }()
		if c.Terrain == nil {
			return r, errors.New("soldier creation requires terrain")
		}
		if c.Level != 2 && g.Y < 0x40 || g.Y >= 0xe0 || !c.AttackEnabled {
			return r, nil
		}
		right := g.X >= 0x80
		if c.Level == 4 && c.Screen >= 17 && !right {
			return r, nil
		}
		if enemySolidAt(c, g.X, g.Y) {
			return r, nil
		}
		x := g.X
		if right {
			x -= 16
		}
		x += 8
		if enemySolidAt(c, x, g.Y) {
			return r, nil
		}
		slot := s.freeSoldierSlot()
		if slot < 0 || c.Screen == 0 {
			return r, nil
		}
		profiles := soldierScreenProfiles[c.Level]
		if int(c.Screen) > len(profiles) {
			return r, errors.New("soldier screen profile is out of range")
		}
		profile := profiles[c.Screen-1]
		if profile == 0xff || profile&0x80 != 0 && c.Frame&1 != 0 || profile&0x40 != 0 && c.Frame&3 != 0 {
			return r, nil
		}
		if g.Generated < 30 && c.GameLoop == 0 {
			for i := 1; i >= 0; i-- {
				p := c.Players[i]
				if p.GameOver == 0 && (right && p.X >= 0xc0 || !right && p.X < 0x40) {
					return r, nil
				}
			}
		}
		if c.Level != 2 && c.FrameScroll != 0 && c.Random&3 == 0 {
			for delay := 2; delay >= 0; delay-- {
				r.NextVariantSeed++
				slot = s.freeSoldierSlot()
				if slot < 0 {
					break
				}
				if err := s.Initialize(slot, 5, c.Level); err != nil {
					return r, err
				}
				e := &s.Slots[slot]
				e.X, e.Y = g.X, g.Y
				e.Attributes = uint8(delay<<4) | (r.NextVariantSeed & 2)
				if !right {
					e.Attributes |= 1
				}
				r.Spawned = append(r.Spawned, slot)
			}
			// Original burst creation does not increment Generated.
			return r, nil
		}
		index := int(profile&0x3f)*4 + int(c.Frame&3)
		if index >= len(soldierInitialAttributes) {
			return r, errors.New("unknown generated soldier attribute profile")
		}
		if err := s.Initialize(slot, 5, c.Level); err != nil {
			return r, err
		}
		e := &s.Slots[slot]
		e.Attributes = soldierInitialAttributes[index]
		if (c.Random+c.Frame)&2 != 0 {
			e.Attributes |= 2
		}
		e.X, e.Y = g.X, g.Y
		if !right {
			e.Attributes++
		}
		g.Generated++
		r.Spawned = append(r.Spawned, slot)
	}
	return r, nil
}

func (g *SoldierGenerator) resetTimer(c EnemyStepContext, carry bool) bool {
	g.Timer = soldierLevelTimers[c.Level]
	subtract := func(value, count uint8) {
		for i := uint8(0); i < count; i++ {
			if g.Timer < value {
				carry = false
				break
			}
			g.Timer -= value
			carry = true
		}
	}
	if c.GameLoop != 0 {
		subtract(40, min(c.GameLoop, 3))
	}
	if c.WeaponStrength != 0 {
		subtract(5, c.WeaponStrength)
	}
	return carry
}
func (g *SoldierGenerator) probe(c EnemyStepContext, x, y uint8) bool {
	if enemyTerrainSample(c, x, y, c.Scroll).Kind != TerrainPlatform {
		return false
	}
	g.X, g.Y = x, y-16
	if g.Routine != 2 {
		g.Routine++
	}
	return true
}
func (s *EnemySystem) freeSoldierSlot() int {
	for i := 6; i >= 0; i-- {
		if s.Slots[i].Routine == 0 {
			return i
		}
	}
	return -1
}
