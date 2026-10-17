package engine

import "fmt"

// stepSelfReferencedDribble restores B53E when 053F is the ball's own object
// index, 6. An already-entered player dribble can write ball action1 without
// reclaiming possession (907E..90B0). B552..B57B then copies the ball's own
// position back to itself. It does not choose a player or run ball physics.
func stepSelfReferencedDribble(original Ball) (Ball, error) {
	if original.Action != BallDribble {
		return original, fmt.Errorf("self-reference requires dribble action, got %d", original.Action)
	}
	// The dribble branch reads only this position. The Player value is a view
	// of the source coordinates, not a fabricated carrier or world actor.
	return StepHeldBallOnCourt(original, Player{Body: original.Body}, NewYorkBallCourt())
}
