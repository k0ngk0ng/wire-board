package engine

import "fmt"

type OriginalMatchConfiguration struct {
	Court                 uint8
	Teams                 [2]uint8
	Characters            [4]uint8
	ControllerAssignments [4]int8
	Mode                  uint8
	Equipment             [4][3]int8
	Seed                  RandomState
}

func DefaultOriginalMatchConfiguration() OriginalMatchConfiguration {
	c := OriginalMatchConfiguration{Court: 1, Teams: [2]uint8{0, 1}, Characters: [4]uint8{0, 1, 0, 1}, ControllerAssignments: [4]int8{0, -1, -1, -1}}
	for i := range c.Equipment {
		c.Equipment[i] = [3]int8{-1, -1, -1}
	}
	return c
}

type OpeningState struct {
	Transition                     MatchTransitionState
	Grades                         [4]CharacterGrades
	Equipment                      [4][3]int8
	BallBounces                    uint8
	BallDeflection                 DeflectionAxes
	ExcludedCatcher                int8
	Links                          [13]uint8
	InputDelays                    [12]uint8
	MountFlags                     [6]uint8
	DetachedCount, ScoreSoundDelay uint8
	TacticCache                    [4]uint8
	Projection                     ProjectionState
}
type OpeningStep struct {
	MotionSurfaceWrites                             []MotionSurfaceWrite
	State                                           OpeningState
	SoundIDs                                        []uint8
	BeginWorldInputs, BeginWorldPreparation, FadeIn bool
	Backgrounds                                     []uint8
}

// NewOriginalMatch creates a cold game from selections and an explicit seed.
// Zero-valued simulation fields come from the original C9B6/FA8C/F8BD clear;
// menu choices replace the old menu, and ROM parameter tables supply grades.
// It does not read an observation fixture, save state, ROM or emulator memory.
func NewOriginalMatch(c OriginalMatchConfiguration) (OpeningState, error) {
	var s OpeningState
	if c.Court >= 8 {
		return s, fmt.Errorf("opening court is not yet supported by complete motion integration")
	}
	for i, v := range c.ControllerAssignments {
		if v < -1 || v > 3 {
			return s, fmt.Errorf("invalid controller assignment for seat %d", i)
		}
	}
	t := &s.Transition
	t.Scene, t.Court, t.Mode = 1, c.Court, c.Mode
	t.Teams, t.Characters, t.ControllerAssignments = c.Teams, c.Characters, c.ControllerAssignments
	t.Random = c.Seed
	for i, v := range c.ControllerAssignments {
		if v >= 0 {
			t.PlayerControls[i] = uint8(v)
		}
	}
	// The original character-selection cache survives into the court scene.
	t.Match.Clock.ScoreLowCache = [2]uint8{c.Characters[0], c.Characters[1]}
	t.Match.Clock.DisplayedSecond = c.Characters[2]
	t.Match.Clock.HUDTurn = c.Characters[3]
	for i := range t.Objects {
		t.Objects[i].Request = ActionRequest{Pending: true}
		t.Objects[i].Animation.HasNextAction = true
	}
	s.Equipment = c.Equipment
	result, err := InitializeOpeningGrades(s)
	if err != nil {
		return OpeningState{}, err
	}
	return result.State, nil
}
func InitializeOpeningGrades(original OpeningState) (OpeningStep, error) {
	if original.Transition.Scene == 2 {
		return OpeningStep{State: original}, fmt.Errorf("uncommitted menu equipment preview is not a match configuration")
	}
	result := OpeningStep{State: original}
	s := &result.State
	t := &s.Transition
	for i := range s.Grades {
		g, err := OriginalCharacterGrades(t.Teams[i/2], t.Characters[i])
		if err != nil {
			return OpeningStep{State: original}, err
		}
		if i < 2 && t.Mode&128 == 0 {
			g, err = ApplyOriginalEquipmentGrades(g, s.Equipment[t.Characters[i]])
			if err != nil {
				return OpeningStep{State: original}, err
			}
		}
		s.Grades[i] = g
	}
	return result, nil
}
func (s OpeningState) CharacterProfiles() ([4]CharacterProfile, error) {
	var profiles [4]CharacterProfile
	for i, g := range s.Grades {
		p := KunioProfile()
		var err error
		p.RunSpeed, err = RunSpeedForGrade(g.Run)
		if err != nil {
			return profiles, err
		}
		p.JumpVelocity, err = JumpVelocityForGrade(g.Jump)
		if err != nil {
			return profiles, err
		}
		p.ShotSkill = g.Shot
		profiles[i] = p
	}
	return profiles, nil
}

