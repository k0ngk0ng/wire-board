package engine

import "fmt"

// PeriodObject contains only the object fields touched by period/reset/result
// transitions. The caller retains ownership links, attack profiles and other
// gameplay state that these original routines leave unchanged.
type PeriodObject struct {
	Body                 Body
	Action               uint8
	Entered              bool
	Request              ActionRequest
	Pose                 Pose
	Animation            AnimationState
	Airborne, FacingLeft bool
}
type MatchTransitionState struct {
	Match                                   MatchState
	Scene, Mode, Court                      uint8
	Objects                                 [13]PeriodObject
	Kinds                                   [13]uint8
	Owners                                  [9]uint8 // object 4..12, with 255 denoting no owner
	PlayerControls                          [4]uint8
	ControllerAssignments                   [4]int8
	Characters                              [4]uint8
	Teams                                   [2]uint8
	Endurance                               [4]uint8
	AIModes, AIPlans                        [4]uint8
	TeamAI                                  [2]uint8
	HoopDurability                          [6]int8
	SpecialHolder, PassTarget               int8
	AuxiliaryLinks                          [2]int8
	ShotCycle                               [4]int8
	TeamFacing                              [2]uint8
	TrackedObject, DisplayFlags, EndCounter uint8
	EndActorFlags                           [4]uint8
	SelectedController                      int8
	// During play these two bytes hold ball action history. The result scene
	// reuses them for controller assignments; adapters must preserve that alias.
	ResultControllers [2]int8
	LastShooter       uint8
	Passages          [9]uint8
	ResultTimer       uint8
	FirstTeamLost     bool
	ResultExitFlag    int8
	TeamWinBits       [2]uint8
	CameraScroll      [4]uint16 // displayed X/Y, next X/Y, in original integer pixels
	Random            RandomState
	Scores            [2]uint16
}
type MatchTransitionStep struct {
	State    MatchTransitionState
	SoundIDs []uint8
	// CPU work in D4FF..D50D, before interrupt time is added by the video driver.
	// This must not be converted into an arbitrary number of logical updates.
	DelayCPUCycles                                                                  uint32
	FadeOut, ClearCourt, NeedsTeamSelection, NeedsVictoryScene, NeedsCampaignEnding bool
	BeginWorldInputs                                                                bool
	HasOutcome                                                                      bool
	WinningTeam                                                                     uint8
	Backgrounds                                                                     []uint8
}

func transitionError(s MatchTransitionState, what string) (MatchTransitionStep, error) {
	return MatchTransitionStep{State: s}, fmt.Errorf("%s cannot run in phase %d entered=%t", what, s.Match.Phase, s.Match.Entered)
}
func periodSetAction(o *PeriodObject, action uint8) { o.Action, o.Entered = action, false }
func periodSetInteger(v Fixed, pixels uint16) Fixed { return Fixed(pixels)*Pixel + (v & 255) }

// InitializeOriginalPeriod restores D257..D303 and its D5CA/D637 helpers.
// Scores persist. It begins at the input phase, bypassing the pre-world stage.
func InitializeOriginalPeriod(original MatchTransitionState) (MatchTransitionStep, error) {
	if original.Match.Phase != MatchPlaying || original.Match.Entered {
		return transitionError(original, "period setup")
	}
	if original.Court >= 8 {
		return MatchTransitionStep{State: original}, fmt.Errorf("unknown original court %d", original.Court)
	}
	r := MatchTransitionStep{State: original, BeginWorldInputs: true}
	s := &r.State
	s.Match.Entered = true
	for i, assignment := range s.ControllerAssignments {
		if assignment < 0 {
			s.PlayerControls[i] = 128
		}
	}
	x := [7]uint16{200, 184, 328, 344, 32, 464, 264}
	depth := [7]uint16{56, 20, 56, 20, 70, 70, 40}
	for i := 0; i < 7; i++ {
		o := &s.Objects[i]
		o.Body.Position.X = periodSetInteger(o.Body.Position.X, x[i])
		o.Body.Position.Depth = periodSetInteger(o.Body.Position.Depth, depth[i])
		if i != 6 {
			o.Body.Position.Height &= 255
		}
		o.FacingLeft = i == 2 || i == 3
	}
	for i := 4; i <= 6; i++ {
		s.Objects[i].Pose = Pose{}
	}
	s.Endurance = [4]uint8{255, 255, 255, 255}
	s.Owners[0], s.Owners[1], s.Owners[2] = 255, 255, 6
	s.SpecialHolder, s.PassTarget = -1, -1
	s.AuxiliaryLinks = [2]int8{-1, -1}
	s.ShotCycle = [4]int8{-1, -1, -1, -1}
	s.TeamAI = [2]uint8{1, 1}
	// D2A9 returns to the stores at D2A0, skipping the one-time TYA.
	s.AIModes = [4]uint8{}
	s.AIPlans = s.AIModes
	for i := 0; i < 6; i++ {
		s.Owners[i+3] = uint8(i + 7)
		periodSetAction(&s.Objects[i], 0)
		periodSetAction(&s.Objects[i+7], 1)
	}
	s.HoopDurability = [6]int8{8, 8, 6, 6, 5, 5}
	s.TeamFacing = [2]uint8{0, 128}
	s.Kinds[6] = 8
	kinds := [4][2]uint8{{10, 11}, {12, 11}, {12, 13}, {13, 14}}
	s.Kinds[4], s.Kinds[5] = kinds[s.Court&3][0], kinds[s.Court&3][1]
	s.Match.Clock.Minutes, s.Match.Clock.Tens, s.Match.Clock.Ones, s.Match.Clock.VideoRemainder = 3, 0, 1, 0
	s.TrackedObject = 6
	return r, nil
}

