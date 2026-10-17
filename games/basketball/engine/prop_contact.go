package engine

import "fmt"

// bank5 $871C/$87EA requests a loose prop reaction without damaging a player,
// initializing motion, or applying the ordinary attacker animation feedback.
func applyOriginalPlayerPropContact(result *CombatStep, source, target uint8) error {
	if err := requestOriginalPropReaction(&result.State, target, result.State.Players[source].Player.Body.Position); err != nil {
		return err
	}
	prop := &result.State.Auxiliary[target-4]
	if !prop.Airborne && uint16(prop.Body.Position.Height>>8) == 0 {
		prop.Action = 13
	}
	return nil
}

// $87EA compares integer X only and preserves all motion and animation state.
func requestOriginalPropReaction(s *CombatState, target uint8, sourcePosition Position) error {
	prop := &s.Auxiliary[target-4]
	if prop.Action == 6 {
		if s.SpecialHolder < 0 || s.SpecialHolder >= 4 {
			return fmt.Errorf("original prop holder %d is outside players", s.SpecialHolder)
		}
		s.Players[s.SpecialHolder].Player.Action = ActionIdle
		s.Players[s.SpecialHolder].Player.Entered = false
		s.SpecialHolder = -1
	}
	prop.Action, prop.Entered = 7, false
	prop.FacingLeft = uint16(sourcePosition.X>>8) >= uint16(prop.Body.Position.X>>8)
	return nil
}

// $876F has its own fixed throw impulse, no endurance subtraction/reset, and
// no ordinary attack-variant feedback. Kind10's capture branch is separate.
func applyOriginalPropContact(result *CombatStep, source, target uint8, ctx CombatContext) error {
	kind := ctx.AuxiliaryKinds[source-4]
	if kind != 10 && kind != 11 && kind != 12 && kind != 13 && kind != 14 {
		return fmt.Errorf("original prop kind %d contact not reconstructed", kind)
	}
	s := &result.State
	attacker := &s.Auxiliary[source-4]
	// $8786..87BA: Kind10's thrown form can cover a ground-level player.
	// The original scans 0642..0647 with AND 7F, not the ordinary low-nibble
	// parent mask. It writes the fixed object reference 4, even for source5.
	if kind == 10 && target < 4 && uint16(s.Players[target].Player.Body.Position.Height>>8) == 0 && attacker.Action == 3 {
		if ctx.PlayerAttachments == nil {
			return fmt.Errorf("kind10 capture requires all incoming player attachments")
		}
		incoming := false
		for i := 0; i < 6; i++ {
			var link uint8
			if i < 4 {
				link = ctx.PlayerAttachments[i]
			} else {
				link = s.Auxiliary[i-4].Attachment
			}
			if link&128 != 0 && link&127 == target {
				incoming = true
				break
			}
		}
		if !incoming {
			s.Players[target].Player.Action, s.Players[target].Player.Entered = Action(0x50), false
			attacker.Action, attacker.Entered = 6, false
			result.ChangeCaptureCounter, result.CaptureCounter = true, 4
			return nil
		}
	}
	var receiverPosition Position
	if target >= 4 {
		receiverPosition = s.Auxiliary[target-4].Body.Position
		if err := requestOriginalPropReaction(s, target, attacker.Body.Position); err != nil {
			return err
		}
	} else {
		if int(ctx.CourtID) >= len(originalCombatDepthPush) {
			return fmt.Errorf("original prop contact court %d not reconstructed", ctx.CourtID)
		}
		player := &s.Players[target].Player
		receiverPosition = player.Body.Position
		player.Action, player.Entered = Action(0x49), false
		player.Body.Gravity = Pixel
		player.Body.Velocity = Velocity{X: 4 * Pixel, Height: 8 * Pixel}
		if attacker.FacingLeft {
			player.Body.Velocity.X = -4 * Pixel
		}
		if originalCombatDepthPush[ctx.CourtID] && uint8(player.Body.Position.Depth>>8) >= 72 {
			player.Body.Velocity.Depth = -Pixel
		}
		result.Sounds = append(result.Sounds, CombatSound{Source: source, Target: target, ID: 21})
	}
	if attacker.Action != 10 && attacker.Action < 16 {
		return requestOriginalPropReaction(s, source, receiverPosition)
	}
	return nil
}
