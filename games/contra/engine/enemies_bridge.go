package contra

import "errors"

// StepBridge keeps background admission, collision deletion, secondary bursts
// and the bridge's own explosion in the original $8C6B..$8D25 sequence.
func (s *EnemySystem) StepBridge(slot int, c EnemyStepContext) error {
	if slot < 0 || slot >= len(s.Slots) || c.Level != 0 || c.Terrain == nil {
		return errors.New("invalid first-stage bridge context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 18 {
		return errors.New("wrong bridge kind")
	}
	switch e.Routine {
	case 1:
		e.scrollOnly(c)
		distance := func(p EnemyTarget, fallback int) int {
			if p.State != 1 {
				return fallback
			}
			d := int(p.X) - int(e.X)
			if d < 0 {
				return -d
			}
			return d
		}
		if min(distance(c.Players[0], 254), distance(c.Players[1], 255)) >= 24 {
			return nil
		}
		e.AnimationDelay = 1
		e.Data[1] = 0
		e.advance()
	case 2:
		e.scrollOnly(c)
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		if e.Data[0] >= 4 || e.Data[1] >= 4 {
			return errors.New("invalid bridge segment")
		}
		if e.Data[1] < 2 {
			code := [...]uint8{0, 0x1a, 0x1b, 0x1c, 0x19, 0x1c, 0x19, 0x1d}[e.Data[0]*2+e.Data[1]]
			if code != 0 {
				if c.ChangeTerrain == nil {
					return errors.New("missing bridge terrain queue")
				}
				x := e.X
				if e.Data[1]&1 == 0 {
					x -= 32
				}
				x -= 12
				r, err := c.ChangeTerrain(EnemyTerrainRequest{X: x, Y: e.Y - 12, Code: code})
				if err != nil {
					return err
				}
				if !r.Accepted {
					e.AnimationDelay = 1
					return nil
				}
				if !r.AddressValid {
					return errors.New("accepted bridge request lacks collision address")
				}
				if err := clearEnemySupertileCollision(c.Terrain, r.Address); err != nil {
					return err
				}
				s.recordGenerationBias(0xff) // E9CA's two-cell loop ends with DEC06=$FF.
			}
		}
		e.Data[1]++
		if e.Data[1] >= 4 {
			e.advance()
			return nil
		}
		s.PendingSounds = append(s.PendingSounds, 0x3d)
		e.AnimationDelay = 4
		x := e.X + [...]uint8{0x10, 0xf0, 0, 0x10}[e.Data[1]]
		y := e.Y + [...]uint8{0x1d, 0, 0xf0, 0}[e.Data[1]]
		return s.createTerrainExplosion(x, y, c.Level, 8)
	case 3, 4:
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-3))
	case 5:
		e.scrollOnly(c)
		e.Data[0]++
		if e.Data[0] >= 4 || uint16(e.X)+32 >= 256 {
			e.remove()
			return nil
		}
		e.X += 32
		e.Sprite = 1
		e.AnimationDelay = 1
		e.Data[1] = 0
		e.setRoutine(2)
	default:
		return errors.New("unknown bridge routine")
	}
	return nil
}

// E989 clears four packed collision quadrants at the accepted nametable
// address. Original byte/column progression is retained across packed bytes.
func clearEnemySupertileCollision(window *CollisionWindow, address uint16) error {
	index := int((address>>8)&7)<<4 | int((address>>4)&12) | int((address>>3)&3)
	column := int(address>>1) & 3
	if window == nil || index+4+(column+1)/4 >= len(window) {
		return errors.New("terrain collision request is outside mapped windows")
	}
	for row := 0; row < 2; row++ {
		for col := 0; col < 2; col++ {
			cell := column + col
			at := index + row*4 + cell/4
			window[at] &= ^(uint8(3) << (6 - 2*(cell&3)))
		}
	}
	return nil
}
