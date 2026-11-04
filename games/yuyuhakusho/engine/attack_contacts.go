package engine

import "errors"

// AttackContactActor contains semantic state consumed or changed by contact
// resolution. Auxiliary names retain their offsets until wider behavior is
// established. CharacterID is the source character, independent of a seat.
type AttackContactActor struct {
	X, Y, Ground                     int32
	VelocityX, VelocityY, Gravity    int32
	Attributes, Depth                uint16
	Layer, CharacterID               uint8
	FrameIndex, FrameDuration        uint8
	FrameTicks, FrameFlags           uint8
	ActionKind, Pose, Completed      uint8
	Facing, GuardMask                uint8
	AttackFlags, AttackValue         uint8
	StunDamage, EffectKind, Reaction uint8
	HitStopTicks, Shake              uint8
	ActionFlags, CancelTicks, Team   uint8
	TargetBit, RetainedHeld          uint8
	InputRetention, Auxiliary77      uint8
	Health, StunValue                int16
	StunTicks                        uint16
	StatusDelay, AttackDelay         uint8
	ContactFlags                     uint8
	TargetMask                       uint32
	Residual, AuxiliaryB8            int32
	ComboCount                       uint8
}

type AttackContactState struct {
	Actors           map[uint16]AttackContactActor
	Attack, Hurt     AnimationShapeBuffer
	Effects          EffectPool
	EffectParameters [16]uint16 // Combo-number effect parameter (+3c).
	PendingSound     uint8
	SlowTicks        uint16
	ComboFlags       uint8
	ArenaLeft        int16
	ArenaRight       int16
}

// ResolveAttackContacts recovers the full ordered attack/hurt loop at 0x1f21e,
// including guard, damage, all eleven reaction constructors, hit/number effect
// allocation and KO sound/slow-time requests. It consumes prior-frame geometry
// and leaves the counters for the collision manager to clear afterward.
func ResolveAttackContacts(state AttackContactState) (AttackContactState, error) {
	before := state
	if state.Attack.Offset > 256 || state.Attack.Offset&15 != 0 || state.Hurt.Offset > 256 || state.Hurt.Offset&15 != 0 {
		return before, errors.New("invalid attack/hurt buffer offset")
	}
	if state.Effects.Cursor&0x7f != 0 {
		return before, errors.New("unaligned attack effect cursor")
	}
	attackCount, hurtCount := int(state.Attack.Offset/16), int(state.Hurt.Offset/16)
	for _, record := range state.Attack.Records[:attackCount] {
		a, ok := state.Actors[record.Owner]
		if !ok || a.Reaction > 10 {
			return before, errors.New("attack owner or reaction is unavailable")
		}
	}
	for _, record := range state.Hurt.Records[:hurtCount] {
		a, ok := state.Actors[record.Owner]
		if !ok || a.CharacterID >= 11 {
			return before, errors.New("hurt owner or character is unavailable")
		}
	}
	if attackCount == 0 || hurtCount == 0 {
		return state, nil
	}
	owners := make(map[uint16]AttackContactActor, len(state.Actors))
	for id, actor := range state.Actors {
		owners[id] = actor
	}
	state.Actors = owners
	for _, attack := range state.Attack.Records[:attackCount] {
		for _, hurt := range state.Hurt.Records[:hurtCount] {
			a, b := owners[attack.Owner], owners[hurt.Owner]
			bit := uint32(1) << (b.TargetBit & 31)
			if a.Team == b.Team || a.Ground>>16 != b.Ground>>16 || a.TargetMask&bit == 0 || !attackContactOverlap(attack, hurt) {
				continue
			}
			a.ContactFlags |= 8
			if int16(a.Ground>>16) == 0x130 {
				a.Layer, b.Layer = 0xbc, 0xc0
			} else {
				a.Layer, b.Layer = 0x3c, 0x40
			}
			a.AttackDelay = 2
			a.TargetMask &^= bit
			if a.FrameTicks == 0 {
				a.FrameTicks = 1
			}
			a.HitStopTicks, a.InputRetention, b.HitStopTicks = 0x10, 0x11, 0x10
			if a.AttackFlags&0x20 != 0 {
				a.HitStopTicks, a.InputRetention, b.HitStopTicks = 0, 1, 0
			}
			b.ActionFlags = 0
			b.Facing = a.Facing ^ 1
			if a.AttackFlags&0x0c&b.GuardMask != 0 {
				if err := resolveGuardContact(&state, &a, &b, attack, hurt); err != nil {
					return before, err
				}
			} else {
				if err := resolveDamagingContact(&state, &a, &b, attack, hurt); err != nil {
					return before, err
				}
			}
			owners[attack.Owner], owners[hurt.Owner] = a, b
		}
	}
	return state, nil
}

