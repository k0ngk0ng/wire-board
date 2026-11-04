package engine

import "errors"

// StepFighter3AirMotion recovers source3 slot22 through15afc, including1863a
// recovery. It creates reserved children but does not schedule them, advance
// animation timing or run the shared actor tail.
func StepFighter3AirMotion(state ChuSpecialMotionState, parameters JumpParameters) (ChuSpecialMotionState, error) {
	body := &state.Body
	if body.ActionKind != 0x22 {
		return state, errors.New("unsupported source3 air-release stage")
	}
	if body.Clock.Completed == 1 {
		body.ExtraJump++
		body.GroundActionState = ResetActionState(body.GroundActionState)
		body.Gravity = parameters.Gravity
		body.VerticalVelocity = (parameters.VerticalVelocity >> 1) + (parameters.VerticalVelocity >> 2)
		body.VelocityX = parameters.HorizontalVelocity
		if body.Facing != 1 {
			body.VelocityX = -body.VelocityX
		}
		body.ActionKind, body.Pose, body.EnabledStages = 2, 7, 0x14
		return state, nil
	}
	if body.Clock.FrameIndex != 2 || body.Clock.Ticks != 0 || state.Children[0].Size != 0 {
		return state, nil
	}
	// Source3 shares these exact child-header/attribute/ground-word writes;
	// the air release's power ownership and continuation remain distinct.
	context := Fighter0MotionState{ChuSpecialMotionState: state}
	main := &state.Children[0]
	initializeFighter0Child(main, context, 0x1313e, 0x40, 252)
	main.X, main.Y = body.X, addThrowCoordinateWord(body.Y, int16(body.VerticalVelocity>>16))
	main.VelocityX, main.Facing = state.AlternateVelocity, body.Facing
	dx := int16(body.VelocityX >> 16)
	if body.Facing != 0 {
		dx, main.VelocityX = -dx, -main.VelocityX
	}
	main.X = addThrowCoordinateWord(main.X, dx)
	main.TargetMask = 0xffffffff
	main.Pose = uint8(state.Power >> 5)
	// Main.Power is deliberately retained. A busy release slot also retains
	// this already-created main projectile and the parent's existing power.
	release := &state.Children[2]
	if release.Size != 0 {
		return state, nil
	}
	initializeFighter0Child(release, context, 0x134c2, 0x60, 248)
	release.X, release.Y = addThrowCoordinateWord(body.X, dx), addThrowCoordinateWord(body.Y, int16(body.VerticalVelocity>>16))
	release.Power, state.Power = state.Power, 0
	return state, nil
}
