package engine

import "errors"

// ReactionEffectExtra contains fields touched by attached reaction effects.
// They are kept alongside the common pool without treating old slot bytes as
// fields that unrelated initializers are allowed to reset.
type ReactionEffectExtra struct {
	Owner   uint16
	OffsetY int16
	Phase   uint8
}

type ReactionMotionState struct {
	ActorID                uint16
	Body                   FighterBodyState
	Controls               FighterControls
	GuardMask              uint8
	StunTicks              uint16
	Charge                 int32
	Pool                   EffectPool
	EffectExtras           [16]ReactionEffectExtra
	OccupiedLanes          uint8
	PendingSound           uint8
	GlobalF568, GlobalF56A uint16
	Left, Right            int16
}

// StepReactionMotion recovers actions 3..9 and 12/13 through their first entry
// into the shared tail at 0x15afc. It does not advance the animation clock or
// run that tail's later input, animation submission and random update stages.
func StepReactionMotion(state ReactionMotionState, parameters JumpParameters) (ReactionMotionState, error) {
	before := state
	body := &state.Body
	switch body.ActionKind {
	case 3:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			transition := GroundStand
			if body.Clock.Flags&4 != 0 {
				transition = GroundCrouch
			}
			body.GroundActionState, _ = EnterGroundAction(body.GroundActionState, transition, state.Controls.Held)
		}
	case 4:
		body.ActionFlags |= 0x20
		if state.GuardMask != 0 {
			body.Pose = 0x2c
			if state.Controls.Held&ButtonDown != 0 {
				body.Pose = 0x2d
			}
		}
		body.Y = body.Ground
		if body.VelocityX > 0 {
			body.VelocityX -= 0x4800
			if body.Facing == 1 {
				body.X += body.VelocityX
			} else {
				body.X -= body.VelocityX
			}
			if body.VelocityX <= 0 {
				body.GroundActionState, _ = EnterGroundAction(body.GroundActionState, GroundStand, state.Controls.Held)
			}
		}
	case 5, 8:
		if body.ActionKind == 8 && body.Clock.FrameIndex == 0 && body.Clock.Ticks == 1 {
			if err := spawnReactionMarker(&state); err != nil {
				return before, err
			}
		}
		if body.Clock.FrameIndex == 0 && body.Clock.Ticks == 1 && body.Health >= 0 && int16(state.StunTicks) > 0 {
			effect := uint8(2)
			if int16(state.StunTicks) > 320 {
				effect = 1
			} else if int16(state.StunTicks) > 128 {
				effect = 0
			}
			if err := spawnReactionStatus(&state, effect); err != nil {
				return before, err
			}
		}
		landed := integrateReactionAir(body)
		if !landed {
			break
		}
		body.Y = body.Ground
		switch body.Pose {
		case 0x24:
			next, pool, occupied, _, err := StepGroundControl(*body, state.Pool, state.OccupiedLanes, state.Controls)
			if err != nil {
				return before, err
			}
			state.Body, state.Pool, state.OccupiedLanes = next, pool, occupied
		case 0x25, 0x27, 0x2f, 0x30:
			body.Team = body.BaseTeam
			state.PendingSound, state.GlobalF56A, state.GlobalF568 = 0x7d, 0x200, 0x20
			if err := spawnReactionDust(&state); err != nil {
				return before, err
			}
			body.ActionKind, body.Pose, body.EnabledStages = 6, 0x28, 0
			body.Gravity, body.VerticalVelocity, body.VelocityX = 0x3800, -0x20000, body.VelocityX>>1
			body.GroundActionState = ResetActionState(body.GroundActionState)
		case 0x28:
			state.PendingSound, state.GlobalF56A, state.GlobalF568 = 0x7d, 0x200, 0x10
			if err := spawnReactionDust(&state); err != nil {
				return before, err
			}
			enterReactionKnockdown(body)
		default:
			state.PendingSound, state.GlobalF56A, state.GlobalF568 = 0x7d, 0x400, 0x20
			body.ActionKind, body.Pose, body.EnabledStages = 0, 0x2a, 0
			body.GroundActionState = ResetActionState(body.GroundActionState)
		}
	case 6:
		if body.Clock.FrameIndex == 0 {
			body.Y = body.Ground
			break
		}
		if !integrateReactionAir(body) {
			break
		}
		// The original does not clamp Y in this landing path before creating dust.
		state.PendingSound, state.GlobalF56A, state.GlobalF568 = 0x7d, 0x200, 0x10
		if err := spawnReactionDust(&state); err != nil {
			return before, err
		}
		enterReactionKnockdown(body)
		if body.Health < 0 {
			body.Health = -0x400
		}
	case 7:
		body.X += body.VelocityX
		if x := int16(body.X >> 16); x < state.Left || x > state.Right {
			body.Team = body.BaseTeam
			state.PendingSound, state.GlobalF56A, state.GlobalF568 = 0x7d, 0x200, 0x20
			body.ActionKind, body.Pose = 5, 0x25
			body.Gravity, body.VerticalVelocity = 0x3800, -0x30000
			body.Facing ^= 1
			body.VelocityX = -0x18000
			if body.Facing == 0 {
				body.VelocityX = 0x18000
			}
			body.GroundActionState = ResetActionState(body.GroundActionState)
		}
	case 9:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
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
	case 12:
		body.Auxiliary7E = 255
		body.Y = body.Ground
		state.StunTicks--
		if uint8(state.Controls.Pressed) != 0 {
			state.StunTicks -= 2
			if body.Clock.Ticks <= 200 {
				body.Clock.Ticks += 2
			}
		}
		if int16(state.StunTicks) < 1 {
			state.StunTicks = 0
			body.GroundActionState, _ = EnterGroundAction(body.GroundActionState, GroundStand, state.Controls.Held)
		}
	case 13:
		state.Charge += body.VelocityX
		if int16(state.Charge>>16) >= 223 {
			state.Charge = 223 << 16
		} else if state.Controls.Held&0x50 != 0 {
			break
		}
		body.GroundActionState, _ = EnterGroundAction(body.GroundActionState, GroundStand, state.Controls.Held)
	default:
		return before, errors.New("unsupported reaction action")
	}
	return state, nil
}

