package engine

import "errors"

// FallingContext describes ownership at the original single-actor action
// boundary. Prop holders and attached riders need different original poses and
// landing table entries; callers must identify them instead of treating them as
// ordinary free players.
type FallingContext struct {
	Computer, HasBall            bool
	HasOtherHeldObject, Attached bool
}

type FallingStep struct {
	Player Player
	Ball   BallCommand
}

// StepFallingPlayer restores bank 6 $9749 for an ordinary actor, including a
// ball holder. Attached actors require the complete original motion binding,
// which supplies the support body and kind. It executes exactly one action
// call, with no input resolution or repeated world update. An error returns the original actor with no commands.
func StepFallingPlayer(original Player, ctx FallingContext, court FlatCourt) (FallingStep, error) {
	initial := FallingStep{Player: original}
	if original.Action != ActionFalling || ctx.HasOtherHeldObject || ctx.Attached && court.originalMotion == nil {
		return initial, errors.New("falling action or attached/prop-holder context has not been reconstructed")
	}
	p := original
	var ball BallCommand
	if !p.Entered {
		initializeAction(&p)
		p.Pose = Pose{Frame: PoseAirborne, Mirrored: p.FacingLeft}
		p.Body.Velocity.Height = signed16(p.Body.Velocity.Height + 15*Pixel/4)
		p.Body.Gravity = Pixel
		if ctx.HasBall {
			ball = BallCommand{Change: true, Action: BallHeld}
		}
	}
	terrain, err := advancePlayerOnCourt(&p, playerTerrainContext{Computer: ctx.Computer, HasBall: ctx.HasBall}, court)
	if err != nil {
		return initial, err
	}
	ball = mergeBallCommands(ball, terrain)
	if !p.Airborne {
		p.Action, p.Entered = ActionLand, false
		if ctx.HasBall {
			p.Action = ActionCarrierLand
		}
	}
	return FallingStep{Player: p, Ball: ball}, nil
}
