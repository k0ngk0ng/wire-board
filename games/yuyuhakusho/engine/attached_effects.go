package engine

import "errors"

type AttachedEffectKind uint8

const (
	AttachedComboNumber AttachedEffectKind = iota
	AttachedStunEffect
)

type AttachedEffectFrame struct {
	Parameter uint16 `json:"parameter"`
	Duration  uint8  `json:"duration"`
}

type AttachedEffectLibrary struct {
	Sequences map[uint32][]AttachedEffectFrame
	OrbitX    [1024]int8
	OrbitY    [1024]int8
	OffsetY   [11]int16
}

type AttachedEffectWorld struct {
	PauseFlags    uint8  `json:"pause_flags"`
	SlowFlag      uint8  `json:"slow_flag"`
	AttributeMask uint16 `json:"attribute_mask"`
	CameraX       int16  `json:"camera_x"`
	CameraY       int16  `json:"camera_y"`
	Shake         int16  `json:"shake"`
}

type AttachedEffectParent struct {
	ObjectID    uint16
	CharacterID uint8
	FrameIndex  uint8
	Ticks       uint8
	ActionKind  uint8
	X, Y        int32
	StunTicks   uint16
}

// Aux48 is a lifetime counter for combo numbers and an owner ID for status
// effects. The status phase is Slot.VelocityX's high word; its low word is
// preserved by the original instructions and is not a second phase value.
type AttachedEffectState struct {
	Kind                           AttachedEffectKind
	ObjectID                       uint16
	Slot                           EffectSlot
	GraphicsSet, TilesBase         uint32
	SkipTiles                      uint8
	GraphicOffsetX, GraphicOffsetY int16
	GraphicCache                   [4]uint32
	Parameter                      uint16
	Aux48                          uint16
	Sequence                       uint32
	ScreenX, ScreenY               int16
	OffsetX, OffsetY               int16
	Index, Delay, Ticks, Flags     uint8
}

type AttachedEffectOutcome struct {
	Draw         bool
	ClearedBytes uint16
}

// StepAttachedEffect recovers the rule stages of 0x14930/0x14ec6 through their
// presentation dispatch, silent return, or complete 0x3556 slot clearing.
// It never allocates a slot or writes another effect/parent object. The caller
// applies ClearedBytes to remaining fields of its stored pool slot.
func StepAttachedEffect(state AttachedEffectState, parent AttachedEffectParent, world AttachedEffectWorld, library AttachedEffectLibrary) (AttachedEffectState, AttachedEffectOutcome, error) {
	original := state
	if state.Kind != AttachedComboNumber && state.Kind != AttachedStunEffect {
		return original, AttachedEffectOutcome{}, errors.New("unsupported attached effect kind")
	}
	if state.Slot.Stage == 0 {
		if state.Kind == AttachedStunEffect && state.Slot.Type >= 3 {
			return original, AttachedEffectOutcome{}, errors.New("status effect style is unavailable")
		}
		state.Slot.Stage += 2
		state.GraphicOffsetX, state.GraphicOffsetY = 0, 0
		state.GraphicCache = [4]uint32{}
		state.GraphicsSet, state.SkipTiles = 0x27f618, 1
		if state.Kind == AttachedComboNumber {
			state.Slot.Depth = 0x100
			state.Slot.Attributes = 0x41eb | world.AttributeMask
		} else {
			state.Slot.Attributes |= world.AttributeMask | 0x1d6
			state.Parameter = 0
			state.Index, state.Ticks, state.Flags = 255, 1, 0
			state.Sequence = 0x15082 + uint32(state.Slot.Type)*6
			// Every style initializer returns directly. It neither decrements
			// the staggered delay nor draws, even while the world is paused.
			return state, AttachedEffectOutcome{}, nil
		}
	}
	if world.PauseFlags&0x80 != 0 || world.SlowFlag != 0 {
		return state, AttachedEffectOutcome{Draw: true}, nil
	}
	if state.Kind == AttachedComboNumber {
		state.Aux48++
		if state.Aux48 == 32 {
			cleared, out, err := clearAttachedEffect(state)
			if err != nil {
				return original, AttachedEffectOutcome{}, err
			}
			return cleared, out, nil
		}
		state.Slot.Y = replaceAttachedEffectHighWord(state.Slot.Y, uint16(int16(state.Slot.Y>>16)-1))
	} else {
		if parent.ObjectID != state.Aux48 {
			return original, AttachedEffectOutcome{}, errors.New("status effect owner is unavailable")
		}
		if parent.ActionKind == 5 && parent.FrameIndex == 0 && parent.Ticks == 0 {
			cleared, out, err := clearAttachedEffect(state)
			if err != nil {
				return original, AttachedEffectOutcome{}, err
			}
			return cleared, out, nil
		}
		if state.Delay != 0 {
			state.Delay--
			return state, AttachedEffectOutcome{}, nil
		}
		if err := advanceAttachedEffectCountdown(&state, library.Sequences); err != nil {
			return original, AttachedEffectOutcome{}, err
		}
		state.Slot.X = replaceAttachedEffectHighWord(state.Slot.X, uint16(parent.X>>16))
		state.Slot.Y = replaceAttachedEffectHighWord(state.Slot.Y, uint16(parent.Y>>16))
		if int16(parent.StunTicks) < 1 {
			cleared, out, err := clearAttachedEffect(state)
			if err != nil {
				return original, AttachedEffectOutcome{}, err
			}
			return cleared, out, nil
		}
		phase := (uint16(uint32(state.Slot.VelocityX)>>16) + 16) & 0x3ff
		state.Slot.VelocityX = replaceAttachedEffectHighWord(state.Slot.VelocityX, phase)
		x := int16(state.Slot.X>>16) + (int16(library.OrbitX[phase]) >> 2)
		y := int16(state.Slot.Y>>16) + (int16(library.OrbitY[phase]) >> 3)
		state.Slot.X = replaceAttachedEffectHighWord(state.Slot.X, uint16(x))
		state.Slot.Y = replaceAttachedEffectHighWord(state.Slot.Y, uint16(y))
		if parent.CharacterID >= uint8(len(library.OffsetY)) {
			return original, AttachedEffectOutcome{}, errors.New("status effect character offset is unavailable")
		}
		state.OffsetY = library.OffsetY[parent.CharacterID]
	}
	projection := ProjectFighter(ProjectionState{WorldX: state.Slot.X, WorldY: state.Slot.Y, ScreenX: state.ScreenX, ScreenY: state.ScreenY,
		OffsetX: state.OffsetX, OffsetY: state.OffsetY, CameraX: world.CameraX, CameraY: world.CameraY, Shake: world.Shake})
	state.ScreenX, state.ScreenY, state.OffsetX, state.OffsetY = projection.ScreenX, projection.ScreenY, projection.OffsetX, projection.OffsetY
	return state, AttachedEffectOutcome{Draw: true}, nil
}

