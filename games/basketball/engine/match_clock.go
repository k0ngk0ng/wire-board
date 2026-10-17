package engine

// MatchClock preserves the original decimal clock and its wrapped video-frame
// accumulator. HUDTurn and caches matter: the original HUD scheduler decides
// whether this update actually calls the clock routine.
type MatchClock struct {
	Minutes, Tens, Ones      uint8
	VideoRemainder           uint8
	DisplayedSecond, HUDTurn uint8
	ScoreLowCache            [2]uint8
}
type MatchPhase uint8

const (
	MatchPlaying      MatchPhase = 1
	MatchPaused       MatchPhase = 2
	MatchPeriodEnding MatchPhase = 5
)

type MatchState struct {
	Clock         MatchClock
	Phase         MatchPhase
	Entered       bool
	Period        uint8
	OvertimeFlags uint8
	// The end check reads the decimal scoreboard cache, updated by the HUD,
	// rather than the live binary score. Keep both timelines explicit.
	DisplayedScores [2][3]uint8
}
type MatchContext struct {
	FrameParity, Buttons uint8
	Holder               uint8
	BallAirborne         bool
	Scores               [2]uint16
	Tactics, TacticCache [4]uint8
}
type MatchStep struct {
	State    MatchState
	SoundIDs []uint8
}

// AdvanceMatchVideoTick is the F25C interrupt contribution, not a logical
// world update. The outer video driver calls it only when that interrupt path
// is active; menu routes that bypass F25C must not call this unconditionally.
func AdvanceMatchVideoTick(s MatchState) MatchStep {
	if s.Phase != MatchPaused || s.Entered {
		s.Clock.VideoRemainder++
	}
	return MatchStep{State: s}
}

// StepMatchClock restores B1CA. A low wrapped accumulator means the next
// second is due. Subtracting 60 wraps into the high byte range, then video
// interrupts increment it back to zero over the following sixty frames.
func StepMatchClock(s MatchState) MatchStep {
	result := MatchStep{State: s}
	c := &result.State.Clock
	if c.Minutes|c.Tens|c.Ones == 0 || c.VideoRemainder >= 60 {
		return result
	}
	c.VideoRemainder -= 60
	if c.Minutes|c.Tens == 0 && c.Ones != 1 && c.Ones < 4 {
		result.SoundIDs = append(result.SoundIDs, 17)
	}
	c.DisplayedSecond = c.Ones
	c.Ones--
	if c.Ones&128 != 0 {
		c.Ones = 9
		c.Tens--
		if c.Tens&128 != 0 {
			c.Tens = 5
			c.Minutes--
		}
	}
	return result
}

// StepMatchHUD restores the gameplay-relevant part of AFF3/B17E/B00C/B074.
// Tile writes are presentation; panel selection, clock calls and decimal score
// caches remain simulation state because subsequent rules read them.
func StepMatchHUD(s MatchState, ctx MatchContext) MatchStep {
	if ctx.FrameParity&1 != 0 {
		return MatchStep{State: s}
	}
	for attempts := 0; attempts < 4; attempts++ {
		s.Clock.HUDTurn = (s.Clock.HUDTurn + 1) & 3
		slot := s.Clock.HUDTurn
		if slot&1 == 0 {
			if s.Clock.Ones != s.Clock.DisplayedSecond {
				break
			}
		} else {
			side := slot >> 1
			actor := side * 2
			if ctx.Tactics[actor] != ctx.TacticCache[actor] || ctx.Tactics[actor+1] != ctx.TacticCache[actor+1] || uint8(ctx.Scores[side]) != s.Clock.ScoreLowCache[side] {
				break
			}
		}
	}
	if s.Clock.HUDTurn&1 == 0 {
		return StepMatchClock(s)
	}
	side := s.Clock.HUDTurn >> 1
	score := ctx.Scores[side]
	s.Clock.ScoreLowCache[side] = uint8(score)
	s.DisplayedScores[side] = [3]uint8{uint8(score / 100), uint8(score % 100 / 10), uint8(score % 10)}
	return MatchStep{State: s}
}

// CheckMatchPause and CheckMatchResume intentionally differ: entering pause
// requires Start alone, while resuming accepts Start alongside other edges.
func CheckMatchPause(s MatchState, buttons uint8) MatchStep {
	if buttons == 0x10 {
		s.Phase, s.Entered = MatchPaused, false
	}
	return MatchStep{State: s}
}
func CheckMatchResume(s MatchState, buttons uint8) MatchStep {
	if buttons&0x10 != 0 {
		s.Phase, s.Entered = MatchPlaying, true
	}
	return MatchStep{State: s}
}

// CheckMatchPeriodEnd is D36C. A tied second period becomes untimed overtime;
// it does not reset the clock. A free airborne ball delays period end. The
// caller still completes the current world step after this requests phase 5.
// Phase-5 presentation and the later period/reset transition are separate.
func CheckMatchPeriodEnd(s MatchState, ctx MatchContext) MatchStep {
	result := MatchStep{State: s}
	if s.Clock.Minutes|s.Clock.Tens|s.Clock.Ones != 0 {
		return result
	}
	if s.Period&1 != 0 && s.DisplayedScores[0] == s.DisplayedScores[1] {
		if s.OvertimeFlags&128 == 0 {
			result.State.OvertimeFlags = 128
			result.SoundIDs = append(result.SoundIDs, 55)
		}
	} else if ctx.Holder != 6 || !ctx.BallAirborne {
		result.State.Phase, result.State.Entered = MatchPeriodEnding, false
		result.SoundIDs = append(result.SoundIDs, 55)
	}
	return result
}
