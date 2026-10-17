package engine

import "fmt"

// AuxiliarySetDownCommand preserves the two different writes of 9BA4: detach
// and restart held action 1, then later set idle and clear only the owner.
type AuxiliarySetDownCommand struct {
	Object          uint8
	Action          uint8
	ClearAttachment bool
	ReleaseOwner    bool
}
type AuxiliarySetDownStep struct {
	Player           Player
	Command          *AuxiliarySetDownCommand
	Ball             BallCommand
	ChangeAttachment bool
	Attachment       uint8
}

// PrepareAuxiliarySetDown restores 9BA4 through the command boundary before
// EEFF. The caller must apply Command to the authoritative auxiliary object
// before running Complete, because court devices observe that ownership.
func PrepareAuxiliarySetDown(original Player, ctx AuxiliaryCarrierContext) (AuxiliarySetDownStep, error) {
	initial := AuxiliarySetDownStep{Player: original}
	result := initial
	if ctx.Actor >= 4 {
		return initial, fmt.Errorf("set-down actor outside original seats")
	}
	p, err := ResolvePlayerRequest(original)
	if err != nil {
		return initial, err
	}
	if p.Action != 0x4f {
		return initial, fmt.Errorf("player is not in original set-down action")
	}
	if !p.Entered {
		initializeAction(&p)
		p.Animation.NextAction, p.Animation.HasNextAction = ActionIdle, true
		stopHorizontal(&p)
	}
	animation, err := AdvanceAnimation(AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 15}, Ticks: 3}, {Pose: Pose{Frame: 5}, Ticks: 4}}, End: RepeatRemaining}, p.Animation, p.FacingLeft)
	if err != nil {
		return initial, err
	}
	p.Animation, p.Pose = animation.State, animation.Pose
	if animation.ChangeAction {
		p.Action, p.Entered = animation.Action, false
	}
	if p.Animation.Cursor != 0 && (p.Animation.Elapsed == 0 || p.Animation.Elapsed == 2) {
		object := uint8(4)
		if ctx.Owners[0] != int8(ctx.Actor) {
			object = 5
		}
		command := AuxiliarySetDownCommand{Object: object}
		if p.Animation.Elapsed == 0 {
			command.Action, command.ClearAttachment = 1, true
		} else {
			command.Action, command.ReleaseOwner = 0, true
		}
		result.Command = &command
	}
	result.Player = p
	return result, nil
}

func CompleteAuxiliarySetDown(prepared AuxiliarySetDownStep, ctx AuxiliaryCarrierContext, court FlatCourt) (AuxiliarySetDownStep, error) {
	result := prepared
	link, ball, err := advanceAuxiliaryCarrierPlayer(&result.Player, ctx, false, court)
	if err != nil {
		return prepared, err
	}
	result.Ball = ball
	if link != ctx.Attachment {
		result.ChangeAttachment, result.Attachment = true, link
	}
	return result, nil
}
