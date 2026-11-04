package engine

import "errors"

type BodySeparationActor struct {
	X            int32
	Ground       int32
	Team         uint8
	ContactFlags uint8
}

type BodySeparationState struct {
	Actors map[uint16]BodySeparationActor
	Buffer AnimationShapeBuffer
}

// SeparateBodies consumes the preceding frame's body records in their original
// ordered-pair sequence (0x1ee1e). Equal-X contacts also move the current record,
// affecting later pairs. Other contacts move only its owner, not the geometry.
// It does not clear the buffer; the collision manager does that after all of
// its consumers have run. The caller's owner map remains unchanged.
func SeparateBodies(state BodySeparationState) (BodySeparationState, error) {
	if state.Buffer.Offset > 256 || state.Buffer.Offset&15 != 0 {
		return state, errors.New("invalid body buffer offset")
	}
	count := int(state.Buffer.Offset / 16)
	for _, record := range state.Buffer.Records[:count] {
		if _, ok := state.Actors[record.Owner]; !ok {
			return state, errors.New("body owner is unavailable")
		}
	}
	if count == 0 {
		return state, nil
	}
	owners := make(map[uint16]BodySeparationActor, len(state.Actors))
	for id, actor := range state.Actors {
		owners[id] = actor
	}
	state.Actors = owners
	for i := 0; i < count; i++ {
		for j := 0; j < count; j++ {
			a, b := &state.Buffer.Records[i], &state.Buffer.Records[j]
			actor, other := owners[a.Owner], owners[b.Owner]
			if actor.Team == other.Team || actor.Ground>>16 != other.Ground>>16 {
				continue
			}
			dx := a.X - b.X
			if dx < 0 {
				dx = -dx // Word negation deliberately retains -32768.
			}
			width := a.Width + b.Width // Byte sum wraps before word comparison.
			penetration := int32(width) - int32(dx)
			if penetration < 0 { // SUB/BLT includes the signed overflow flag.
				continue
			}
			dy := a.Y - b.Y
			if dy < 0 {
				dy = -dy
			}
			if int32(a.Height+b.Height) < int32(dy) {
				continue
			}
			actor.ContactFlags |= 1
			displacement := uint32(uint16(penetration)) << 15
			x := int32(uint32(uint16(a.X))<<16 | uint32(a.Preserved[0]))
			otherX := int32(uint32(uint16(b.X))<<16 | uint32(b.Preserved[0]))
			if otherX > x {
				actor.X = int32(uint32(actor.X) - displacement - 1)
			} else {
				actor.X = int32(uint32(actor.X) + displacement + 1)
				if otherX == x {
					x = int32(uint32(x) + displacement + 1)
					a.X, a.Preserved[0] = int16(x>>16), uint16(x)
				}
			}
			owners[a.Owner] = actor
		}
	}
	return state, nil
}
