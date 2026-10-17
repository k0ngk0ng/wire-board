package engine

import (
	"errors"
	"fmt"
)

type AIMode uint8

const (
	AIRecover        AIMode = 0
	AIPossessionWait AIMode = 1
	AIPursueBall     AIMode = 2
	AIDefend         AIMode = 9
	AIWaitIdle       AIMode = 20
	AIPause          AIMode = 27
	AICollectHoop    AIMode = 40
)

// CourtZone is the original team-relative, nonuniform court partition. The
// high nibble is forward progress; the low nibble is court depth.
type CourtZone uint8

func CourtZoneForActor(position Position, actor uint8) (CourtZone, error) {
	if actor >= 4 {
		return 0, errors.New("invalid basketball actor")
	}
	x, z := position.X>>8, position.Depth>>8
	if x < 0 || x >= 512 || z < 0 || z > 255 {
		return 0, errors.New("position outside original court-zone domain")
	}
	band := uint8(0)
	for _, edge := range [...]Fixed{88, 120, 184, 264, 344, 408, 440} {
		if x >= edge {
			band++
		}
	}
	if actor >= 2 {
		band = 7 - band
	}
	depth := uint8(0)
	for _, edge := range [...]Fixed{32, 43, 53, 64, 72} {
		if z >= edge {
			depth++
		}
	}
	return CourtZone(band<<4 | depth), nil
}

type AIControlState struct {
	Mode          AIMode `json:"mode"`
	Entered       bool   `json:"entered"`
	Plan          AIMode `json:"plan"`
	Counter       int8   `json:"counter"`
	PlanParameter uint8  `json:"planParameter"`
	// Object IDs match world seats (0..3), ball (6), and hoops (7..12).
	// -1 means no tracked object; other modes may retain player targets.
	TrackedObject int8      `json:"trackedObject"`
	Destination   CourtZone `json:"destination"`
}

type AIHoopView struct {
	Position Position
	Action   HoopAction
	Owner    int8
}

type AIControlView struct {
	TeamAI          *TeamAIState
	HumanTactics    *[2]uint8
	PursuitWorld    *WorldActionState
	PlayerControls  *[4]uint8
	PursuitReserved *PursuitReservedBytes
	PursuitAlias74  *PursuitAlias74
	Actor           uint8
	Action          Action
	ActionEntered   bool
	Heading         Heading
	FacingLeft      bool
	Position        Position
	HasBall         bool
	Hoops           [6]AIHoopView
	Players         [4]Position
	BallPosition    Position
	BallHolder      int8
	Random          RandomState
	BallAction      BallAction
	BallFacingLeft  bool
	BallLanding     Position
	PursuitOptions  *[4]AITacticalOption
	Team            uint8
	Difficulty      uint8
	ScoreDifference uint8
	CourtID         uint8
	CachedZones     *[4]CourtZone
	ShotContext     AIShotContext
	TeammateLink    uint8
	TeamHasHuman    bool
	Airborne        bool
	PlayerActions   [4]Action
	Auxiliary       *[2]AuxiliaryState
}

// A decision chooses original actions or emits pickup/movement commands.
// The actor phase applies those commands; the decision cannot assign positions,
// hits, scores or other physics outcomes.
type AIControlDecision struct {
	TeammateControl    AITeammateControlCommand
	ClearCommandStatus bool
	Control            AIControlState
	Action             Action
	ActionEntered      bool
	Heading            Heading
	FacingLeft         bool
	BindHoop           int
	Random             RandomState
	Ball               BallCommand
	BindBall           bool
	ReactionReady      bool
	ShotPlan           AIShotCommand
	Engagement         AIEngagementCommand
}

func originalAIActionAvailable(action Action) bool {
	a := uint8(action)
	return a <= 6 || a >= 0x10 && a <= 0x12 || a >= 0x14 && a <= 0x16 || a == 0x1a || a == 0x1f || a >= 0x20 && a <= 0x23 || a >= 0x30 && a <= 0x32 || a >= 0x3d && a <= 0x3f || a == 0x6d || a == 0x6e || a >= 0x70 && a <= 0x73 || a >= 0x76 && a <= 0x7f
}

func chooseAIAction(result *AIControlDecision, action Action) {
	if result.Action != action {
		result.Action = action
		result.ActionEntered = false
	}
}

