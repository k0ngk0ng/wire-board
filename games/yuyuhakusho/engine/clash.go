package engine

import "errors"

// ClashActor contains only fields read or written by attack-clash resolution.
// Actor IDs and target bits are distinct: the original bit instruction masks
// a target's bit number to five bits, independently of object identity.
type ClashActor struct {
	Team           uint8
	Ground         int32
	Y              int32
	TargetBit      uint8
	TargetMask     uint32
	AttackValue    uint8
	AttackFlags    uint8
	FrameTicks     uint8
	ContactFlags   uint8
	HitStopTicks   uint8
	InputRetention uint8
	CancelTicks    uint8
	Residual       int32
	Shake          uint8
	Attributes     uint16
	Layer          uint8
	Depth          uint16
}

type ClashState struct {
	Actors       map[uint16]ClashActor
	Buffer       AnimationShapeBuffer
	Effects      EffectPool
	PendingSound uint8
}

// ResolveClashes recovers the complete ordered loop at 0x1f090. Its shapes
// belong to the preceding actor-update round; the collision manager clears
// their counter only after every consumer has run. It initializes effects but
// does not advance their lifetimes or consume the pending sound request.
func ResolveClashes(state ClashState) (ClashState, error) {
	if state.Buffer.Offset > 256 || state.Buffer.Offset&15 != 0 {
		return state, errors.New("invalid clash buffer offset")
	}
	count := int(state.Buffer.Offset / 16)
	for _, record := range state.Buffer.Records[:count] {
		if _, ok := state.Actors[record.Owner]; !ok {
			return state, errors.New("clash owner is unavailable")
		}
	}
	if count == 0 {
		return state, nil
	}
	original := state
	state.Actors = make(map[uint16]ClashActor, len(original.Actors))
	for id, actor := range original.Actors {
		state.Actors[id] = actor
	}
	for _, source := range state.Buffer.Records[:count] {
		for _, target := range state.Buffer.Records[:count] {
			a, b := state.Actors[source.Owner], state.Actors[target.Owner]
			if a.Team == b.Team || a.Ground>>16 != b.Ground>>16 ||
				a.TargetMask&(uint32(1)<<(b.TargetBit&31)) == 0 || b.TargetMask&(uint32(1)<<(a.TargetBit&31)) == 0 {
				continue
			}
			dx, dy := source.X-target.X, source.Y-target.Y
			if dx < 0 {
				dx = -dx
			}
			if dy < 0 {
				dy = -dy
			}
			// The original byte additions and word differences wrap before
			// signed comparisons. In particular NEG.W(-32768) stays -32768.
			if int16(source.Width+target.Width) < dx || int16(source.Height+target.Height) < dy {
				continue
			}
			state.PendingSound = 0x77
			pool, index, err := AllocateEffect(state.Effects)
			if err != nil {
				return original, err
			}
			state.Effects = pool
			if index >= 0 {
				slot := &state.Effects.Slots[index]
				slot.Type = 0x0c
				// ADD.W precedes ASR.W; fractions in the reused slot survive.
				x, y := (source.X+target.X)>>1, (source.Y+target.Y)>>1
				slot.X = int32(uint32(uint16(x))<<16 | uint32(uint16(slot.X)))
				slot.Y = int32(uint32(uint16(y))<<16 | uint32(uint16(slot.Y)))
				slot.Attributes = a.Attributes & 0x9800
				slot.Depth, slot.Layer = a.Depth, a.Layer-4
			}
			applyClashSide(&a, &b)
			applyClashSide(&b, &a)
			state.Actors[source.Owner], state.Actors[target.Owner] = a, b
		}
	}
	return state, nil
}

func applyClashSide(actor, other *ClashActor) {
	actor.ContactFlags |= 4
	if other.AttackValue >= actor.AttackValue {
		actor.ContactFlags |= 0x10
	}
	actor.TargetMask &^= uint32(1) << (other.TargetBit & 31)
	if actor.FrameTicks == 0 {
		actor.FrameTicks = 1
	}
	actor.HitStopTicks, actor.InputRetention = 0x1c, 0x1d
	if actor.AttackFlags&0x10 == 0 {
		if actor.Ground == actor.Y {
			actor.CancelTicks = 10
		} else {
			actor.CancelTicks = 20
		}
	}
	switch actor.AttackFlags & 3 {
	case 0:
		other.Residual, other.Shake = 0x19000, 1
	case 1:
		other.Residual, other.Shake = 0x23000, 2
	case 2:
		other.Residual, other.Shake = 0x30c00, 3
	}
	if actor.Ground>>16 != actor.Y>>16 {
		other.Residual = 0x23000
	}
}
