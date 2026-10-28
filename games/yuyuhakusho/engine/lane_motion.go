package engine

import "errors"

// StepLaneAction recovers actions 10/11 through their landing transition,
// stopping before the shared action tail. Ground height and render depth move
// along with the lane arc; they are not merely a second drawing offset.
func StepLaneAction(body FighterBodyState, pool EffectPool, occupied uint8) (FighterBodyState, EffectPool, uint8, [2]int, bool, error) {
	before, oldOccupied := body, occupied
	none := [2]int{-1, -1}
	var drift, acceleration int32
	var layerStep uint8
	var depthStep uint16
	switch body.ActionKind {
	case 10:
		drift, acceleration, layerStep, depthStep = 0xa800, 0x4800, 3, 3
	case 11:
		drift, acceleration, layerStep, depthStep = 0x5000, 0x1000, 2, 1
	default:
		return body, pool, occupied, none, false, errors.New("unsupported lane motion action")
	}
	if body.Pose == 0x0f {
		occupied |= 2
		body.Ground -= drift
		body.Layer += layerStep
		body.Depth -= depthStep
	}
	if body.Pose == 0x10 {
		occupied |= 1
		body.Ground += drift
		body.Layer -= layerStep
		body.Depth += depthStep
		if int16(body.Depth) >= 0x100 {
			body.Depth = 0x100
		}
	}
	body.X += body.VelocityX
	body.VerticalVelocity += acceleration
	body.Y += body.VerticalVelocity
	if body.VerticalVelocity < 0 || int16(body.Y>>16) < int16(body.Ground>>16) {
		return body, pool, occupied, none, false, nil
	}
	if body.Pose == 0x0f {
		body.Ground, body.Depth, body.Layer = 0x1300000, 0xd8, 0xc0
	}
	if body.Pose == 0x10 {
		body.Ground, body.Depth, body.Layer = 0x1400000, 0x100, 0x40
	}
	body.Y = body.Ground
	body, next, slots, err := BeginLanding(body, pool)
	if err != nil {
		return before, pool, oldOccupied, none, false, err
	}
	// Unlike the normal air action, lane landing does not clear ExtraJump.
	return body, next, occupied, slots, true, nil
}
