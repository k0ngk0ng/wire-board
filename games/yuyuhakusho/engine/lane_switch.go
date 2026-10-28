package engine

// BeginLaneSwitch recovers the complete 0x186be constructor. It creates dust
// from the old position/layer, then chooses the original lane-dependent arc.
// The returned occupancy byte belongs to the shared world state and is cleared
// even when the pool has no room for either effect. Later lane motion is separate.
func BeginLaneSwitch(body FighterBodyState, pool EffectPool, occupied uint8) (FighterBodyState, EffectPool, uint8, [2]int, error) {
	updated, slots, err := SpawnMovementDust(pool, EffectSource{X: body.X, Y: body.Y, Attributes: body.Attributes, Layer: body.Layer, Depth: body.Depth})
	if err != nil {
		return body, pool, occupied, slots, err
	}
	body.VelocityX = 0
	body.EnabledStages = 0
	if int16(body.Ground>>16) == 0x140 {
		body.ActionKind = 11
		body.VerticalVelocity = -0x20000
		if occupied&2 != 0 {
			body.ActionKind = 10
			body.VerticalVelocity = -0x40000
		}
		body.Pose = 0x0f
		body.Ground = 0x13f0000
		body.Layer += 0x40
	} else {
		body.ActionKind = 11
		body.VerticalVelocity = -0x18000
		if occupied&1 != 0 {
			body.ActionKind = 10
			body.VerticalVelocity = -0x30000
		}
		body.Pose = 0x10
		body.Ground = 0x1310000
		body.Layer -= 0x40
		body.Depth = 200
	}
	body.GroundActionState = ResetActionState(body.GroundActionState)
	return body, updated, 0, slots, nil
}
