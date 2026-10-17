package engine

import "errors"

type RestBallContext struct{ CameraShake uint8 }

// StepRestingBall restores bank6 B52E. Rest does not initialize or advance an
// animation, and does not set Entered. EC4A stops only horizontal movement;
// gravity and vertical movement still pass through the original terrain phase.
func StepRestingBall(original Ball, ctx RestBallContext, court FlatCourt) (Ball, error) {
	if original.Action != BallRest {
		return original, errors.New("ball is not resting")
	}
	b := original
	if ctx.CameraShake == 0 {
		b.Pose = Pose{}
	}
	b.Body.Velocity.X, b.Body.Velocity.Depth = 0, 0
	if err := advanceBallOnCourt(&b, court); err != nil {
		return original, err
	}
	return b, nil
}

// StepRollingBall restores B6A1 and fixed EA5C parameter7. The X stop test is
// before friction; depth friction deliberately crosses zero without clamping.
// Only entry clears the excluded catcher, independently of ball ownership.
func StepRollingBall(original Ball, court FlatCourt) (LooseBallStep, error) {
	initial := LooseBallStep{Ball: original}
	if original.Action != BallRolling {
		return initial, errors.New("ball is not rolling")
	}
	result := initial
	b := &result.Ball
	if !b.Entered {
		b.Entered = true
		b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 0, 0, 0
		result.ClearExcludedCatcher = true
	}
	if err := advanceBallSpin(b); err != nil {
		return initial, err
	}
	if err := advanceBallOnCourt(b, court); err != nil {
		return initial, err
	}
	v := &b.Body.Velocity
	if v.X == 0 {
		b.Action, b.Entered = BallRest, false
	} else {
		// Original table EAC1[7] points to EB09:
		// +0040,+0040,-0040,-0040 for left/back/right/front.
		if v.X < 0 {
			v.X = min(v.X+64, 0)
		} else {
			v.X = max(v.X-64, 0)
		}
		if v.Depth < 0 {
			v.Depth = signed16(v.Depth + 64)
		} else if v.Depth > 0 {
			v.Depth = signed16(v.Depth - 64)
		}
	}
	return result, nil
}
