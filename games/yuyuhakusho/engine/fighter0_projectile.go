package engine

import "errors"

type Fighter0ProjectileKind uint8

const (
	Fighter0ScatterProjectile Fighter0ProjectileKind = iota
	Fighter0ReleaseLight
)

// Both callbacks reuse the canonical projectile owners. The release light
// uses AnimationSet as its immutable short-sequence identity; it has no parent
// dependency or second copy of the same object fields.
type Fighter0ProjectileState struct {
	Kind       Fighter0ProjectileKind
	Projectile ChuProjectileState
}

type Fighter0ProjectileLibrary struct {
	Animations AnimationLibrary
	Sequences  map[uint32][]ChuAttachmentFrame
}

// StepFighter0Projectile recovers the complete rule callbacks 12c96/134c2,
// stopping before1580 sprite emission or after3556 prefix deletion. Source0's
// parent motion, shared1313e projectile and world scheduling remain separate.
func StepFighter0Projectile(state Fighter0ProjectileState, environment ChuProjectileEnvironment, library Fighter0ProjectileLibrary) (Fighter0ProjectileState, ChuProjectileOutcome, error) {
	return stepFighter0ProjectileWithSound(state, environment, library, nil)
}

func stepFighter0ProjectileWithSound(state Fighter0ProjectileState, environment ChuProjectileEnvironment, library Fighter0ProjectileLibrary, soundPort *worldSoundPort) (Fighter0ProjectileState, ChuProjectileOutcome, error) {
	before := state
	var outcome ChuProjectileOutcome
	var err error
	switch state.Kind {
	case Fighter0ScatterProjectile:
		state.Projectile, outcome, err = stepFighter0Scatter(state.Projectile, environment, library.Animations, soundPort)
	case Fighter0ReleaseLight:
		state.Projectile, outcome, err = stepFighter0ReleaseLight(state.Projectile, environment, library.Sequences)
	default:
		err = errors.New("unsupported source0 child/projectile kind")
	}
	if err != nil {
		return before, ChuProjectileOutcome{}, err
	}
	return state, outcome, nil
}

func initializeFighter0ProjectileGraphics(state *ChuProjectileState, palette uint16) {
	state.Stage += 2
	state.Animation.GraphicOffsetX, state.Animation.GraphicOffsetY, state.Animation.GraphicParameter = 0, 0, 0
	state.GraphicCache = [4]uint32{}
	state.GraphicSet, state.TileBase = 0xd8f1a, 0x240000
	state.Animation.Attributes |= palette
}

func projectFighter0Projectile(state *ChuProjectileState, world ChuProjectileEnvironment) {
	projection := ProjectFighter(ProjectionState{WorldX: state.Animation.WorldX, WorldY: state.Animation.WorldY,
		ScreenX: state.ScreenX, ScreenY: state.ScreenY, OffsetX: state.ScreenOffsetX, OffsetY: state.ScreenOffsetY,
		CameraX: world.CameraX, CameraY: world.CameraY, Shake: world.Shake})
	state.ScreenX, state.ScreenY, state.ScreenOffsetX, state.ScreenOffsetY = projection.ScreenX, projection.ScreenY, projection.OffsetX, projection.OffsetY
}

