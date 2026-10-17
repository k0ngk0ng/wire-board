package engine

import "errors"

// AuxiliaryState describes either court object 4 or 5. Owner is an original
// object reference (-1 means loose); it is not a player control flag.
type AuxiliaryState struct {
	// Court4's conveyor changes this kind on wrap. Zero retains the fixed
	// original court catalog for legacy/static-court states.
	Kind                 uint8 `json:"kind,omitempty"`
	Body                 Body
	Action               uint8
	Entered              bool
	Pose                 Pose
	Animation            AnimationState
	Heading              Heading
	Airborne, FacingLeft bool
	Owner                int8
	Attachment           uint8
	Request              ActionRequest
	NewYorkPassage       uint8
	ObjectLink           int8
}
type AuxiliaryLaunchContext struct {
	ArcFlags uint8
	// Required for a non-player owner: DFF8 reads the masked owner's height.
	// Owner FF aliases the last background column buffers, not player0.
	OwnerHeight *Fixed
}
type AuxiliaryLaunchPlan struct {
	Target    Position
	ArcHeight Fixed
}

type AuxiliaryContext struct {
	Support *RiderSupport
	Launch  *AuxiliaryLaunchContext
	Owner   *Player
	// Actions such as 8 read the facing of any original object owner.
	OwnerFacing   *bool
	OwnerKind     uint8
	SpecialOwner  *Player
	Object, Court uint8
	SpecialHolder int8
}
type AuxiliaryStep struct {
	LaunchPlan         *AuxiliaryLaunchPlan
	ChangePoseSource   bool
	PoseSource         uint8
	State              AuxiliaryState
	ChangeLinkedPose   bool
	LinkedObject       uint8
	LinkedPose         Pose
	ClearSpecialHolder bool
}

// auxiliaryNewYorkCourt preserves bank-4 $8225/$8232 object-specific limits.
// The New York surface/passage geometry is shared, but actor boundaries are not.
func auxiliaryNewYorkCourt() FlatCourt {
	court := NewYorkGroundCourt()
	court.MinX, court.LeftReset = 47*Pixel, 48*Pixel
	court.MaxX, court.RightReset = 480*Pixel, 479*Pixel
	return court
}

// StepAuxiliaryRest runs one original auxiliary-object action call. Other
// object actions fail explicitly until their carry/throw interactions exist.
func StepAuxiliaryRest(original AuxiliaryState, ctx AuxiliaryContext) (AuxiliaryStep, error) {
	return stepAuxiliaryRestOnCourt(original, ctx, auxiliaryNewYorkCourt())
}

func stepAuxiliaryRestOnCourt(original AuxiliaryState, ctx AuxiliaryContext, court FlatCourt) (AuxiliaryStep, error) {
	initial := AuxiliaryStep{State: original}
	result := initial
	s := &result.State
	if ctx.Object < 4 || ctx.Object > 5 {
		return initial, errors.New("auxiliary object must be 4 or 5")
	}
	if ctx.Court != 1 && !((ctx.Court == 0 || ctx.Court == 2 || ctx.Court == 3 || ctx.Court == 4 || ctx.Court == 5) && court.originalMotion != nil) {
		return initial, errors.New("auxiliary court requires its restored original motion binding")
	}
	if s.Action != 0 && s.Action != 7 && s.Action != 12 && s.Action != 13 {
		return initial, errors.New("auxiliary carry or throw action is not reconstructed")
	}
	if s.Attachment&128 != 0 && (s.Action != 7 || s.Entered) {
		return initial, errors.New("auxiliary attached-object motion is not reconstructed")
	}
	action := s.Action
	if !s.Entered {
		s.Entered = true
		if action == 0 {
			s.Body.Velocity.X, s.Body.Velocity.Depth = 0, 0
			if s.Owner >= 7 {
				if s.Owner >= 13 {
					return initial, errors.New("auxiliary linked pose target is outside world objects")
				}
				result.ChangeLinkedPose, result.LinkedObject, result.LinkedPose = true, uint8(s.Owner), Pose{Frame: 2}
			}
		} else if action == 7 {
			// $B1BD clears ownership and attachments before original motion.
			// Facing and the queued next-animation action survive entry.
			s.Owner, s.Attachment = -1, 0
			s.Animation.Cursor, s.Animation.Elapsed, s.Animation.Loops = 0, 0, 0
			s.Body.Velocity, s.Body.Gravity = Velocity{X: 4 * Pixel, Height: 10 * Pixel}, Pixel
			if uint16(s.Body.Position.Height>>8) != 0 && uint8(s.Body.Position.Depth>>8) >= 72 {
				s.Body.Velocity.Depth = -Pixel
			}
			if s.FacingLeft {
				s.Body.Velocity.X = -4 * Pixel
			}
			result.ClearSpecialHolder = ctx.Object == 4 && ctx.SpecialHolder != -1
		} else if action == 12 {
			s.Animation.Cursor, s.Animation.Elapsed, s.Animation.Loops = 0, 0, 0
			s.Body.Velocity.Height, s.Body.Gravity = Pixel/16, Pixel/2
			s.ObjectLink, s.Owner = -1, -1
		} else {
			// $B3EA preserves facing across action initialization and sets
			// a finite slide, with the original $EC4A/$EC59 full stop.
			s.Animation = AnimationState{NextAction: ActionIdle, HasNextAction: true}
			s.Body.Velocity, s.Body.Gravity = Velocity{X: 10 * Pixel}, 0
			if s.FacingLeft {
				s.Body.Velocity.X = -10 * Pixel
			}
		}
	}
	if action == 7 || action == 12 || action == 13 {
		spec := AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 0}, Ticks: 255}, {Pose: Pose{Frame: 0}, Ticks: 255}}, End: RepeatForever}
		if action == 13 {
			spec = AnimationSpec{Frames: []AnimationFrame{{Ticks: 3}, {Ticks: 16}}, End: RepeatRemaining}
		}
		animation, err := AdvanceAnimation(spec, s.Animation, s.FacingLeft)
		if err != nil {
			return initial, err
		}
		s.Animation, s.Pose = animation.State, animation.Pose
		if animation.ChangeAction {
			s.Action, s.Entered = uint8(animation.Action), false
		}
		// $B3DC reads the old airborne flag before $EEFF advances physics.
		if action == 12 && !s.Airborne {
			s.Action, s.Entered = 0, false
		}
	}
	if court.originalMotion != nil {
		if err := court.originalMotion.auxiliary(s); err != nil {
			return initial, err
		}
	} else {
		// This adapter shares only restored New York terrain calculations. No
		// player action dispatcher or player-specific input/AI is invoked.
		p := Player{Body: s.Body, Action: Action(s.Action), Entered: s.Entered, Pose: s.Pose, Animation: s.Animation, Airborne: s.Airborne, FacingLeft: s.FacingLeft, Request: s.Request, NewYorkPassage: s.NewYorkPassage, Heading: HeadingNone}
		if err := advanceAuxiliaryOnCourt(&p, s.Owner < 0, court); err != nil {
			return initial, err
		}
		s.Body, s.Action, s.Entered, s.Pose, s.Animation = p.Body, uint8(p.Action), p.Entered, p.Pose, p.Animation
		s.Airborne, s.FacingLeft, s.Request, s.NewYorkPassage = p.Airborne, p.FacingLeft, p.Request, p.NewYorkPassage

	}
	if action == 0 && s.Airborne && s.Owner < 0 {
		s.Action, s.Entered = 12, false
	}
	if action == 13 && s.Body.Velocity.X == 0 {
		s.Action, s.Entered = 0, false
	}
	if action == 7 && !s.Airborne {
		s.Action, s.Entered = 0, false
		s.Body.Velocity.X, s.Body.Velocity.Depth = 0, 0
	}
	return result, nil
}

