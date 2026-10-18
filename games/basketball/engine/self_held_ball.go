package engine

import "fmt"

// prepareSelfReferencedHeldBall restores B585..B5B9 for owner6. The source's
// owner-action 5A/5C checks cannot match the ball's current action2 here.
func prepareSelfReferencedHeldBall(original Ball) (Ball, bool, error) {
	if original.Action != BallHeld {
		return original, false, fmt.Errorf("self held preparation action %d", original.Action)
	}
	b := original
	if b.Entered {
		return b, false, nil
	}
	b.Entered = true
	b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 0, 0, 0
	b.Body.Velocity = Velocity{}
	b.Body.Gravity = 0
	return b, true, nil
}

// applySelfReferencedHeldBallOffset restores B5C7..B5F3 using the current ball
// as the real source object. Court motion may have changed its action, pose or
// coordinates. Read those resulting fields rather than retaining an old view.
func applySelfReferencedHeldBallOffset(original Ball) (Ball, error) {
	b := original
	if int(b.Action) >= len(originalHeldBallActionOffsets) {
		return original, fmt.Errorf("self held offset action %d outside source table", b.Action)
	}
	offset := originalHeldBallOffsets[originalHeldBallActionOffsets[b.Action]]
	if b.Pose.Frame == 30 {
		offset = originalHeldBallOffsets[heldWithOverheadPose]
	}
	dx := Fixed(offset.X) * Pixel
	if b.Pose.Mirrored {
		dx = -dx
	}
	b.Body.Position.X = signed24(b.Body.Position.X + dx)
	b.Body.Position.Height = signed24(b.Body.Position.Height + Fixed(offset.Height)*Pixel)
	b.Body.Position.Depth = Fixed(uint8((b.Body.Position.Depth>>8)-Fixed(offset.SubtractDepth)))*Pixel + (b.Body.Position.Depth & 255)
	if b.Pose.Frame >= 59 && b.Pose.Frame < 63 {
		b.Pose = Pose{Frame: 127}
	}
	return b, nil
}

// completeSelfReferencedHeldBall preserves animation -> EEFF -> live source
// selection -> EC7B. Changing the holder during motion requires a different
// source object; do not silently continue using self or choose a player.
func completeSelfReferencedHeldBall(prepared Ball, court FlatCourt, loadHolder func() int8) (Ball, error) {
	if prepared.Action != BallHeld || loadHolder == nil {
		return prepared, fmt.Errorf("invalid self held completion context")
	}
	b := prepared
	// Owner6 is this actual ball. It cannot select a player low-jump header,
	// but a tail selected before losing the player owner must keep advancing.
	e := animateOriginalHeldBall(&b, Player{Action: Action(b.Action), Animation: b.Animation})
	if e != nil {
		return prepared, e
	}
	if e = advanceBallOnCourt(&b, court); e != nil {
		return prepared, e
	}
	if holder := loadHolder(); holder != 6 {
		return prepared, fmt.Errorf("self held post-motion owner %d requires its live source", holder)
	}
	result, e := applySelfReferencedHeldBallOffset(b)
	if e != nil {
		return prepared, e
	}
	return result, nil
}
