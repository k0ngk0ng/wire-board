package engine

import "errors"

// Visual reuses only the audited common fields, not source8's callback rules.
type Fighter1LinearProjectileState struct {
	Visual                       Fighter8ProjectileState
	VelocityX, VelocityY, Ground int32
	ContactFlags                 uint8
}

// Phase/Remaining are high-word views of existing Aux48/Body.Gravity. The
// caller retains their low words, including after canonical prefix deletion.
type Fighter1WaveProjectileState struct {
	Visual             Fighter8ProjectileState
	OriginX, VelocityY int32
	Phase, Remaining   uint16
	ContactFlags       uint8
}

func initializeFighter1Visual(v *Fighter8ProjectileState, pose uint8, attributes uint16) {
	v.Header.Stage += 2
	v.Animation.GraphicOffsetX, v.Animation.GraphicOffsetY, v.Animation.GraphicParameter = 0, 0, 0
	v.GraphicCache = [4]uint32{}
	v.GraphicsSet, v.TilesBase = 0xd8f1a, 0x240000
	v.Animation.AnimationSet, v.Animation.ShapeSet = 0xd92ca, 0xd9a3a
	v.Animation.Pose = pose
	v.Animation.Attributes |= attributes
	v.Animation.Clock.Duration, v.Animation.Clock.Ticks = 8, 255
}

func submitFighter1Visual(v Fighter8ProjectileState, world ChuAttachmentWorld, library AnimationLibrary, soundPort *worldSoundPort) (Fighter8ProjectileState, error) {
	s, err := submitAnimationRecordWithSound(AnimationSubmissionState{Actor: v.Animation, Buffers: v.Buffers, PendingSound: v.PendingSound, SoundQueue: v.SoundQueue}, library, soundPort)
	if err != nil {
		return v, err
	}
	v.Animation, v.Buffers, v.PendingSound, v.SoundQueue = s.Actor, s.Buffers, s.PendingSound, s.SoundQueue
	p := ProjectFighter(ProjectionState{WorldX: v.Animation.WorldX, WorldY: v.Animation.WorldY, ScreenX: v.ScreenX, ScreenY: v.ScreenY, OffsetX: v.ScreenOffsetX, OffsetY: v.ScreenOffsetY, CameraX: world.CameraX, CameraY: world.CameraY, Shake: world.Shake})
	v.ScreenX, v.ScreenY, v.ScreenOffsetX, v.ScreenOffsetY = p.ScreenX, p.ScreenY, p.OffsetX, p.OffsetY
	return v, nil
}

func clearFighter1Visual(v Fighter8ProjectileState) (Fighter8ProjectileState, ChuProjectileOutcome, error) {
	size := v.Header.Size
	if size == 0 {
		size = 64
	}
	count := size / 16 * 16
	if count == 0 || count > 256 {
		return v, ChuProjectileOutcome{}, errors.New("source1 projectile clear exceeds logical slot")
	}
	// The shared narrow visual fields have the exact same byte owners. This
	// helper clears a resolved prefix and executes no source8 callback logic.
	return clearFighter8Projectile(v)
}

// StepFighter1LinearProjectile recovers12ee8 through12fc8/1580 or13006/3556.
// Animation completion is not a deletion condition for this moving object.
func StepFighter1LinearProjectile(state Fighter1LinearProjectileState, world ChuProjectileEnvironment, library AnimationLibrary) (Fighter1LinearProjectileState, ChuProjectileOutcome, error) {
	return stepFighter1LinearProjectileWithSound(state, world, library, nil)
}

