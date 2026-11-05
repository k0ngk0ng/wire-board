package engine

import "errors"

// Fighter7MotionState views the existing +48, child input, camera-left and
// peer hit-stop owners used by source7 MAIN. OtherFighters contains exactly
// the three other fixed parent slots, including empty or defeated parents.
type Fighter7MotionState struct {
	ChuSpecialMotionState
	Left          int16
	ChildExtras   [3]Fighter4ChildExtra
	OtherFighters map[uint16]TailFighterEnergy
}

// StepFighter7Motion recovers source7 MAIN39..3d and its shared motion entries
// before the common action tail. Animation timing and child updates are caller
// stages. Peer changes are returned in a copied map for one atomic merge.
func StepFighter7Motion(state Fighter7MotionState, parameters JumpParameters) (Fighter7MotionState, error) {
	original := state
	validID := false
	for _, id := range [...]uint16{0x8400, 0x8800, 0x8c00, 0x9000} {
		if id == state.ObjectID {
			validID = true
			continue
		}
		if _, ok := state.OtherFighters[id]; !ok {
			return original, errors.New("source7 motion requires all three other fixed parent slots")
		}
	}
	if !validID || len(state.OtherFighters) != 3 {
		return original, errors.New("source7 motion has an invalid parent or peer slot")
	}
	state.OtherFighters = make(map[uint16]TailFighterEnergy, 3)
	for id, fighter := range original.OtherFighters {
		state.OtherFighters[id] = fighter
	}
	body := &state.Body
	switch body.ActionKind {
	case 0x0f, 0x14, 0x45:
		view, err := StepChuSpecialMotion(state.ChuSpecialMotionState, parameters)
		if err != nil {
			return original, err
		}
		state.ChuSpecialMotionState = view
	case 0x11:
		view, err := StepFighter10Motion(Fighter10MotionState{ObjectID: state.ObjectID, Body: *body}, parameters)
		if err != nil {
			return original, err
		}
		state.Body = view.Body
	case 0x13:
		view, err := StepFighter3Motion(Fighter3MotionState{ChuSpecialMotionState: state.ChuSpecialMotionState}, parameters)
		if err != nil {
			return original, err
		}
		state.ChuSpecialMotionState = view.ChuSpecialMotionState
	case 0x39:
		// The constructor already applies facing to VX. Contact stores its
		// landing variant in bit0 of the highest byte of the existing +48 long.
		body.X += body.VelocityX
		body.VerticalVelocity += body.Gravity
		body.Y += body.VerticalVelocity
		if body.VerticalVelocity < 0 {
			break
		}
		if state.ContactFlags&12 != 0 {
			state.AlternateVelocity = int32(uint32(state.AlternateVelocity) | 0x01000000)
		}
		if int16(body.Ground>>16) <= int16(body.Y>>16) {
			body.Y = body.Ground
			body.ActionKind, body.Pose = 3, 0xa2
			if uint32(state.AlternateVelocity)&0x01000000 != 0 {
				body.Pose = 0xa1
			}
			body.GroundActionState = ResetActionState(body.GroundActionState)
		} else if body.Clock.FrameIndex == 0 && state.ContactFlags&1 != 0 {
			body.Clock.Ticks = 0
			body.Clock.FrameIndex++
		}
	case 0x3a:
		if body.Clock.Completed != 1 {
			break
		}
		body.Pose, body.ActionKind, body.Request, body.EnabledStages = 0xa5, 3, 0, 0
		body.Y = body.Ground
		x := state.Left
		switch uint8(uint32(state.AlternateVelocity) >> 24) {
		case 0:
			x += 0x20
		case 1:
			x += 0xa0
		case 2:
			x += 0x120
		}
		body.X = replaceChuSpecialHighWord(body.X, uint16(x))
		body.GroundActionState = ResetActionState(body.GroundActionState)
	case 0x3b:
		if body.Clock.Completed != 1 {
			break
		}
		body.Pose, body.ActionKind, body.Request, body.EnabledStages = 0xa7, 0x3c, 0, 0
		body.GroundActionState = ResetActionState(body.GroundActionState)
		for i, config := range [...]struct {
			behavior  uint32
			attribute uint16
			layer     uint8
		}{{0x136e6, 0x70, 252}, {0x1391e, 0x60, 4}} {
			child := &state.Children[i]
			if child.Size != 0 {
				continue
			}
			initializeChuSpecialChild(child, state.ChuSpecialMotionState, config.behavior, config.attribute, config.layer)
			child.Attributes = (body.Attributes+config.attribute)&0x9fff | 0x6000
			child.Y, child.X, child.Facing = addThrowCoordinateWord(body.Y, -32), body.X, body.Facing
			offset := int16(body.VelocityX >> 16)
			if body.Facing != 0 {
				offset = -offset
			}
			child.X = addThrowCoordinateWord(child.X, offset)
			child.TargetMask = 0xffffffff
		}
	case 0x3c:
		state.Energy -= 0x4000
		exit := state.Energy < 0
		if exit {
			state.Energy = 0
		} else {
			state.ChildExtras[0].Held, state.ChildExtras[0].Pressed = uint8(state.Controls.Held), uint8(state.Controls.Pressed)
			if state.Children[0].Size == 0 {
				exit = true
			} else {
				state.ChildExtras[1].Held, state.ChildExtras[1].Pressed = uint8(state.Controls.Held), uint8(state.Controls.Pressed)
				exit = state.Controls.Pressed&ButtonC != 0
			}
		}
		if exit {
			body.Clock.Completed = 0
			body.Pose, body.ActionKind = 0xa8, 3
			body.GroundActionState = ResetActionState(body.GroundActionState)
		}
	case 0x3d:
		if body.Clock.Completed == 1 {
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.FrameIndex != 2 || body.Clock.Ticks != 0 {
			break
		}
		child := &state.Children[2]
		if child.Size == 0 {
			child.Behavior, child.Size, child.Stage = 0x15094, 0x100, 0
		}
		for id, fighter := range state.OtherFighters {
			if fighter.Team != body.Team {
				fighter.HitStopTicks = 255
				state.OtherFighters[id] = fighter
			}
		}
	default:
		return original, errors.New("unsupported source7 motion action")
	}
	return state, nil
}
