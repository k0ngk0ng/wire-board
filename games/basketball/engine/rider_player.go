package engine

import "fmt"

const ActionRiding Action = 0x3d

// RiderSupport is the already-updated parent in the action scheduler. Its
// animation pose, rather than just its action, determines the standing height.
type RiderSupport struct {
	Body Body
	Pose Pose
	Kind uint8
}

type RiderPlayerState struct {
	Player    Player
	JumpPhase int8
}

type RiderPlayerContext struct {
	Actor, Attachment uint8
	Support           RiderSupport
	HasBall, Computer bool
	AuxiliaryOwners   [2]int8
}

type RiderPlayerStep struct {
	State             RiderPlayerState
	Attachment        uint8
	Ball              BallCommand
	ChangeSupportLink bool
	SupportObject     uint8
}

func animateRider(p *Player, profile CharacterProfile) error {
	if p.Action != ActionRiding {
		return animatePlayer(p, profile, p.Action)
	}
	// Original bank6 animation table AA82[3D] -> AB45: 00 FF FF.
	step, err := AdvanceAnimation(AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 0}, Ticks: 255}}, End: RepeatForever}, p.Animation, p.FacingLeft)
	if err != nil {
		return err
	}
	p.Animation, p.Pose = step.State, step.Pose
	return nil
}

func attachRiderPosition(p *Player, support RiderSupport) {
	heights := [16]uint8{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 16, 40, 8, 8, 13, 16}
	height := heights[support.Kind]
	if support.Kind < 8 {
		pose := support.Pose.Frame
		if pose == 5 || pose >= 0x47 && pose < 0x4b {
			height = 24
		} else if pose == 0x19 || pose >= 0x4b {
			height = 16
		}
	}
	// EEFF's attached path copies only integer bytes. Each rider retains its
	// own fractions; no velocity/gravity integration runs on this path.
	p.Body.Position.X = signed24((support.Body.Position.X &^ 255) | (p.Body.Position.X & 255))
	p.Body.Position.Depth = (support.Body.Position.Depth &^ 255) | (p.Body.Position.Depth & 255)
	p.Body.Position.Height = signed24(((support.Body.Position.Height &^ 255) + Fixed(height)*Pixel) | (p.Body.Position.Height & 255))
}

// StepRiderPlayer restores bank6 9B45 and its attached EEFF branch. Landing on
// a parent is decided earlier by the rider-link phase; this does not invent a
// collision, run the parent again or advance the ball's physical position.
func StepRiderPlayer(original RiderPlayerState, ctx RiderPlayerContext, profile CharacterProfile, court FlatCourt) (RiderPlayerStep, error) {
	initial := RiderPlayerStep{State: original, Attachment: ctx.Attachment}
	parent := ctx.Attachment & 15
	if original.Player.Action != ActionRiding || ctx.Actor >= 4 || parent >= 13 || ctx.Support.Kind >= 16 {
		return initial, fmt.Errorf("rider action or support context not reconstructed")
	}
	if request := original.Player.Request; request.Pending {
		if request.Action >= 106 {
			return initial, fmt.Errorf("rider request %d is outside the original table", request.Action)
		}
		// Fixed E18B applies the original E208 acceptance masks before the
		// action handler. These accepted transitions require a different
		// mounted handler; retaining 3D would silently discard player input.
		switch request.Action {
		case 4, 7, 9, 0x14, 0x17, 0x19, 0x24, 0x27, 0x29, 0x34, 0x37, 0x39,
			0x3e, 0x3f, 0x40, 0x41, 0x42, 0x43, 0x44, 0x45, 0x5d, 0x60, 0x64:
			return initial, fmt.Errorf("mounted transition from riding to %d not reconstructed", request.Action)
		}
	}
	result := initial
	p := &result.State.Player
	if !p.Entered {
		initializeAction(p)
		p.Body.Velocity, p.Body.Gravity = ctx.Support.Body.Velocity, ctx.Support.Body.Gravity
		if ctx.HasBall {
			result.State.JumpPhase = 0
			result.Ball = BallCommand{Change: true, Action: BallHeld}
		}
	}
	faceHeading(p)
	if err := animateRider(p, profile); err != nil {
		return initial, err
	}
	if ctx.AuxiliaryOwners[0] == int8(ctx.Actor) || ctx.AuxiliaryOwners[1] == int8(ctx.Actor) {
		p.Pose = Pose{Frame: 15, Mirrored: p.FacingLeft}
	}
	if parent < 4 {
		result.ChangeSupportLink, result.SupportObject = true, parent
	}
	if ctx.Attachment&128 != 0 && p.Airborne {
		attachRiderPosition(p, ctx.Support)
		return result, nil
	}
	// 9B90 jumps directly to the idle handler with the request's action ID.
	// It resets animation once more and clears planar movement before the
	// ordinary terrain step, even when the request still names Riding.
	p.Action, p.Entered = p.Request.Action, false
	result.Attachment = 0
	initializeAction(p)
	stopHorizontal(p)
	if !p.Airborne {
		stopVertical(p)
	}
	if err := animateRider(p, profile); err != nil {
		return initial, err
	}
	if court.originalMotion != nil {
		court.originalMotion.attachment = &result.Attachment
	}
	ball, err := advancePlayerOnCourt(p, playerTerrainContext{Computer: ctx.Computer, HasBall: ctx.HasBall}, court)
	if err != nil {
		return initial, err
	}
	result.Ball = mergeBallCommands(result.Ball, ball)
	if p.Airborne {
		p.Action, p.Entered = ActionFalling, false
	}
	return result, nil
}
