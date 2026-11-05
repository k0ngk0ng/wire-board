package engine

import "errors"

// Fighter5ProjectileState is a narrow view of the three source5 children.
// Aux48 and Gravity retain their complete canonical longs. Owner is a view of
// the existing contextual +fe word, not a second persistent owner.
type Fighter5ProjectileState struct {
	Visual                                Fighter8ProjectileState
	VelocityX, VerticalVelocity, Ground   int32
	Aux48, Gravity                        int32
	Depth, Owner                          uint16
	Layer, Team, ActionKind, ContactFlags uint8
	HitStopTicks                          uint8
	Pool                                  EffectPool
}

// Held is the parent's +71 low byte; RawHeld at +74 is not an input here.
type Fighter5ProjectileParent struct {
	ObjectID                  uint16
	X, Y                      int32
	Pose                      uint8
	Held                      Buttons
	HitStopTicks, RetainTicks uint8
}

// StepFighter5Projectile recovers1414e/143e4/146ba through their actual draw,
// direct reflection return, or completed clear boundary. Parent and child
// updates commit together; collision scheduling is owned by the world.
func StepFighter5Projectile(state Fighter5ProjectileState, parent Fighter5ProjectileParent, environment ChuProjectileEnvironment, library AnimationLibrary) (Fighter5ProjectileState, Fighter5ProjectileParent, ChuProjectileOutcome, error) {
	before, oldParent := state, parent
	behavior := state.Visual.Header.Behavior
	if behavior == 0x146ba {
		return stepFighter5Follower(state, parent, environment)
	}
	if behavior != 0x1414e && behavior != 0x143e4 {
		return before, oldParent, ChuProjectileOutcome{}, errors.New("source5 child callback is unavailable")
	}
	v := &state.Visual
	if v.Header.Stage == 0 {
		initializeFighter5ChildVisual(v, environment.PaletteAttributes, false)
	}
	v.Health = 255
	if environment.PauseFlags&0x80 != 0 || environment.SlowFlag != 0 {
		return state, parent, ChuProjectileOutcome{Draw: true}, nil
	}
	fail := func(err error) (Fighter5ProjectileState, Fighter5ProjectileParent, ChuProjectileOutcome, error) {
		return before, oldParent, ChuProjectileOutcome{}, err
	}
	v.Shake = 0
	v.Animation.Clock = StepAnimationClock(v.Animation.Clock)
	distance := int16(32)
	if behavior == 0x143e4 {
		distance = 64
	}
	if int16(state.Ground>>16)-distance <= int16(v.Animation.WorldY>>16) {
		state.Aux48 += 0x1000000
		if uint8(uint32(state.Aux48)>>24)&3 == 0 {
			if err := spawnFighter5ProjectileEffect(&state, true, false); err != nil {
				return fail(err)
			}
		}
	}
	if behavior == 0x143e4 && v.Animation.Pose == 0x19 {
		if parent.ObjectID != state.Owner {
			return fail(errors.New("source5 steered child parent identity mismatch"))
		}
		if parent.Held&ButtonUp != 0 {
			if state.VerticalVelocity >= -0x60000 {
				state.VerticalVelocity -= 0x4000
			}
		} else if parent.Held&ButtonDown != 0 && state.VerticalVelocity <= 0x60000 {
			state.VerticalVelocity += 0x4000
		}
	}
	v.Animation.WorldX += state.VelocityX
	v.Animation.WorldY += state.VerticalVelocity
	if behavior == 0x1414e && v.Animation.Pose == 0x17 || behavior == 0x143e4 && v.Animation.Pose == 0x18 {
		v.Animation.Auxiliary64 |= 0x20
	}
	x, y := int16(v.Animation.WorldX>>16), int16(v.Animation.WorldY>>16)
	impact := int16(state.Ground>>16) <= y
	if !impact && (y < 0 || x > environment.ArenaRight+32 || x < environment.ArenaLeft-32) {
		next, out, err := clearFighter5Projectile(state)
		if err != nil {
			return fail(err)
		}
		return next, parent, out, nil
	}
	if !impact && state.ActionKind == 0 && state.ContactFlags&4 != 0 {
		if parent.ObjectID != state.Owner {
			return fail(errors.New("source5 reflected child parent identity mismatch"))
		}
		parent.HitStopTicks, parent.RetainTicks = 0, 0
		variant := uint8(uint32(state.Gravity) >> 24)
		if variant < 4 {
			state.ContactFlags, state.ActionKind, state.Team = 0, 1, state.Team-16
			state.VerticalVelocity = 0
			state.VelocityX = 0x80000
			v.Animation.Pose = 0x14
			if behavior == 0x143e4 {
				state.VelocityX, v.Animation.Pose = 0x60000, 0x19
			} else if variant == 1 || variant == 2 {
				v.Animation.Pose = 0x15
			} else if variant == 3 {
				v.Animation.Pose = 0x16
			}
			if variant >= 2 {
				state.VelocityX, state.VerticalVelocity = 0x5a000, -0x5a000
				if variant == 3 {
					state.VerticalVelocity = 0x5a000
				}
			}
			if v.Animation.Facing&1 != 0 {
				state.VelocityX = -state.VelocityX
			}
			return state, parent, ChuProjectileOutcome{}, nil
		}
	}
	impact = impact || state.ContactFlags&0x18 != 0
	if impact {
		if err := spawnFighter5ProjectileEffect(&state, false, behavior == 0x143e4); err != nil {
			return fail(err)
		}
		next, out, err := clearFighter5Projectile(state)
		if err != nil {
			return fail(err)
		}
		return next, parent, out, nil
	}
	if err := submitFighter5Projectile(v, environment, library); err != nil {
		return fail(err)
	}
	return state, parent, ChuProjectileOutcome{Draw: true}, nil
}