func stepFighter1LinearProjectileWithSound(state Fighter1LinearProjectileState, world ChuProjectileEnvironment, library AnimationLibrary, soundPort *worldSoundPort) (Fighter1LinearProjectileState, ChuProjectileOutcome, error) {
	before := state
	v := &state.Visual
	if v.Header.Stage == 0 {
		initializeFighter1Visual(v, 10, world.PaletteAttributes)
	}
	v.Health = 255
	if world.PauseFlags&128 != 0 || world.SlowFlag != 0 {
		return state, ChuProjectileOutcome{Draw: true}, nil
	}
	v.Shake = 0
	v.Animation.Clock = StepAnimationClock(v.Animation.Clock)
	v.Animation.WorldX += state.VelocityX
	v.Animation.WorldY += state.VelocityY
	remove := state.VelocityY >= 0 && int16(state.Ground>>16) <= int16(v.Animation.WorldY>>16)
	if !remove {
		x := int16(v.Animation.WorldX >> 16)
		remove = x > world.ArenaRight+32 || x < world.ArenaLeft-32 || state.ContactFlags&0x18 != 0
	}
	if remove {
		next, out, err := clearFighter1Visual(*v)
		if err != nil {
			return before, ChuProjectileOutcome{}, err
		}
		state.Visual = next
		if out.ClearedBytes >= 32 {
			state.VelocityX, state.VelocityY = 0, 0
		}
		if out.ClearedBytes >= 144 {
			state.ContactFlags = 0
		}
		if out.ClearedBytes >= 160 {
			state.Ground = 0
		}
		return state, out, nil
	}
	var err error
	state.Visual, err = submitFighter1Visual(*v, ChuAttachmentWorld{PauseFlags: world.PauseFlags, SlowFlag: world.SlowFlag, AttributeMask: world.PaletteAttributes, CameraX: world.CameraX, CameraY: world.CameraY, Shake: world.Shake}, library, soundPort)
	if err != nil {
		return before, ChuProjectileOutcome{}, err
	}
	return state, ChuProjectileOutcome{Draw: true}, nil
}

// StepFighter1WaveProjectile recovers1300c using the existing signed1024-byte
// wave. OriginX is a position, not a velocity; only its high word is displaced.
func StepFighter1WaveProjectile(state Fighter1WaveProjectileState, world ChuAttachmentWorld, library AnimationLibrary, wave []int8) (Fighter1WaveProjectileState, ChuProjectileOutcome, error) {
	return stepFighter1WaveProjectileWithSound(state, world, library, wave, nil)
}

func stepFighter1WaveProjectileWithSound(state Fighter1WaveProjectileState, world ChuAttachmentWorld, library AnimationLibrary, wave []int8, soundPort *worldSoundPort) (Fighter1WaveProjectileState, ChuProjectileOutcome, error) {
	before := state
	v := &state.Visual
	if v.Header.Stage == 0 {
		initializeFighter1Visual(v, 11, world.AttributeMask)
		state.Phase, state.Remaining = 0x2c0, 24
	}
	v.Health = 255
	if world.PauseFlags&128 != 0 || world.SlowFlag != 0 {
		return state, ChuProjectileOutcome{Draw: true}, nil
	}
	v.Shake = 0
	v.Animation.Clock = StepAnimationClock(v.Animation.Clock)
	state.Phase = (state.Phase + 0x40) & 0x3ff
	if len(wave) != 1024 {
		return before, ChuProjectileOutcome{}, errors.New("source1 signed wave table is unavailable")
	}
	dx := int16(wave[state.Phase]) >> 1
	if v.Animation.Facing != 0 {
		dx = -dx
	}
	v.Animation.WorldX = addThrowCoordinateWord(state.OriginX, dx)
	v.Animation.WorldY += state.VelocityY
	state.Remaining--
	if state.Remaining == 0 || state.ContactFlags&0x18 != 0 {
		next, out, err := clearFighter1Visual(*v)
		if err != nil {
			return before, ChuProjectileOutcome{}, err
		}
		state.Visual = next
		if out.ClearedBytes >= 32 {
			state.OriginX, state.VelocityY = 0, 0
		}
		if out.ClearedBytes >= 80 {
			state.Phase, state.Remaining = 0, 0
		}
		if out.ClearedBytes >= 144 {
			state.ContactFlags = 0
		}
		return state, out, nil
	}
	var err error
	state.Visual, err = submitFighter1Visual(*v, world, library, soundPort)
	if err != nil {
		return before, ChuProjectileOutcome{}, err
	}
	return state, ChuProjectileOutcome{Draw: true}, nil
}