func attackContactOverlap(a, b AnimationShapeRecord) bool {
	dx, dy := a.X-b.X, a.Y-b.Y
	if dx < 0 {
		dx = -dx // The word minimum deliberately stays negative.
	}
	if dy < 0 {
		dy = -dy
	}
	return int32(a.Width+b.Width) >= int32(dx) && int32(a.Height+b.Height) >= int32(dy)
}

func resolveGuardContact(state *AttackContactState, a, b *AttackContactActor, attack, hurt AnimationShapeRecord) error {
	strength := a.AttackFlags & 3
	if strength < 3 {
		b.VelocityX = [...]int32{0x2b000, 0x39000, 0x4c000}[strength]
		b.VelocityY = [...]int32{-0x40000, -0x50000, -0x60000}[strength]
		b.Shake = strength + 1
		if err := spawnAttackContactEffect(state, *a, attack, hurt, strength+3, 0x77); err != nil {
			return err
		}
	}
	if a.Ground>>16 != a.Y>>16 {
		b.VelocityX = 0x3d000
	}
	x := int16(b.X >> 16)
	if x <= state.ArenaLeft+16 || x >= state.ArenaRight-16 {
		a.Residual = b.VelocityX
	}
	if a.AttackFlags&0x10 != 0 {
		b.Health -= int16((a.AttackValue >> 2) + 1)
		if b.Health < 0 {
			b.Health = 0
		}
	}
	b.ActionKind, b.Pose = 4, 0x2c
	if b.RetainedHeld&2 != 0 {
		b.Pose = 0x2d
	}
	resetAttackContactReaction(b, false)
	return nil
}

func resolveDamagingContact(state *AttackContactState, a, b *AttackContactActor, attack, hurt AnimationShapeRecord) error {
	b.StunTicks, b.StatusDelay = 0, 255
	b.Health -= int16(a.AttackValue)
	b.StunValue -= int16(a.StunDamage)
	if err := advanceAttackCombo(state, a, b, attack, hurt); err != nil {
		return err
	}
	strength := a.AttackFlags & 3
	if strength < 3 {
		b.VelocityX = [...]int32{0x30000, 0x40000, 0x56000}[strength]
		b.VelocityY = [...]int32{-0x40000, -0x50000, -0x60000}[strength]
		b.Shake = strength + 1
		effect, sound := strength, uint8(0x7a)+strength
		if a.EffectKind != 1 {
			effect, sound = strength+6, 0x78
		}
		if err := spawnAttackContactEffect(state, *a, attack, hurt, effect, sound); err != nil {
			return err
		}
	}
	if a.Ground>>16 != a.Y>>16 {
		b.VelocityX = 0x46000
	}
	x := int16(b.X >> 16)
	if x <= state.ArenaLeft || x >= state.ArenaRight {
		a.Residual = b.VelocityX
	}
	if b.Health >= 0 {
		b.StunTicks = 0
		if b.StunValue < 0 {
			b.StunTicks = uint16(-b.StunValue+4) << 4
			if a.Reaction <= 4 {
				a.Reaction = 5
			}
		}
	} else {
		state.PendingSound = [...]uint8{0x29, 0x19, 0x13, 0x0b, 0x21, 0x39, 0x55, 0x4d, 0x32, 0x3f, 0x48}[b.CharacterID]
		state.SlowTicks = 0xa0
		if a.Reaction <= 4 {
			a.Reaction = 5
		}
	}
	constructAttackReaction(a, b)
	return nil
}