func initializeFighter5ChildVisual(v *Fighter8ProjectileState, attributes uint16, follower bool) {
	v.Header.Stage += 2
	v.Animation.GraphicOffsetX, v.Animation.GraphicOffsetY, v.Animation.GraphicParameter = 0, 0, 0
	v.GraphicCache = [4]uint32{}
	v.GraphicsSet, v.TilesBase = 0xd8f1a, 0x240000
	v.Animation.Attributes |= attributes
	if follower {
		v.Animation.AnimationSet = 0x1479c
		v.Animation.Clock.FrameIndex, v.Animation.Clock.Ticks, v.Animation.Clock.Flags = 255, 1, 0
	} else {
		v.Animation.AnimationSet, v.Animation.ShapeSet = 0xd92ca, 0xd9a3a
		v.Animation.Clock.Duration, v.Animation.Clock.Ticks = 8, 255
	}
}

func stepFighter5Follower(state Fighter5ProjectileState, parent Fighter5ProjectileParent, environment ChuProjectileEnvironment) (Fighter5ProjectileState, Fighter5ProjectileParent, ChuProjectileOutcome, error) {
	before := state
	v := &state.Visual
	if v.Header.Stage == 0 {
		initializeFighter5ChildVisual(v, environment.PaletteAttributes, true)
	}
	if environment.PauseFlags&0x80 != 0 || environment.SlowFlag != 0 {
		return state, parent, ChuProjectileOutcome{Draw: true}, nil
	}
	fail := func(err error) (Fighter5ProjectileState, Fighter5ProjectileParent, ChuProjectileOutcome, error) {
		return before, parent, ChuProjectileOutcome{}, err
	}
	v.Animation.Clock.Ticks--
	if v.Animation.Clock.Ticks == 0 {
		v.Animation.Clock.FrameIndex++
		// Immutable type/delay records at1479c, independently read by the
		// original CPU before this rule was implemented. No ROM is executed.
		frames := [...]struct{ Graphic, Delay uint8 }{{1, 4}, {2, 4}, {3, 4}, {4, 4}, {5, 4}, {6, 4}, {7, 4}, {8, 4}, {0, 0}}
		index := int(v.Animation.Clock.FrameIndex)
		if v.Animation.AnimationSet != 0x1479c || index >= len(frames) {
			return fail(errors.New("source5 follower timing record is unavailable"))
		}
		v.Animation.GraphicParameter = uint16(frames[index].Graphic) * 2
		v.Animation.Clock.Ticks = frames[index].Delay
		if v.Animation.Clock.Ticks == 0 {
			next, out, err := clearFighter5Projectile(state)
			if err != nil {
				return fail(err)
			}
			return next, parent, out, nil
		}
	}
	if v.Animation.ObjectID < 0x200 || parent.ObjectID != v.Animation.ObjectID-0x200 {
		return fail(errors.New("source5 follower parent identity mismatch"))
	}
	v.Animation.WorldX, v.Animation.WorldY = parent.X, parent.Y
	v.ScreenOffsetX, v.ScreenOffsetY = int16(state.VelocityX>>16), int16(state.VerticalVelocity>>16)
	if parent.Pose != 0xa3 {
		next, out, err := clearFighter5Projectile(state)
		if err != nil {
			return fail(err)
		}
		return next, parent, out, nil
	}
	projectFighter5Projectile(v, environment)
	return state, parent, ChuProjectileOutcome{Draw: true}, nil
}

