package engine

import "errors"

type EffectMotionFrame struct {
	Graphic  uint8 `json:"graphic"`
	Duration uint8 `json:"duration"`
}

type EffectMotionType struct {
	Sequence      uint32 `json:"sequence"`
	GraphicSet    uint32 `json:"graphic_set"`
	TileBase      uint32 `json:"tile_base"`
	SetTileBase   bool   `json:"set_tile_base"`
	AttributeOR   uint16 `json:"attribute_or"`
	ClearVelocity bool   `json:"clear_velocity"`
	DynamicTiles  bool   `json:"dynamic_tiles"`
	SetSkipTiles  bool   `json:"set_skip_tiles"`
	SkipTiles     uint8  `json:"skip_tiles"`
	Sound         uint8  `json:"sound"`
}

// EffectMotionLibrary is decoded immutable rule data. Sequence identifiers
// preserve source identity; the rule engine does not read or execute a ROM.
type EffectMotionLibrary struct {
	Types     []EffectMotionType             `json:"types"`
	Sequences map[uint32][]EffectMotionFrame `json:"sequences"`
}

// EffectMotionState extends the allocator's shared slot with fields used by
// the generic 0x149ac lifecycle. Other callback kinds have separate lifecycles.
type EffectMotionState struct {
	ObjectID                        uint16
	Slot                            EffectSlot
	VerticalVelocity                int32
	GraphicSet, TileBase, Sequence  uint32
	GraphicCache                    [4]uint32
	GraphicOffsetX, GraphicOffsetY  int16
	GraphicParameter                uint16
	SkipTiles                       uint8
	FrameIndex, Delay, Ticks, Flags uint8
	FollowOwner                     uint16
	ScreenX, ScreenY                int16
	ScreenOffsetX, ScreenOffsetY    int16
}

type EffectMotionWorld struct {
	PauseFlags        uint8    `json:"pause_flags"`
	SlowFlag          uint8    `json:"slow_flag"`
	PaletteAttributes uint16   `json:"palette_attributes"`
	TileCursor        uint8    `json:"tile_cursor"`
	SoundQueue        [4]uint8 `json:"sound_queue"`
	CameraX           int16    `json:"camera_x"`
	CameraY           int16    `json:"camera_y"`
	Shake             int16    `json:"shake"`
}

type EffectMotionOwner struct {
	X, Y int32
}

type EffectMotionOutcome struct {
	Draw, Deleted bool
	ClearedBytes  uint16
}

