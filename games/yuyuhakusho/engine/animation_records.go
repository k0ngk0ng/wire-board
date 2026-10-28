package engine

import "errors"

// AnimationFrameKey identifies immutable data. Set identifiers preserve source
// identity during recovery; this engine never executes them or reads a ROM.
type AnimationFrameKey struct {
	AnimationSet uint32 `json:"animation_set"`
	ShapeSet     uint32 `json:"shape_set"`
	Pose         uint8  `json:"pose"`
	Index        uint8  `json:"index"`
}

type AnimationShapeDefinition struct {
	Width   uint8 `json:"width"`
	OffsetX int16 `json:"offset_x"`
	Height  uint8 `json:"height"`
	OffsetY int16 `json:"offset_y"`
}

// AnimationRecordData is decoded once from the immutable animation/shape data.
// The first three lists retain their terminal width bit for validation; the
// fourth class is exactly one record and does not use that bit as a terminator.
type AnimationRecordData struct {
	Duration         uint8                         `json:"duration"`
	Flags            uint8                         `json:"flags"`
	EnabledStages    uint8                         `json:"enabled_stages"`
	Sound            uint8                         `json:"sound"`
	OffsetX          int16                         `json:"offset_x"`
	OffsetY          int16                         `json:"offset_y"`
	GraphicParameter uint16                        `json:"graphic_parameter"`
	ShapeFlags       uint8                         `json:"shape_flags"`
	Metadata         [4]uint8                      `json:"metadata"`
	Groups           [4][]AnimationShapeDefinition `json:"groups"`
}

type AnimationLibrary map[AnimationFrameKey]AnimationRecordData

type AnimationActorState struct {
	ObjectID         uint16         `json:"object_id"`
	AnimationSet     uint32         `json:"animation_set"`
	ShapeSet         uint32         `json:"shape_set"`
	Pose             uint8          `json:"pose"`
	Clock            AnimationClock `json:"clock"`
	Facing           uint8          `json:"facing"`
	ActionFlags      uint8          `json:"action_flags"`
	EnabledStages    uint8          `json:"enabled_stages"`
	Attributes       uint16         `json:"attributes"`
	GraphicOffsetX   int16          `json:"graphic_offset_x"`
	GraphicOffsetY   int16          `json:"graphic_offset_y"`
	WorldX           int32          `json:"world_x"`
	WorldY           int32          `json:"world_y"`
	GraphicParameter uint16         `json:"graphic_parameter"`
	Auxiliary94      uint32         `json:"auxiliary_94"`
	Auxiliary64      uint8          `json:"auxiliary_64"`
	Value65          uint8          `json:"value_65"`
	Value66          uint8          `json:"value_66"`
	Value67          uint8          `json:"value_67"`
	Value68          uint8          `json:"value_68"`
}

// The four preserved words are not written by this producer. They remain in
// persisted records until the consumers establish their meaning.
type AnimationShapeRecord struct {
	X, Y          int16
	Width, Height uint8
	Owner         uint16
	Preserved     [4]uint16
}

type AnimationShapeBuffer struct {
	Offset  uint16
	Records [16]AnimationShapeRecord
}

type AnimationSubmissionState struct {
	Actor        AnimationActorState
	Buffers      [4]AnimationShapeBuffer
	PendingSound uint8
	SoundQueue   [4]uint8
}

// SubmitAnimationRecord recovers 0x1eb40 and the 0x21960 queue tail. Callers
// supply a read-only library; record selection uses this actor's own pose and
// clock index. It does not advance the animation clock or consume sound slots.
func SubmitAnimationRecord(state AnimationSubmissionState, library AnimationLibrary) (AnimationSubmissionState, error) {
	key := AnimationFrameKey{AnimationSet: state.Actor.AnimationSet, ShapeSet: state.Actor.ShapeSet, Pose: state.Actor.Pose, Index: state.Actor.Clock.FrameIndex}
	data, ok := library[key]
	if !ok {
		return state, errors.New("animation frame is unavailable")
	}
	for group, list := range data.Groups {
		active := data.ShapeFlags&(0x40>>group) != 0
		if !active {
			if len(list) != 0 {
				return state, errors.New("inactive shape class has records")
			}
			continue
		}
		if len(list) == 0 || group == 3 && len(list) != 1 {
			return state, errors.New("invalid shape list length")
		}
		if state.Buffers[group].Offset < 255 && state.Buffers[group].Offset&15 != 0 {
			return state, errors.New("unaligned shape buffer offset")
		}
		if group < 3 {
			for i, s := range list {
				if (s.Width&0x80 != 0) != (i == len(list)-1) {
					return state, errors.New("invalid shape termination marker")
				}
			}
		}
	}
	actor := &state.Actor
	actor.Clock.Duration = data.Duration
	actor.Clock.Flags = data.Flags
	actor.EnabledStages = data.EnabledStages | actor.ActionFlags
	if actor.Clock.Ticks == 0 {
		if data.Sound != 0 {
			state.PendingSound = data.Sound
		}
		if data.Flags&0x10 != 0 {
			actor.Auxiliary94 = 0xffffffff
		}
	}
	actor.GraphicParameter = data.GraphicParameter
	offsetX := data.OffsetX
	if actor.Facing != 0 {
		offsetX = -offsetX
	}
	flips := (data.Flags ^ actor.Facing) & 3
	actor.Attributes = actor.Attributes&^0x1800 | uint16(flips)<<11
	actor.GraphicOffsetX, actor.GraphicOffsetY = offsetX, data.OffsetY
	x, y := int16(actor.WorldX>>16)+offsetX, int16(actor.WorldY>>16)+data.OffsetY
	actor.Value65 = 0
	if data.ShapeFlags&0x40 != 0 {
		actor.Value65, actor.Value66, actor.Auxiliary64 = data.Metadata[0], data.Metadata[1], data.Metadata[2]
		actor.Value67, actor.Value68 = data.Metadata[3]>>4, data.Metadata[3]&15
	}
	for group, list := range data.Groups {
		if data.ShapeFlags&(0x40>>group) == 0 {
			continue
		}
		buffer := &state.Buffers[group]
		for _, shape := range list {
			if buffer.Offset >= 255 {
				buffer.Offset = 240
			}
			record := &buffer.Records[buffer.Offset/16]
			dx, dy := shape.OffsetX, shape.OffsetY
			if flips&1 != 0 {
				dx = -dx
			}
			if flips&2 != 0 {
				dy = -dy
			}
			record.Owner = actor.ObjectID
			record.Width, record.Height = shape.Width, shape.Height
			if group < 3 {
				record.Width &= 0x7f
			}
			record.X, record.Y = x+dx, y+dy
			buffer.Offset += 16
		}
	}
	if state.PendingSound != 0 {
		request := state.PendingSound
		state.PendingSound = 0
		for i, value := range state.SoundQueue {
			if value < 128 || value == request {
				state.SoundQueue[i] = request
				break
			}
		}
	}
	return state, nil
}