// WaitOriginalPeriodEnd restores phase 5. The first call latches entry; the
// next waits and requests phase 4. No winner is declared at this stage.
func WaitOriginalPeriodEnd(original MatchTransitionState) (MatchTransitionStep, error) {
	if original.Match.Phase != MatchPeriodEnding {
		return transitionError(original, "period end wait")
	}
	r := MatchTransitionStep{State: original}
	if !r.State.Match.Entered {
		r.State.Match.Entered = true
		r.State.EndCounter = 0
	} else {
		r.DelayCPUCycles = 717316
		r.State.Match.Phase = 4
		r.State.Match.Entered = false
	}
	return r, nil
}

// ResetOriginalPeriod is phase 4, D420. It clears the exact original fields,
// retaining scores, airborne flags, fractions and animation repeat counters.
func ResetOriginalPeriod(original MatchTransitionState) (MatchTransitionStep, error) {
	if original.Match.Phase != 4 {
		return transitionError(original, "between-period reset")
	}
	r := MatchTransitionStep{State: original, SoundIDs: []uint8{0}, FadeOut: true, ClearCourt: true}
	s := &r.State
	s.CameraScroll[2], s.CameraScroll[3] = 0, 0
	s.DisplayFlags = 0
	for i := range s.Objects {
		o := &s.Objects[i]
		periodSetAction(o, 0)
		o.Request = ActionRequest{Action: ActionIdle, Pending: true}
		o.Body.Velocity = Velocity{}
		o.Body.Gravity = 0
		o.Body.Position.X &= 65535
		o.Body.Position.Height &= 255
		o.Animation.Elapsed, o.Animation.Cursor = 0, 0
		o.Pose = Pose{Frame: 127}
	}
	s.Match.Clock.ScoreLowCache = [2]uint8{s.Characters[0], s.Characters[1]}
	s.Match.Clock.DisplayedSecond = s.Characters[2]
	s.Match.Clock.HUDTurn = s.Characters[3]
	s.EndActorFlags = [4]uint8{}
	s.SelectedController = 0
	s.ResultControllers = [2]int8{}
	s.LastShooter = 0
	s.Passages = [9]uint8{}
	s.Match.Period++
	if s.Match.Period&1 != 0 {
		s.Scene = 2
		s.Match.Phase = 4
		s.Match.Entered = false
		r.NeedsTeamSelection = true
	} else {
		s.Match.Phase = 6
		s.Match.Entered = false
		s.CameraScroll[2], s.CameraScroll[3] = s.CameraScroll[0], s.CameraScroll[1]
	}
	return r, nil
}

