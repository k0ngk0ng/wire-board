package engine

// StepOwnedBallInput restores bank 6 $8467. It runs after all player inputs/AI
// and before player action physics. A nil owner leaves a free ball untouched;
// the owner's current action, not its pending action request, selects the path.
func StepOwnedBallInput(ball Ball, owner *Player) Ball {
	return ApplyBallCommand(ball, ownedBallInputCommand(ball.Action, owner))
}

func ownedBallInputCommand(action BallAction, owner *Player) BallCommand {
	if owner == nil {
		return BallCommand{}
	}
	if owner.Action >= ActionDribbleIdle && owner.Action <= ActionDribbleBrake {
		if action != BallDribble {
			return BallCommand{Change: true, Action: BallDribble}
		}
		return BallCommand{}
	}
	// The original stores byte zero, which is an unprocessed rest request.
	return BallCommand{ChangeRequest: true, Request: BallRequest{Action: BallRest, Pending: true}}
}

// ApplyBallCommand applies independently emitted action and request writes.
// A command never implicitly clears an unrelated request or animation state.
func ApplyBallCommand(ball Ball, command BallCommand) Ball {
	if command.Change {
		ball.Action, ball.Entered = command.Action, false
	}
	if command.ChangeRequest {
		ball.Request = command.Request
	}
	return ball
}

func mergeBallCommands(previous, next BallCommand) BallCommand {
	if next.Change {
		previous.Change, previous.Action = true, next.Action
	}
	if next.ChangeRequest {
		previous.ChangeRequest, previous.Request = true, next.Request
	}
	return previous
}
