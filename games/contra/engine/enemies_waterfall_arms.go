package contra

import "errors"

// Dragon arms use the original shared pool links: Data[2] points outward,
// Data[3] inward. The shoulder's VX fraction stores its hand slot, VX high is
// the cumulative angle, and VY holds wave direction/combined rotation timers.
func (s *EnemySystem) StepDragonArm(slot int, c EnemyStepContext, b WaterfallBossContext) error {
	if slot < 0 || slot >= 16 || c.Level != 2 {
		return errors.New("invalid Dragon arm context")
	}
	e := &s.Slots[slot]
	if e.Routine == 0 {
		return nil
	}
	if e.Kind != 21 || e.Routine > 8 || b.DestroyedArms == nil {
		return errors.New("invalid Dragon arm state")
	}
	if e.Routine >= 3 && e.Routine <= 5 {
		if _, err := s.dragonChain(slot, 2); err != nil {
			return err
		}
		if _, err := s.dragonChain(slot, 3); err != nil {
			return err
		}
		if e.Routine == 4 && e.Data[3]&0x80 != 0 && (e.Frame > 4 || uint8(e.VelocityX) >= 16) {
			return errors.New("invalid Dragon arm attack state")
		}
	}
	switch e.Routine {
	case 1:
		e.Data[0] = 0x38
		e.X += 8
		if e.Attributes&1 != 0 {
			e.Data[0] = 0x28
			e.X -= 16
		}
		e.VariableA = e.Data[0]
		e.Data[3] = 255
		e.Frame = 4
		e.Data[1] = uint8(slot)
		e.advance()
	case 2:
		if e.Data[3]&0x80 != 0 && (e.Data[1] >= 16 || e.Frame == 0 || e.Frame > 4) {
			return errors.New("invalid Dragon arm creation link")
		}
		e.scrollOnly(c)
		if b.ScrollComplete == 0 || b.PaletteTimer != 0 || e.Data[3]&0x80 == 0 {
			return nil
		}
		child := s.freeSlot()
		if child < 0 {
			return nil
		}
		if err := s.Initialize(child, 21, c.Level); err != nil {
			return err
		}
		n := &s.Slots[child]
		n.Routine = 2
		n.StateWidth = 0x8c
		n.ScoreCollision = 0x52
		n.HP = 0xf1
		n.Data[0] = 0
		n.Attributes = e.Attributes
		n.Y = e.Y
		n.X = e.X
		n.Data[3] = e.Data[1]
		previous := e.Data[1]
		e.Data[1] = uint8(child)
		s.Slots[previous].Data[2] = uint8(child)
		e.Frame--
		if e.Frame != 0 {
			return nil
		}
		n.Data[2] = 255
		n.HP = 0x10
		n.StateWidth = 0x0c
		n.Data[1] = 1
		n.AnimationDelay = 0x20
		e.VelocityX = int16(uint16(e.VelocityX)&0xff00 | uint16(child))
		chain, err := s.dragonChain(slot, 2)
		if err != nil {
			return err
		}
		for _, i := range chain {
			s.Slots[i].advance()
		}
		e.Data[1] = 0
		e.Frame = 0
	case 3:
		e.scrollOnly(c)
		if e.AnimationDelay != 0 {
			if c.Frame&1 != 0 {
				e.AnimationDelay--
			}
			return nil
		}
		if e.Data[1] == 0 {
			return nil
		}
		e.dragonSprite()
		y := uint16(e.Y)<<8 | uint16(e.FractionY)
		y -= 181
		e.Y = uint8(y >> 8)
		e.FractionY = uint8(y)
		x := uint16(e.X)<<8 | uint16(e.FractionX)
		if e.Attributes&1 != 0 {
			x -= 181
		} else {
			x += 181
		}
		e.X = uint8(x >> 8)
		e.FractionX = uint8(x)
		if e.Data[1]&0x80 != 0 {
			return nil
		}
		e.Data[1]++
		if e.Data[1] < 16 {
			return nil
		}
		if e.Data[3] >= 16 {
			return errors.New("Dragon shoulder cannot extend as a child")
		}
		e.Data[1] = 255
		previous := &s.Slots[e.Data[3]]
		previous.Data[1] = 1
		previous.AnimationDelay = 0
		if previous.Data[3]&0x80 != 0 {
			chain, err := s.dragonChain(int(e.Data[3]), 2)
			if err != nil {
				return err
			}
			for _, i := range chain {
				s.Slots[i].advance()
				s.Slots[i].Data[1] = 0
			}
			e.Frame = 0
		}
	case 4:
		e.dragonSprite()
		if e.Data[3]&0x80 == 0 {
			return nil
		}
		if err := s.dragonPattern(slot, c); err != nil {
			return err
		}
		if e.Frame != 4 {
			if err := s.dragonAnimate(slot); err != nil {
				return err
			}
		}
		return s.dragonPositions(slot)
	case 5:
		if e.Data[2]&0x80 != 0 {
			*b.DestroyedArms++
			chain, err := s.dragonChain(slot, 3)
			if err != nil {
				return err
			}
			for _, i := range chain[1:] {
				s.markDestroyed(i, c.Level)
			}
		}
		e.advance()
	default:
		return s.StepExplosion(slot, c, ExplosionPhase(e.Routine-6))
	}
	return nil
}