func replaceAttachedEffectHighWord(value int32, word uint16) int32 {
	return int32(uint32(word)<<16 | uint32(uint16(value)))
}

func advanceAttachedEffectCountdown(state *AttachedEffectState, library map[uint32][]AttachedEffectFrame) error {
	for {
		state.Ticks--
		if state.Ticks != 0 {
			return nil
		}
		state.Index++
		frames, ok := library[state.Sequence]
		if !ok || len(frames) < 2 || len(frames) > 256 || frames[len(frames)-1].Duration != 0 {
			return errors.New("attached effect sequence is unavailable")
		}
		for _, frame := range frames[:len(frames)-1] {
			if frame.Duration == 0 || frame.Parameter > 510 || frame.Parameter&1 != 0 {
				return errors.New("invalid attached effect sequence")
			}
		}
		if int(state.Index) >= len(frames) {
			return errors.New("attached effect frame is outside sequence")
		}
		frame := frames[state.Index]
		state.Parameter, state.Ticks = frame.Parameter, frame.Duration
		if frame.Duration != 0 {
			return nil
		}
		state.Index, state.Ticks = 255, 1
	}
}

func clearAttachedEffect(state AttachedEffectState) (AttachedEffectState, AttachedEffectOutcome, error) {
	size := state.Slot.Size
	if size == 0 {
		size = 64
	}
	count := size / 16 * 16
	if count == 0 || count > 128 {
		return state, AttachedEffectOutcome{}, errors.New("effect clear exceeds validated slot storage")
	}
	state.Slot.Behavior, state.Slot.Size, state.Slot.Stage, state.ScreenX, state.ScreenY = 0, 0, 0, 0, 0
	if count >= 32 {
		state.Slot.VelocityX, state.GraphicsSet = 0, 0
	}
	if count >= 48 {
		state.TilesBase, state.Slot.Attributes, state.Slot.Layer, state.SkipTiles = 0, 0, 0, 0
		state.GraphicOffsetX, state.GraphicOffsetY, state.GraphicCache[0] = 0, 0, 0
	}
	if count >= 64 {
		state.GraphicCache[1], state.GraphicCache[2], state.GraphicCache[3], state.Parameter, state.Slot.Depth = 0, 0, 0, 0, 0
	}
	if count >= 80 {
		state.Slot.X, state.Slot.Y, state.Aux48 = 0, 0, 0
	}
	if count >= 96 {
		state.Sequence, state.OffsetX, state.OffsetY, state.Slot.Type = 0, 0, 0, 0
		state.Index, state.Delay, state.Ticks, state.Flags = 0, 0, 0, 0
	}
	return state, AttachedEffectOutcome{ClearedBytes: count}, nil
}
