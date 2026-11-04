package engine

import "errors"

// ThrowContactRecord is the immutable semantic content consumed from a 32-byte
// grab record. Unlike ordinary shape records, its X and Y are adjacent words.
type ThrowContactRecord struct {
	X, Y                       int16
	Width, Height              uint8
	SourcePose, TargetPose     uint8
	SourceAction, TargetAction uint8
	TargetFacing               uint8
	Owner                      uint16
	OffsetX, OffsetY           int16
}

// Offset is a used byte offset, not a record count. The producer's original
// capacity remains separate from the consumer's validated slice boundary.
type ThrowContactBuffer struct {
	Offset  uint16
	Records []ThrowContactRecord
}

type ThrowContactActor struct {
	X, Y, Ground  int32
	Team          uint8
	Delay         uint8
	EnabledStages uint8
	FrameFlags    uint8
	ContactFlags  uint8
	TargetMask    uint32
	Facing        uint8
	InputFlags    uint8
	Layer         uint8
	Pose          uint8
	Action        uint8
	FrameIndex    uint8
	FrameDuration uint8
	FrameTicks    uint8
	Auxiliary60   uint8
	AttackFlags   uint8
	RetainedPress uint8
	CancelTicks   uint8
}

type ThrowContactState struct {
	Actors      map[uint16]ThrowContactActor
	Throws      ThrowContactBuffer
	Hurts       AnimationShapeBuffer
	Left, Right int16
}

// ResolveThrowContacts recovers 0x1eee2 and its layer helper. It consumes the
// preceding actor-update round's records without clearing their counters.
// Contact selects the observed poses/actions and aligns the actors; subsequent
// throw motion and damage are separate stages. Caller-owned state is unchanged.
func ResolveThrowContacts(state ThrowContactState) (ThrowContactState, error) {
	if state.Throws.Offset&31 != 0 || int(state.Throws.Offset/32) > len(state.Throws.Records) {
		return state, errors.New("invalid throw buffer offset")
	}
	if state.Hurts.Offset > 256 || state.Hurts.Offset&15 != 0 {
		return state, errors.New("invalid throw target buffer offset")
	}
	throwCount, hurtCount := int(state.Throws.Offset/32), int(state.Hurts.Offset/16)
	for _, record := range state.Throws.Records[:throwCount] {
		if _, ok := state.Actors[record.Owner]; !ok {
			return state, errors.New("throw source owner is unavailable")
		}
	}
	for _, record := range state.Hurts.Records[:hurtCount] {
		if _, ok := state.Actors[record.Owner]; !ok {
			return state, errors.New("throw target owner is unavailable")
		}
	}
	if throwCount == 0 || hurtCount == 0 {
		return state, nil
	}
	original := state
	state.Actors = make(map[uint16]ThrowContactActor, len(original.Actors))
	for id, actor := range original.Actors {
		state.Actors[id] = actor
	}
	for _, record := range state.Throws.Records[:throwCount] {
		for _, hurt := range state.Hurts.Records[:hurtCount] {
			source, target := state.Actors[record.Owner], state.Actors[hurt.Owner]
			if target.Delay != 0 || source.Team == target.Team || source.Ground>>16 != target.Ground>>16 ||
				target.EnabledStages&0x20 != 0 || source.FrameFlags&8 != target.FrameFlags&8 {
				continue
			}
			dx, dy := record.X-hurt.X, record.Y-hurt.Y
			if dx < 0 {
				dx = -dx
			}
			if dy < 0 {
				dy = -dy
			}
			// Preserve word negation (including -32768) and byte extent sums
			// before the overflow-aware signed-word comparisons.
			if int16(record.Width+hurt.Width) < dx || int16(record.Height+hurt.Height) < dy {
				continue
			}
			source.ContactFlags |= 2
			if int16(source.Ground>>16) == 0x130 {
				source.Layer, target.Layer = 0xbc, 0xc0
			} else {
				source.Layer, target.Layer = 0x3c, 0x40
			}
			target.TargetMask = 0
			source.Facing = 0
			if source.InputFlags&8 == 0 {
				source.Facing = 1
			}
			target.Facing = record.TargetFacing
			target.X, target.Y = source.X, addThrowCoordinateWord(source.Y, record.OffsetY)
			if source.Facing == 0 {
				target.X = addThrowCoordinateWord(target.X, record.OffsetX)
			} else {
				target.X = addThrowCoordinateWord(target.X, -record.OffsetX)
				target.Facing ^= 1
			}
			// The original adjusts only the source, using wrapped word
			// differences tested by TST.W. The target is not clamped again.
			if delta := state.Left - int16(target.X>>16); delta >= 0 {
				source.X = addThrowCoordinateWord(source.X, delta)
			}
			if delta := state.Right - int16(target.X>>16); delta < 0 {
				source.X = addThrowCoordinateWord(source.X, delta)
			}
			resetThrowContactActor(&source, record.SourcePose, record.SourceAction)
			source.EnabledStages = 0x20
			resetThrowContactActor(&target, record.TargetPose, record.TargetAction)
			state.Actors[record.Owner], state.Actors[hurt.Owner] = source, target
		}
	}
	return state, nil
}

func addThrowCoordinateWord(value int32, delta int16) int32 {
	hi := int16(value>>16) + delta
	return int32(uint32(uint16(hi))<<16 | uint32(value)&0xffff)
}

func resetThrowContactActor(actor *ThrowContactActor, pose, action uint8) {
	actor.Pose, actor.Action = pose, action
	actor.FrameIndex, actor.FrameDuration, actor.FrameTicks = 0, 8, 255
	actor.Auxiliary60, actor.AttackFlags, actor.RetainedPress = 0, 0, 0
	actor.EnabledStages, actor.CancelTicks = 0, 0
}
