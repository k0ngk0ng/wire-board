package contra

import "errors"

type ExplosionPhase uint8

const (
	ExplosionStart ExplosionPhase = iota
	ExplosionAnimate
	ExplosionRemove
	ExplosionMortarStart
)

// StepExplosion is the common destruction sequence at $E67C/$E6E1/$E737.
// Per-kind dispatch selects these phases from its own original routine table;
// sounds, visible explosion frames and final removal retain their source order.
func (s *EnemySystem) StepExplosion(slot int, c EnemyStepContext, phase ExplosionPhase) error {
	if slot < 0 || slot >= len(s.Slots) || phase > ExplosionMortarStart {
		return errors.New("invalid explosion context")
	}
	e := &s.Slots[slot]
	switch phase {
	case ExplosionStart, ExplosionMortarStart:
		if phase == ExplosionMortarStart {
			// E683 changes the collision shape and keeps the first explosion
			// frames hazardous; E67C uses the non-contact destruction path.
			e.ScoreCollision = 0x0d
			e.StateWidth = e.StateWidth&0xbe | 0x80
		} else {
			e.StateWidth |= 0x81
		}
		if e.StateWidth&2 != 0 {
			s.PendingSounds = append(s.PendingSounds, 0x2f)
		}
		e.SpriteAttributes = e.SpriteAttributes&0xfc | 6
		if e.Sprite == 0 {
			e.remove()
			return nil
		}
		e.Frame = 0xff
		e.Sprite = 1
		e.scrollOnly(c)
		e.AnimationDelay = 1
		e.advance()
	case ExplosionAnimate:
		count := uint8(3)
		if e.StateWidth&8 != 0 {
			count = 4
		}
		e.scrollOnly(c)
		if e.Routine == 0 {
			return nil
		}
		e.AnimationDelay--
		if e.AnimationDelay != 0 {
			return nil
		}
		e.Frame++
		if e.Frame >= count {
			e.advance()
			return nil
		}
		if e.Frame+1 >= count {
			e.StateWidth |= 0x81
		}
		e.AnimationDelay = 10
		if e.StateWidth&8 != 0 {
			c.recordThrowWork(0x5f) // original large explosion table identity.
			e.Sprite = [...]uint8{0x37, 0x35, 0x36, 0x37}[e.Frame]
		} else {
			c.recordThrowWork(0x5c) // original small explosion table identity.
			e.Sprite = [...]uint8{0x38, 0x39, 0x3a}[e.Frame]
		}
	case ExplosionRemove:
		e.scrollOnly(c)
		e.remove()
	}
	return nil
}
