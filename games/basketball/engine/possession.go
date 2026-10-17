package engine

import "errors"

const (
	ActionDribbleIdle Action = 0x10 + iota
	ActionDribbleWalk
	ActionDribbleRun
	ActionDribbleBrake
	ActionCarrierJump
	ActionCarrierRunningJump
	ActionCarrierLand
	ActionPass
	ActionAirPass
	ActionAimShot
	ActionAimAirShot
	ActionShoot
	ActionAirShoot
	ActionCatchBall
	ActionReceiveBall
	ActionCatchAir
)

// Carrier adds the original shot-style cycle to ordinary player movement.
// A negative cycle means no special shot is selected.
type Carrier struct {
	Player    Player `json:"player"`
	ShotCycle int8   `json:"shotCycle"`
}

type CarrierContext struct {
	HasBall        bool
	Teammate       Position
	BallAction     BallAction
	Computer       bool
	SkipJumpWindup bool
	DunkSearch     DunkSearchContext
	CarryingPlayer bool
	ShotRelease    ShotReleaseContext
	// Non-nil only for restored mounted actions. The parent has already run
	// in the original dependency order; EEFF copies its integer coordinates.
	AttachedSupport *RiderSupport
}

type BallCommand struct {
	Change        bool
	Action        BallAction
	ChangeRequest bool
	Request       BallRequest
}

type CarrierStep struct {
	Carrier Carrier
	Ball    BallCommand
	// The original world updates this player once more when takeoff begins.
	Revisit           bool
	BindBall          bool
	ChangeAimWindow   bool
	AimWindow         uint8
	ChangeDunkTarget  bool
	DunkTargetObject  uint8
	ChangeSpecialShot bool
	SpecialShot       uint8
	ShotReleased      bool
	ChangeRandom      bool
	Random            RandomState
}

func carrierTransition(from, to Action) bool {
	switch to {
	case ActionDribbleRun:
		return from == ActionDribbleIdle || from == ActionDribbleWalk || from == ActionDribbleBrake
	case ActionDribbleBrake:
		return from == ActionJump || from == ActionDribbleRun
	case ActionCarrierRunningJump:
		return from == ActionDribbleRun
	case ActionDribbleIdle:
		return from == ActionIdle || from == ActionDribbleWalk
	case ActionDribbleWalk:
		return from == ActionIdle || from == ActionDribbleIdle || from == ActionCatchBall || from == ActionReceiveBall
	case ActionCarrierJump:
		return from == ActionDribbleIdle || from == ActionDribbleWalk || from == ActionDribbleBrake || from == ActionCarrierJump || from == ActionCarrierRunningJump || from == ActionCatchBall || from == ActionReceiveBall || from == ActionCatchAir
	case ActionPass, ActionAimShot:
		return from >= ActionDribbleIdle && from <= ActionDribbleBrake || from == ActionCatchBall
	case ActionAirPass, ActionAimAirShot:
		return from == ActionCarrierJump || from == ActionCarrierRunningJump || from == ActionCatchAir
	case ActionShoot:
		return from == ActionAimShot
	case ActionAirShoot:
		return from == ActionAimAirShot
	case ActionCatchBall:
		return from == ActionDribbleWalk || from >= 0x70 && from <= 0x73
	case ActionCatchAir:
		return from == ActionJump || from == ActionRunningJump || from == 0x70
	}
	return false
}

func ordinaryCarrierAction(a Action) bool {
	switch a {
	case ActionDribbleIdle, ActionDribbleWalk, ActionDribbleRun, ActionDribbleBrake, ActionCarrierJump, ActionCarrierRunningJump, ActionCarrierLand, ActionPass, ActionAirPass, ActionAimShot, ActionAimAirShot, ActionShoot, ActionAirShoot, ActionCatchBall, ActionReceiveBall, ActionCatchAir, ActionPickUp:
		return true
	}
	return false
}

