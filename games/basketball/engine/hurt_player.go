package engine

import "errors"

type HurtState struct {
	Player    Player
	Endurance uint8
}
type HurtContext struct {
	Actor                                     uint8
	Computer                                  bool
	RecoveryGrade                             uint8
	Random                                    RandomState
	BallHolder                                int8
	BallVelocityX                             Fixed
	AuxiliaryOwners                           [2]int8
	HoopOwners                                [6]int8
	SpecialHolder, AssistedActor, LinkedActor int8
	Attachments                               [4]uint8
}
type HurtObjectCommand struct {
	Object          uint8
	SetAction       bool
	Action          uint8
	SetOwner        bool
	Owner           int8
	SetFacing       bool
	FacingLeft      bool
	ClearAttachment bool
	// Original 8B67 support operation, ordered between assisted action and link clear.
	DetachSupport bool
}
type HurtStep struct {
	State                           HurtState
	Random                          RandomState
	Objects                         []HurtObjectCommand
	Ball                            BallCommand
	ReleaseBall, ChangeBallVelocity bool
	BallVelocityX                   Fixed
	ClearSpecialHolder              bool
}

func hurtAnimation(action Action) (AnimationSpec, error) {
	f := func(p, t uint8) AnimationFrame {
		return AnimationFrame{Pose: Pose{Frame: p & 127, Mirrored: p&128 != 0}, Ticks: t}
	}
	switch action {
	case 0x48:
		return AnimationSpec{Frames: []AnimationFrame{f(19, 8)}, End: RepeatForever}, nil
	case 0x49:
		return AnimationSpec{Frames: []AnimationFrame{f(19, 255), f(23, 2), f(183, 255)}, End: RepeatForever}, nil
	case 0x4a:
		return AnimationSpec{Frames: []AnimationFrame{f(23, 24)}, End: RepeatRemaining}, nil
	case 0x47:
		return AnimationSpec{Frames: []AnimationFrame{f(37, 5), f(13, 4), f(37, 4), f(13, 4)}, End: RepeatRemaining}, nil
	}
	return AnimationSpec{}, errors.New("hurt action is not reconstructed")
}

// PrepareHurtPlayer ends before animation and EEFF. Commands must be committed
// in order before motion loads the world, including the 8B67 support operation.
func PrepareHurtPlayer(original HurtState, ctx HurtContext) (HurtStep, error) {
	initial := HurtStep{State: original, Random: ctx.Random}
	result := initial
	fail := func(msg string) (HurtStep, error) { return initial, errors.New(msg) }
	if ctx.Actor >= 4 {
		return fail("invalid hurt actor")
	}
	p := &result.State.Player
	action := p.Action
	_, err := hurtAnimation(action)
	if err != nil {
		return initial, err
	}
	emit := func(c HurtObjectCommand) { result.Objects = append(result.Objects, c) }
	clearLink := func(actor uint8) { emit(HurtObjectCommand{Object: actor, ClearAttachment: true}) }
	dropAux := func() {
		for i, owner := range ctx.AuxiliaryOwners {
			if owner == int8(ctx.Actor) {
				emit(HurtObjectCommand{Object: uint8(4 + i), SetAction: true, Action: 7, SetOwner: true, Owner: -1, SetFacing: true, FacingLeft: p.FacingLeft})
				return
			}
		}
	}
	dropHoop := func() bool {
		for i, owner := range ctx.HoopOwners {
			if owner == int8(ctx.Actor) {
				emit(HurtObjectCommand{Object: uint8(7 + i), SetAction: true, Action: 8, SetOwner: true, Owner: -1})
				return true
			}
		}
		return false
	}
	dropSpecial := func() {
		if ctx.SpecialHolder == int8(ctx.Actor) {
			emit(HurtObjectCommand{Object: 4, SetAction: true, Action: 7})
			result.ClearSpecialHolder = true
		}
	}
	dropBall := func() {
		result.Ball = BallCommand{Change: true, Action: BallAction(11)}
		result.ReleaseBall = true
		result.ChangeBallVelocity = true
		if action == 0x48 {
			result.BallVelocityX = 3 * Pixel
			if p.FacingLeft {
				result.BallVelocityX = -3 * Pixel
			}
		} else {
			integer := Fixed(-2 * Pixel)
			if p.Body.Velocity.X < 0 {
				integer = 4 * Pixel
			}
			result.BallVelocityX = integer + (ctx.BallVelocityX & 255)
		}
	}
	if !p.Entered {
		if action == 0x47 && ctx.RecoveryGrade >= 8 {
			return fail("hurt recovery grade outside original table")
		}
		initializeAction(p)
		switch action {
		case 0x48:
			stopVertical(p)
			p.Body.Velocity.Height = 5 * Pixel
			p.Body.Gravity = Pixel
			p.Body.Velocity.Depth = 0
			dropAux()
			teammate := ctx.Actor ^ 1
			if ctx.Attachments[teammate]&128 != 0 && ctx.Attachments[teammate]&15 == ctx.Actor {
				clearLink(teammate)
			}
			if ctx.AssistedActor >= 0 {
				if ctx.AssistedActor >= 13 {
					return fail("hurt assisted actor outside object set")
				}
				emit(HurtObjectCommand{Object: uint8(ctx.AssistedActor), SetAction: true, Action: 11})
			}
			clearLink(ctx.Actor)
			if !dropHoop() {
				if ctx.BallHolder == int8(ctx.Actor) {
					dropBall()
				} else {
					dropSpecial()
				}
			}
		case 0x49:
			p.FacingLeft = p.Body.Velocity.X >= 0
			if ctx.AssistedActor >= 0 {
				if ctx.AssistedActor >= 13 {
					return fail("hurt assisted actor outside object set")
				}
				emit(HurtObjectCommand{Object: uint8(ctx.AssistedActor), SetAction: true, Action: 11})
			}
			if ctx.Attachments[ctx.Actor]&128 != 0 {
				emit(HurtObjectCommand{Object: ctx.Actor, DetachSupport: true})
			}
			clearLink(ctx.Actor)
			if ctx.LinkedActor >= 0 {
				if ctx.LinkedActor >= 4 {
					return fail("hurt linked actor is not a player")
				}
				clearLink(uint8(ctx.LinkedActor))
			} else {
				dropAux()
			}
			if ctx.BallHolder == int8(ctx.Actor) {
				dropBall()
				dropHoop()
			} else if ctx.SpecialHolder == int8(ctx.Actor) {
				dropSpecial()
			} else {
				dropHoop()
			}
		case 0x4a:
			stopHorizontal(p)
			stopVertical(p)
			next := ActionLand
			if ctx.Computer {
				var value byte
				result.Random, value = NextRandom(result.Random)
				next = [4]Action{0x67, 0x63, ActionLand, ActionLand}[(value>>1)&3]
			}
			p.Animation.NextAction, p.Animation.HasNextAction = next, true
		case 0x47:
			p.Animation.Elapsed = 1
			p.Animation.NextAction, p.Animation.HasNextAction = ActionIdle, true
			stopHorizontal(p)
			p.Animation.Loops = 7 - ctx.RecoveryGrade
			result.State.Endurance = 255
		}
	}
	return result, nil
}

