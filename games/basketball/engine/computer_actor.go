package engine

import "errors"

type ComputerActorState struct {
	Carrier   HoopCarrier    `json:"carrier"`
	Control   AIControlState `json:"control"`
	JumpPhase int8           `json:"jumpPhase,omitempty"`
	Dodge     DodgeMemory    `json:"dodge"`
}

type ComputerActorContext struct {
	HumanTactics    *[2]uint8
	MountFlags      *[6]uint8
	Character       uint8
	PursuitWorld    *WorldActionState
	PlayerControls  *[4]uint8
	PursuitReserved *PursuitReservedBytes
	PursuitAlias74  *PursuitAlias74
	Seat            uint8
	HasBall         bool
	Hoops           [6]Hoop
	Players         [4]Position
	BallHolder      int8
	BallPosition    Position
	Random          RandomState
	BallAction      BallAction
	BallFacingLeft  bool
	BallLanding     Position
	PursuitOptions  *[4]AITacticalOption
	Team            uint8
	TeamHasHuman    bool
	PlayerActions   [4]Action
	Auxiliary       *[2]AuxiliaryState
	SkipJumpWindup  bool
	Difficulty      uint8
	ScoreDifference uint8
	CourtID         uint8
	CachedZones     *[4]CourtZone
	TeammateLink    uint8
}

type ComputerActorStep struct {
	ClearCommandStatus bool
	State              ComputerActorState
	Hoops              [6]Hoop
	Random             RandomState
	Ball               BallCommand
	BindBall           bool
}

// AdvanceComputerActor joins the original AI and one player's action phase.
// It binds pickups and emits ball/hoop actions, but leaves object physics to the
// world phase after all players, preserving the original ordering.
func AdvanceComputerActor(original ComputerActorState, ctx ComputerActorContext, profile CharacterProfile, court FlatCourt) (ComputerActorStep, error) {
	initial := ComputerActorStep{State: original, Hoops: ctx.Hoops, Random: ctx.Random}
	result := initial
	p := &result.State.Carrier.Player
	view := AIControlView{Actor: ctx.Seat, Action: p.Action, ActionEntered: p.Entered, Heading: p.Heading, FacingLeft: p.FacingLeft, Position: p.Body.Position, HasBall: ctx.HasBall}
	view.Players, view.BallHolder, view.BallPosition, view.Random = ctx.Players, ctx.BallHolder, ctx.BallPosition, ctx.Random
	view.BallAction, view.BallFacingLeft, view.BallLanding, view.PursuitOptions = ctx.BallAction, ctx.BallFacingLeft, ctx.BallLanding, ctx.PursuitOptions
	view.Team, view.TeamHasHuman, view.PlayerActions, view.Airborne = ctx.Team, ctx.TeamHasHuman, ctx.PlayerActions, p.Airborne
	view.Auxiliary = ctx.Auxiliary
	view.PursuitAlias74 = ctx.PursuitAlias74
	view.HumanTactics = ctx.HumanTactics
	view.PursuitWorld, view.PlayerControls, view.PursuitReserved = ctx.PursuitWorld, ctx.PlayerControls, ctx.PursuitReserved
	view.Difficulty, view.CourtID, view.TeammateLink = ctx.Difficulty, ctx.CourtID, ctx.TeammateLink
	view.ScoreDifference = ctx.ScoreDifference
	view.CachedZones = ctx.CachedZones
	view.ShotContext = AIShotContext{MountFlags: ctx.MountFlags, JumpVelocity: profile.JumpVelocity, Character: ctx.Character, Court: ctx.CourtID, JumpPhase: original.JumpPhase, VerticalSpeed: int8(p.Body.Velocity.Height >> 8), HorizontalSpeed: int8(p.Body.Velocity.X >> 8), ShotSkill: profile.ShotSkill}
	for i, h := range result.Hoops {
		owner := int8(-1)
		if h.Carried {
			owner = int8(h.CarrierSeat)
		}
		view.Hoops[i] = AIHoopView{Position: h.Position, Action: h.Action, Owner: owner}
	}
	decision, err := StepOriginalAIControl(original.Control, view)
	if err != nil {
		return initial, err
	}
	return advanceComputerActorDecision(original, ctx, decision, profile, court)
}

