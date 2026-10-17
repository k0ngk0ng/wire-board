package engine

import "fmt"

const (
	ActionCapturedIdle Action = 0x50 + iota
	ActionCapturedWalk
	ActionCapturedRun
	ActionCapturedBrake
	ActionCapturedJump
	ActionCapturedRunningJump
	ActionCapturedLand
)

type CapturedPlayerState struct {
	Player    Player
	ShotCycle int8
}
type CapturedPlayerContext struct {
	Actor                      uint8
	Computer, HasBall          bool
	Attachment, CaptureCounter uint8
}
type CapturedPlayerPreparation struct {
	State               CapturedPlayerState
	Attachment          uint8
	ChangeSpecialHolder bool
	SpecialHolder       int8
	ReleaseHolder       bool
	Ball                BallCommand
	ChangeBallVelocityX bool
	BallVelocityX       Fixed
	SoundIDs            []uint8
}
type CapturedPlayerCompletion struct {
	State                CapturedPlayerState
	Attachment           uint8
	Ball                 BallCommand
	ChangeCaptureCounter bool
	CaptureCounter       uint8
	// Original $958E writes action7 to fixed object4. Its handler clears the
	// special-holder binding later; this command does not remove the container.
	ReleaseContainer bool
}

func capturedAction(a Action) bool { return a >= ActionCapturedIdle && a <= ActionCapturedLand }

// PrepareCapturedPlayer preserves all entry writes before animation/motion.
// In particular, a caught ball is released through request11, not action11.
func PrepareCapturedPlayer(original CapturedPlayerState, ctx CapturedPlayerContext, profile CharacterProfile) (CapturedPlayerPreparation, error) {
	initial := CapturedPlayerPreparation{State: original, Attachment: ctx.Attachment}
	if ctx.Actor >= 4 || !capturedAction(original.Player.Action) || original.Player.Heading > HeadingNone {
		return initial, fmt.Errorf("captured player action/context outside restored chain")
	}
	result := initial
	p := &result.State.Player
	action := p.Action
	if !p.Entered {
		initializeAction(p)
		switch action {
		case ActionCapturedIdle:
			result.ChangeSpecialHolder, result.SpecialHolder = true, int8(ctx.Actor)
			p.Animation.NextAction, p.Animation.HasNextAction = ActionCapturedIdle, true
			if ctx.Computer {
				p.Animation.NextAction = ActionCapturedJump
			}
			if ctx.HasBall {
				result.ReleaseHolder = true
				result.Ball = BallCommand{ChangeRequest: true, Request: BallRequest{Action: BallDropped, Pending: true}}
				result.ChangeBallVelocityX, result.BallVelocityX = true, 2*Pixel
				if p.Pose.Mirrored {
					result.BallVelocityX = -2 * Pixel
				}
			}
		case ActionCapturedRun:
			result.State.ShotCycle = 8
			if p.Heading != HeadingNone {
				setRunHeading(p, profile.RunSpeed)
			}
		case ActionCapturedJump, ActionCapturedRunningJump:
			result.Attachment = 0
			if action == ActionCapturedJump {
				travel := profile.JumpTravel[p.Heading]
				p.Body.Velocity.X, p.Body.Velocity.Depth = travel.X, travel.Depth
			}
			// EA28 is an additive impulse, including the prior fractional velocity.
			p.Body.Velocity.Height = signed16(p.Body.Velocity.Height + 2272)
			p.Body.Gravity = signed16(p.Body.Gravity + 288)
		case ActionCapturedLand:
			p.Animation.NextAction, p.Animation.HasNextAction = ActionCapturedIdle, true
			stopHorizontal(p)
			stopVertical(p)
		}
	}
	if action == ActionCapturedRun {
		p.Body.Velocity.Depth = 0
		if p.Heading != HeadingNone {
			if ctx.Computer {
				setRunHeading(p, profile.RunSpeed)
			}
			setRunningDepth(p)
		}
		if ctx.HasBall {
			if p.Pose.Frame == 7 {
				result.SoundIDs = []uint8{1}
			}
			if !ctx.Computer && p.Animation.Cursor == 0 && p.Animation.Elapsed == 0 {
				result.State.ShotCycle--
				if result.State.ShotCycle == 0 {
					p.Animation.Cursor = 3
				} else if result.State.ShotCycle < 0 {
					result.State.ShotCycle = -1
				}
			}
		}
	}
	if action == ActionCapturedWalk || action == ActionCapturedRun || action == ActionCapturedJump || action == ActionCapturedRunningJump {
		faceHeading(p)
	}
	return result, nil
}