// CompleteHurtPlayer contains animation, motion and the post-motion landing
// branches. Preparation commands are consumed before calling this function.
func CompleteHurtPlayer(prepared HurtStep, ctx HurtContext, court FlatCourt) (HurtStep, error) {
	initial := prepared
	result := prepared
	result.Objects = nil
	result.Ball = BallCommand{}
	result.ReleaseBall, result.ChangeBallVelocity, result.ClearSpecialHolder = false, false, false
	p := &result.State.Player
	action := p.Action
	spec, err := hurtAnimation(action)
	if err != nil {
		return initial, err
	}

	step, err := AdvanceAnimation(spec, p.Animation, p.FacingLeft)
	if err != nil {
		return initial, err
	}
	p.Animation, p.Pose = step.State, step.Pose
	if step.ChangeAction {
		p.Action, p.Entered = step.Action, false
	}
	if action == 0x47 && p.Animation.Cursor == 0 && p.Animation.Elapsed == 0 {
		p.Animation.Cursor = 2
	}
	ball, err := advancePlayerOnCourt(p, playerTerrainContext{Computer: ctx.Computer, HasBall: ctx.BallHolder == int8(ctx.Actor) && !result.ReleaseBall}, court)
	if err != nil {
		return initial, err
	}
	if ball.Change || ball.ChangeRequest {
		result.Ball = ball
	}
	if action == 0x49 {
		if uint8(p.Body.Velocity.Height>>8) == 3 {
			p.Animation.Cursor++
			p.Animation.Elapsed = 0
		}
		if !p.Airborne {
			p.Action, p.Entered = 0x4a, false
			if ctx.RecoveryGrade >= 7 {
				p.Action = ActionLand
			}
		}
	} else if action == 0x48 && !p.Airborne {
		p.Action, p.Entered = ActionIdle, false
		if result.State.Endurance == 0 {
			p.Action = 0x47
		}
	}
	return result, nil
}

// StepHurtPlayer preserves the isolated free-player API. Attached entry needs
// the actual world support, so callers must use the world preparation adapter.
func StepHurtPlayer(original HurtState, ctx HurtContext, court FlatCourt) (HurtStep, error) {
	initial := HurtStep{State: original, Random: ctx.Random}
	prepared, e := PrepareHurtPlayer(original, ctx)
	if e != nil {
		return initial, e
	}
	for _, command := range prepared.Objects {
		if command.DetachSupport {
			return initial, errors.New("hurt rider detachment requires world support")
		}
	}
	if prepared.ReleaseBall {
		ctx.BallHolder = 6
	}
	complete, e := CompleteHurtPlayer(prepared, ctx, court)
	if e != nil {
		return initial, e
	}
	result := prepared
	result.State = complete.State
	if complete.Ball.Change || complete.Ball.ChangeRequest {
		result.Ball = complete.Ball
	}
	return result, nil
}
