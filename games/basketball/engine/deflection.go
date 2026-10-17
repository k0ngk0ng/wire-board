package engine

import "errors"

// ResolveBallRequest preserves the original acceptance table independently of
// action physics. Ineligible requests remain pending; accepted requests reset
// the action exactly once. Requests outside the restored table remain explicit errors.
func ResolveBallRequest(original Ball) (Ball, error) {
	b := original
	if !b.Request.Pending {
		return b, nil
	}
	allowed := false
	switch b.Request.Action {
	case BallRest:
		allowed = b.Action == BallRolling
	case BallDribble:
		allowed = b.Action == BallHeld
	case BallHeld:
		allowed = b.Action == BallRest || b.Action == BallDribble || (b.Action >= BallRolling && b.Action <= BallDeflected)
	case BallPass, BallShot:
		allowed = b.Action == BallDribble || b.Action == BallHeld || b.Action == BallKnockback
	case BallRolling:
		allowed = b.Action == BallRest || b.Action == BallBounce || b.Action == BallDeflected
	case BallBounce:
		allowed = b.Action == BallRolling || b.Action == BallDeflected
	case BallKnockback:
		allowed = b.Action == BallRolling || b.Action == BallBounce || b.Action == BallDeflected
	case BallDeflected:
		allowed = b.Action >= BallPass && b.Action <= BallKnockback || b.Action == 9 || b.Action == 10
	case BallDropped:
		// The captured-player entry queues original request11 before the
		// ball phase. Its acceptance table permits only held/dribbling balls.
		allowed = b.Action == BallDribble || b.Action == BallHeld
	default:
		return original, errors.New("special ball request not reconstructed")
	}
	if allowed {
		b.Action, b.Entered = b.Request.Action, false
		b.Request.Pending = false
	}
	return b, nil
}

func StepDeflectedBall(original Ball, court FlatCourt) (Ball, error) {
	b := original
	if b.Action != BallDeflected {
		return original, errors.New("ball is not in deflected flight")
	}
	if !b.Entered {
		b.Entered = true
		b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 0, 0, 0
		if b.Body.Gravity < 0 {
			b.Body.Gravity = Pixel
			b.Body.Velocity.Height = 0
		} else if b.Body.Gravity == 0 {
			b.Body.Gravity = Pixel
		}
		if b.Deflection&DeflectX != 0 {
			b.Body.Velocity.X = signed16(-b.Body.Velocity.X)
		}
		if b.Deflection&DeflectHeight != 0 {
			b.Body.Velocity.Height = signed16(-b.Body.Velocity.Height)
		}
	}
	// Unlike X/height initialization, the original depth branch is reached
	// on each action call. The world contact phase owns the lifetime of flags.
	if b.Deflection&DeflectDepth != 0 {
		b.Body.Velocity.Depth = signed16(-b.Body.Velocity.Depth)
	}
	if err := advanceBallSpin(&b); err != nil {
		return original, err
	}
	if err := advanceBallOnCourt(&b, court); err != nil {
		return original, err
	}
	if !b.Airborne {
		b.Request = BallRequest{Action: BallBounce, Pending: true}
	}
	return b, nil
}
