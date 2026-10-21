package engine

// recoverUnsupportedLooseBall is an explicit platform rule repair, not an
// original ROM handler. The caller gates it with the persisted match policy
// and only invokes it after a successful active-playing ball dispatch.
//
// The original rolling/rest handlers can lose support with zero gravity. A
// narrowly eligible free ball enters the already reconstructed original action
// 10 on its NEXT dispatch, retaining coordinates, horizontal motion and RNG.
// That handler supplies its source falling parameters and normal bounce tail.
func recoverUnsupportedLooseBall(s *WorldActionState, courtID uint8) error {
	b := &s.Ball
	if s.Holder != 6 || s.BallLink&128 != 0 || !b.Airborne ||
		(b.Action != BallRest && b.Action != BallRolling) || b.Request.Pending ||
		b.Body.Velocity.Height != 0 || b.Body.Gravity != 0 {
		return nil
	}
	surface, err := OriginalCourtSurface(courtID, b.Body.Position.X, b.Body.Position.Depth)
	if err != nil {
		return err
	}
	// Original terrain compares integer height. A fractional offset above the
	// same integer support is not an unsupported suspension.
	if b.Body.Position.Height>>8 <= surface.Height()>>8 {
		return nil
	}
	b.Action, b.Entered = 10, false
	return nil
}
