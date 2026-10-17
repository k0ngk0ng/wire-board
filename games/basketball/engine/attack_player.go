package engine

import "fmt"

type AttackState struct {
	Player        Player
	AttackVariant uint8
	JumpPhase     int8
}
type AttackContext struct {
	Team, Character    uint8
	Computer, HasBall  bool
	Attachment         uint8
	EnhancementEnabled bool
	Enhancement        uint8
}
type AttackStep struct {
	State    AttackState
	Revisit  bool
	Ball     BallCommand
	SoundIDs []uint8
}

// StepPlayerAttack executes one original attack handler. Variant selection,
// animation and movement are independent from the subsequent contact resolver.
func StepPlayerAttack(original AttackState, ctx AttackContext, profile CharacterProfile, court FlatCourt) (AttackStep, error) {
	initial := AttackStep{State: original}
	result := initial
	fail := func(message string) (AttackStep, error) { return initial, fmt.Errorf("original attack: %s", message) }
	p := &result.State.Player
	action := p.Action
	if action < 0x60 || action > 0x68 || ctx.Team >= 8 || ctx.Character >= 4 || p.Heading > HeadingNone {
		return fail("action or character profile outside recovered table")
	}
	if ctx.Attachment&128 != 0 {
		return fail("attached-player attack physics is not reconstructed")
	}
	role := int(ctx.Team)*4 + int(ctx.Character)
	kind := int(action & 15)
	selected := originalAttackProfiles[kind][role]
	if ctx.Team == 0 && ctx.EnhancementEnabled && ctx.Enhancement == 14 && (action == 0x60 || action == 0x64) {
		override := 0
		if action == 0x60 {
			override = 1
		}
		selected = originalAttackProfiles[kind][override]
	}
	switch selected.Handler {
	case 0, 2, 3, 4, 5, 6, 11, 15, 16, 19, 24, 29, 30:
	default:
		return fail(fmt.Sprintf("handler %d is not reconstructed", selected.Handler))
	}
	enteredNow := !p.Entered
	if enteredNow {
		result.State.AttackVariant = selected.Variant
	}
	spec, ok := originalAttackAnimations[result.State.AttackVariant]
	if !ok {
		return fail("animation variant missing")
	}
	next := func(a Action) { p.Animation.NextAction, p.Animation.HasNextAction = a, true }
	refreshRunSpeed := func() {
		if p.Heading == HeadingUp || p.Heading == HeadingDown {
			return
		}
		p.Body.Velocity.X = profile.RunSpeed
		if p.Heading >= HeadingDownLeft {
			p.Body.Velocity.X = -profile.RunSpeed
		}
	}
	if enteredNow {
		initializeAction(p)
		switch selected.Handler {
		case 0, 5:
			if selected.Handler == 5 {
				faceHeading(p)
			}
			if selected.Handler == 0 && ctx.Computer && (action == 0x62 || action == 0x66) {
				refreshRunSpeed()
			}
			p.Body.Velocity.Depth = 0
			next(ActionIdle)
		case 24:
			p.Body.Velocity.Depth = 0
			next(ActionIdle)
			if ctx.Computer {
				refreshRunSpeed()
			}
		case 2:
			// $A618 replaces only gravity's integer byte; preserve its fraction.
			p.Body.Gravity = Pixel + (p.Body.Gravity & 255)
			next(action)
		case 6:
			// $A76A retains the current velocities/gravity and repeats this
			// attack until the post-motion airborne check selects landing.
			next(action)
		case 11:
			result.State.JumpPhase = -32
			next(ActionJump)
			setAttackHorizontalSpeed(p, 2*Pixel, p.Pose.Mirrored)
			// $E9DA parameter5 adds these values, retaining fractional input.
			p.Body.Velocity.Height = signed16(p.Body.Velocity.Height + 0x8e0)
			p.Body.Gravity = signed16(p.Body.Gravity + 0x140)
		case 3:
			faceHeading(p)
			result.State.JumpPhase = -32
			next(ActionJump)
			speed := Fixed(6 * Pixel)
			if result.State.AttackVariant == 0x31 {
				speed = 8 * Pixel
			}
			if p.FacingLeft {
				speed = -speed
			}
			p.Body.Velocity.X = speed
			p.Body.Velocity.Height = signed16(p.Body.Velocity.Height + 0x960)
			p.Body.Gravity = signed16(p.Body.Gravity + 0x160)
		case 4:
			result.Revisit = true
			next(ActionFalling)
			stopVertical(p)
			p.Body.Velocity.Height = 6 * Pixel
			p.Body.Gravity = -Pixel
			v := result.State.AttackVariant
			if v != 2 && v != 0x1d && v != 0x4e {
				p.Body.Velocity.Height = 12 * Pixel
				p.Body.Gravity = Pixel
			}
		case 15:
			p.Body.Velocity.Depth = 0
			next(ActionLand)
			faceHeading(p)
			p.Body.Velocity.X = 6 * Pixel
			// $A8D7 uses the previously displayed pose sign, not the updated facing.
			if p.Pose.Mirrored {
				p.Body.Velocity.X = -6 * Pixel
			}
		case 19:
			stopVertical(p)
			next(ActionFalling)
			p.Body.Velocity.Depth = 0
		case 16:
			stopHorizontal(p)
			p.Body.Velocity.Height = 0x480
			next(ActionFalling)
			switch ctx.Team {
			case 4:
				result.State.JumpPhase = -32
				next(ActionJump)
			case 6:
				stopVertical(p)
				// $A936 writes ten into the fractional byte, not ten pixels.
				p.Body.Velocity.Height = 10
			}
			if ctx.Team == 4 || ctx.Team == 5 || ctx.Team == 6 {
				setAttackHorizontalSpeed(p, 4*Pixel, p.Pose.Mirrored)
			}
		case 29:
			faceHeading(p)
			setAttackHorizontalSpeed(p, 5*Pixel, p.FacingLeft)
			p.Body.Velocity.Height, p.Body.Gravity = 10*Pixel, 2*Pixel
		case 30:
			faceHeading(p)
			stopHorizontal(p)
			setAttackHorizontalSpeed(p, 8*Pixel, p.FacingLeft)
			p.Body.Velocity.Height, p.Body.Gravity = 2*Pixel, 0
		}
	}
	if !enteredNow {
		switch selected.Handler {
		case 29:
			if p.Animation.Cursor == 2 {
				setAttackHorizontalSpeed(p, -5*Pixel, p.FacingLeft)
				p.Body.Velocity.Height, p.Body.Gravity = 4*Pixel, Pixel
			}
		case 30:
			switch p.Animation.Cursor {
			case 1:
				p.Body.Velocity.Height, p.Body.Velocity.X = 7*Pixel, 0
			case 2:
				p.Body.Gravity = Pixel
			}
		}
	}
	if enteredNow || selected.Handler == 19 {
		result.SoundIDs = append(result.SoundIDs, originalAttackSounds[kind][role])
	}
	if selected.Handler == 3 && !enteredNow && !p.Airborne {
		p.Action, p.Entered = ActionLand, false
	}
	// $A68F's entry jumps directly to motion; its first pose is retained.
	if selected.Handler != 29 || !enteredNow {
		animation, err := AdvanceAnimation(spec, p.Animation, p.FacingLeft)
		if err != nil {
			return initial, err
		}
		p.Animation, p.Pose = animation.State, animation.Pose
		if animation.ChangeAction {
			p.Action, p.Entered = animation.Action, false
		}
	}
	if selected.Handler == 0 || selected.Handler == 5 {
		if p.Body.Velocity.X > 0 {
			p.Body.Velocity.X = max(0, p.Body.Velocity.X-Pixel/2)
		} else if p.Body.Velocity.X < 0 {
			p.Body.Velocity.X = min(0, p.Body.Velocity.X+Pixel/2)
		}
	}
	if selected.Handler == 15 && p.Action == ActionLand && !p.Entered && p.Airborne {
		p.Action = ActionFalling
	}
	if selected.Handler == 19 && !p.Airborne {
		p.Action, p.Entered = ActionLand, false
	}
	if selected.Handler == 29 && !enteredNow && !p.Airborne {
		p.Action, p.Entered = ActionLand, false
	}
	var err error
	result.Ball, err = advancePlayerOnCourt(p, playerTerrainContext{Computer: ctx.Computer, HasBall: ctx.HasBall}, court)
	if err != nil {
		return initial, err
	}
	if (selected.Handler == 2 || selected.Handler == 6 || selected.Handler == 11 || selected.Handler == 30) && !p.Airborne {
		p.Action, p.Entered = ActionLand, false
	}
	if selected.Handler == 4 {
		if !p.Airborne {
			p.Action, p.Entered = ActionLand, false
		} else if p.Action == ActionFalling && !p.Entered {
			stopVertical(p)
			p.Body.Gravity = Pixel
		}
	}
	return result, nil
}

func setAttackHorizontalSpeed(p *Player, speed Fixed, mirrored bool) {
	if mirrored {
		speed = -speed
	}
	p.Body.Velocity.X = speed
}
