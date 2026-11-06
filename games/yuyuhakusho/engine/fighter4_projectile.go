package engine

import "errors"

// Fighter4DragonSample contains the six used history planes of one slot.
// Status owns only the original plane's high byte.
type Fighter4DragonSample struct {
	X, Y                        int16
	Angle, Depth                uint16
	Layer, SkipGraphics, Status uint8
}

type Fighter4DragonHistory [4][32]Fighter4DragonSample

// Fighter4DragonCurve is immutable signed-word data at the original34000
// base, with a validated neighboring prefix/suffix rather than angular wrap.
type Fighter4DragonCurve struct {
	FirstIndex int16   `json:"first_index"`
	Values     []int16 `json:"values"`
}

type Fighter4DragonLibrary struct {
	Animations AnimationLibrary
	Curve      Fighter4DragonCurve
}

// These narrow views borrow existing canonical object fields. Only DragonBank
// and Fighter4DragonHistory introduce new state owners.
type Fighter4DragonTrailExtra struct{ Owner, Bank uint16 }

type Fighter4DragonHeadState struct {
	Visual                                  Fighter8ProjectileState
	Direction, EffectiveAngle               int32
	WavePhase, Age, SoundClock, DragonBank  uint16
	Depth                                   uint16
	Ground                                  int16
	Layer, SkipGraphics, Held, ContactFlags uint8
	Pool                                    EffectPool
	EffectExtras                            [16]Fighter4DragonTrailExtra
}

type Fighter4DragonTrailState struct {
	Header                       MatchObjectHeader
	Draw                         WorldDraw
	WorldX, WorldY               int32
	ScreenOffsetX, ScreenOffsetY int16
	Angle, Age, Bank             uint16
}

type Fighter4BoostState struct {
	Visual Fighter8ProjectileState
	Depth  uint16
	Layer  uint8
}

type Fighter4BoostParent struct {
	Body           FighterBodyState
	Energy         int32
	AttackValue    uint8
	MovementSource uint32
	StatusDelay    uint8
	StunTicks      uint16
}

// StepFighter4DragonHead recovers13bb6 through this callback's single1580
// request or its completed3556 prefix clear. Trail callbacks are separate.
func StepFighter4DragonHead(state Fighter4DragonHeadState, history Fighter4DragonHistory, environment ChuProjectileEnvironment, library Fighter4DragonLibrary) (Fighter4DragonHeadState, Fighter4DragonHistory, ChuProjectileOutcome, error) {
	return stepFighter4DragonHeadWithSound(state, history, environment, library, nil)
}