func integrateReactionAir(body *FighterBodyState) bool {
	body.X += body.VelocityX
	body.VerticalVelocity += body.Gravity
	body.Y += body.VerticalVelocity
	return body.VerticalVelocity >= 0 && int16(body.Ground>>16) <= int16(body.Y>>16)
}

func spawnReactionDust(state *ReactionMotionState) error {
	pool, _, err := SpawnMovementDust(state.Pool, state.Body.MovementDustSource())
	if err == nil {
		state.Pool = pool
	}
	return err
}

func enterReactionKnockdown(body *FighterBodyState) {
	body.ActionKind, body.Pose, body.EnabledStages = 0, 0x29, 0
	body.GroundActionState = ResetActionState(body.GroundActionState)
}

func spawnReactionMarker(state *ReactionMotionState) error {
	pool, index, err := AllocateEffect(state.Pool)
	if err != nil {
		return err
	}
	state.Pool = pool
	if index < 0 {
		return nil
	}
	slot := &state.Pool.Slots[index]
	slot.Type = 0x15
	slot.Depth, slot.Layer = state.Body.Depth, state.Body.Layer-4
	slot.Attributes = state.Body.Attributes&0x8000 | 0x4000
	if state.Body.Facing != 0 {
		slot.Attributes |= 0x800
	}
	state.EffectExtras[index].Owner = state.ActorID
	state.EffectExtras[index].OffsetY = -0x30
	return nil
}

func spawnReactionStatus(state *ReactionMotionState, effect uint8) error {
	for _, phase := range [...]uint8{0, 0x15, 0x2a} {
		pool, index, err := AllocateEffect(state.Pool)
		if err != nil {
			return err
		}
		state.Pool = pool
		if index < 0 {
			continue
		}
		slot := &state.Pool.Slots[index]
		slot.Behavior = 0x14ec6
		slot.Type = effect
		slot.Depth, slot.Layer = state.Body.Depth, state.Body.Layer-4
		slot.Attributes = state.Body.Attributes&0x8800 | 0x2000
		if effect == 2 {
			slot.Attributes = state.Body.Attributes&0x8800 | 0x4000
		}
		state.EffectExtras[index].Owner = state.ActorID
		state.EffectExtras[index].Phase = phase
	}
	return nil
}
