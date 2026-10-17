package engine

import "errors"

// $ABCE/$B213 and the eight entries at $9620 select the next personal
// strategy using the current team's signed score-difference tier. This phase
// consumes no RNG and preserves the existing action and recovery plan.
func advanceOriginalScoreStrategy(result *AIControlDecision, view AIControlView) error {
	if result.Control.Entered {
		return nil
	}
	if view.ScoreDifference&127 > 3 {
		return errors.New("original score-strategy tier outside table")
	}
	index := view.ScoreDifference
	if index != 0 && view.Actor >= 2 {
		index ^= 128
	}
	if index&128 != 0 {
		index = (index & 127) + 4
	}
	result.Control.Mode = [8]AIMode{12, 12, 8, 8, 12, 12, 9, 9}[index]
	result.Control.Entered = false
	return nil
}
