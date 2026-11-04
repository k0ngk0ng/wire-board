package engine

import "errors"

// Fighter8ProjectileState is147ae's narrow view. Every field maps to its
// existing canonical owner; Animation holds the only logical ObjectID.
// Parent, velocity, power, team, layer and FX state are not inputs to this rule.
type Fighter8ProjectileState struct {
	Header                       MatchObjectHeader
	Animation                    AnimationActorState
	GraphicsSet, TilesBase       uint32
	GraphicCache                 [4]uint32
	Health                       int16
	Shake                        uint8
	ScreenX, ScreenY             int16
	ScreenOffsetX, ScreenOffsetY int16
	Buffers                      [4]AnimationShapeBuffer
	PendingSound                 uint8
	SoundQueue                   [4]uint8
}

// StepFighter8Projectile stops at1483e's draw request or after1487c/3556's
// clear. It advances its own animation, never follows or updates a parent.
func StepFighter8Projectile(state Fighter8ProjectileState, world ChuAttachmentWorld, library AnimationLibrary) (Fighter8ProjectileState, ChuProjectileOutcome, error) {
	before := state
	if state.Header.Stage == 0 {
		state.Header.Stage += 2
		state.Animation.GraphicOffsetX, state.Animation.GraphicOffsetY, state.Animation.GraphicParameter = 0, 0, 0
		state.GraphicCache = [4]uint32{}
		state.GraphicsSet, state.TilesBase = 0xd104a, 0x1e0000
		state.Animation.AnimationSet, state.Animation.ShapeSet = 0xd1e5a, 0xd2c6a
		state.Animation.Attributes |= world.AttributeMask
		state.Animation.Clock.Duration, state.Animation.Clock.Ticks = 8, 255
	}
	state.Health = 255
	if world.PauseFlags&0x80 != 0 || world.SlowFlag != 0 {
		return state, ChuProjectileOutcome{Draw: true}, nil
	}
	state.Shake = 0
	state.Animation.Clock = StepAnimationClock(state.Animation.Clock)
	if state.Animation.Clock.Completed == 1 {
		next, outcome, err := clearFighter8Projectile(state)
		if err != nil {
			return before, ChuProjectileOutcome{}, err
		}
		return next, outcome, nil
	}
	submission, err := SubmitAnimationRecord(AnimationSubmissionState{Actor: state.Animation, Buffers: state.Buffers,
		PendingSound: state.PendingSound, SoundQueue: state.SoundQueue}, library)
	if err != nil {
		return before, ChuProjectileOutcome{}, err
	}
	state.Animation, state.Buffers, state.PendingSound, state.SoundQueue = submission.Actor, submission.Buffers, submission.PendingSound, submission.SoundQueue
	projection := ProjectFighter(ProjectionState{WorldX: state.Animation.WorldX, WorldY: state.Animation.WorldY,
		ScreenX: state.ScreenX, ScreenY: state.ScreenY, OffsetX: state.ScreenOffsetX, OffsetY: state.ScreenOffsetY,
		CameraX: world.CameraX, CameraY: world.CameraY, Shake: world.Shake})
	state.ScreenX, state.ScreenY, state.ScreenOffsetX, state.ScreenOffsetY = projection.ScreenX, projection.ScreenY, projection.OffsetX, projection.OffsetY
	return state, ChuProjectileOutcome{Draw: true}, nil
}

func clearFighter8Projectile(state Fighter8ProjectileState) (Fighter8ProjectileState, ChuProjectileOutcome, error) {
	size := state.Header.Size
	if size == 0 {
		size = 64
	}
	count := size / 16 * 16
	if count == 0 || count > 256 {
		return state, ChuProjectileOutcome{}, errors.New("source8 child clear exceeds logical slot")
	}
	state.Header, state.ScreenX, state.ScreenY = MatchObjectHeader{}, 0, 0
	if count >= 32 {
		state.GraphicsSet = 0
	}
	if count >= 48 {
		state.TilesBase, state.Animation.Attributes = 0, 0
		state.Animation.GraphicOffsetX, state.Animation.GraphicOffsetY, state.GraphicCache[0] = 0, 0, 0
	}
	if count >= 64 {
		state.GraphicCache[1], state.GraphicCache[2], state.GraphicCache[3], state.Animation.GraphicParameter = 0, 0, 0, 0
	}
	if count >= 80 {
		state.Animation.WorldX, state.Animation.WorldY = 0, 0
	}
	if count >= 96 {
		state.Animation.AnimationSet, state.ScreenOffsetX, state.ScreenOffsetY = 0, 0, 0
		state.Animation.Clock.FrameIndex, state.Animation.Clock.Duration, state.Animation.Clock.Ticks, state.Animation.Clock.Flags, state.Animation.Pose = 0, 0, 0, 0, 0
	}
	if count >= 112 {
		state.Animation.Clock.Completed, state.Animation.Facing, state.Animation.Auxiliary64 = 0, 0, 0
		state.Animation.Value65, state.Animation.Value66, state.Animation.Value67, state.Animation.Value68, state.Shake = 0, 0, 0, 0, 0
		state.Animation.EnabledStages, state.Animation.ActionFlags = 0, 0
	}
	if count >= 128 {
		state.Health = 0
	}
	if count >= 144 {
		state.Animation.ShapeSet = 0
	}
	if count >= 160 {
		state.Animation.Auxiliary94 = 0
	}
	return state, ChuProjectileOutcome{Deleted: true, ClearedBytes: count}, nil
}