// StepOriginalMatchResult restores the semantic part of BA40, stopping before
// BC96's celebration-actor calls. It consumes the shared original RNG and
// preserves campaign progression, versus-mode exit and controller selection.
func StepOriginalMatchResult(original MatchTransitionState, buttons uint8) (MatchTransitionStep, error) {
	if original.Match.Phase != 6 {
		return transitionError(original, "match result")
	}
	r := MatchTransitionStep{State: original, HasOutcome: true}
	s := &r.State
	if !s.Match.Entered {
		if s.Court >= 8 || s.Teams[1] >= 8 {
			return MatchTransitionStep{State: original}, fmt.Errorf("unknown result court or team")
		}
		s.Match.Entered = true
		s.ResultTimer = 0
		for i := 0; i < 4; i++ {
			s.Objects[i].Airborne = false
			s.Objects[i].Pose = Pose{Frame: 127}
		}
		s.FirstTeamLost = s.Scores[0] <= s.Scores[1]
		controllerBase := 0
		if s.FirstTeamLost {
			for i := 0; i < 4; i++ {
				s.Objects[i].Pose.Mirrored = true
			}
			s.Objects[0].Body.Velocity.X, s.Objects[1].Body.Velocity.X = -192, -192
			for i, a := range [4]uint8{0x61, 0x61, 0x5b, 0x60} {
				periodSetAction(&s.Objects[i], a)
			}
			if s.Mode&128 != 0 && s.Mode&127 != 0 {
				controllerBase = 2
			}
		} else {
			s.Objects[2].Body.Velocity.X, s.Objects[3].Body.Velocity.X = 192, 192
			for i, a := range [4]uint8{0x5b, 0x60, 0x61, 0x61} {
				periodSetAction(&s.Objects[i], a)
			}
		}
		left, right := s.ControllerAssignments[controllerBase], s.ControllerAssignments[controllerBase+1]
		if right < 0 {
			s.ResultControllers = [2]int8{-1, left}
		} else {
			s.ResultControllers = [2]int8{left, right}
		}
		s.SelectedController = s.ResultControllers[0]
		s.CameraScroll = [4]uint16{0, 296, 0, 296}
		for i, x := range [4]uint16{80, 48, 176, 192} {
			o := &s.Objects[i]
			o.Body.Position.X = periodSetInteger(o.Body.Position.X, x)
			o.Body.Position.Depth = periodSetInteger(o.Body.Position.Depth, [4]uint16{56, 28, 56, 28}[i])
			o.Body.Position.Height &= 255
			s.Kinds[i] = 17
		}
		if s.Mode&128 == 0 {
			if s.FirstTeamLost {
				s.Match.Period -= 2
			} else {
				index := ((s.Match.Period >> 1) + 1) & 1
				s.TeamWinBits[index] |= [8]uint8{0, 1, 2, 4, 8, 16, 32, 64}[s.Teams[1]]
			}
		}
		r.Backgrounds = []uint8{s.Court + 101, 99}
		r.SoundIDs = []uint8{253, 255, 253, 255}
	} else {
		s.ResultTimer++
		if s.ResultTimer != 144 {
			var value uint8
			s.Random, value = NextRandom(s.Random)
			if value&31 == 0 {
				r.SoundIDs = append(r.SoundIDs, 50)
			}
		}
		if s.ResultTimer == 144 || buttons&0x90 != 0 {
			r.FadeOut = true
			for i := 0; i < 4; i++ {
				o := &s.Objects[i]
				o.Body.Velocity.X, o.Body.Velocity.Height, o.Body.Gravity = 0, 0, 0
				o.Body.Position.Height &= 255
				periodSetAction(o, 0)
				s.EndActorFlags[i] = 0
			}
			if s.Mode&128 != 0 {
				s.Scene = 2
				s.Match.Phase, s.Match.Entered = 2, false
				s.ControllerAssignments = [4]int8{-1, -1, -1, -1}
				if s.ResultExitFlag < 0 {
					s.Mode = 129
				}
				r.NeedsTeamSelection = true
			} else if s.Match.Period == 28 || s.FirstTeamLost {
				s.Scene = 7
				s.Match.Phase, s.Match.Entered = 0, false
				r.NeedsCampaignEnding = true
			} else {
				s.Match.Phase, s.Match.Entered = 7, false
				r.NeedsVictoryScene = true
				for i, x := range [4]uint16{104, 128, 128, 0} {
					o := &s.Objects[i]
					o.Body.Velocity.X = 0
					s.EndActorFlags[i] = 0
					periodSetAction(o, [4]uint8{98, 99, 100, 0}[i])
					o.Pose = Pose{Frame: 127, Mirrored: i != 0}
					o.Body.Position.X = periodSetInteger(o.Body.Position.X, s.CameraScroll[0]+x)
					s.Kinds[i] = 18
				}
				s.Objects[1].Body.Position.Depth = periodSetInteger(s.Objects[1].Body.Position.Depth, 56)
				s.Match.Clock.ScoreLowCache[0] = s.Characters[0]
				s.Match.Clock.DisplayedSecond = s.Characters[2]
				s.Characters[0], s.Characters[2] = 0, s.Teams[1]
			}
		}
	}
	if s.FirstTeamLost {
		r.WinningTeam = 1
	}
	return r, nil
}