func steerOriginalZone(current, target CourtZone, actor uint8) Heading {
	var buttons Buttons
	if current>>4 < target>>4 {
		buttons = ButtonRight
	} else if current>>4 > target>>4 {
		buttons = ButtonLeft
	}
	if buttons != 0 && actor >= 2 {
		buttons ^= ButtonRight | ButtonLeft
	}
	if current&15 < target&15 {
		buttons |= ButtonUp
	} else if current&15 > target&15 {
		buttons |= ButtonDown
	}
	return HeadingForButtons(buttons)
}

// approachOriginalHoop preserves the original asymmetric depth window and
// approach direction; it is not a replacement pathfinder or nearest-object AI.
func approachOriginalLooseObject(result *AIControlDecision, player, hoop Position) bool {
	dx := int32((player.X >> 8) - (hoop.X >> 8))
	if absPass(dx) < 9 {
		dz := uint8((player.Depth >> 8) - (hoop.Depth >> 8))
		if dz >= 248 || dz < 8 {
			return true
		}
		result.Heading = HeadingDown
		if player.Depth>>8 < hoop.Depth>>8 {
			result.Heading = HeadingUp
		}
		chooseAIAction(result, ActionWalk)
		return false
	}
	buttons := ButtonLeft | ButtonDown
	if dx < 0 {
		buttons = ButtonRight | ButtonDown
	}
	result.Heading = HeadingForButtons(buttons)
	chooseAIAction(result, ActionRun)
	return false
}