func stepFighter4DragonHeadWithSound(state Fighter4DragonHeadState, history Fighter4DragonHistory, environment ChuProjectileEnvironment, library Fighter4DragonLibrary, soundPort *worldSoundPort) (Fighter4DragonHeadState, Fighter4DragonHistory, ChuProjectileOutcome, error) {
	before, oldHistory := state, history
	v := &state.Visual
	if v.Header.Stage == 0 {
		initializeFighter4DragonVisual(v, environment.PaletteAttributes)
	}
	v.Health = 255
	if environment.PauseFlags&0x80 != 0 || environment.SlowFlag != 0 {
		return state, history, ChuProjectileOutcome{Draw: true}, nil
	}
	fail := func(err error) (Fighter4DragonHeadState, Fighter4DragonHistory, ChuProjectileOutcome, error) {
		return before, oldHistory, ChuProjectileOutcome{}, err
	}
	bank, err := fighter4DragonBank(state.DragonBank)
	if err != nil {
		return fail(err)
	}
	v.Shake = 0
	v.Animation.Clock = StepAnimationClock(v.Animation.Clock)
	if state.SoundClock == 0 {
		v.PendingSound = 0xf0
	}
	state.SoundClock = (state.SoundClock + 1) & 255
	state.WavePhase = (state.WavePhase + 32) & 1023
	wave, err := library.Curve.word(int16(state.WavePhase))
	if err != nil {
		return fail(err)
	}
	state.EffectiveAngle = int32(wave) << 7
	steering := [...]int16{0, 512, 0, 0, -256, -384, -128, 0, 256, 384, 128, 0, 0, 0, 0, 0}
	direction := uint16(uint32(state.Direction) >> 16)
	if held := state.Held & 15; held != 0 {
		candidate := (direction + uint16(steering[held])) & 1023
		if int16(candidate) > 512 {
			direction -= 16
		} else {
			direction += 16
		}
	}
	direction &= 1023
	state.Direction = replaceChuSpecialHighWord(state.Direction, direction)
	state.EffectiveAngle += state.Direction
	angle := uint16(uint32(state.EffectiveAngle) >> 16)
	if int16(angle) > 512 {
		v.Animation.Facing |= 1
	} else {
		v.Animation.Facing &^= 1
	}
	poses := [...]uint8{14, 15, 16, 17, 18, 17, 16, 15}
	v.Animation.Pose = poses[((angle+64)&1023)>>7]
	x, err := library.Curve.word(int16(angle))
	if err != nil {
		return fail(err)
	}
	y, err := library.Curve.word(int16(angle) + 256)
	if err != nil {
		return fail(err)
	}
	v.Animation.WorldX += int32(x) << 4
	v.Animation.WorldY -= int32(y) << 4
	state.Age++
	if int16(state.Age) >= 2048 {
		state.Age = 1024
	}
	index := state.Age & 31
	history[bank][index] = Fighter4DragonSample{X: int16(v.Animation.WorldX >> 16), Y: int16(v.Animation.WorldY >> 16), Angle: angle, Depth: state.Depth, Layer: state.Layer, SkipGraphics: state.SkipGraphics}
	if int16(state.Age) <= 31 && state.Age&3 == 0 {
		pool, selected, allocErr := AllocateEffect(state.Pool)
		if allocErr != nil {
			return fail(allocErr)
		}
		state.Pool = pool
		if selected >= 0 {
			slot := &state.Pool.Slots[selected]
			slot.Behavior = 0x13e9a
			slot.Attributes = (v.Animation.Attributes+0x18)&0x9fff | 0x4000
			slot.Depth, slot.Layer = state.Depth, state.Layer
			state.EffectExtras[selected] = Fighter4DragonTrailExtra{Owner: v.Animation.ObjectID, Bank: state.DragonBank}
		}
	}
	xw, yw := int16(v.Animation.WorldX>>16), int16(v.Animation.WorldY>>16)
	impact := state.Ground <= yw
	outside := !impact && (yw < 0 || xw > environment.ArenaRight+64 || xw < environment.ArenaLeft-64)
	if !impact && !outside {
		impact = state.ContactFlags&0x18 != 0
	}
	if impact || outside {
		if impact {
			history[bank][index].Status = 2
			pool, selected, allocErr := AllocateEffect(state.Pool)
			if allocErr != nil {
				return fail(allocErr)
			}
			state.Pool = pool
			if selected >= 0 {
				slot := &state.Pool.Slots[selected]
				slot.Type = 0x1d
				slot.X, slot.Y = replaceChuSpecialHighWord(slot.X, uint16(xw)), replaceChuSpecialHighWord(slot.Y, uint16(yw))
				slot.Attributes = v.Animation.Attributes&0x9800 | 0x2000
				slot.Depth, slot.Layer = state.Depth, state.Layer-4
			}
		} else {
			history[bank][index].Status = 1
		}
		next, out, clearErr := clearFighter4DragonHead(state)
		if clearErr != nil {
			return fail(clearErr)
		}
		return next, history, out, nil
	}
	updated, err := submitFighter4DragonVisual(*v, ChuAttachmentWorld{CameraX: environment.CameraX, CameraY: environment.CameraY, Shake: environment.Shake}, library.Animations, soundPort)
	if err != nil {
		return fail(err)
	}
	state.Visual = updated
	return state, history, ChuProjectileOutcome{Draw: true}, nil
}

func fighter4DragonBank(bank uint16) (int, error) {
	if bank & ^uint16(0x600) != 0 {
		return 0, errors.New("source4 dragon bank is not one of four aligned banks")
	}
	return int(bank >> 9), nil
}

func (curve Fighter4DragonCurve) word(index int16) (int16, error) {
	i := int(index) - int(curve.FirstIndex)
	if i < 0 || i >= len(curve.Values) {
		return 0, errors.New("source4 dragon signed-word curve index is unavailable")
	}
	return curve.Values[i], nil
}

func initializeFighter4DragonVisual(v *Fighter8ProjectileState, attributes uint16) {
	v.Header.Stage += 2
	v.Animation.GraphicOffsetX, v.Animation.GraphicOffsetY, v.Animation.GraphicParameter = 0, 0, 0
	v.GraphicCache = [4]uint32{}
	v.GraphicsSet, v.TilesBase = 0xd8f1a, 0x240000
	v.Animation.AnimationSet, v.Animation.ShapeSet = 0xd92ca, 0xd9a3a
	v.Animation.Attributes |= attributes
	v.Animation.Clock.Duration, v.Animation.Clock.Ticks = 8, 255
}