func (e *Enemy) dragonSprite() {
	e.Sprite = 0x7a
	if e.Data[2]&0x80 != 0 {
		e.Sprite = 0x7b
	}
}

// The original assumes valid finite links. An invalid saved graph is rejected
// rather than hanging the server or inventing a successful Boss outcome.
func (s *EnemySystem) dragonChain(slot, index int) ([]int, error) {
	result := []int{}
	seen := [16]bool{}
	for slot >= 0 {
		if slot >= 16 || seen[slot] {
			return nil, errors.New("invalid or cyclic Dragon arm link")
		}
		seen[slot] = true
		result = append(result, slot)
		link := s.Slots[slot].Data[index]
		if link&0x80 != 0 {
			break
		}
		slot = int(link)
	}
	return result, nil
}

func (s *EnemySystem) dragonFire(slot int, c EnemyStepContext) error {
	e := &s.Slots[slot]
	e.VariableA--
	if e.VariableA != 0 {
		return nil
	}
	e.VariableA = 0x90
	hand := int(uint8(e.VelocityX))
	if hand >= 16 {
		return errors.New("invalid Dragon hand reference")
	}
	h := &s.Slots[hand]
	angle, err := shooterAimAtPlayer(c, closestShooterTarget(h, c), h.X, h.Y, 1)
	if err != nil {
		return err
	}
	_, err = s.CreateEnemyShot(c, 0x80|angle, 5, h.X, h.Y)
	return err
}

func (s *EnemySystem) dragonPattern(slot int, c EnemyStepContext) error {
	e := &s.Slots[slot]
	side := e.Attributes & 1
	if e.Frame < 4 {
		if err := s.dragonFire(slot, c); err != nil {
			return err
		}
	}
	switch e.Frame {
	case 0:
		down := uint8(e.VelocityY) != 0
		limit := [2]uint8{0x14, 0x0c}[side]
		if down {
			limit = [2]uint8{0x2c, 0x34}[side]
		}
		if e.Data[0] != limit {
			table := [3]uint8{0x40, 0xc0, 0x40}
			index := side
			if down {
				index++
			}
			e.Data[1] = table[index]
			return nil
		}
		if down {
			e.AttackDelay++
			if e.AttackDelay == 3 {
				e.Frame++
			}
		}
		e.AnimationDelay = 3
		e.VelocityY = int16(uint16(e.VelocityY) ^ 1)
	case 1:
		if uint8(uint16(e.VelocityY)>>8) == 0 {
			e.Frame++
		}
	case 2:
		chain, err := s.dragonChain(slot, 2)
		if err != nil {
			return err
		}
		for _, i := range chain[1:] {
			n := &s.Slots[i]
			if n.Data[0] != [2]uint8{8, 0x38}[side] {
				n.Data[1] = [2]uint8{0x40, 0xc0}[side]
				return nil
			}
		}
		// The last successful CMP leaves carry set for ADC Frame.
		random := c.Random
		if c.SharedRandom != nil {
			random = c.SharedRandom.Value
		}
		e.AttackDelay = [4]uint8{0x40, 0x60, 0x30, 0x70}[(random+c.Frame+1)&3]
		e.Frame++
	case 3:
		e.AttackDelay--
		if e.AttackDelay == 0 {
			e.AttackDelay = 0xc0
			e.Frame++
		}
	case 4:
		if err := s.dragonSeek(slot, c); err != nil {
			return err
		}
		e.AttackDelay--
		if e.AttackDelay == 0 {
			e.VelocityY = int16(uint16(e.VelocityY) & 0xff00)
			e.AttackDelay = 0
			e.Frame = 0
		}
	default:
		return errors.New("invalid Dragon arm pattern")
	}
	return nil
}

func (s *EnemySystem) dragonAnimate(slot int) error {
	chain, err := s.dragonChain(slot, 2)
	if err != nil {
		return err
	}
	inherited := int8(0)
	combined := uint8(0)
	for _, i := range chain {
		e := &s.Slots[i]
		delta := int8(0)
		if e.AnimationDelay != 0 {
			e.AnimationDelay--
		} else if e.Data[1] != 0 {
			if int8(e.Data[1]) > 0 {
				e.Data[1]--
				delta = 1
			} else {
				e.Data[1]++
				delta = -1
			}
		}
		steps := int8(uint8(delta) + uint8(inherited))
		correction := int8(0)
		for steps > 0 {
			if e.Data[3]&0x80 == 0 && e.Data[0] == 8 {
				if delta > 0 {
					e.Data[1] = 0
				} else {
					e.AnimationDelay = 1
					e.Data[1]--
				}
			} else {
				correction--
				e.Data[0] = (e.Data[0] + 1) & 63
			}
			steps--
		}
		for steps < 0 {
			if e.Data[3]&0x80 == 0 && e.Data[0] == 0x38 {
				if delta < 0 {
					e.Data[1] = 0
				} else {
					e.AnimationDelay = 1
					e.Data[1]++
				}
			} else {
				correction++
				e.Data[0] = (e.Data[0] - 1) & 63
			}
			steps++
		}
		inherited = int8(uint8(inherited) + uint8(correction))
		combined |= e.Data[1]
	}
	e := &s.Slots[slot]
	e.VelocityY = int16(uint16(combined)<<8 | uint16(uint8(e.VelocityY)))
	return nil
}

