package engine

import "fmt"

type AuxiliaryCarrierContext struct {
	HasBall            bool
	Support            *RiderSupport
	EnhancementEnabled bool
	Team               uint8
	JumpEnhancement    *uint8
	CarriedPlayer      *int8
	Actor              uint8
	Computer           bool
	PickupFlags        uint8
	PickupFlagsKnown   bool
	Owners             [2]int8
	Attachment         uint8
}
type AuxiliaryCarrierStep struct {
	Ball             BallCommand
	ChangeAttachment bool
	Attachment       uint8
	ChangeAimWindow  bool
	AimWindow        uint8
	ChangeShotCycle  bool
	ShotCycle        int8
	ClearSpecialShot bool
	Revisit          bool
	SoundIDs         []uint8
	Player           Player
	ChangeAuxiliary  bool
	Object           uint8
	Action           uint8
	BindOwner        bool
}

func auxiliaryCarrierAnimation(action Action) (AnimationSpec, error) {
	frame := func(p, t uint8) AnimationFrame { return AnimationFrame{Pose: Pose{Frame: p}, Ticks: t} }
	spec := AnimationSpec{End: RepeatForever}
	switch action {
	case 0x20, 0x23:
		spec.Frames = []AnimationFrame{frame(15, 255)}
	case 0x21:
		spec.Frames = []AnimationFrame{frame(16, 3), frame(15, 2), frame(17, 3), frame(15, 2)}
	case 0x22:
		spec.Frames = []AnimationFrame{frame(18, 2), frame(15, 2)}
	case 0x24, 0x25:
		spec.Frames = []AnimationFrame{frame(7, 1), frame(18, 255)}
	case 0x26:
		spec.Frames = []AnimationFrame{frame(16, 2)}
		spec.End = RepeatRemaining
	case 0x27:
		spec.Frames = []AnimationFrame{frame(43, 2), frame(20, 4)}
		spec.End = RepeatRemaining
	case 0x28:
		spec.Frames = []AnimationFrame{frame(43, 2), frame(20, 4), frame(4, 255)}
		spec.End = RepeatRemaining
	case 0x29:
		spec.Frames = []AnimationFrame{frame(11, 4)}
		spec.End = RepeatRemaining
	case 0x2a:
		spec.Frames = []AnimationFrame{frame(11, 255)}
	case 0x2b:
		spec.Frames = []AnimationFrame{frame(12, 8)}
		spec.End = RepeatRemaining
	case 0x2c:
		spec.Frames = []AnimationFrame{frame(12, 5), frame(4, 255)}
	case 0x4e:
		spec.Frames = []AnimationFrame{frame(5, 2), frame(15, 3)}
		spec.End = RepeatRemaining
	default:
		return spec, fmt.Errorf("auxiliary carrier animation %x not reconstructed", action)
	}
	return spec, nil
}

func advanceAuxiliaryCarrierPlayer(p *Player, ctx AuxiliaryCarrierContext, detached bool, court FlatCourt) (uint8, BallCommand, error) {
	link := ctx.Attachment
	if detached {
		link = 0
	}
	if link&128 != 0 {
		if ctx.Support == nil || ctx.Support.Kind >= 16 {
			return link, BallCommand{}, fmt.Errorf("attached carrier requires original support")
		}
		attachRiderPosition(p, *ctx.Support)
		return link, BallCommand{}, nil
	}
	// 8B12 clears the shared attachment before EEFF. A transaction-local binding
	// must observe that write during motion, not only after this handler returns.
	originalBinding := court.originalMotion
	var binding courtMotionBinding
	if originalBinding != nil {
		binding = *originalBinding
		binding.attachment = &link
		court.originalMotion = &binding
	}
	ball, err := advancePlayerOnCourt(p, playerTerrainContext{Computer: ctx.Computer, HasBall: ctx.HasBall}, court)
	if originalBinding != nil {
		originalBinding.resetJumpPhase = binding.resetJumpPhase
	}
	return link, ball, err
}

// StepAuxiliaryCarrier restores the original prop pickup, ground carry motion,
// immediate held jumps, held falling and A throws. Requests precede handler
// dispatch; object commands are committed before the later auxiliary phase.
func StepAuxiliaryCarrier(original Player, ctx AuxiliaryCarrierContext, profile CharacterProfile, court FlatCourt) (AuxiliaryCarrierStep, error) {
	resolved, err := ResolvePlayerRequest(original)
	if err != nil {
		return AuxiliaryCarrierStep{Player: original}, err
	}
	return stepResolvedAuxiliaryCarrier(resolved, ctx, profile, court)
}

