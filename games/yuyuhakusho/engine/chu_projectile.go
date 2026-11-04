package engine

import "errors"

// ChuProjectileState owns the complete rule state read or written by 0x1313e.
// Animation owns world coordinates and frame fields; screen coordinates are
// separate. Resource identifiers retain provenance but are never executed.
type ChuProjectileState struct {
	Animation                              AnimationActorState
	Behavior, GraphicSet, TileBase         uint32
	Size, Stage                            uint16
	GraphicCache                           [4]uint32
	VelocityX, VerticalVelocity, Ground    int32
	Power                                  uint16
	TrailPhase, Shake, ContactFlags, Layer uint8
	Team, TargetBit                        uint8
	Health                                 int16
	Depth                                  uint16
	ScreenX, ScreenY                       int16
	ScreenOffsetX, ScreenOffsetY           int16
	Pool                                   EffectPool
	Buffers                                [4]AnimationShapeBuffer
	PendingSound                           uint8
	SoundQueue                             [4]uint8
}

type ChuProjectileEnvironment struct {
	PauseFlags        uint8  `json:"pause_flags"`
	SlowFlag          uint8  `json:"slow_flag"`
	PaletteAttributes uint16 `json:"palette_attributes"`
	ArenaLeft         int16  `json:"arena_left"`
	ArenaRight        int16  `json:"arena_right"`
	CameraX           int16  `json:"camera_x"`
	CameraY           int16  `json:"camera_y"`
	Shake             int16  `json:"shake"`
}

// Draw requests the separate presentation stage. Deleted reports the complete
// original object-clear operation within its validated logical slot boundary.
type ChuProjectileOutcome struct {
	Draw, Deleted bool
	ClearedBytes  uint16
}

// StepChuProjectile recovers initialization, clock/motion, trail/impact
// allocation, contact/boundary lifetime, animation-shape submission and
// projection. It stops at the original 0x1580 draw request or completed 0x3556
// deletion; hardware sprite encoding and world collision scheduling are not
// executed by this rule function.
func StepChuProjectile(state ChuProjectileState, environment ChuProjectileEnvironment, library AnimationLibrary) (ChuProjectileState, ChuProjectileOutcome, error) {
	before := state
	if state.Stage == 0 {
		state.Stage += 2
		state.Animation.GraphicOffsetX, state.Animation.GraphicOffsetY, state.Animation.GraphicParameter = 0, 0, 0
		state.GraphicCache = [4]uint32{}
		state.GraphicSet, state.TileBase = 0xd8f1a, 0x240000
		state.Animation.AnimationSet, state.Animation.ShapeSet = 0xd92ca, 0xd9a3a
		state.Animation.Attributes |= environment.PaletteAttributes
		state.Animation.Clock.Duration, state.Animation.Clock.Ticks = 8, 255
	}
	state.Health = 255
	if environment.PauseFlags&0x80 != 0 || environment.SlowFlag != 0 {
		return state, ChuProjectileOutcome{Draw: true}, nil
	}
	state.Shake = 0
	state.Animation.Clock = StepAnimationClock(state.Animation.Clock)
	if state.Power > 5 && int16(state.Ground>>16)-64 <= int16(state.Animation.WorldY>>16) {
		state.TrailPhase++
		if state.TrailPhase&3 == 0 {
			if err := spawnChuProjectileTrail(&state); err != nil {
				return before, ChuProjectileOutcome{}, err
			}
		}
	}
	state.Animation.WorldX += state.VelocityX
	state.Animation.WorldY += state.VerticalVelocity
	impact := state.VerticalVelocity >= 0 && int16(state.Ground>>16) <= int16(state.Animation.WorldY>>16)
	if !impact {
		x := int16(state.Animation.WorldX >> 16)
		if x > environment.ArenaRight+32 || x < environment.ArenaLeft-32 {
			next, result, err := clearChuProjectile(state)
			if err != nil {
				return before, ChuProjectileOutcome{}, err
			}
			return next, result, nil
		}
		impact = state.ContactFlags&0x18 != 0
	}
	if impact {
		if state.Power > 7 {
			return before, ChuProjectileOutcome{}, errors.New("Chu projectile impact power is outside decoded table")
		}
		if err := spawnChuProjectileImpact(&state); err != nil {
			return before, ChuProjectileOutcome{}, err
		}
		next, result, err := clearChuProjectile(state)
		if err != nil {
			return before, ChuProjectileOutcome{}, err
		}
		return next, result, nil
	}
	animation, err := SubmitAnimationRecord(AnimationSubmissionState{Actor: state.Animation, Buffers: state.Buffers, PendingSound: state.PendingSound, SoundQueue: state.SoundQueue}, library)
	if err != nil {
		return before, ChuProjectileOutcome{}, err
	}
	state.Animation, state.Buffers, state.PendingSound, state.SoundQueue = animation.Actor, animation.Buffers, animation.PendingSound, animation.SoundQueue
	projection := ProjectFighter(ProjectionState{WorldX: state.Animation.WorldX, WorldY: state.Animation.WorldY,
		ScreenX: state.ScreenX, ScreenY: state.ScreenY, OffsetX: state.ScreenOffsetX, OffsetY: state.ScreenOffsetY,
		CameraX: environment.CameraX, CameraY: environment.CameraY, Shake: environment.Shake})
	state.ScreenX, state.ScreenY, state.ScreenOffsetX, state.ScreenOffsetY = projection.ScreenX, projection.ScreenY, projection.OffsetX, projection.OffsetY
	return state, ChuProjectileOutcome{Draw: true}, nil
}

