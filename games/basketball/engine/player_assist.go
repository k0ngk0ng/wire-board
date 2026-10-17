package engine

import "fmt"

type PlayerAssistState struct {
	Player      Player
	LinkedActor int8
}
type PlayerAssistContext struct {
	Computer, HasBall, CarriedBody bool
	Attachment                     uint8
}
type PlayerAssistStep struct {
	State                PlayerAssistState
	Ball                 BallCommand
	ChangeTarget         bool
	Target               uint8
	TargetAction         Action
	ChangeFirstEndurance bool
	FirstEndurance       uint8
}

// StepPlayerAssist restores the ordinary teammate-launch windup and release.
// Contact and the launched teammate's subsequent3E action remain separate.
func preparePlayerAssist(original PlayerAssistState, ctx PlayerAssistContext) (PlayerAssistStep, error) {
	initial := PlayerAssistStep{State: original}
	result := initial
	p := &result.State.Player
	if p.Action != 9 && p.Action != 11 {
		return initial, fmt.Errorf("original player assist action not reconstructed")
	}
	action := p.Action
	if !p.Entered {
		if action == 11 && (original.LinkedActor < -1 || original.LinkedActor >= 4) {
			return initial, fmt.Errorf("original player assist requires linked teammate")
		}
		initializeAction(p)
		p.Animation.NextAction, p.Animation.HasNextAction = 11, true
		if action == 11 {
			p.Animation.NextAction = ActionIdle
			if ctx.Attachment&128 != 0 {
				p.Animation.NextAction = ActionRiding
			}
			faceHeading(p)
			p.Body.Velocity.Depth = 0
			if original.LinkedActor == -1 {
				// STA $0512,Y with Y=FF writes actor0 endurance at $0611.
				result.ChangeFirstEndurance, result.FirstEndurance = true, 0x3e
			} else {
				result.ChangeTarget, result.Target, result.TargetAction = true, uint8(original.LinkedActor), ActionLaunched
			}
			result.State.LinkedActor = -1
		}
	}
	frame, ticks := uint8(11), uint8(4)
	if action == 11 {
		frame, ticks = 12, 8
	}
	anim, e := AdvanceAnimation(AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: frame}, Ticks: ticks}}, End: RepeatRemaining}, p.Animation, p.FacingLeft)
	if e != nil {
		return initial, e
	}
	p.Animation, p.Pose = anim.State, anim.Pose
	if anim.ChangeAction {
		p.Action, p.Entered = anim.Action, false
	}
	// Original EA5C parameter2: horizontal motion saturates, depth may cross0.
	if p.Body.Velocity.X > 0 {
		p.Body.Velocity.X = max(0, p.Body.Velocity.X-128)
	} else if p.Body.Velocity.X < 0 {
		p.Body.Velocity.X = min(0, p.Body.Velocity.X+128)
	}
	if p.Body.Velocity.Depth > 0 {
		p.Body.Velocity.Depth = signed16(p.Body.Velocity.Depth - 16)
	} else if p.Body.Velocity.Depth < 0 {
		p.Body.Velocity.Depth = signed16(p.Body.Velocity.Depth + 16)
	}
	return result, nil
}

func StepPlayerAssist(original PlayerAssistState, ctx PlayerAssistContext, court FlatCourt) (PlayerAssistStep, error) {
	initial := PlayerAssistStep{State: original}
	if original.Player.Action == 9 && ctx.CarriedBody {
		return initial, fmt.Errorf("original carried-body assist requires paired motion")
	}
	result, e := preparePlayerAssist(original, ctx)
	if e != nil {
		return initial, e
	}
	p := &result.State.Player
	result.Ball, e = advancePlayerOnCourt(p, playerTerrainContext{Computer: ctx.Computer, HasBall: ctx.HasBall}, court)
	if e != nil {
		return initial, e
	}
	if original.Player.Action == 9 && p.Airborne && p.Body.Gravity == 0 {
		p.Body.Velocity.Height = signed16(p.Body.Velocity.Height + 0x3c0)
		p.Body.Gravity = 0x1e0
	}
	return result, nil
}
