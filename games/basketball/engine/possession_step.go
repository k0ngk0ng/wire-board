package engine

import "errors"

type PossessionState struct {
	Carrier            Carrier     `json:"carrier"`
	Input              InputMemory `json:"input"`
	Ball               Ball        `json:"ball"`
	HasBall            bool        `json:"hasBall"`
	RimFlags           [6]uint8    `json:"rimFlags"`
	LastShot           ShotPlan    `json:"lastShot"`
	ShotOriginX        Fixed       `json:"shotOriginX"`
	Hoops              [6]Hoop     `json:"hoops"`
	Random             RandomState `json:"random"`
	Scores             [2]uint16   `json:"scores"`
	ScoreSoundDelay    uint8       `json:"scoreSoundDelay"`
	PreviousBallAction BallAction  `json:"previousBallAction"`
	ObservedBallAction BallAction  `json:"observedBallAction"`
	HoopClimbers       [6]bool     `json:"hoopClimbers"`
	HoopContactLatch   uint8       `json:"hoopContactLatch"`
}

// StepControlledPossession runs the restored input, action and ball phases in
// order. The receiver position is another player's world state; the controlled
// player and ball carry their own state continuously. Full-world contact and
// special shots are not substituted with invented behavior.
func StepControlledPossession(original PossessionState, frame ControllerFrame, receiver Position, profile CharacterProfile, court FlatCourt) (PossessionState, error) {
	s := original
	s.Hoops = ResetHoopScoreLocks(s.Hoops, s.HasBall)
	detached := uint8(0)
	for _, h := range s.Hoops {
		if h.Action == HoopInactive || h.Action >= HoopCarried {
			detached++
		}
	}
	if s.Ball.Action != s.ObservedBallAction {
		s.PreviousBallAction = s.ObservedBallAction
		s.ObservedBallAction = s.Ball.Action
	}
	rim, err := ResolveRimContact(RimState{Ball: s.Ball, Hoops: s.Hoops, Random: s.Random, Scores: s.Scores, ScoreSoundDelay: s.ScoreSoundDelay}, RimContext{PreviousBallAction: s.PreviousBallAction, SelectedHoop: int(s.LastShot.Hoop), ShotOriginX: s.ShotOriginX, ShooterAction: s.Carrier.Player.Action, ShotSkill: profile.ShotSkill, ScoringBonus: profile.ScoringBonus})
	if err != nil {
		return original, err
	}
	s.Ball, s.Hoops, s.Random, s.Scores, s.ScoreSoundDelay = rim.State.Ball, rim.State.Hoops, rim.State.Random, rim.State.Scores, rim.State.ScoreSoundDelay
	p := s.Carrier.Player
	ctx := FreeInputContext{Action: p.Action, PoseFacingLeft: p.Pose.Mirrored, Airborne: p.Airborne, Request: p.Request}
	var resolved InputResult
	if freeAction(p.Action) {
		resolved, err = ResolveFreeInput(ctx, s.Input, frame)
	} else {
		resolved, err = ResolveCarrierInput(CarrierInputContext{Player: ctx, HasBall: s.HasBall}, s.Input, frame)
	}
	if err != nil {
		return original, err
	}
	s.Input = resolved.Memory
	s.Carrier.Player.Request, s.Carrier.Player.Heading = resolved.Request, resolved.Heading
	if s.HasBall {
		s.Ball = StepOwnedBallInput(s.Ball, &s.Carrier.Player)
	}
	if freeAction(p.Action) {
		s.Carrier.Player, err = StepFreePlayer(s.Carrier.Player, profile, court)
		if err != nil {
			return original, err
		}
	} else {
		for pass := 0; pass < 2; pass++ {
			origin := s.Carrier.Player.Body.Position.X
			step, err := StepCarrierAction(s.Carrier, CarrierContext{HasBall: s.HasBall, Teammate: receiver, BallAction: s.Ball.Action}, profile, court)
			if err != nil {
				return original, err
			}
			s.Carrier = step.Carrier
			if step.BindBall {
				s.HasBall = true
			}
			s.Ball = ApplyBallCommand(s.Ball, step.Ball)
			if step.Ball.Change {
				if step.Ball.Action == BallShot {
					s.Ball.FacingLeft = s.Carrier.Player.FacingLeft
					s.ShotOriginX = origin
				}
			}
			if !step.Revisit {
				break
			}
			if pass == 1 {
				return original, errors.New("unexpected repeated carrier takeoff")
			}
		}
	}
	s.Ball, err = ResolveBallRequest(s.Ball)
	if err != nil {
		return original, err
	}
	switch s.Ball.Action {
	case BallDribble, BallHeld:
		if !s.HasBall {
			return original, errors.New("held ball has no local carrier")
		}
		s.Ball, err = StepHeldBall(s.Ball, s.Carrier.Player)
	case BallPass:
		s.Ball, err = StepPassedBall(s.Ball, receiver)
		s.HasBall = false
	case BallShot:
		if !s.Ball.Entered {
			s.LastShot, err = ChooseOrdinaryShot(s.Ball.Body.Position, s.Carrier.Player.Body.Position.Height, s.Ball.FacingLeft, s.RimFlags)
			if err != nil {
				return original, err
			}
		}
		s.Ball, err = StepShotBall(s.Ball, s.LastShot, NewYorkBallCourt())
		s.HasBall = false
	case BallBounce:
		s.Ball, err = StepBouncingBall(s.Ball, NewYorkBallCourt())
		s.HasBall = false
	case BallDeflected:
		s.Ball, err = StepDeflectedBall(s.Ball, NewYorkBallCourt())
		s.HasBall = false
	default:
		return original, errors.New("ball action not yet restored in possession world step")
	}
	if err != nil {
		return original, err
	}
	for i, h := range s.Hoops {
		// Isolated player/ball replays may omit hoop objects entirely.
		if h == (Hoop{}) {
			continue
		}
		step, err := StepHoopAction(h, HoopContext{Index: uint8(i), DetachedCount: detached, MountFlags: s.RimFlags, ContactLatch: s.HoopContactLatch, HasClimber: s.HoopClimbers[i]}, court)
		if err != nil {
			return original, err
		}
		s.Hoops[i], s.RimFlags, s.HoopContactLatch = step.Hoop, step.MountFlags, step.ContactLatch
	}
	return s, nil
}