func advanceAttackCombo(state *AttackContactState, a, b *AttackContactActor, attack, hurt AnimationShapeRecord) error {
	if b.Pose < 0x20 || b.Pose > 0x30 || b.Pose > 0x27 && b.Pose < 0x2e {
		b.ComboCount = 0
		return nil
	}
	b.ComboCount++
	if b.ComboCount >= 0x10 {
		b.ComboCount = 0x10
		state.ComboFlags |= 1
	}
	high := int16(a.AuxiliaryB8>>16) + int16(b.ComboCount)*2
	a.AuxiliaryB8 = int32(uint32(uint16(high))<<16 | uint32(uint16(a.AuxiliaryB8)))
	if high >= 0xdf {
		a.AuxiliaryB8 = 0xdf0000
	}
	next, index, err := AllocateEffect(state.Effects)
	if err != nil {
		return err
	}
	state.Effects = next
	if index >= 0 {
		slot := &state.Effects.Slots[index]
		slot.Behavior = 0x14930
		slot.X = attackContactCoordinate(slot.X, (attack.X+hurt.X)>>1)
		slot.Y = attackContactCoordinate(slot.Y, (attack.Y+hurt.Y)>>1)
		slot.Layer = a.Layer - 8
		state.EffectParameters[index] = uint16(b.ComboCount+0x81) << 1
	}
	return nil
}

func spawnAttackContactEffect(state *AttackContactState, a AttackContactActor, attack, hurt AnimationShapeRecord, effect, sound uint8) error {
	if a.EffectKind == 0 {
		return nil
	}
	state.PendingSound = sound
	next, index, err := AllocateEffect(state.Effects)
	if err != nil {
		return err
	}
	state.Effects = next
	if index >= 0 {
		slot := &state.Effects.Slots[index]
		slot.Type = effect
		slot.X = attackContactCoordinate(slot.X, (attack.X+hurt.X)>>1)
		slot.Y = attackContactCoordinate(slot.Y, (attack.Y+hurt.Y)>>1)
		slot.Attributes = a.Attributes & 0x9800
		slot.Depth, slot.Layer = a.Depth, a.Layer-4
	}
	return nil
}

func attackContactCoordinate(previous int32, high int16) int32 {
	return int32(uint32(uint16(high))<<16 | uint32(uint16(previous)))
}

func constructAttackReaction(a, b *AttackContactActor) {
	clearFlags := false
	reaction := a.Reaction
	if reaction <= 3 && b.FrameFlags&0x0c == 8 {
		if a.AttackFlags&0x10 != 0 {
			reaction = 5
		} else {
			b.ActionKind, b.Pose, b.Gravity, b.VelocityY = 5, 0x24, 0x3800, -0x40000
			b.VelocityX = 0x1c000
			if b.Facing == 0 {
				b.VelocityX = -b.VelocityX
			}
			resetAttackContactReaction(b, false)
			return
		}
	}
	switch reaction {
	case 0, 1, 2, 3:
		b.ActionKind = 4
		if reaction == 2 || reaction == 3 {
			b.ActionKind, a.Residual = 3, 0
		}
		b.Pose = 0x20 + reaction&1
		if b.FrameFlags&0x0c == 4 {
			b.Pose = 0x22
		}
	case 4:
		b.ActionKind, b.Pose, b.Gravity, b.VelocityY = 5, 0x23, 0x3800, -0x40000
		b.VelocityX = 0x18000
		if b.Facing == 0 {
			b.VelocityX = -b.VelocityX
		}
	case 5, 9, 10:
		b.ActionKind, b.Pose, b.Gravity = 5, 0x25, 0x3800
		if reaction == 9 {
			b.Pose, clearFlags = 0x2f, true
		} else if reaction == 10 {
			b.ActionKind = 8
		}
		b.VelocityX = 0x18000
		if b.Facing == 0 {
			b.VelocityX = -b.VelocityX
		}
	case 6:
		b.Team, b.ActionKind, b.Pose, b.Facing = a.Team, 7, 0x26, b.Facing^1
		b.VelocityX = -0x60000
		if b.Facing == 0 {
			b.VelocityX = 0x60000
		}
	case 7:
		b.Team, b.ActionKind, b.Pose = a.Team, 5, 0x27
		b.Gravity, b.VelocityX, b.VelocityY, clearFlags = 0, 0, 0x60000, true
	case 8:
		b.ActionKind, b.Pose = 5, 0x30
		b.Gravity, b.VelocityX, b.VelocityY, clearFlags = 0x3800, 0, -0x70000, true
	}
	resetAttackContactReaction(b, clearFlags)
}

func resetAttackContactReaction(b *AttackContactActor, clearFlags bool) {
	b.ActionFlags, b.CancelTicks = 0x20, 0x20
	if clearFlags {
		b.ActionFlags, b.CancelTicks = 0, 0
	}
	b.FrameIndex, b.FrameDuration, b.FrameTicks, b.Completed = 0, 8, 255, 0
	b.AttackFlags, b.Auxiliary77 = 0, 0
}