func submitFighter4DragonVisual(v Fighter8ProjectileState, world ChuAttachmentWorld, library AnimationLibrary, soundPort *worldSoundPort) (Fighter8ProjectileState, error) {
	s, err := submitAnimationRecordWithSound(AnimationSubmissionState{Actor: v.Animation, Buffers: v.Buffers, PendingSound: v.PendingSound, SoundQueue: v.SoundQueue}, library, soundPort)
	if err != nil {
		return v, err
	}
	v.Animation, v.Buffers, v.PendingSound, v.SoundQueue = s.Actor, s.Buffers, s.PendingSound, s.SoundQueue
	p := ProjectFighter(ProjectionState{WorldX: v.Animation.WorldX, WorldY: v.Animation.WorldY, ScreenX: v.ScreenX, ScreenY: v.ScreenY, OffsetX: v.ScreenOffsetX, OffsetY: v.ScreenOffsetY, CameraX: world.CameraX, CameraY: world.CameraY, Shake: world.Shake})
	v.ScreenX, v.ScreenY, v.ScreenOffsetX, v.ScreenOffsetY = p.ScreenX, p.ScreenY, p.OffsetX, p.OffsetY
	return v, nil
}

func clearFighter4DragonHead(state Fighter4DragonHeadState) (Fighter4DragonHeadState, ChuProjectileOutcome, error) {
	v, out, err := clearFighter8Projectile(state.Visual)
	if err != nil {
		return state, ChuProjectileOutcome{}, err
	}
	state.Visual = v
	count := out.ClearedBytes
	if count >= 32 {
		state.EffectiveAngle, state.Age = 0, 0
	}
	if count >= 48 {
		state.Layer, state.SkipGraphics = 0, 0
	}
	if count >= 64 {
		state.Depth = 0
	}
	if count >= 80 {
		state.Direction, state.WavePhase = 0, 0
	}
	if count >= 128 {
		state.Held = 0
	}
	if count >= 144 {
		state.ContactFlags = 0
	}
	if count >= 160 {
		state.Ground = 0
	}
	if count >= 192 {
		state.SoundClock = 0
	}
	if count >= 256 {
		state.DragonBank = 0
	}
	return state, out, nil
}

// StepFighter4DragonTrail recovers13e9a; it neither follows an owner nor
// advances AN timing. History is read-only, including retained old samples.
func StepFighter4DragonTrail(state Fighter4DragonTrailState, history Fighter4DragonHistory, environment ChuAttachmentWorld) (Fighter4DragonTrailState, ChuProjectileOutcome, error) {
	before := state
	if state.Header.Stage == 0 {
		state.Header.Stage += 2
		state.Draw.OffsetX, state.Draw.OffsetY, state.Draw.Parameter = 0, 0, 0
		state.Draw.Cache = [4]uint32{}
		state.Draw.GraphicsSet, state.Draw.TileBank = 0xd8f1a, 0x240000
		state.Draw.Attributes |= environment.AttributeMask
	}
	if environment.PauseFlags&0x80 != 0 || environment.SlowFlag != 0 {
		return state, ChuProjectileOutcome{Draw: true}, nil
	}
	bank, err := fighter4DragonBank(state.Bank)
	if err != nil {
		return before, ChuProjectileOutcome{}, err
	}
	state.Age = (state.Age + 1) & 31
	sample := history[bank][state.Age]
	state.WorldX, state.WorldY = replaceChuSpecialHighWord(state.WorldX, uint16(sample.X)), replaceChuSpecialHighWord(state.WorldY, uint16(sample.Y))
	state.Angle = sample.Angle
	state.Draw.Depth, state.Draw.Layer = sample.Depth, sample.Layer
	if int16(state.Angle) > 512 {
		state.Draw.Attributes |= 0x800
	} else {
		state.Draw.Attributes &^= 0x800
	}
	if (state.Angle+256)&1023 > 512 {
		state.Draw.Attributes |= 0x1000
	} else {
		state.Draw.Attributes &^= 0x1000
	}
	parameters := [...]uint16{90, 88, 86, 88, 90, 88, 86, 88}
	state.Draw.Parameter = parameters[((state.Angle+64)&1023)>>7]
	if sample.Status == 1 || sample.Status == 2 {
		next, out, err := clearFighter4DragonTrail(state)
		if err != nil {
			return before, ChuProjectileOutcome{}, err
		}
		return next, out, nil
	}
	p := ProjectFighter(ProjectionState{WorldX: state.WorldX, WorldY: state.WorldY, ScreenX: int16(state.Draw.ScreenX), ScreenY: int16(state.Draw.ScreenY), OffsetX: state.ScreenOffsetX, OffsetY: state.ScreenOffsetY, CameraX: environment.CameraX, CameraY: environment.CameraY, Shake: environment.Shake})
	state.Draw.ScreenX, state.Draw.ScreenY, state.ScreenOffsetX, state.ScreenOffsetY = uint16(p.ScreenX), uint16(p.ScreenY), p.OffsetX, p.OffsetY
	return state, ChuProjectileOutcome{Draw: true}, nil
}

