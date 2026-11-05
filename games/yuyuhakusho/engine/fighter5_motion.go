package engine

import "errors"

// Fighter5ChildExtra is a narrow initializer view of existing child owners.
// Gravity maps Body.Gravity as one long; Owner maps the contextual +fe word.
type Fighter5ChildExtra struct {
	Gravity int32
	Owner   uint16
}

// Fighter5MotionState keeps the common +48/+4c owners and only adds the child
// initializer fields absent from ChuSpecialChild. Child callbacks run elsewhere.
type Fighter5MotionState struct {
	ChuSpecialMotionState
	ChildExtras [3]Fighter5ChildExtra
}

// StepFighter5Motion recovers source5 slots11/2d/2e/2f/30/31/32 through the
// pre-tail boundary. The caller resolves parameters from the current immutable
// movement source, advances animation timing and schedules children separately.
func StepFighter5Motion(state Fighter5MotionState, parameters JumpParameters) (Fighter5MotionState, error) {
	original := state
	body := &state.Body
	switch body.ActionKind {
	case 0x11, 0x2d:
		next, err := StepFighter10Motion(Fighter10MotionState{ObjectID: state.ObjectID, Body: state.Body, Controls: state.Controls,
			Aux48: state.AlternateVelocity, ContactFlags: state.ContactFlags, Pool: state.Pool}, parameters)
		if err != nil {
			return original, err
		}
		state.Body, state.Controls, state.AlternateVelocity, state.ContactFlags, state.Pool = next.Body, next.Controls, next.Aux48, next.ContactFlags, next.Pool
	case 0x2e:
		if body.Clock.Completed&1 != 0 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.FrameIndex == 0 && state.ContactFlags&4 != 0 {
			next := FinishFighter5Combination(Fighter5AttackState{ChuAttackState: ChuAttackState{Body: *body}}, body.Pose != 0xa4)
			state.Body = next.Body
		}
	case 0x2f, 0x30, 0x32:
		if body.Clock.Completed == 1 {
			if body.ActionKind == 0x30 {
				finishFighter5AirProjectile(body, parameters)
			} else {
				body.Clock.Completed = 0
				enterChuSpecialGround(body, false)
			}
			break
		}
		if body.Clock.FrameIndex == 0 && body.Clock.Ticks == 0 && state.Children[0].Size == 0 {
			initializeFighter5Projectile(&state)
		}
	case 0x31:
		counter := uint16(uint32(state.AlternateVelocity)>>16) + 1
		state.AlternateVelocity = replaceChuSpecialHighWord(state.AlternateVelocity, counter)
		if int16(counter) > 31 {
			variant := uint8(uint32(body.Gravity) >> 24)
			if variant <= 2 {
				next, err := BeginFighter5FinalProjectile(Fighter5AttackState{ChuAttackState: ChuAttackState{Body: *body}, AlternateVelocity: state.AlternateVelocity}, variant)
				if err != nil {
					return original, err
				}
				state.Body = next.Body
			}
			break
		}
		child := &state.Children[1]
		if child.Size != 0 {
			break
		}
		initializeChuSpecialChild(child, state.ChuSpecialMotionState, 0x146ba, 0x70, 252)
		child.VerticalVelocity = replaceChuSpecialHighWord(child.VerticalVelocity, 0xffc0)
		velocity := int16(30)
		if body.Facing != 0 {
			velocity = -velocity
		}
		child.VelocityX = replaceChuSpecialHighWord(child.VelocityX, uint16(velocity))
	default:
		return original, errors.New("unsupported source5 motion action")
	}
	return state, nil
}

func initializeFighter5Projectile(state *Fighter5MotionState) {
	body := &state.Body
	behavior, pose := uint32(0x1414e), uint8(0x17)
	dx, dy := int16(28), int16(-64)
	if body.ActionKind == 0x30 {
		dx, dy = 20, -52
	} else if body.ActionKind == 0x32 {
		behavior, pose = 0x143e4, 0x18
	}
	child := &state.Children[0]
	initializeChuSpecialChild(child, state.ChuSpecialMotionState, behavior, 0x60, 252)
	child.Team += 16
	child.VelocityX, child.VerticalVelocity = body.VelocityX, body.VerticalVelocity
	child.Facing = body.Facing
	if body.Facing != 0 {
		dx = -dx
		child.VelocityX = -child.VelocityX
	}
	child.X, child.Y = addThrowCoordinateWord(body.X, dx), addThrowCoordinateWord(body.Y, dy)
	child.TargetMask, child.Pose = 0xffffffff, pose
	extra := &state.ChildExtras[0]
	extra.Gravity = int32(uint32(extra.Gravity)&0x00ffffff | uint32(body.Gravity)&0xff000000)
	extra.Owner = state.ObjectID
}

func finishFighter5AirProjectile(body *FighterBodyState, parameters JumpParameters) {
	body.ExtraJump++
	body.GroundActionState = ResetActionState(body.GroundActionState)
	body.Gravity = parameters.Gravity
	body.VerticalVelocity = (parameters.VerticalVelocity >> 1) + (parameters.VerticalVelocity >> 2)
	body.VelocityX = parameters.HorizontalVelocity
	if body.Facing != 1 {
		body.VelocityX = -body.VelocityX
	}
	body.ActionKind, body.Pose, body.EnabledStages = 2, 7, 0x14
}
