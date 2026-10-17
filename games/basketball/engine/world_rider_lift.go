package engine

import "fmt"

// ActionRiderLift is bank6 $92F9: the support crouches, then raises the
// linked rider into action3F. It is not an ordinary jump or a basket throw.
const ActionRiderLift Action = 7

func advanceWorldRiderLift(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Actors[actor]
	p := &a.Player
	if !p.Entered {
		if s.Holder < 0 || s.Holder >= 13 {
			return fmt.Errorf("rider lift holder %d outside original object table", s.Holder)
		}
		initializeAction(p)
		stopHorizontal(p)
		target := worldLoopPositions(*s)[s.Holder]
		p.FacingLeft = (uint16(p.Body.Position.X>>8)-uint16(target.X>>8))&0x8000 == 0
		p.Animation.NextAction, p.Animation.HasNextAction = ActionIdle, true
		if s.PlayerLinks[actor]&128 != 0 {
			p.Animation.NextAction = ActionRiding
		}
	}
	if p.Animation.Cursor == 1 {
		target := a.LinkedActor
		switch {
		case target >= 0 && target < 4:
			s.Actors[target].Player.Action, s.Actors[target].Player.Entered = 0x3f, false
		case target == -1:
			// STA $0512,Y with Y=FF writes $0611: original actor0 endurance.
			// This is a real write even when no teammate is currently linked.
			s.Actors[0].Endurance = 0x3f
		default:
			return fmt.Errorf("rider lift target %d outside restored link values", target)
		}
		a.LinkedActor = -1
	}
	spec := AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 43}, Ticks: 4}, {Pose: Pose{Frame: 20}, Ticks: 1}, {Pose: Pose{Frame: 20}, Ticks: 6}}, End: RepeatRemaining}
	animation, err := AdvanceAnimation(spec, p.Animation, p.FacingLeft)
	if err != nil {
		return err
	}
	p.Animation, p.Pose = animation.State, animation.Pose
	if animation.ChangeAction {
		p.Action, p.Entered = animation.Action, false
	}
	ball, err := advancePlayerOnCourt(p, playerTerrainContext{Computer: c.Controls[actor]&128 != 0, HasBall: s.Holder == int8(actor)}, court)
	if err != nil {
		return err
	}
	s.Ball = ApplyBallCommand(s.Ball, ball)
	return nil
}
