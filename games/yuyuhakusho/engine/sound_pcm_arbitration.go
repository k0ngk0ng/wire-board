package engine

// PCMArbitrationCommand is a temporary interpretation of immutable command
// data. The same descriptor byte controls single-mode pan or dual-mode routing;
// it is not stored as two independently mutable values.
type PCMArbitrationCommand struct {
	Flags, RoutingOrPan uint8
}

// PCMArbitrationView contains the actual bytes sampled by the dispatcher.
// Playback/DSP state owns these fields; this copied view is never persisted as
// a second Z80 image. Busy includes pending80 and active40: only zero is free.
type PCMArbitrationView struct {
	ModeFlags            uint8
	SlotFlags, SlotBusy  [2]uint8
	ChoiceBits, DuckMode uint8
}

type PCMDestination uint8

const (
	PCMReject PCMDestination = iota
	PCMSingle
	PCMSlotA
	PCMSlotB
)

// PCMArbitrationDecision describes the M68K submission intent, not a completed
// Z80 acknowledgement, sample lifetime, synthesis result or native-media time.
type PCMArbitrationDecision struct {
	Destination PCMDestination
	ChoiceBits  uint8
	StartDual   bool
	DuckMode    uint8
}

// SelectPCMCommand recovers37d66..3807c's choice/duck semantics. Command and
// playback bytes are inputs; only the caller applies accepted request writes.
func SelectPCMCommand(command PCMArbitrationCommand, view PCMArbitrationView) PCMArbitrationDecision {
	decision := PCMArbitrationDecision{ChoiceBits: view.ChoiceBits, DuckMode: view.DuckMode}
	priority := command.Flags & 0xc0
	a, b := view.SlotFlags[0]&0xc0, view.SlotFlags[1]&0xc0
	accept := func(destination PCMDestination) PCMArbitrationDecision {
		decision.Destination = destination
		decision.StartDual = destination != PCMSingle && view.ModeFlags&1 == 0
		switch destination {
		case PCMSlotA:
			decision.ChoiceBits |= 1
		case PCMSlotB:
			decision.ChoiceBits &^= 1
		}
		if command.Flags&0x20 != 0 && view.ModeFlags&0x20 == 0 && view.DuckMode != 2 {
			decision.DuckMode = 1
		}
		return decision
	}
	if command.Flags&1 == 0 {
		if view.ModeFlags&1 == 0 {
			if priority < view.ModeFlags&0xc0 {
				return decision
			}
		} else if priority < a || priority < b {
			return decision
		}
		return accept(PCMSingle)
	}
	if view.ModeFlags&1 == 0 && priority < view.ModeFlags&0xc0 {
		return decision
	}
	switch command.RoutingOrPan & 0xc0 {
	case 0x80:
		if priority >= a {
			return accept(PCMSlotA)
		}
	case 0x40:
		if priority >= b {
			return accept(PCMSlotB)
		}
	default:
		if view.SlotBusy[0] == 0 {
			return accept(PCMSlotA)
		}
		if view.SlotBusy[1] == 0 {
			return accept(PCMSlotB)
		}
		if view.ChoiceBits&1 == 0 {
			if priority >= a {
				return accept(PCMSlotA)
			}
			if priority >= b {
				return accept(PCMSlotB)
			}
		} else {
			if priority >= b {
				return accept(PCMSlotB)
			}
			if priority >= a {
				return accept(PCMSlotA)
			}
		}
	}
	return decision
}
