package engine

import "errors"

const BallDropped BallAction = 11

type DroppedBallContext struct{ CourtID uint8 }

// LooseBallStep carries the $0561 exclusion reset separately from ball
// ownership. The hurt/action caller has already released its owner.
type LooseBallStep struct {
	Ball                 Ball
	ClearExcludedCatcher bool
}

func advanceDroppedBallSpin(b *Ball) error {
	// Original action-11 table $BE8E differs from ordinary shot/knockback
	// rotation: three ticks per pose and an initial tail of poses 2 and 3.
	frames := []AnimationFrame{{Pose: Pose{Frame: 0}, Ticks: 3}, {Pose: Pose{Frame: 1}, Ticks: 3}, {Pose: Pose{Frame: 2}, Ticks: 3}, {Pose: Pose{Frame: 3}, Ticks: 3}}
	state := b.Animation
	lead := state.Cursor >= 5
	if lead {
		frames = frames[2:]
		state.Cursor -= 5
	}
	step, err := AdvanceAnimation(AnimationSpec{Frames: frames, End: RepeatForever}, state, b.FacingLeft)
	if err != nil {
		return err
	}
	if lead && !(step.State.Cursor == 0 && step.State.Elapsed == 0) {
		step.State.Cursor += 5
	}
	b.Animation, b.Pose = step.State, step.Pose
	return nil
}

// StepDroppedBall restores $B8EC. Horizontal motion supplied by the hurt
// phase survives entry; the ball receives its original upward impulse here.
func StepDroppedBall(original Ball, ctx DroppedBallContext, court FlatCourt) (LooseBallStep, error) {
	initial := LooseBallStep{Ball: original}
	result := initial
	b := &result.Ball
	if b.Action != BallDropped {
		return initial, errors.New("ball is not in dropped flight")
	}
	if !b.Entered {
		thresholds := [8]uint8{95, 95, 72, 95, 95, 72, 95, 72}
		if int(ctx.CourtID) >= len(thresholds) {
			return initial, errors.New("original dropped-ball court not reconstructed")
		}
		b.Entered = true
		b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 5, 0, 0
		b.Body.Velocity.Height, b.Body.Gravity = 5*Pixel, Pixel
		b.BouncesRemaining = 48
		result.ClearExcludedCatcher = true
		if uint8(b.Body.Position.Depth>>8) >= thresholds[ctx.CourtID] {
			b.Body.Velocity.Depth = -Pixel
		}
	}
	if err := advanceDroppedBallSpin(b); err != nil {
		return initial, err
	}
	if err := advanceBallOnCourt(b, court); err != nil {
		return initial, err
	}
	if !b.Airborne {
		b.Action, b.Entered = BallBounce, false
	}
	return result, nil
}

// StepKnockedBackBall restores $B7A0. It reverses horizontal velocity once,
// preserves the existing vertical impulse, then spins and falls normally.
func StepKnockedBackBall(original Ball, court FlatCourt) (LooseBallStep, error) {
	initial := LooseBallStep{Ball: original}
	result := initial
	b := &result.Ball
	if b.Action != BallKnockback {
		return initial, errors.New("ball is not in knockback flight")
	}
	if !b.Entered {
		b.Entered = true
		b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 5, 0, 0
		b.Body.Velocity.X = signed16(-b.Body.Velocity.X)
		b.Body.Velocity.Depth = 0
		b.Body.Gravity = Pixel
		result.ClearExcludedCatcher = true
	}
	if err := advanceBallSpin(b); err != nil {
		return initial, err
	}
	if err := advanceBallOnCourt(b, court); err != nil {
		return initial, err
	}
	if !b.Airborne {
		b.BouncesRemaining = 132
		b.Action, b.Entered = BallBounce, false
	}
	return result, nil
}