func stepFighter0Scatter(state ChuProjectileState, world ChuProjectileEnvironment, library AnimationLibrary, soundPort *worldSoundPort) (ChuProjectileState, ChuProjectileOutcome, error) {
	if state.Stage == 0 {
		initializeFighter0ProjectileGraphics(&state, world.PaletteAttributes)
		state.Animation.AnimationSet, state.Animation.ShapeSet = 0xd92ca, 0xd9a3a
		// The creator's index, flags and completion byte survive initialization.
		state.Animation.Clock.Duration, state.Animation.Clock.Ticks = 8, 255
	}
	state.Health = 255
	if world.PauseFlags&0x80 != 0 || world.SlowFlag != 0 {
		return state, ChuProjectileOutcome{Draw: true}, nil
	}
	state.Shake = 0
	state.Animation.Clock = StepAnimationClock(state.Animation.Clock)
	if state.Animation.Clock.Completed == 1 {
		return clearChuProjectile(state)
	}
	state.Animation.WorldX += state.VelocityX
	state.Animation.WorldY += state.VerticalVelocity
	impact := state.VerticalVelocity >= 0 && int16(state.Ground>>16) <= int16(state.Animation.WorldY>>16)
	if !impact {
		x := int16(state.Animation.WorldX >> 16)
		if x > world.ArenaRight+32 || x < world.ArenaLeft-32 {
			return clearChuProjectile(state)
		}
		impact = state.ContactFlags&0x18 != 0
	}
	if impact {
		pool, index, err := AllocateEffect(state.Pool)
		if err != nil {
			return state, ChuProjectileOutcome{}, err
		}
		state.Pool = pool
		if index >= 0 {
			slot := &state.Pool.Slots[index]
			slot.Type = 0x18
			// 1fdbe writes only the coordinate high words, retaining fractions.
			slot.X = int32(uint32(uint16(state.Animation.WorldX>>16))<<16 | uint32(uint16(slot.X)))
			slot.Y = int32(uint32(uint16(state.Animation.WorldY>>16))<<16 | uint32(uint16(slot.Y)))
			slot.Attributes = state.Animation.Attributes&0x9800 | 0x2000
			slot.Depth, slot.Layer = state.Depth, state.Layer-4
		}
		return clearChuProjectile(state)
	}
	submission, err := submitAnimationRecordWithSound(AnimationSubmissionState{Actor: state.Animation, Buffers: state.Buffers,
		PendingSound: state.PendingSound, SoundQueue: state.SoundQueue}, library, soundPort)
	if err != nil {
		return state, ChuProjectileOutcome{}, err
	}
	state.Animation, state.Buffers, state.PendingSound, state.SoundQueue = submission.Actor, submission.Buffers, submission.PendingSound, submission.SoundQueue
	projectFighter0Projectile(&state, world)
	return state, ChuProjectileOutcome{Draw: true}, nil
}

func stepFighter0ReleaseLight(state ChuProjectileState, world ChuProjectileEnvironment, sequences map[uint32][]ChuAttachmentFrame) (ChuProjectileState, ChuProjectileOutcome, error) {
	if state.Stage == 0 {
		initializeFighter0ProjectileGraphics(&state, world.PaletteAttributes)
		state.Animation.AnimationSet = 0x135ee
		state.Animation.Clock.FrameIndex, state.Animation.Clock.Ticks, state.Animation.Clock.Flags = 255, 1, 0
	}
	if world.PauseFlags&0x80 != 0 || world.SlowFlag != 0 {
		return state, ChuProjectileOutcome{Draw: true}, nil
	}
	level := int16(state.Power) >> 5
	if level == 0 {
		state.Animation.AnimationSet = 0x135ee
	} else if low := uint8(level); low >= 1 && low <= 7 {
		state.Animation.AnimationSet = 0x135ee + uint32(low)*6
	}
	state.Animation.Clock.Ticks--
	if state.Animation.Clock.Ticks == 0 {
		state.Animation.Clock.FrameIndex++
		frames, exists := sequences[state.Animation.AnimationSet]
		if !exists || len(frames) < 2 || len(frames) > 256 || frames[len(frames)-1].Duration != 0 {
			return state, ChuProjectileOutcome{}, errors.New("source0 release sequence is unavailable or unterminated")
		}
		for i, frame := range frames {
			if frame.Parameter > 510 || frame.Parameter&1 != 0 || i < len(frames)-1 && frame.Duration == 0 {
				return state, ChuProjectileOutcome{}, errors.New("invalid source0 release sequence")
			}
		}
		index := int(state.Animation.Clock.FrameIndex)
		if index >= len(frames) {
			return state, ChuProjectileOutcome{}, errors.New("source0 release frame is outside finite sequence")
		}
		frame := frames[index]
		state.Animation.GraphicParameter, state.Animation.Clock.Ticks = frame.Parameter, frame.Duration
		if frame.Duration == 0 {
			return clearChuProjectile(state)
		}
	}
	projectFighter0Projectile(&state, world)
	return state, ChuProjectileOutcome{Draw: true}, nil
}