func spawnChuProjectileTrail(state *ChuProjectileState) error {
	pool, index, err := AllocateEffect(state.Pool)
	if err != nil {
		return err
	}
	state.Pool = pool
	if index >= 0 {
		slot := &state.Pool.Slots[index]
		slot.Type = 0x20
		slot.X, slot.Y = state.Animation.WorldX, state.Ground
		slot.Depth, slot.Layer = state.Depth, state.Layer-4
		slot.Attributes = state.Animation.Attributes & 0xe000
		if state.Animation.Facing != 0 {
			slot.Attributes |= 0x0800
		}
	}
	return nil
}

func spawnChuProjectileImpact(state *ChuProjectileState) error {
	pool, index, err := AllocateEffect(state.Pool)
	if err != nil {
		return err
	}
	state.Pool = pool
	if index >= 0 {
		slot := &state.Pool.Slots[index]
		x, y := int16(state.Animation.WorldX>>16), int16(state.Animation.WorldY>>16)
		slot.Type = 0x18
		if state.Power >= 6 {
			slot.Type, y = 0x19, y-16
		} else if state.Power >= 3 {
			slot.Type = 0x1d
		}
		// The impact helpers write coordinate words, retaining old fractions.
		slot.X = int32(uint32(uint16(x))<<16 | uint32(uint16(slot.X)))
		slot.Y = int32(uint32(uint16(y))<<16 | uint32(uint16(slot.Y)))
		slot.Attributes = state.Animation.Attributes&0x9800 | 0x2000
		slot.Depth, slot.Layer = state.Depth, state.Layer-4
	}
	return nil
}

func clearChuProjectile(state ChuProjectileState) (ChuProjectileState, ChuProjectileOutcome, error) {
	count := (state.Size >> 4) << 4
	if state.Size == 0 {
		count = 64
	}
	if count == 0 || count > 256 {
		return state, ChuProjectileOutcome{}, errors.New("Chu projectile deletion exceeds logical slot boundary")
	}
	// 0x3556 clears sixteen-byte blocks. Keep semantic fields beyond the
	// declared prefix, including fractions or old metadata not in that prefix.
	state.Behavior, state.Size, state.Stage, state.ScreenX, state.ScreenY = 0, 0, 0, 0, 0
	if count >= 32 {
		state.VelocityX, state.VerticalVelocity, state.GraphicSet = 0, 0, 0
	}
	if count >= 48 {
		state.TileBase, state.Animation.Attributes, state.Layer = 0, 0, 0
		state.Animation.GraphicOffsetX, state.Animation.GraphicOffsetY, state.GraphicCache[0] = 0, 0, 0
	}
	if count >= 64 {
		state.GraphicCache[1], state.GraphicCache[2], state.GraphicCache[3], state.Animation.GraphicParameter, state.Depth = 0, 0, 0, 0, 0
	}
	if count >= 80 {
		state.Animation.WorldX, state.Animation.WorldY, state.TrailPhase = 0, 0, 0
	}
	if count >= 96 {
		state.Animation.AnimationSet, state.ScreenOffsetX, state.ScreenOffsetY = 0, 0, 0
		state.Animation.Clock.FrameIndex, state.Animation.Clock.Duration, state.Animation.Clock.Ticks, state.Animation.Clock.Flags, state.Animation.Pose = 0, 0, 0, 0, 0
	}
	if count >= 112 {
		state.Animation.Clock.Completed, state.Animation.Facing, state.Animation.Auxiliary64 = 0, 0, 0
		state.Animation.Value65, state.Animation.Value66, state.Animation.Value67, state.Animation.Value68, state.Shake = 0, 0, 0, 0, 0
		state.Animation.EnabledStages, state.Animation.ActionFlags = 0, 0
		state.Team = 0
	}
	if count >= 128 {
		state.Health, state.Power, state.TargetBit = 0, 0, 0
	}
	if count >= 144 {
		state.ContactFlags, state.Animation.ShapeSet = 0, 0
	}
	if count >= 160 {
		state.Animation.Auxiliary94, state.Ground = 0, 0
	}
	return state, ChuProjectileOutcome{Deleted: true, ClearedBytes: count}, nil
}