// StepOriginalAIControl dispatches recovered original tactical modes.
// Unrestored branches fail rather than inventing chase, shot or scoring policies.
// All input is public, current court/actor state.
func StepOriginalAIControl(original AIControlState, view AIControlView) (AIControlDecision, error) {
	initial := AIControlDecision{Control: original, Action: view.Action, ActionEntered: view.ActionEntered, Heading: view.Heading, FacingLeft: view.FacingLeft, BindHoop: -1, Random: view.Random}
	result := initial
	if view.Actor >= 4 {
		return initial, errors.New("invalid AI actor")
	}
	control, dispatch, err := originalAIControlDispatch(original, view.Action, view.HasBall)
	if err != nil {
		return initial, err
	}
	if !dispatch {
		return initial, nil
	}
	result.Control = control
	s := &result.Control
	changeMode := func(mode AIMode) { s.Mode = mode; s.Entered = false }
	switch s.Mode {
	case AIRecover:
		s.Entered = true
		if view.Action < 3 || view.Action == 0x3d || view.Action == 0x1d {
			mode := s.Plan
			if mode == 17 && !view.HasBall {
				mode = 8
			}
			changeMode(mode)
		}
	case AIPossessionWait:
		if !s.Entered {
			s.Entered = true
			if !view.TeamHasHuman && view.Action == ActionCatchAir && view.Team == 7 && s.Plan == 33 {
				changeMode(33)
				return result, nil
			}
		}
		if !view.HasBall {
			changeMode(AIRecover)
		} else if view.Action == Action(0x3e) {
			changeMode(31)
		} else if view.Action >= ActionDribbleIdle && view.Action <= ActionDribbleRun {
			if view.Airborne {
				if view.Action == ActionDribbleIdle {
					result.Action, result.ActionEntered = ActionCarrierJump, false
				}
			} else {
				changeMode(s.Plan)
			}
		}
	case AIPursueBall:
		if !s.Entered {
			s.Entered = true
			s.Destination = 0x10
			s.Counter = 0
			s.Plan = AIPursueBall
		}
		if view.Action >= ActionJump {
			return result, nil
		}
		if err := advanceOriginalPursuit(&result, view); err != nil {
			return initial, err
		}
	case AIWaitIdle:
		s.Entered = true
		if view.Action == ActionIdle {
			changeMode(s.Plan)
		}
	case AIMode(32):
		if err := advanceOriginalScoreStrategy(&result, view); err != nil {
			return initial, err
		}
	case AIMode(33):
		if err := advanceOriginalTeamMode33(&result, view); err != nil {
			return initial, err
		}
	case AIMode(34):
		if err := advanceOriginalTeamMode34(&result, view); err != nil {
			return initial, err
		}
	case AIMode(35):
		if err := advanceOriginalTeamMode35(&result, view); err != nil {
			return initial, err
		}
	case AIPause:
		if !s.Entered {
			s.Entered = true
			s.Counter = 8
		}
		s.Counter--
		if s.Counter < 0 {
			changeMode(s.Plan)
		}
	case AIMode(4), AIMode(12):
		if err := advanceOriginalMarking(&result, view); err != nil {
			return initial, err
		}
	case AIMode(5):
		if err := advanceOriginalHomeGuard(&result, view); err != nil {
			return initial, err
		}
	case AIMode(6):
		ctx := view.ShotContext
		ctx.Court = view.CourtID
		if err := advanceOriginalShotPlan(&result, view, ctx); err != nil {
			return initial, err
		}
	case AIMode(31):
		ctx := view.ShotContext
		ctx.Court = view.CourtID
		if err := advanceOriginalLaunchShot(&result, view, ctx); err != nil {
			return initial, err
		}
	case AIMode(7):
		if err := advanceOriginalEngagement(&result, view, AIEngagementContext{TeammateLink: view.TeammateLink, VerticalSpeed: view.ShotContext.VerticalSpeed}); err != nil {
			return initial, err
		}
	case AIMode(23), AIMode(24), AIMode(25):
		if !s.Entered {
			s.Entered = true
			s.Counter = 0
			s.PlanParameter = [3]uint8{4, 5, 7}[s.Mode-23]
		}
		ctx := view.ShotContext
		ctx.Court = view.CourtID
		if err := advanceOriginalShotPlan(&result, view, ctx); err != nil {
			return initial, err
		}
	case AIMode(21), AIMode(22):
		if err := advanceOriginalCommandResume(&result, view); err != nil {
			return initial, err
		}
	case AIMode(29):
		if err := advanceOriginalRiderApproach(&result, view); err != nil {
			return initial, err
		}
	case AIMode(28):
		if err := advanceOriginalTeamSupport(&result, view); err != nil {
			return initial, err
		}
	case AIMode(26):
		if err := advanceOriginalDribbleEvasion(&result, view, AIEvasionContext{Court: view.CourtID}); err != nil {
			return initial, err
		}
	case AIMode(18):
		if err := advanceOriginalJumpShotSetup(&result, view); err != nil {
			return initial, err
		}
	case AIMode(16), AIMode(17):
		if err := advanceOriginalDribbleAttack(&result, view, AIDribbleContext{CourtID: view.CourtID}); err != nil {
			return initial, err
		}
	case AIMode(8):
		if !s.Entered {
			s.Plan = s.Mode
			s.Entered = true
			chooseAIAction(&result, ActionRun)
			var value byte
			result.Random, value = NextRandom(result.Random)
			s.PlanParameter = (value & 31) + 32
			s.Counter = 0
		}
		if err := advanceOriginalForwardPosition(&result, view); err != nil {
			return initial, err
		}
	case AIMode(19), AIMode(39):
		if err := advanceOriginalApproachDelay(&result, view); err != nil {
			return initial, err
		}
	case AIDefend:
		if !s.Entered {
			s.Plan = AIDefend
			s.Entered = true
			chooseAIAction(&result, ActionRun)
			var value byte
			result.Random, value = NextRandom(result.Random)
			s.PlanParameter = (value & 31) + 32
			s.Counter = 0
		}
		if err := advanceOriginalDefence(&result, view); err != nil {
			return initial, err
		}
	case AICollectHoop:
		if !s.Entered {
			s.Entered = true
			s.Counter = 0
		}
		if view.HasBall {
			changeMode(6)
			s.Plan = 4
			return result, nil
		}
		tracked := int(s.TrackedObject) - 7
		if tracked >= 0 && tracked < 6 && view.Hoops[tracked].Owner == int8(view.Actor) {
			if s.Counter == 0 {
				if view.Action >= 0x30 && view.Action < 0x3c {
					s.Counter++
				}
				return result, nil
			}
			chooseAIAction(&result, ActionHoopCarryRun)
			s.Destination = 0x62
			zone, err := originalAIZone(view, view.Actor)
			if err != nil {
				return initial, err
			}
			result.Heading = steerOriginalZone(zone, s.Destination, view.Actor)
			if result.Heading == HeadingNone {
				result.Action, result.ActionEntered = ActionHoopAim, false
				result.Heading = HeadingRight
				if view.Actor >= 2 {
					result.Heading = HeadingLeft
				}
				changeMode(AIRecover)
			}
			return result, nil
		}
		found := -1
		for i, h := range view.Hoops {
			if h.Action == HoopInactive && h.Position.Height>>8 == 0 {
				found = i
				break
			}
		}
		if found < 0 {
			changeMode(4)
			return result, nil
		}
		s.TrackedObject = int8(found + 7)
		if approachOriginalLooseObject(&result, view.Position, view.Hoops[found].Position) && view.Position.Height>>8 == 0 {
			result.Action, result.ActionEntered = ActionPickUpHoop, false
			result.BindHoop = found
		} else {
			s.TrackedObject = -1
		}
	default:
		return initial, fmt.Errorf("original AI strategy mode %d not reconstructed", s.Mode)
	}
	return result, nil
}
