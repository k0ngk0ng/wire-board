package engine

import "errors"

type ChuAttachmentKind uint8

const (
	ChuChargeAttachment ChuAttachmentKind = iota
	ChuReleaseAttachment
)

type ChuAttachmentFrame struct {
	Parameter uint16 `json:"parameter"`
	Duration  uint8  `json:"duration"`
}

// Sequence identities preserve immutable data provenance, never instructions
// executed by the engine. Charge countdowns and generic release records use
// distinct libraries because their original timing/data formats differ.
type ChuAttachmentLibrary struct {
	Sequences  map[uint32][]ChuAttachmentFrame
	Animations AnimationLibrary
}

type ChuAttachmentParent struct {
	CharacterID uint8
	FrameIndex  uint8
	Pose        uint8
	X, Y        int32
	Power       int16
}

type ChuAttachmentWorld struct {
	PauseFlags    uint8  `json:"pause_flags"`
	SlowFlag      uint8  `json:"slow_flag"`
	AttributeMask uint16 `json:"attribute_mask"`
	CameraX       int16  `json:"camera_x"`
	CameraY       int16  `json:"camera_y"`
	Shake         int16  `json:"shake"`
}

type ChuAttachmentState struct {
	Kind                         ChuAttachmentKind
	Actor                        AnimationActorState
	Behavior                     uint32
	Size, Stage                  uint16
	GraphicsSet, TilesBase       uint32
	GraphicCache                 [4]uint32
	Power, Health                int16
	ScreenX, ScreenY             int16
	ScreenOffsetX, ScreenOffsetY int16
	SourceOffsetX, SourceOffsetY int16
	Buffers                      [4]AnimationShapeBuffer
	PendingSound                 uint8
	SoundQueue                   [4]uint8
}

type ChuAttachmentOutcome struct {
	Draw         bool
	ClearedBytes uint16
}

// StepChuAttachment recovers the complete rule stage of the charge/release
// callbacks, ending at their 0x1580 presentation dispatch or after 0x3556 clear.
// Parent/world values are read-only inputs. Drawing does not encode an MD DMA
// or sprite queue here; an independent renderer consumes the returned object.
func StepChuAttachment(state ChuAttachmentState, parent ChuAttachmentParent, world ChuAttachmentWorld, library ChuAttachmentLibrary) (ChuAttachmentState, ChuAttachmentOutcome, error) {
	return stepChuAttachmentWithSound(state, parent, world, library, nil)
}

func stepChuAttachmentWithSound(state ChuAttachmentState, parent ChuAttachmentParent, world ChuAttachmentWorld, library ChuAttachmentLibrary, soundPort *worldSoundPort) (ChuAttachmentState, ChuAttachmentOutcome, error) {
	original := state
	if state.Kind != ChuChargeAttachment && state.Kind != ChuReleaseAttachment {
		return original, ChuAttachmentOutcome{}, errors.New("unsupported Chu attachment kind")
	}
	if state.Stage == 0 {
		state.Stage += 2
		state.Actor.GraphicOffsetX, state.Actor.GraphicOffsetY, state.Actor.GraphicParameter = 0, 0, 0
		state.GraphicCache = [4]uint32{}
		state.GraphicsSet, state.TilesBase = 0xd8f1a, 0x240000
		state.Actor.Attributes |= world.AttributeMask
		if state.Kind == ChuChargeAttachment {
			state.Actor.AnimationSet = 0x13492
			state.Actor.Clock.FrameIndex, state.Actor.Clock.Ticks, state.Actor.Clock.Flags = 255, 1, 0
		} else {
			state.Actor.AnimationSet, state.Actor.ShapeSet = 0xd92ca, 0xd9a3a
			state.Actor.Clock.Duration, state.Actor.Clock.Ticks = 8, 255
		}
	}
	if state.Kind == ChuReleaseAttachment {
		state.Health = 255
	}
	if world.PauseFlags&0x80 != 0 || world.SlowFlag != 0 {
		return state, ChuAttachmentOutcome{Draw: true}, nil
	}
	if state.Kind == ChuChargeAttachment {
		level := state.Power >> 5
		if level == 0 {
			state.Actor.AnimationSet = 0x13492
		} else if low := uint8(level); low >= 1 && low <= 7 {
			// Only the first comparison uses the full word. Later original
			// comparisons use its low byte, including noncanonical aliases.
			state.Actor.AnimationSet = 0x13492 + uint32(low)*6
		}
		if err := advanceChuAttachmentCountdown(&state, library.Sequences); err != nil {
			return original, ChuAttachmentOutcome{}, err
		}
		state.ScreenOffsetX, state.ScreenOffsetY = state.SourceOffsetX, state.SourceOffsetY
		state.Actor.WorldX, state.Actor.WorldY = parent.X, parent.Y
		state.Power += 16
		if uint16(parent.Power) <= uint16(state.Power) {
			state.Power = parent.Power
		} else {
			state.Actor.Clock.FrameIndex, state.Actor.Clock.Ticks = 0, 1
		}
		alive := true
		switch parent.CharacterID {
		case 0, 3:
			alive = parent.FrameIndex == 4
		case 9:
			alive = parent.Pose == 0xa6
		}
		if !alive {
			next, outcome, err := clearChuAttachment(state)
			if err != nil {
				return original, ChuAttachmentOutcome{}, err
			}
			return next, outcome, nil
		}
	} else {
		state.Actor.Clock = StepAnimationClock(state.Actor.Clock)
		if state.Actor.Clock.Completed == 1 {
			next, outcome, err := clearChuAttachment(state)
			if err != nil {
				return original, ChuAttachmentOutcome{}, err
			}
			return next, outcome, nil
		}
		state.Actor.WorldX, state.Actor.WorldY = parent.X, parent.Y
		if parent.Pose != 0xa7 {
			next, outcome, err := clearChuAttachment(state)
			if err != nil {
				return original, ChuAttachmentOutcome{}, err
			}
			return next, outcome, nil
		}
		submission, err := submitAnimationRecordWithSound(AnimationSubmissionState{Actor: state.Actor, Buffers: state.Buffers, PendingSound: state.PendingSound, SoundQueue: state.SoundQueue}, library.Animations, soundPort)
		if err != nil {
			return original, ChuAttachmentOutcome{}, err
		}
		state.Actor, state.Buffers, state.PendingSound, state.SoundQueue = submission.Actor, submission.Buffers, submission.PendingSound, submission.SoundQueue
	}
	projection := ProjectFighter(ProjectionState{WorldX: state.Actor.WorldX, WorldY: state.Actor.WorldY, ScreenX: state.ScreenX, ScreenY: state.ScreenY,
		OffsetX: state.ScreenOffsetX, OffsetY: state.ScreenOffsetY, CameraX: world.CameraX, CameraY: world.CameraY, Shake: world.Shake})
	state.ScreenX, state.ScreenY, state.ScreenOffsetX, state.ScreenOffsetY = projection.ScreenX, projection.ScreenY, projection.OffsetX, projection.OffsetY
	return state, ChuAttachmentOutcome{Draw: true}, nil
}