// The world dispatcher has already accepted requests. In particular $9FA7's
// negative role entry jumps straight to $23 without another E18B acceptance.
func stepResolvedAuxiliaryCarrier(original Player, ctx AuxiliaryCarrierContext, profile CharacterProfile, court FlatCourt) (AuxiliaryCarrierStep, error) {
	initial := AuxiliaryCarrierStep{Player: original}
	result := initial
	if ctx.Actor >= 4 || original.Heading > HeadingNone {
		return initial, fmt.Errorf("invalid auxiliary carrier actor")
	}
	p := &result.Player
	action := p.Action
	if action != 0x4e && action != ActionFalling && (action < 0x20 || action > 0x2c) {
		return initial, fmt.Errorf("auxiliary player action %x not reconstructed", action)
	}
	hasProp := ctx.Owners[0] == int8(ctx.Actor) || ctx.Owners[1] == int8(ctx.Actor)
	if action == 0x22 && ctx.HasBall {
		return initial, fmt.Errorf("simultaneous prop/ball run charge is not reconstructed")
	}
	if action == 0x28 || action == 0x2a || action == 0x2c {
		if ctx.CarriedPlayer == nil {
			return initial, fmt.Errorf("air prop throw requires original carried-player reference")
		}
		if *ctx.CarriedPlayer == int8(ctx.Actor) {
			return initial, fmt.Errorf("carried-player air throw is not reconstructed")
		}
	}
	selectedObject := func() (uint8, error) {
		object := uint8(4)
		if ctx.Owners[0] != int8(ctx.Actor) {
			object = 5
		}
		if ctx.Owners[object-4] != int8(ctx.Actor) {
			return 0, fmt.Errorf("A throw has no bound auxiliary")
		}
		return object, nil
	}
	if !p.Entered {
		initializeAction(p)
		switch action {
		case 0x4e:
			if !ctx.Computer && !ctx.PickupFlagsKnown {
				return initial, fmt.Errorf("human pickup requires original contact marks")
			}
			stopHorizontal(p)
			p.Animation.NextAction, p.Animation.HasNextAction = 0x20, true
			object := uint8(4)
			if ctx.Computer {
				if ctx.Owners[0] != int8(ctx.Actor) {
					object = 5
				}
			} else if ctx.PickupFlags&32 != 0 {
				object = 5
			}
			result.ChangeAuxiliary, result.Object, result.Action, result.BindOwner = true, object, 1, true
		case 0x20:
			stopHorizontal(p)
			if !p.Airborne {
				stopVertical(p)
			}
		case 0x22:
			result.ChangeShotCycle, result.ShotCycle = true, 8
			if p.Heading != HeadingNone {
				setRunHeading(p, profile.RunSpeed)
			}
		case 0x24, 0x25:
			result.Revisit = true
			result.SoundIDs = append(result.SoundIDs, 23)
			if action == 0x24 {
				v := profile.JumpTravel[p.Heading]
				p.Body.Velocity.X, p.Body.Velocity.Depth = v.X, v.Depth
			}
			p.Body.Velocity.Height, p.Body.Gravity = profile.JumpVelocity, Pixel
			if ctx.EnhancementEnabled && ctx.Team == 0 {
				if ctx.JumpEnhancement == nil {
					return initial, fmt.Errorf("carry jump requires original equipment source")
				}
				if *ctx.JumpEnhancement == 23 {
					p.Body.Gravity = Pixel / 2
				}
			}
		case 0x26:
			stopHorizontal(p)
			stopVertical(p)
			p.Animation.NextAction, p.Animation.HasNextAction = 0x20, true
			result.ClearSpecialShot = true
		case 0x29:
			p.Animation.NextAction, p.Animation.HasNextAction = 0x2b, true
			result.ChangeAimWindow, result.AimWindow = true, 32
		case 0x2a, 0x2b, 0x2c:
			object, e := selectedObject()
			if e != nil {
				return initial, e
			}
			result.ChangeAuxiliary, result.Object = true, object
			switch action {
			case 0x2a:
				result.Action = 1
				result.ChangeAimWindow, result.AimWindow = true, 32
			case 0x2b:
				result.Action = 2
				p.Animation.NextAction, p.Animation.HasNextAction = 0, true
				if ctx.Attachment&128 != 0 {
					p.Animation.NextAction = ActionRiding
				}
				faceHeading(p)
				p.Body.Velocity.Depth = 0
			case 0x2c:
				result.Action = 15
			}
		case 0x27, 0x28:
			object, e := selectedObject()
			if e != nil {
				return initial, e
			}
			result.ChangeAuxiliary, result.Object, result.Action = true, object, 14
			if action == 0x27 {
				p.Animation.NextAction, p.Animation.HasNextAction = 0, true
				if ctx.Attachment&128 != 0 {
					p.Animation.NextAction = ActionRiding
				}
			}
		case ActionFalling:
			if ctx.HasBall {
				result.Ball = BallCommand{Change: true, Action: BallHeld}
			}
			pose := PoseAirborne
			if hasProp {
				pose = 18
			}
			p.Pose = Pose{Frame: pose, Mirrored: p.FacingLeft}
			p.Body.Velocity.Height = signed16(p.Body.Velocity.Height + 15*Pixel/4)
			p.Body.Gravity = Pixel
		}
	}
	if action == 0x22 {
		if !hasProp {
			p.Request = ActionRequest{Action: ActionRun, Pending: true}
		}
		if p.Heading != HeadingNone && ctx.Computer {
			setRunHeading(p, profile.RunSpeed)
		}
		setRunningDepth(p)
	}
	if action == 0x21 || action == 0x22 || action == 0x24 || action == 0x25 || action == 0x2c {
		faceHeading(p)
	}
	if action != ActionFalling {
		spec, e := auxiliaryCarrierAnimation(action)
		if e != nil {
			return initial, e
		}
		next, e := AdvanceAnimation(spec, p.Animation, p.FacingLeft)
		if e != nil {
			return initial, e
		}
		p.Animation, p.Pose = next.State, next.Pose
		if next.ChangeAction {
			p.Action, p.Entered = next.Action, false
		}
	}
	if action == 0x21 {
		v := profile.Walk[p.Heading]
		p.Body.Velocity.X, p.Body.Velocity.Depth = v.X, v.Depth
	}
	if action == 0x23 {
		brakePlayer(p, profile)
	}
	if action == 0x27 || action == 0x29 || action == 0x2b {
		p.Body.Velocity.X = approachZero(p.Body.Velocity.X, 128)
		if p.Body.Velocity.Depth > 0 {
			p.Body.Velocity.Depth -= 16
		} else if p.Body.Velocity.Depth < 0 {
			p.Body.Velocity.Depth += 16
		}
	}
	if action == 0x2a && !p.Airborne {
		p.Action, p.Entered = 0x26, false
	}
	link, terrainBall, motionErr := advanceAuxiliaryCarrierPlayer(p, ctx, result.Revisit, court)
	if motionErr != nil {
		return initial, motionErr
	}
	result.Ball = mergeBallCommands(result.Ball, terrainBall)
	if result.Revisit || link != ctx.Attachment {
		result.ChangeAttachment, result.Attachment = true, link
	}
	switch action {
	case 0x20:
		if p.Airborne {
			p.Action, p.Entered = ActionFalling, false
			if ctx.Attachment&128 != 0 {
				p.Action = ActionRiding
			}
		}
	case 0x21, 0x22:
		if p.Airborne {
			p.Action, p.Entered = ActionFalling, false
		}
	case 0x23:
		if p.Body.Velocity.X == 0 {
			p.Action, p.Entered = 0x20, false
		}
	case 0x24, 0x25:
		if !hasProp {
			p.Action, p.Entered = ActionFalling, false
		}
		if !p.Airborne {
			p.Action, p.Entered = 0x26, false
		}
	case 0x28:
		if !p.Airborne {
			p.Action, p.Entered = ActionLand, false
		}
	case 0x29:
		if p.Airborne && p.Body.Gravity == 0 {
			p.Body.Velocity.Height = signed16(p.Body.Velocity.Height + 960)
			p.Body.Gravity = signed16(p.Body.Gravity + 480)
		}
	case 0x2c:
		if !p.Airborne {
			p.Action, p.Entered = ActionIdle, false
		}
	case ActionFalling:
		if !p.Airborne {
			p.Action, p.Entered = ActionLand, false
			if hasProp {
				p.Action = 0x26
			}
		}
	}
	return result, nil
}
