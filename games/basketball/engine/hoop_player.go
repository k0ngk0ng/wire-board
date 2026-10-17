package engine

import "errors"

const (
	ActionHoopCarryIdle Action = 0x30
	ActionHoopCarryRun  Action = 0x32
	ActionHoopAim       Action = 0x39
	ActionHoopThrow     Action = 0x3b
	ActionPickUp        Action = 0x57
	ActionPickUpHoop           = ActionPickUp
)

type HoopCarrier struct {
	Player    Player `json:"player"`
	ShotCycle int8   `json:"shotCycle"`
	AimWindow uint8  `json:"aimWindow"`
}

type HoopCarrierContext struct {
	Computer bool
	HeldHoop int
}

type HoopCommand struct {
	Index        int
	Action       HoopAction
	ChangeAction bool
	FacingLeft   bool
	ChangeFacing bool
}

type HoopCarrierStep struct {
	State HoopCarrier
	Hoop  HoopCommand
}

func RunSpeedForGrade(grade uint8) (Fixed, error) {
	if grade > 7 {
		return 0, errors.New("run grade outside original attribute table")
	}
	return 4*Pixel + Fixed(grade)*64, nil
}

func JumpVelocityForGrade(grade uint8) (Fixed, error) {
	if grade > 7 {
		return 0, errors.New("jump grade outside original attribute table")
	}
	return 7*Pixel + Fixed(grade)*128, nil
}

func setRunHeading(p *Player, speed Fixed) {
	switch p.Heading {
	case HeadingUpRight, HeadingRight, HeadingDownRight:
		p.Body.Velocity.X = speed
	case HeadingDownLeft, HeadingLeft, HeadingUpLeft:
		p.Body.Velocity.X = -speed
	}
}

// StepHoopCarrierAction restores the observed unattached, ground-level hoop
// pickup/carry/run/aim/throw phases. Pickup eligibility and ownership binding
// precede this phase; it cannot assign a distant hoop or invent an AI decision.
func StepHoopCarrierAction(original HoopCarrier, ctx HoopCarrierContext, profile CharacterProfile, court FlatCourt) (HoopCarrierStep, error) {
	result := HoopCarrierStep{State: original}
	fail := func(message string) (HoopCarrierStep, error) {
		return HoopCarrierStep{State: original}, errors.New(message)
	}
	p := &result.State.Player
	if p.Heading > HeadingNone {
		return fail("invalid hoop-carrier heading")
	}
	// E18B dispatches accepted requests before entering the selected handler.
	// AI can select pickup while a prior CatchBall request remains rejected;
	// preserve that pending request rather than treating it as an interruption.
	resolved, err := ResolvePlayerRequest(*p)
	if err != nil {
		return HoopCarrierStep{State: original}, err
	}
	*p = resolved
	action := p.Action
	switch action {
	case ActionHoopCarryIdle, ActionHoopCarryRun, ActionHoopAim, ActionHoopThrow, ActionPickUpHoop:
	default:
		return fail("hoop-carrier action not reconstructed")
	}
	if !p.Entered {
		initializeAction(p)
		switch action {
		case ActionHoopCarryIdle:
			stopHorizontal(p)
			if !p.Airborne {
				stopVertical(p)
			}
		case ActionPickUpHoop:
			if ctx.HeldHoop < 0 || ctx.HeldHoop >= 6 {
				return fail("hoop pickup has no prior ownership binding")
			}
			stopHorizontal(p)
			p.Animation.NextAction, p.Animation.HasNextAction = ActionHoopCarryIdle, true
			result.Hoop = HoopCommand{Index: ctx.HeldHoop, Action: HoopCarried, ChangeAction: true}
		case ActionHoopCarryRun:
			result.State.ShotCycle = 8
			setRunHeading(p, profile.RunSpeed)
		case ActionHoopAim:
			p.Animation.NextAction, p.Animation.HasNextAction = ActionHoopAim, true
			if ctx.Computer {
				p.Animation.NextAction = ActionHoopThrow
			}
			result.State.AimWindow = 32
		case ActionHoopThrow:
			if ctx.HeldHoop < 0 || ctx.HeldHoop >= 6 {
				return fail("hoop throw has no prior owner binding")
			}
			p.Animation.NextAction, p.Animation.HasNextAction = ActionIdle, true
			faceHeading(p)
			p.Body.Velocity.Depth = 0
			result.Hoop = HoopCommand{Index: ctx.HeldHoop, Action: HoopThrown, ChangeAction: true, FacingLeft: p.FacingLeft, ChangeFacing: true}
		}
	}
	if action == ActionHoopCarryRun {
		p.Body.Velocity.Depth = 0
		if p.Heading != HeadingNone {
			if ctx.Computer {
				setRunHeading(p, profile.RunSpeed)
			}
			switch p.Heading {
			case HeadingUp, HeadingUpRight, HeadingUpLeft:
				p.Body.Velocity.Depth = 2 * Pixel
			case HeadingDown, HeadingDownRight, HeadingDownLeft:
				p.Body.Velocity.Depth = -2 * Pixel
			}
		}
		faceHeading(p)
	}
	if err := animatePlayer(p, profile, action); err != nil {
		return HoopCarrierStep{State: original}, err
	}
	if action == ActionHoopAim || action == ActionHoopThrow {
		p.Body.Velocity.X = approachZero(p.Body.Velocity.X, 128)
		if p.Body.Velocity.Depth > 0 {
			p.Body.Velocity.Depth -= 16
		} else if p.Body.Velocity.Depth < 0 {
			p.Body.Velocity.Depth += 16
		}
	}
	if err := advanceOnFlatCourt(p, court); err != nil {
		return HoopCarrierStep{State: original}, err
	}
	if p.Airborne {
		return fail("airborne hoop-carrier action not reconstructed")
	}
	return result, nil
}