func advanceChuAttachmentCountdown(state *ChuAttachmentState, library map[uint32][]ChuAttachmentFrame) error {
	for {
		state.Actor.Clock.Ticks--
		if state.Actor.Clock.Ticks != 0 {
			return nil
		}
		state.Actor.Clock.FrameIndex++
		frames, ok := library[state.Actor.AnimationSet]
		if !ok || len(frames) < 2 || len(frames) > 256 || frames[len(frames)-1].Duration != 0 {
			return errors.New("charge attachment sequence is unavailable")
		}
		for _, frame := range frames[:len(frames)-1] {
			if frame.Duration == 0 || frame.Parameter > 510 || frame.Parameter&1 != 0 {
				return errors.New("invalid charge attachment sequence")
			}
		}
		index := int(state.Actor.Clock.FrameIndex)
		if index >= len(frames) {
			return errors.New("charge attachment frame is outside sequence")
		}
		frame := frames[index]
		state.Actor.GraphicParameter, state.Actor.Clock.Ticks = frame.Parameter, frame.Duration
		if frame.Duration != 0 {
			return nil
		}
		state.Actor.Clock.FrameIndex, state.Actor.Clock.Ticks = 255, 1
	}
}

func clearChuAttachment(state ChuAttachmentState) (ChuAttachmentState, ChuAttachmentOutcome, error) {
	size := state.Size
	if size == 0 {
		size = 64
	}
	count := size / 16 * 16
	if count == 0 || count > 256 {
		return state, ChuAttachmentOutcome{}, errors.New("attachment clear exceeds validated object storage")
	}
	// Object identity/kind and world buffers are not stored inside the cleared
	// object bytes. Each group below follows the original sixteen-byte stores;
	// partial clear sizes preserve semantic fields beyond the cleared prefix.
	state.Behavior, state.Size, state.Stage, state.ScreenX, state.ScreenY = 0, 0, 0, 0, 0
	if count >= 32 {
		state.SourceOffsetX, state.SourceOffsetY, state.GraphicsSet = 0, 0, 0
	}
	if count >= 48 {
		state.TilesBase, state.Actor.Attributes, state.Actor.GraphicOffsetX, state.Actor.GraphicOffsetY, state.GraphicCache[0] = 0, 0, 0, 0, 0
	}
	if count >= 64 {
		state.GraphicCache[1], state.GraphicCache[2], state.GraphicCache[3], state.Actor.GraphicParameter = 0, 0, 0, 0
	}
	if count >= 80 {
		state.Actor.WorldX, state.Actor.WorldY = 0, 0
	}
	if count >= 96 {
		state.Actor.AnimationSet, state.ScreenOffsetX, state.ScreenOffsetY, state.Actor.Pose = 0, 0, 0, 0
		state.Actor.Clock.FrameIndex, state.Actor.Clock.Duration, state.Actor.Clock.Ticks, state.Actor.Clock.Flags = 0, 0, 0, 0
	}
	if count >= 112 {
		state.Actor.Clock.Completed, state.Actor.Facing, state.Actor.EnabledStages, state.Actor.ActionFlags = 0, 0, 0, 0
		state.Actor.Auxiliary64, state.Actor.Value65, state.Actor.Value66, state.Actor.Value67, state.Actor.Value68 = 0, 0, 0, 0, 0
	}
	if count >= 128 {
		state.Health, state.Power = 0, 0
	}
	if count >= 144 {
		state.Actor.ShapeSet = 0
	}
	if count >= 160 {
		state.Actor.Auxiliary94 = 0
	}
	return state, ChuAttachmentOutcome{ClearedBytes: count}, nil
}