// StepCarrierAction is one player action call, before the ball's own update.
// Its context excludes contact combat, linked riders, basket contact and special
// shots. Court motion retains the restored New York passage transitions.
// Effects describe the ball action; they never fabricate a score or trajectory.
func StepCarrierAction(original Carrier, ctx CarrierContext, profile CharacterProfile, court FlatCourt) (CarrierStep, error) {
	result := CarrierStep{Carrier: original}
	p := &result.Carrier.Player
	fail := func(message string) (CarrierStep, error) { return CarrierStep{Carrier: original}, errors.New(message) }
	if p.Heading > HeadingNone {
		return fail("carrier action has not been reconstructed")
	}
	if p.Request.Pending && carrierTransition(p.Action, p.Request.Action) {
		p.Action, p.Entered = p.Request.Action, false
		p.Request.Pending = false
	}
	if !ordinaryCarrierAction(p.Action) {
		return fail("carrier transition has not been reconstructed")
	}
	action := p.Action
	if ctx.AttachedSupport != nil && (action != ActionReceiveBall || ctx.AttachedSupport.Kind >= 16 || ctx.CarryingPlayer) {
		return fail("attached carrier action or support has not been reconstructed")
	}
	if ctx.CarryingPlayer && (action == ActionAimShot || action == ActionAimAirShot) {
		return fail("carried-player shooting motion is not reconstructed")
	}
	setBall := func(a BallAction) { result.Ball.Change = true; result.Ball.Action = a }
	if !p.Entered {
		initializeAction(p)
		switch action {
		case ActionCarrierJump, ActionCarrierRunningJump:
			if ctx.SkipJumpWindup {
				p.Animation.Cursor = 1
				launchCarrierJump(&result, ctx, profile)
				return result, nil
			}
		case ActionDribbleRun:
			result.Carrier.ShotCycle = 8
			setRunHeading(p, profile.RunSpeed)
		case ActionDribbleBrake:
			result.Carrier.ShotCycle = -1
		case ActionReceiveBall:
			result.Carrier.ShotCycle = -1
			stopHorizontal(p)
			setBall(BallHeld)
			p.Animation.NextAction, p.Animation.HasNextAction = ActionCatchBall, true
			if ctx.AttachedSupport != nil {
				p.Animation.NextAction = ActionRiding
			}
		case ActionPickUp:
			stopHorizontal(p)
			p.Animation.NextAction, p.Animation.HasNextAction = ActionCatchBall, true
			setBall(BallHeld)
			result.BindBall = true
		case ActionCatchBall:
			stopHorizontal(p)
			setBall(BallHeld)
			p.Animation.NextAction, p.Animation.HasNextAction = ActionDribbleIdle, true
		case ActionCatchAir:
			setBall(BallHeld)
			result.Revisit = true
		case ActionDribbleIdle:
			stopHorizontal(p)
			result.Carrier.ShotCycle = -1
		case ActionCarrierLand:
			stopHorizontal(p)
			stopVertical(p)
			p.Animation.NextAction, p.Animation.HasNextAction = ActionDribbleIdle, true
		case ActionPass, ActionAirPass:
			if action == ActionPass {
				stopHorizontal(p)
				p.Animation.NextAction, p.Animation.HasNextAction = ActionIdle, true
			}
			// Original passing direction uses whole-pixel X, not fraction bits.
			if p.Body.Position.X/Pixel != ctx.Teammate.X/Pixel {
				p.FacingLeft = p.Body.Position.X/Pixel > ctx.Teammate.X/Pixel
			}
			setBall(BallHeld)
		case ActionAimShot:
			p.Animation.NextAction, p.Animation.HasNextAction = ActionIdle, false
			if ctx.Computer || result.Carrier.ShotCycle == 0 {
				p.Animation.NextAction, p.Animation.HasNextAction = ActionShoot, true
			}
			setBall(BallHeld)
		case ActionAimAirShot:
			result.ChangeAimWindow, result.AimWindow = true, 32
			setBall(BallHeld)
		case ActionShoot, ActionAirShoot:
			release, err := ChooseOriginalShotRelease(action, result.Carrier.ShotCycle, ctx.ShotRelease)
			if err != nil {
				return CarrierStep{Carrier: original}, err
			}
			result.ChangeSpecialShot, result.SpecialShot = true, release.SpecialShot
			result.ShotReleased = true
			if release.UsedRandom {
				result.ChangeRandom, result.Random = true, release.Random
			}
			if release.ChangeCursor {
				p.Animation.Cursor, p.Animation.Elapsed = release.Cursor, 0
			}

			faceHeading(p)
			if action == ActionShoot {
				p.Animation.NextAction, p.Animation.HasNextAction = ActionIdle, true
				p.Body.Velocity.Depth = 0
				result.Carrier.ShotCycle = -1
			}
			setBall(release.BallAction)
		}
	}
	if action == ActionDribbleRun {
		p.Body.Velocity.Depth = 0
		if p.Heading != HeadingNone {
			if ctx.Computer {
				setRunHeading(p, profile.RunSpeed)
			}
			setRunningDepth(p)
		}
		if ctx.HasBall && !ctx.Computer && p.Animation.Cursor == 0 && p.Animation.Elapsed == 0 {
			result.Carrier.ShotCycle--
			if result.Carrier.ShotCycle == 0 {
				p.Animation.Cursor = 3
			} else if result.Carrier.ShotCycle < 0 {
				result.Carrier.ShotCycle = -1
			}
		}
	}
	if action == ActionDribbleWalk || action == ActionDribbleRun || action == ActionCarrierJump || action == ActionCarrierRunningJump || action == ActionCatchAir {
		faceHeading(p)
	}
	if action == ActionAimShot {
		result.ChangeAimWindow, result.AimWindow = true, 32
	}
	var animationErr error
	if action == ActionDribbleRun {
		animationErr = animateCarrierRun(p, profile)
	} else if action == ActionShoot || action == ActionAirShoot {
		animationErr = animateOriginalShotRelease(p, profile, action)
	} else {
		animationErr = animatePlayer(p, profile, action)
	}
	if animationErr != nil {
		return CarrierStep{Carrier: original}, animationErr
	}
	switch action {
	case ActionDribbleBrake:
		brakePlayer(p, profile)
	case ActionDribbleIdle, ActionDribbleWalk:
		if ctx.BallAction != BallDribble {
			setBall(BallDribble)
		}
		if action == ActionDribbleWalk {
			v := profile.Walk[p.Heading]
			p.Body.Velocity.X, p.Body.Velocity.Depth = v.X, v.Depth
		}
	case ActionPass, ActionAirPass:
		if p.Animation.Cursor == 1 {
			setBall(BallPass)
		}
	case ActionAimShot, ActionShoot:
		p.Body.Velocity.X = approachZero(p.Body.Velocity.X, 128)
		if p.Body.Velocity.Depth > 0 {
			p.Body.Velocity.Depth -= 16
		} else if p.Body.Velocity.Depth < 0 {
			p.Body.Velocity.Depth += 16
		}
	case ActionAimAirShot:
		selectOriginalDunkTarget(&result, ctx.DunkSearch)
		result.Carrier.ShotCycle++
		if uint8(result.Carrier.ShotCycle) >= 8 {
			result.Carrier.ShotCycle = -1
		}
		if !p.Airborne {
			p.Action, p.Entered = ActionCarrierLand, false
			setBall(BallHeld)
		}
	case ActionAirShoot:
		if !p.Airborne {
			p.Action, p.Entered = ActionLand, false
			stopHorizontal(p)
		}
	}
	var terrain BallCommand
	if ctx.AttachedSupport != nil {
		attachRiderPosition(p, *ctx.AttachedSupport)
	} else {
		var err error
		terrain, err = advancePlayerOnCourt(p, playerTerrainContext{HasBall: ctx.HasBall, Computer: ctx.Computer}, court)
		if err != nil {
			return CarrierStep{Carrier: original}, err
		}
	}
	if terrain.Change {
		result.Ball.Change, result.Ball.Action = true, terrain.Action
	}
	if terrain.ChangeRequest {
		result.Ball.ChangeRequest, result.Ball.Request = true, terrain.Request
	}
	if action == ActionAimShot {
		selectOriginalDunkTarget(&result, ctx.DunkSearch)
		if p.Airborne && p.Body.Gravity == 0 {
			p.Body.Velocity.Height = signed16(p.Body.Velocity.Height + 15*Pixel/4)
			p.Body.Gravity = 15 * Pixel / 8
		}
	}
	if (action == ActionCarrierJump || action == ActionCarrierRunningJump) && p.Animation.Cursor != 0 {
		if p.Animation.Elapsed == 0 {
			launchCarrierJump(&result, ctx, profile)
		}
		if !p.Airborne {
			p.Action, p.Entered = ActionLand, false
			if ctx.HasBall {
				p.Action = ActionCarrierLand
			}
		}
	}
	if action == ActionAirPass && !p.Airborne {
		stopHorizontal(p)
		p.Action, p.Entered = ActionLand, false
		if ctx.HasBall {
			p.Action = ActionCarrierLand
		}
	}
	if action == ActionCatchAir && !p.Airborne {
		p.Action, p.Entered = ActionCarrierLand, false
	}
	if (action == ActionDribbleWalk || action == ActionDribbleRun) && p.Airborne {
		p.Action, p.Entered = ActionFalling, false
	}
	if action == ActionDribbleBrake && p.Body.Velocity.X == 0 {
		p.Action, p.Entered = ActionDribbleIdle, false
	}
	return result, nil
}

func approachZero(value, amount Fixed) Fixed {
	if value > 0 {
		return max(0, value-amount)
	}
	if value < 0 {
		return min(0, value+amount)
	}
	return 0
}