func advanceComputerActorDecision(original ComputerActorState, ctx ComputerActorContext, decision AIControlDecision, profile CharacterProfile, court FlatCourt) (ComputerActorStep, error) {
	initial := ComputerActorStep{State: original, Hoops: ctx.Hoops, Random: ctx.Random}
	result := initial
	result.State = ApplyOriginalAIActorDecision(original, decision)
	result.ClearCommandStatus = decision.ClearCommandStatus
	p := &result.State.Carrier.Player
	result.Random = decision.Random
	result.Ball, result.BindBall = decision.Ball, decision.BindBall
	if decision.BindHoop >= 0 {
		h := &result.Hoops[decision.BindHoop]
		h.Carried, h.CarrierSeat, h.CanReceive = true, ctx.Seat, false
	}
	held := -1
	for i, h := range result.Hoops {
		if h.Carried && h.CarrierSeat == ctx.Seat {
			held = i
			break
		}
	}
	hasBall := ctx.HasBall || decision.BindBall
	ballAction := ctx.BallAction
	if result.Ball.Change {
		ballAction = result.Ball.Action
	}
	if hasBall {
		result.Ball = mergeBallCommands(result.Ball, ownedBallInputCommand(ballAction, p))
		if result.Ball.Change {
			ballAction = result.Ball.Action
		}
	}
	if p.Action == ActionDodge {
		carrier := Carrier{Player: *p, ShotCycle: result.State.Carrier.ShotCycle}
		step, err := StepCarrierDodge(carrier, result.State.Dodge, CarrierContext{HasBall: hasBall, Computer: true, BallAction: ballAction}, profile, court)
		if err != nil {
			return initial, err
		}
		result.State.Carrier.Player, result.State.Carrier.ShotCycle = step.Carrier.Player, step.Carrier.ShotCycle
		result.State.Dodge = step.Memory
		result.Ball = mergeBallCommands(result.Ball, step.Ball)
		return result, nil
	}
	if p.Action == ActionPickUp && held < 0 || p.Action != ActionPickUp && ordinaryCarrierAction(p.Action) {
		if (p.Action == ActionReceiveBall || p.Action == ActionCatchBall || p.Action == ActionCatchAir) && !hasBall {
			return initial, errors.New("catch action has no ball ownership")
		}
		carrier := Carrier{Player: *p, ShotCycle: result.State.Carrier.ShotCycle}
		for pass := 0; pass < 2; pass++ {
			step, err := StepCarrierAction(carrier, CarrierContext{HasBall: hasBall, BallAction: ballAction, Computer: true, Teammate: ctx.Players[ctx.Seat^1], SkipJumpWindup: ctx.SkipJumpWindup || result.State.JumpPhase < 0}, profile, court)
			if err != nil {
				return initial, err
			}
			carrier = step.Carrier
			if step.BindBall {
				result.BindBall = true
				hasBall = true
			}
			result.Ball = mergeBallCommands(result.Ball, step.Ball)
			if step.Ball.Change {
				ballAction = step.Ball.Action
			}
			if !step.Revisit {
				break
			}
			if pass == 1 {
				return initial, errors.New("unexpected repeated catch action")
			}
		}
		result.State.Carrier.Player, result.State.Carrier.ShotCycle = carrier.Player, carrier.ShotCycle
		return result, nil
	}
	if hasBall && !(p.Action == ActionPickUp && held >= 0) {
		return initial, errors.New("computer ball-possession action integration not reconstructed")
	}
	if freeAction(p.Action) {
		if p.Action == ActionRun && !p.Entered {
			result.State.Carrier.ShotCycle = 8
		}
		var err error
		*p, err = StepFreePlayerWithContext(*p, FreePlayerContext{Computer: true, JumpPhase: result.State.JumpPhase}, profile, court)
		if err != nil {
			return initial, err
		}
		return result, nil
	}
	step, err := StepHoopCarrierAction(result.State.Carrier, HoopCarrierContext{Computer: true, HeldHoop: held}, profile, court)
	if err != nil {
		return initial, err
	}
	result.State.Carrier = step.State
	if step.Hoop.ChangeAction {
		result.Hoops[step.Hoop.Index].Action = step.Hoop.Action
		result.Hoops[step.Hoop.Index].Entered = false
	}
	if step.Hoop.ChangeFacing {
		result.Hoops[step.Hoop.Index].FacingLeft = step.Hoop.FacingLeft
	}
	return result, nil
}