// StepEffectMotion recovers the generic FX rules through their actual return,
// draw request or completed deletion. Initialization returns without drawing,
// before pause/slow-time checks. Delay returns also omit drawing and movement.
// Owners and immutable library data remain unchanged, including on error.
func StepEffectMotion(state EffectMotionState, world EffectMotionWorld, owners map[uint16]EffectMotionOwner, library EffectMotionLibrary) (EffectMotionState, EffectMotionWorld, EffectMotionOutcome, error) {
	before, beforeWorld := state, world
	if state.Slot.Stage == 0 {
		if int(state.Slot.Type) >= len(library.Types) {
			return before, beforeWorld, EffectMotionOutcome{}, errors.New("generic FX type initializer is unavailable")
		}
		definition := library.Types[state.Slot.Type]
		state.Slot.Stage += 2
		state.Slot.Attributes |= world.PaletteAttributes
		state.GraphicOffsetX, state.GraphicOffsetY, state.GraphicParameter = 0, 0, 0
		state.GraphicCache = [4]uint32{}
		state.FrameIndex, state.Ticks, state.Flags = 255, 1, 0
		state.Sequence, state.GraphicSet = definition.Sequence, definition.GraphicSet
		state.Slot.Attributes |= definition.AttributeOR
		if definition.SetTileBase {
			state.TileBase = definition.TileBase
		}
		if definition.ClearVelocity {
			state.Slot.VelocityX, state.VerticalVelocity = 0, 0
		}
		if definition.DynamicTiles {
			// ADD.B wraps D0's low byte before ADD.W changes attributes.
			delta := uint16(uint8(0x80 + world.TileCursor))
			state.Slot.Attributes += delta
			world.TileCursor += 16
			if world.TileCursor >= 64 {
				world.TileCursor = 0
			}
		}
		if definition.SetSkipTiles {
			state.SkipTiles = definition.SkipTiles
		}
		if definition.Sound != 0 {
			for i, queued := range world.SoundQueue {
				if queued < 128 || queued == definition.Sound {
					world.SoundQueue[i] = definition.Sound
					break
				}
			}
		}
		return state, world, EffectMotionOutcome{}, nil
	}
	if world.PauseFlags&0x80 != 0 || world.SlowFlag != 0 {
		return state, world, EffectMotionOutcome{Draw: true}, nil
	}
	if state.Delay != 0 {
		state.Delay--
		return state, world, EffectMotionOutcome{}, nil
	}
	state.Ticks--
	if state.Ticks == 0 {
		state.FrameIndex++
		frames, ok := library.Sequences[state.Sequence]
		if !ok || int(state.FrameIndex) >= len(frames) {
			return before, beforeWorld, EffectMotionOutcome{}, errors.New("generic FX sequence frame is unavailable")
		}
		frame := frames[state.FrameIndex]
		state.GraphicParameter, state.Ticks = uint16(frame.Graphic)*2, frame.Duration
		if state.Ticks == 0 {
			next, result, err := clearEffectMotion(state)
			if err != nil {
				return before, beforeWorld, EffectMotionOutcome{}, err
			}
			return next, world, result, nil
		}
	}
	if state.FollowOwner == 0 {
		state.Slot.X += state.Slot.VelocityX
		state.Slot.Y += state.VerticalVelocity
	} else {
		owner := EffectMotionOwner{X: state.Slot.X, Y: state.Slot.Y}
		if state.FollowOwner != state.ObjectID {
			var ok bool
			owner, ok = owners[state.FollowOwner]
			if !ok {
				return before, beforeWorld, EffectMotionOutcome{}, errors.New("generic FX follow owner is unavailable")
			}
		}
		// MOVE.W copies the owner's high coordinate word only.
		state.Slot.X = int32(uint32(owner.X)&0xffff0000 | uint32(state.Slot.X)&0xffff)
		state.Slot.Y = int32(uint32(owner.Y)&0xffff0000 | uint32(state.Slot.Y)&0xffff)
	}
	projection := ProjectFighter(ProjectionState{WorldX: state.Slot.X, WorldY: state.Slot.Y, ScreenX: state.ScreenX, ScreenY: state.ScreenY,
		OffsetX: state.ScreenOffsetX, OffsetY: state.ScreenOffsetY, CameraX: world.CameraX, CameraY: world.CameraY, Shake: world.Shake})
	state.ScreenX, state.ScreenY, state.ScreenOffsetX, state.ScreenOffsetY = projection.ScreenX, projection.ScreenY, projection.OffsetX, projection.OffsetY
	return state, world, EffectMotionOutcome{Draw: true}, nil
}

func clearEffectMotion(state EffectMotionState) (EffectMotionState, EffectMotionOutcome, error) {
	count := (state.Slot.Size >> 4) << 4
	if state.Slot.Size == 0 {
		count = 64
	}
	if count == 0 || count > 128 {
		return state, EffectMotionOutcome{}, errors.New("generic FX deletion exceeds logical slot boundary")
	}
	state.Slot.Behavior, state.Slot.Size, state.Slot.Stage, state.ScreenX, state.ScreenY = 0, 0, 0, 0, 0
	if count >= 32 {
		state.Slot.VelocityX, state.VerticalVelocity, state.GraphicSet = 0, 0, 0
	}
	if count >= 48 {
		state.TileBase, state.Slot.Attributes, state.Slot.Layer, state.SkipTiles = 0, 0, 0, 0
		state.GraphicOffsetX, state.GraphicOffsetY, state.GraphicCache[0] = 0, 0, 0
	}
	if count >= 64 {
		state.GraphicCache[1], state.GraphicCache[2], state.GraphicCache[3], state.GraphicParameter, state.Slot.Depth = 0, 0, 0, 0, 0
	}
	if count >= 80 {
		state.Slot.X, state.Slot.Y, state.FollowOwner = 0, 0, 0
	}
	if count >= 96 {
		state.Sequence, state.ScreenOffsetX, state.ScreenOffsetY = 0, 0, 0
		state.Slot.Type, state.FrameIndex, state.Delay, state.Ticks, state.Flags = 0, 0, 0, 0, 0
	}
	return state, EffectMotionOutcome{Deleted: true, ClearedBytes: count}, nil
}