func clearFighter4DragonTrail(state Fighter4DragonTrailState) (Fighter4DragonTrailState, ChuProjectileOutcome, error) {
	size := state.Header.Size
	if size == 0 {
		size = 64
	}
	count := size / 16 * 16
	if count == 0 || count > 128 {
		return state, ChuProjectileOutcome{}, errors.New("source4 dragon trail clear exceeds its logical slot")
	}
	state.Header = MatchObjectHeader{}
	state.Draw.ScreenX, state.Draw.ScreenY = 0, 0
	if count >= 32 {
		state.Angle, state.Age, state.Draw.GraphicsSet = 0, 0, 0
	}
	if count >= 48 {
		state.Draw.TileBank, state.Draw.Attributes, state.Draw.Layer, state.Draw.SkipTiles = 0, 0, 0, 0
		state.Draw.OffsetX, state.Draw.OffsetY, state.Draw.Cache[0] = 0, 0, 0
	}
	if count >= 64 {
		state.Draw.Cache[1], state.Draw.Cache[2], state.Draw.Cache[3], state.Draw.Parameter, state.Draw.Depth = 0, 0, 0, 0, 0
	}
	if count >= 80 {
		state.WorldX, state.WorldY, state.Bank = 0, 0, 0
	}
	if count >= 96 {
		state.ScreenOffsetX, state.ScreenOffsetY = 0, 0
	}
	return state, ChuProjectileOutcome{Deleted: true, ClearedBytes: count}, nil
}

// StepFighter4Boost recovers13fc8 and the complete negative-energy parent
// constructor. Child and parent are committed together only on success.
func StepFighter4Boost(state Fighter4BoostState, parent Fighter4BoostParent, environment ChuAttachmentWorld, library AnimationLibrary) (Fighter4BoostState, Fighter4BoostParent, ChuProjectileOutcome, error) {
	return stepFighter4BoostWithSound(state, parent, environment, library, nil)
}

func stepFighter4BoostWithSound(state Fighter4BoostState, parent Fighter4BoostParent, environment ChuAttachmentWorld, library AnimationLibrary, soundPort *worldSoundPort) (Fighter4BoostState, Fighter4BoostParent, ChuProjectileOutcome, error) {
	before, oldParent := state, parent
	v := &state.Visual
	if v.Header.Stage == 0 {
		initializeFighter4DragonVisual(v, environment.AttributeMask)
		v.Animation.Pose = 19
	}
	v.Health = 255
	if environment.PauseFlags&0x80 != 0 || environment.SlowFlag != 0 {
		return state, parent, ChuProjectileOutcome{Draw: true}, nil
	}
	v.Shake = 0
	v.Animation.Clock = StepAnimationClock(v.Animation.Clock)
	v.Animation.WorldX = replaceChuSpecialHighWord(v.Animation.WorldX, uint16(parent.Body.X>>16))
	v.Animation.WorldY = replaceChuSpecialHighWord(v.Animation.WorldY, uint16(parent.Body.Y>>16))
	v.Animation.Facing = parent.Body.Facing
	state.Depth, state.Layer = parent.Body.Depth, parent.Body.Layer+4
	parent.Body.Auxiliary64 |= 0x10
	parent.AttackValue <<= 1
	if parent.Body.Health >= 0 && parent.Body.Pose != 15 && parent.Body.Pose != 16 {
		if int16(parent.Energy>>16) >= 220 {
			parent.Energy -= 0x40000
		} else {
			parent.Energy -= 0x5000
		}
	}
	if int16(parent.Energy>>16) < 0 {
		parent.MovementSource, parent.StatusDelay = 0x12578, 255
		parent.Body.Auxiliary7E, parent.StunTicks = -1, 0x100
		parent.Body.ActionKind, parent.Body.Pose = 5, 37
		parent.Body.Gravity, parent.Body.VerticalVelocity = 0x3800, -0x60000
		parent.Body.VelocityX = -0x18000
		if parent.Body.Facing != 0 {
			parent.Body.VelocityX = 0x18000
		}
		parent.Body.EnabledStages, parent.Body.ActionFlags = 0x20, 0x20
		parent.Body.Clock.FrameIndex, parent.Body.Clock.Duration, parent.Body.Clock.Ticks, parent.Body.Clock.Completed = 0, 8, 255, 0
		updated, out, err := clearFighter8Projectile(*v)
		if err != nil {
			return before, oldParent, ChuProjectileOutcome{}, err
		}
		state.Visual = updated
		if out.ClearedBytes >= 48 {
			state.Layer = 0
		}
		if out.ClearedBytes >= 64 {
			state.Depth = 0
		}
		return state, parent, out, nil
	}
	updated, err := submitFighter4DragonVisual(*v, environment, library, soundPort)
	if err != nil {
		return before, oldParent, ChuProjectileOutcome{}, err
	}
	state.Visual = updated
	return state, parent, ChuProjectileOutcome{Draw: true}, nil
}