func animateCapturedPlayer(p *Player, action Action) error {
	frame := func(pose, ticks uint8) AnimationFrame {
		return AnimationFrame{Pose: Pose{Frame: pose & 127, Mirrored: pose&128 != 0}, Ticks: ticks}
	}
	spec := AnimationSpec{End: RepeatForever}
	state := p.Animation
	tail := false
	switch action {
	case ActionCapturedIdle:
		spec.Frames = []AnimationFrame{frame(24, 8)}
		spec.End = RepeatRemaining
	case ActionCapturedWalk:
		spec.Frames = []AnimationFrame{frame(25, 2), frame(24, 2), frame(26, 2), frame(24, 2)}
	case ActionCapturedRun:
		spec.Frames = []AnimationFrame{frame(27, 2), frame(24, 2)}
		if state.Cursor >= 3 {
			// Shared run can select cursor3 while holding a ball. Preserve the
			// original ABE8-relative adjacent bytes FF FF 1D 04 FE.
			tail = true
			state.Cursor -= 3
			spec.Frames = []AnimationFrame{frame(255, 255), frame(29, 4)}
			spec.End = RepeatRemaining
		}
	case ActionCapturedBrake:
		spec.Frames = []AnimationFrame{frame(24, 8)}
	case ActionCapturedJump, ActionCapturedRunningJump:
		spec.Frames = []AnimationFrame{frame(28, 255)}
	case ActionCapturedLand:
		spec.Frames = []AnimationFrame{frame(29, 4)}
		spec.End = RepeatRemaining
	}
	step, e := AdvanceAnimation(spec, state, p.FacingLeft)
	if e != nil {
		return e
	}
	if tail && (step.State.Cursor != 0 || step.State.Elapsed != 0) {
		step.State.Cursor += 3
	}
	p.Animation, p.Pose = step.State, step.Pose
	if step.ChangeAction {
		p.Action, p.Entered = step.Action, false
	}
	return nil
}

func CompleteCapturedPlayer(original CapturedPlayerState, ctx CapturedPlayerContext, profile CharacterProfile, court FlatCourt) (CapturedPlayerCompletion, error) {
	initial := CapturedPlayerCompletion{State: original, Attachment: ctx.Attachment, CaptureCounter: ctx.CaptureCounter}
	if ctx.Actor >= 4 || !capturedAction(original.Player.Action) {
		return initial, fmt.Errorf("captured completion outside restored chain")
	}
	result := initial
	p := &result.State.Player
	action := p.Action
	if e := animateCapturedPlayer(p, action); e != nil {
		return initial, e
	}
	switch action {
	case ActionCapturedIdle:
		stopHorizontal(p)
	case ActionCapturedWalk:
		travel := profile.Walk[p.Heading]
		p.Body.Velocity.X, p.Body.Velocity.Depth = travel.X, travel.Depth
	case ActionCapturedBrake:
		brakePlayer(p, profile)
	}
	if court.originalMotion != nil {
		// Keep the transaction's binding identity: court devices write the
		// selected actor's resetJumpPhase here for the outer dispatcher.
		// Only the attachment destination is temporary for this completion.
		binding := court.originalMotion
		previousAttachment := binding.attachment
		binding.attachment = &result.Attachment
		defer func() { binding.attachment = previousAttachment }()
	} else if result.Attachment&128 != 0 {
		return initial, fmt.Errorf("captured attached motion requires original world binding")
	}
	ball, e := advancePlayerOnCourt(p, playerTerrainContext{Computer: ctx.Computer, HasBall: ctx.HasBall}, court)
	if e != nil {
		return initial, e
	}
	result.Ball = ball
	switch action {
	case ActionCapturedRun:
		if p.Airborne {
			p.Action, p.Entered = ActionFalling, false
		}
	case ActionCapturedBrake:
		if p.Body.Velocity.X == 0 {
			p.Action, p.Entered = ActionCapturedIdle, false
		}
	case ActionCapturedJump, ActionCapturedRunningJump:
		if !p.Airborne {
			result.ChangeCaptureCounter = true
			result.CaptureCounter--
			if int8(result.CaptureCounter) < 0 {
				p.Action, p.Entered = ActionLand, false
				result.ReleaseContainer = true
			} else {
				p.Action, p.Entered = ActionCapturedLand, false
				if result.Attachment&128 != 0 {
					p.Action = 0x40
				}
			}
		}
	}
	return result, nil
}