// InitializeOriginalCourt restores D1B5 and bank1 BE2B's logical hoop setup.
// Background tiles and DMA are renderer requests, not runtime ROM operations.
func InitializeOriginalCourt(original OpeningState) (OpeningStep, error) {
	if original.Transition.Scene != 1 || original.Transition.Match.Phase != 0 || original.Transition.Match.Entered || original.Transition.Court >= 8 {
		return OpeningStep{State: original}, fmt.Errorf("court scene initialization requires unentered scene 1 phase 0")
	}
	r := OpeningStep{State: original, SoundIDs: []uint8{253, 255}, Backgrounds: []uint8{0}}
	s := &r.State
	t := &s.Transition
	t.Match.Entered = true
	for i := range t.Objects {
		t.Objects[i].Pose = Pose{Frame: 127}
		s.Links[i] = 0
	}
	t.Match.OvertimeFlags = 0
	s.InputDelays = [12]uint8{}
	t.Match.Clock.DisplayedSecond = 255
	for i := 7; i < 13; i++ {
		o := &t.Objects[i]
		t.Kinds[i] = 9
		o.Pose = Pose{}
		o.Airborne = false
		o.Body.Position.X = periodSetInteger(o.Body.Position.X, [2]uint16{65, 463}[(i-7)%2])
		o.Body.Position.Depth = periodSetInteger(o.Body.Position.Depth, 43)
		o.Body.Position.Height = periodSetInteger(o.Body.Position.Height, [3]uint16{80, 176, 272}[(i-7)/2])
	}
	s.DetachedCount = 0
	s.MountFlags = [6]uint8{1, 1, 1, 1, 1, 1}
	s.ScoreSoundDelay = 255
	t.Kinds[0], t.Kinds[1] = t.Teams[0], t.Teams[0]
	t.Kinds[2], t.Kinds[3] = t.Teams[1], t.Teams[1]
	t.ResultTimer = 255
	t.FirstTeamLost = true
	s.TacticCache[0], s.TacticCache[2] = 255, 255
	t.CameraScroll[2], t.CameraScroll[3] = 128, 297
	return r, nil
}
func StartOriginalCourt(original OpeningState) (OpeningStep, error) {
	t := original.Transition
	if t.Scene != 1 || t.Match.Phase != 0 || !t.Match.Entered || t.Court >= 8 {
		return OpeningStep{State: original}, fmt.Errorf("court opening start requires entered court scene")
	}
	r := OpeningStep{State: original, SoundIDs: []uint8{[8]uint8{59, 66, 61, 64, 60, 62, 63, 65}[t.Court]}}
	r.State.Transition.Match.Phase, r.State.Transition.Match.Entered = 8, false
	return r, nil
}
func openingBall(s OpeningState) Ball {
	o := s.Transition.Objects[6]
	return Ball{Body: o.Body, Action: BallAction(o.Action), Entered: o.Entered, Request: BallRequest{Action: BallAction(o.Request.Action), Pending: o.Request.Pending}, Pose: o.Pose, Animation: o.Animation, Airborne: o.Airborne, FacingLeft: o.FacingLeft, BouncesRemaining: s.BallBounces, Deflection: s.BallDeflection}
}
func setOpeningBall(s *OpeningState, b Ball) {
	s.Transition.Objects[6] = PeriodObject{Body: b.Body, Action: uint8(b.Action), Entered: b.Entered, Request: ActionRequest{Action: Action(b.Request.Action), Pending: b.Request.Pending}, Pose: b.Pose, Animation: b.Animation, Airborne: b.Airborne, FacingLeft: b.FacingLeft}
	s.BallBounces, s.BallDeflection = b.BouncesRemaining, b.Deflection
}