// StepFighter5ProjectileHitStop is the independently recovered142b4/1458a
// slave continuation. It does not initialize, advance AN timing, or move.
func StepFighter5ProjectileHitStop(state Fighter5ProjectileState, environment ChuProjectileEnvironment, library AnimationLibrary) (Fighter5ProjectileState, ChuProjectileOutcome, error) {
	before := state
	if state.Visual.Header.Behavior != 0x1414e && state.Visual.Header.Behavior != 0x143e4 {
		return before, ChuProjectileOutcome{}, errors.New("source5 child hit-stop continuation is unavailable")
	}
	state.HitStopTicks--
	if state.Visual.Shake > 1 {
		state.Visual.Shake--
	}
	offset := int16(state.Visual.Shake)
	if state.HitStopTicks&1 != 0 {
		offset = -offset
	}
	state.Visual.ScreenOffsetX += offset
	if err := submitFighter5Projectile(&state.Visual, environment, library); err != nil {
		return before, ChuProjectileOutcome{}, err
	}
	return state, ChuProjectileOutcome{Draw: true}, nil
}

func spawnFighter5ProjectileEffect(state *Fighter5ProjectileState, trail, large bool) error {
	pool, index, err := AllocateEffect(state.Pool)
	if err != nil {
		return err
	}
	state.Pool = pool
	if index < 0 {
		return nil
	}
	v := state.Visual.Animation
	slot := &state.Pool.Slots[index]
	slot.Depth, slot.Layer = state.Depth, state.Layer-4
	if trail {
		slot.Type, slot.X, slot.Y = 0x20, v.WorldX, state.Ground
		slot.Attributes = v.Attributes & 0xe000
		if v.Facing != 0 {
			slot.Attributes |= 0x0800
		}
	} else {
		x, y := uint16(v.WorldX>>16), uint16(v.WorldY>>16)
		slot.Type = 0x18
		if large {
			slot.Type, y = 0x19, y-16
		}
		slot.X, slot.Y = replaceChuSpecialHighWord(slot.X, x), replaceChuSpecialHighWord(slot.Y, y)
		slot.Attributes = v.Attributes&0x9800 | 0x2000
	}
	return nil
}

func submitFighter5Projectile(v *Fighter8ProjectileState, environment ChuProjectileEnvironment, library AnimationLibrary) error {
	next, err := SubmitAnimationRecord(AnimationSubmissionState{Actor: v.Animation, Buffers: v.Buffers, PendingSound: v.PendingSound, SoundQueue: v.SoundQueue}, library)
	if err != nil {
		return err
	}
	v.Animation, v.Buffers, v.PendingSound, v.SoundQueue = next.Actor, next.Buffers, next.PendingSound, next.SoundQueue
	projectFighter5Projectile(v, environment)
	return nil
}

func projectFighter5Projectile(v *Fighter8ProjectileState, environment ChuProjectileEnvironment) {
	p := ProjectFighter(ProjectionState{WorldX: v.Animation.WorldX, WorldY: v.Animation.WorldY, ScreenX: v.ScreenX, ScreenY: v.ScreenY,
		OffsetX: v.ScreenOffsetX, OffsetY: v.ScreenOffsetY, CameraX: environment.CameraX, CameraY: environment.CameraY, Shake: environment.Shake})
	v.ScreenX, v.ScreenY, v.ScreenOffsetX, v.ScreenOffsetY = p.ScreenX, p.ScreenY, p.OffsetX, p.OffsetY
}

func clearFighter5Projectile(state Fighter5ProjectileState) (Fighter5ProjectileState, ChuProjectileOutcome, error) {
	size := state.Visual.Header.Size
	if size == 0 {
		size = 64
	}
	count := size / 16 * 16
	if count == 0 || count > 256 {
		return state, ChuProjectileOutcome{}, errors.New("source5 child clear exceeds its logical slot")
	}
	v, out, err := clearFighter8Projectile(state.Visual)
	if err != nil {
		return state, ChuProjectileOutcome{}, err
	}
	state.Visual = v
	if count >= 32 {
		state.VelocityX, state.VerticalVelocity = 0, 0
	}
	if count >= 48 {
		state.Layer = 0
	}
	if count >= 64 {
		state.Depth = 0
	}
	if count >= 80 {
		state.Aux48, state.Gravity = 0, 0
	}
	if count >= 96 {
		state.ActionKind = 0
	}
	if count >= 112 {
		state.HitStopTicks, state.Team = 0, 0
	}
	if count >= 144 {
		state.ContactFlags = 0
	}
	if count >= 160 {
		state.Ground = 0
	}
	if count >= 256 {
		state.Owner = 0
	}
	return state, out, nil
}