func (s *EnemySystem) dragonPositions(slot int) error {
	chain, err := s.dragonChain(slot, 2)
	if err != nil {
		return err
	}
	e := &s.Slots[slot]
	e.VelocityX = int16(uint16(e.Data[0])<<8 | uint16(uint8(e.VelocityX)))
	for n := 1; n < len(chain); n++ {
		p := &s.Slots[chain[n-1]]
		e = &s.Slots[chain[n]]
		angle := (uint8(uint16(p.VelocityX)>>8) + e.Data[0]) & 63
		e.VelocityX = int16(uint16(angle)<<8 | uint16(uint8(e.VelocityX)))
		e.Y = p.Y + dragonArmPositions[angle]
		e.X = p.X + dragonArmPositions[angle+16]
	}
	return nil
}

func (s *EnemySystem) dragonSeek(slot int, c EnemyStepContext) error {
	hand := int(uint8(s.Slots[slot].VelocityX))
	if hand >= 16 {
		return errors.New("invalid Dragon hand reference")
	}
	actor := closestShooterTarget(&s.Slots[hand], c)
	if c.Players[actor].State != 1 {
		actor ^= 1
	}
	target := c.Players[actor]
	if target.State != 1 {
		target.X = 128
		target.Y = 255
	} else if c.SceneType&1 != 0 {
		target.Y = 0xb0
	}
	chain, err := s.dragonChain(hand, 3)
	if err != nil {
		return err
	}
	for n := 0; n+1 < len(chain); n++ {
		child, parent := chain[n], chain[n+1]
		p := &s.Slots[parent]
		e := &s.Slots[child]
		desired := dragonAim64(p.X, p.Y, target.X, target.Y)
		current := uint8(uint16(e.VelocityX) >> 8)
		if desired == current {
			continue
		}
		mid := (current + 32) & 63
		delta := int8(-1)
		if current < 32 {
			if desired >= current && desired < mid {
				delta = 1
			}
		} else if desired >= current || desired < mid {
			delta = 1
		}
		chosen := child
		limit := uint8(8)
		if delta < 0 {
			limit = 0x38
		}
		for s.Slots[chosen].Data[3]&0x80 == 0 && s.Slots[chosen].Data[0] == limit {
			chosen = int(s.Slots[chosen].Data[3])
		}
		if chosen == child && e.Data[2]&0x80 == 0 {
			next := &s.Slots[e.Data[2]]
			next.Data[0] = (next.Data[0] - uint8(delta)) & 63
		}
		s.Slots[chosen].Data[0] = (s.Slots[chosen].Data[0] + uint8(delta)) & 63
		return nil
	}
	return nil
}

func dragonAim64(x, y, tx, ty uint8) uint8 {
	dx, dy := int(tx)-int(x), int(ty)-int(y)
	left, up := dx < 0, dy < 0
	if left {
		dx = -dx
	}
	if up {
		dy = -dy
	}
	v := dragonAimQuarter[(dy>>5)*4+(dx>>6)]
	if dx&32 == 0 {
		v >>= 4
	}
	v &= 15
	if left {
		v = 32 - v
	}
	if up {
		v = (64 - v) & 63
	}
	return v
}

// ROM bank0/1 $9EA8..$9EF7 and fixed-bank $F558..$F577.
var dragonArmPositions = [80]uint8{0, 1, 3, 4, 6, 7, 8, 10, 11, 12, 13, 14, 14, 15, 15, 15, 15, 15, 15, 15, 14, 14, 13, 12, 11, 10, 8, 7, 6, 4, 3, 1, 0, 255, 253, 252, 250, 249, 248, 246, 245, 244, 243, 242, 242, 241, 241, 241, 241, 241, 241, 241, 242, 242, 243, 244, 245, 246, 248, 249, 250, 252, 253, 255, 0, 1, 3, 4, 6, 7, 8, 10, 11, 12, 13, 14, 14, 15, 15, 15}
var dragonAimQuarter = [32]uint8{0x80, 0, 0, 0, 0xf8, 0x53, 0x32, 0x21, 0xfb, 0x86, 0x54, 0x33, 0xfd, 0xa8, 0x75, 0x54, 0xfe, 0xb9, 0x87, 0x65, 0xfe, 0xcb, 0x98, 0x76, 0xfe, 0xdb, 0xa9, 0x87, 0xff, 0xdc, 0xba, 0x98}