func advanceAuxiliaryOnCourt(p *Player, computer bool, court FlatCourt) error {
	if p.NewYorkPassage != 0 || (p.Action != 7 && p.Action != 13) {
		_, err := advancePlayerOnCourt(p, playerTerrainContext{Computer: computer}, court)
		return err
	}
	court.recordMotionSurface(p.Body.Position)
	previous := p.Body.Position
	next := IntegrateMotion(p.Body)
	clampPlayerTravel(&next, court)
	// bank4 $8857/$88BA: these auxiliary actions rebound from the New
	// York passage mouth. They never enter the player's underground path.
	raised := court.NewYorkPassages && previous.Depth >= 73*Pixel && (next.Position.X < court.RaisedLeftEnd || next.Position.X >= court.RaisedRightStart)
	if raised && next.Position.Height>>8 < court.RaisedFloor>>8 && !(next.Velocity.Height < 0 && previous.Height>>8 >= court.RaisedFloor>>8) {
		p.Action, p.Entered, p.FacingLeft = 7, false, !p.FacingLeft
	}
	// $8249 leaves its candidate floor at zero when the integrated height
	// is negative. Landing then still runs $86AF's side/depth resolution;
	// the raised old-floor cache must not lift the object to 64 pixels.
	if next.Position.Height < 0 && next.Velocity.Height < 0 && previous.Height >= 0 {
		next.Position.Height, next.Velocity.Height = 0, 0
		p.Body, p.Airborne = next, false
		advanceAuxiliaryGroundSides(p, next, previous, court)
		p.Body.Gravity = 0
		return nil
	}
	return advancePlayerSurface(p, next, previous, court)
}

func advanceAuxiliaryGroundSides(p *Player, next Body, previous Position, court FlatCourt) {
	b := &p.Body
	oldFloor := courtSurface(court, previous.X, previous.Depth)
	ahead := func() {
		if oldFloor > court.Floor {
			b.Position.Depth = surfaceDepth(b.Position.Depth, previous.Depth)
		}
	}
	blocked := false
	xFloor := courtSurface(court, next.Position.X, previous.Depth)
	if xFloor > 0 {
		if previous.X>>8 < next.Position.X>>8 {
			b.Position.X = (previous.X &^ 2047) + 7*Pixel + (previous.X & 255)
		} else if previous.X>>8 > next.Position.X>>8 {
			b.Position.X = (previous.X &^ 2047) + (previous.X & 255)
		}
		ahead()
		blocked = true
	}
	yFloor := courtSurface(court, previous.X, next.Position.Depth)
	if yFloor > 0 {
		b.Position.Depth = (previous.Depth &^ 2047) + 7*Pixel + (previous.Depth & 255)
		ahead()
		blocked = true
	}
	if !blocked {
		b.Position.X, b.Position.Depth = previous.X, previous.Depth
	}
}
