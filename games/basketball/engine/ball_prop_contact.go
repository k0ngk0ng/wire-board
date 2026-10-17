package engine

import "fmt"

// Bank5 $8596 handles a ball candidate of 4/5 before the player-catch branch's
// ownership guards. It requests later ball/prop reactions; it does not itself
// integrate either object, transfer possession, or play an impact sound.
func applyOriginalBallPropContact(result *InteractionStep, candidate int8) error {
	if candidate < 4 || candidate > 5 {
		return fmt.Errorf("ball-contact auxiliary candidate %d outside original objects", candidate)
	}
	s := &result.State
	prop := s.Auxiliary[candidate-4]
	if s.Ball.Action == BallDeflected || s.Ball.Action == 10 {
		// For actions 8/10, an idle prop must be elevated and in front of depth 71.
		// Integer coordinate bytes are tested; fractional height alone is zero.
		if prop.Action != 0 || uint16(prop.Body.Position.Height>>8) == 0 || uint8(prop.Body.Position.Depth>>8) >= 71 {
			return nil
		}
	}
	if int8(s.Ball.Body.Gravity>>8) <= 0 {
		s.Ball.Body.Gravity = Pixel
	}
	s.Ball.Action, s.Ball.Entered = BallKnockback, false
	reaction := CombatState{Players: s.Players, Auxiliary: s.Auxiliary, SpecialHolder: s.SpecialHolder}
	if err := requestOriginalPropReaction(&reaction, uint8(candidate), s.Ball.Body.Position); err != nil {
		return err
	}
	s.Players, s.Auxiliary, s.SpecialHolder = reaction.Players, reaction.Auxiliary, reaction.SpecialHolder
	return nil
}
