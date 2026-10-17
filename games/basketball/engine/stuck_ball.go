package engine

import "fmt"

type StuckBallStep struct {
	Ball          Ball
	ReleaseHolder bool
}

// StepStuckBall restores bank 6 B7EE, a ball trapped on the selected rim.
// The shared bounce byte is its 96-update countdown. This is separate from a
// court device's ordinary-shot relaunch; reaching zero enters action 10.
func StepStuckBall(original Ball, hoopHeight Fixed, court FlatCourt) (StuckBallStep, error) {
	initial := StuckBallStep{Ball: original}
	result := initial
	b := &result.Ball
	if b.Action != BallStuck {
		return initial, fmt.Errorf("ball is not stuck on a rim")
	}
	if !b.Entered {
		b.Entered = true
		b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 0, 0, 0
		b.Body.Velocity = Velocity{X: Pixel}
		b.Body.Gravity = 0
		b.BouncesRemaining = 96
		b.Body.Position.Depth = 42*Pixel + (b.Body.Position.Depth & 255)
		b.Body.Position.Height = signed24((hoopHeight &^ 255) + 14*Pixel + (b.Body.Position.Height & 255))
		result.ReleaseHolder = true
	}
	step, err := AdvanceAnimation(AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 0}, Ticks: 1}, {Pose: Pose{Frame: 1}, Ticks: 3}, {Pose: Pose{Frame: 2}, Ticks: 3}, {Pose: Pose{Frame: 3}, Ticks: 3}, {Pose: Pose{Frame: 0}, Ticks: 2}}, End: RepeatForever}, b.Animation, b.FacingLeft)
	if err != nil {
		return initial, err
	}
	b.Animation, b.Pose = step.State, step.Pose
	half := uint8(b.Body.Position.X >> 16)
	if half > 1 {
		return initial, fmt.Errorf("stuck ball outside original court halves")
	}
	bounds := [2][3]uint8{{54, 70, 62}, {198, 214, 206}}[half]
	x := uint8(b.Body.Position.X >> 8)
	reflectVelocity := false
	if x < bounds[0] {
		x = bounds[0] + 1
		reflectVelocity = b.Body.Velocity.X < 0
	} else if x >= bounds[1] {
		x = bounds[1] - 1
		reflectVelocity = b.Body.Velocity.X >= 0
	}
	if reflectVelocity {
		b.Body.Velocity.X = signed16(-b.Body.Velocity.X)
		b.Body.Position.X = Fixed(half)*65536 + Fixed(x)*Pixel + (b.Body.Position.X & 255)
	}
	if x < bounds[2] {
		b.Body.Velocity.X = signed16(b.Body.Velocity.X + 32)
	} else {
		b.Body.Velocity.X = signed16(b.Body.Velocity.X - 32)
	}
	b.BouncesRemaining--
	if b.BouncesRemaining == 0 {
		b.Action, b.Entered = 10, false
	}
	if err := advanceBallOnCourt(b, court); err != nil {
		return initial, err
	}
	return result, nil
}