// StepOriginalRefereeOpening executes one D527 call. The referee waits 28
// calls before throwing, then runs the same restored ball physics as a match.
// The outer loop owns RNG consumption, video ticks and draw-list reset.
func StepOriginalRefereeOpening(original OpeningState) (OpeningStep, error) {
	if original.Transition.Scene != 1 || original.Transition.Match.Phase != 8 || original.Transition.Court >= 8 {
		return OpeningStep{State: original}, fmt.Errorf("referee opening requires a supported court in phase 8")
	}
	r := OpeningStep{State: original}
	s := &r.State
	t := &s.Transition
	if !t.Match.Entered {
		t.Match.Entered = true
		for i, x := range [7]uint16{200, 184, 328, 344, 32, 464, 264} {
			o := &t.Objects[i]
			o.Body.Position.X = periodSetInteger(o.Body.Position.X, x)
			o.Body.Position.Depth = periodSetInteger(o.Body.Position.Depth, [7]uint16{56, 20, 56, 20, 70, 70, 40}[i])
			if i != 6 {
				o.Body.Position.Height &= 255
			}
			o.FacingLeft = i == 2 || i == 3
		}
		t.Endurance = [4]uint8{255, 255, 255, 255}
		t.Kinds[6], t.Kinds[4] = 8, 9
		poses := [7]uint8{0, 0, 128, 128, 3, 255, 0}
		for i, p := range poses {
			t.Objects[i].Pose = Pose{Frame: p & 127, Mirrored: p&128 != 0}
		}
		t.Objects[4].Body.Position.X = periodSetInteger(t.Objects[4].Body.Position.X, 264)
		t.Objects[4].Body.Position.Depth = periodSetInteger(t.Objects[4].Body.Position.Depth, 41)
		t.Objects[4].Animation.Elapsed = 0
		periodSetAction(&t.Objects[6], 0)
		t.Objects[6].Body.Position.Height = boardReplaceLow(t.Objects[6].Body.Position.Height, 8)
		t.Owners[2] = 6
		t.DisplayFlags = 0
	} else {
		if t.Objects[4].Animation.Elapsed == 0 {
			r.FadeIn = true
		}
		t.Objects[4].Animation.Elapsed++
		switch t.Objects[4].Animation.Elapsed {
		case 8:
			r.SoundIDs = append(r.SoundIDs, 55)
		case 28:
			periodSetAction(&t.Objects[6], 7)
			t.Objects[6].Body.Velocity.Height = 16*Pixel + (t.Objects[6].Body.Velocity.Height & 255)
			t.Objects[4].Pose = Pose{Frame: 4}
		case 32:
			t.Match.Phase, t.Match.Entered = MatchPlaying, false
		}
	}
	ball := openingBall(*s)
	court := NewYorkBallCourt().withMotionSurfaceWrites(6, &r.MotionSurfaceWrites)
	if t.Court != 1 {
		court = originalOpeningMotionCourt(s, &r.SoundIDs, &r.MotionSurfaceWrites)
	}
	switch ball.Action {
	case BallRest:
		if t.EndCounter == 0 {
			ball.Pose = Pose{}
		}
		ball.Body.Velocity.X, ball.Body.Velocity.Depth = 0, 0
		if err := advanceBallOnCourt(&ball, court); err != nil {
			return OpeningStep{State: original}, err
		}
	case BallKnockback:
		next, err := StepKnockedBackBall(ball, court)
		if err != nil {
			return OpeningStep{State: original}, err
		}
		ball = next.Ball
		if next.ClearExcludedCatcher {
			s.ExcludedCatcher = -1
		}
	default:
		return OpeningStep{State: original}, fmt.Errorf("unexpected opening ball action %d", ball.Action)
	}
	setOpeningBall(s, ball)
	pc := ProjectionContext{CameraX: t.CameraScroll[2], CameraY: t.CameraScroll[3], LayoutID: t.Scene}
	for i, o := range t.Objects {
		pc.Positions[i] = o.Body.Position
		pc.Poses[i] = o.Pose.Frame
		if o.Pose.Mirrored {
			pc.Poses[i] |= 128
		}
	}
	for i := uint8(0); i < 7; i++ {
		p, err := ProjectOriginalObject(s.Projection, i, pc)
		if err != nil {
			return OpeningStep{State: original}, err
		}
		s.Projection = p
	}
	return r, nil
}
func InitializeOpeningPeriod(original OpeningState) (OpeningStep, error) {
	next, err := InitializeOriginalPeriod(original.Transition)
	if err != nil {
		return OpeningStep{State: original}, err
	}
	r := OpeningStep{State: original, BeginWorldInputs: true, SoundIDs: next.SoundIDs}
	r.State.Transition = next.State
	return r, nil
}
func EnterOriginalTeamTransition(original OpeningState) (OpeningStep, error) {
	if original.Transition.Match.Phase != 3 {
		return OpeningStep{State: original}, fmt.Errorf("team transition requires phase 3")
	}
	r := OpeningStep{State: original, BeginWorldPreparation: true}
	if !r.State.Transition.Match.Entered {
		r.State.Transition.Match.Entered = true
		r.State.Transition.TeamAI = [2]uint8{4, 4}
	}
	return r, nil
}
