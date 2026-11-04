package engine

// Fighter2ProjectileState uses the shared projectile fields and the already
// canonical hit-stop byte at69. It owns no parent or duplicate position state.
type Fighter2ProjectileState struct {
	Projectile   ChuProjectileState
	HitStopTicks uint8
}

// StepFighter2Projectile recovers12dd4 through the1580 draw request or3556
// prefix clear. This object never follows a parent or moves by its velocity.
// Nonzero hit-stop freezes its clock while animation submission still runs.
func StepFighter2Projectile(state Fighter2ProjectileState, environment ChuProjectileEnvironment, library AnimationLibrary) (Fighter2ProjectileState, ChuProjectileOutcome, error) {
	before := state
	s := &state.Projectile
	if s.Stage == 0 {
		initializeFighter0ProjectileGraphics(s, environment.PaletteAttributes)
		s.Animation.AnimationSet, s.Animation.ShapeSet = 0xd92ca, 0xd9a3a
		s.Animation.Clock.Duration, s.Animation.Clock.Ticks = 8, 255
		// Both original comparisons retain the old coordinate in D0, even
		// if the first clamp already wrote the destination's high word.
		x := int16(s.Animation.WorldX >> 16)
		if x <= environment.ArenaLeft {
			s.Animation.WorldX = replaceChuSpecialHighWord(s.Animation.WorldX, uint16(environment.ArenaLeft))
		}
		if x >= environment.ArenaRight {
			s.Animation.WorldX = replaceChuSpecialHighWord(s.Animation.WorldX, uint16(environment.ArenaRight))
		}
	}
	s.Health = 255
	if environment.PauseFlags&0x80 != 0 || environment.SlowFlag != 0 {
		return state, ChuProjectileOutcome{Draw: true}, nil
	}
	if state.HitStopTicks != 0 {
		state.HitStopTicks--
		if s.Shake > 1 {
			s.Shake--
		}
		if state.HitStopTicks&1 == 0 {
			s.ScreenOffsetX += int16(s.Shake)
		} else {
			s.ScreenOffsetX -= int16(s.Shake)
		}
	} else {
		s.Shake = 0
		s.Animation.Clock = StepAnimationClock(s.Animation.Clock)
		if s.Animation.Clock.Completed == 1 {
			next, outcome, err := clearChuProjectile(*s)
			if err != nil {
				return before, ChuProjectileOutcome{}, err
			}
			state.Projectile = next
			if outcome.ClearedBytes >= 112 {
				state.HitStopTicks = 0
			}
			return state, outcome, nil
		}
		if s.Animation.Pose == 9 && s.Animation.Clock.Ticks == 0 {
			pool, _, err := SpawnMovementDust(s.Pool, EffectSource{X: s.Animation.WorldX, Y: s.Animation.WorldY,
				Attributes: s.Animation.Attributes, Depth: s.Depth, Layer: s.Layer})
			if err != nil {
				return before, ChuProjectileOutcome{}, err
			}
			s.Pool = pool
		}
	}
	submission, err := SubmitAnimationRecord(AnimationSubmissionState{Actor: s.Animation, Buffers: s.Buffers,
		PendingSound: s.PendingSound, SoundQueue: s.SoundQueue}, library)
	if err != nil {
		return before, ChuProjectileOutcome{}, err
	}
	s.Animation, s.Buffers, s.PendingSound, s.SoundQueue = submission.Actor, submission.Buffers, submission.PendingSound, submission.SoundQueue
	projectFighter0Projectile(s, environment)
	return state, ChuProjectileOutcome{Draw: true}, nil
}
