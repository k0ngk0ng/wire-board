package engine

import "fmt"

// CombatActor adds the original contact-response parameters without folding
// attack rules into the shared Player. Endurance is $0611, a saturating
// stagger reserve; its reset to 255 on launch is not conventional hit points.
type CombatActor struct {
	Player        Player `json:"player"`
	AttackVariant uint8  `json:"attackVariant"`
	AttackGrade   uint8  `json:"attackGrade"`
	Endurance     uint8  `json:"endurance"`
}

type CombatState struct {
	Players       [4]CombatActor    `json:"players"`
	Auxiliary     [2]AuxiliaryState `json:"auxiliary"`
	SpecialHolder int8              `json:"specialHolder"`
}

type CombatContext struct {
	// Kind10 capture scans all four player links and both auxiliary links.
	// Nil leaves the player portion unknown for isolated callers.
	PlayerAttachments *[4]uint8
	// Candidates preserve the completed search order for emitters 0..5.
	// Any negative byte means no contact, matching the original sign test.
	Candidates     [6]int8
	CourtID        uint8
	AuxiliaryKinds [2]uint8
}

type CombatSound struct{ Source, Target, ID uint8 }
type CombatStep struct {
	ChangeCaptureCounter bool
	CaptureCounter       uint8
	State                CombatState
	Sounds               []CombatSound
}

func applyOriginalPlayerContact(result *CombatStep, source, target uint8, court uint8) error {
	attacker, receiver := &result.State.Players[source], &result.State.Players[target]
	if int(attacker.AttackVariant) >= len(originalCombatVariants) {
		return fmt.Errorf("original attack response variant %d not recovered", attacker.AttackVariant)
	}
	profile := originalCombatProfiles[originalCombatVariants[attacker.AttackVariant]]
	player := &receiver.Player
	sound := profile[3]
	stagger := profile[1] != 0 && player.Body.Position.Height>>8 == 0 && player.Action != Action(0x47) && attacker.AttackGrade < 7
	if stagger {
		damage := originalEnduranceDamage[attacker.AttackGrade]
		if receiver.Endurance >= damage {
			receiver.Endurance -= damage
		} else {
			receiver.Endurance = 0
		}
		player.Action, player.Entered = Action(0x48), false
		player.Body.Velocity.X = Fixed(int8(profile[0])) * Pixel
	} else {
		if int(court) >= len(originalCombatDepthPush) {
			return fmt.Errorf("original combat court %d not recovered", court)
		}
		player.Action, player.Entered = Action(0x49), false
		player.Body.Gravity = Pixel
		player.Body.Velocity = Velocity{X: Fixed(int8(profile[4])) * Pixel, Height: Fixed(int8(profile[5])) * Pixel}
		if originalCombatDepthPush[court] && uint8(player.Body.Position.Depth>>8) >= 72 {
			player.Body.Velocity.Depth = -Pixel
		}
		receiver.Endurance = 255
		sound = 21
	}
	if attacker.Player.FacingLeft {
		player.Body.Velocity.X = signed16(-player.Body.Velocity.X)
	}
	result.Sounds = append(result.Sounds, CombatSound{Source: source, Target: target, ID: sound})
	feedback := int8(profile[2])
	if feedback >= 0 {
		if feedback != 0 {
			attacker.Player.Animation.Cursor += uint8(feedback)
			attacker.Player.Animation.Elapsed = 0
		}
	} else {
		// $8738 preserves the old animation clock for leftward recoil but
		// writes 255 for the opposite branch. Do not normalize this asymmetry.
		if uint16(attacker.Player.Body.Velocity.X)&0x8000 == 0 {
			attacker.Player.Body.Velocity.X = -384
			attacker.Player.Animation.Elapsed = 255
		} else {
			attacker.Player.Body.Velocity.X = 384
		}
	}
	return nil
}

// StepOriginalCombat is the original $84B9..$84C3 response phase. It consumes
// already-found contacts, changes actions/impulses, and returns sound events.
// It neither advances movement nor drops a held ball; those happen later in
// the original action handlers. Unrestored prop kinds remain explicit errors.
func StepOriginalCombat(original CombatState, ctx CombatContext) (CombatStep, error) {
	initial := CombatStep{State: original}
	result := initial
	for source, target := range ctx.Candidates {
		if target < 0 {
			continue
		}
		if target >= 6 {
			return initial, fmt.Errorf("original prop contact response %d -> %d not reconstructed", source, target)
		}
		if source >= 4 {
			if source == int(target) {
				return initial, fmt.Errorf("invalid self contact for prop %d", source)
			}
			if err := applyOriginalPropContact(&result, uint8(source), uint8(target), ctx); err != nil {
				return initial, err
			}
			continue
		}
		if target >= 4 {
			if err := applyOriginalPlayerPropContact(&result, uint8(source), uint8(target)); err != nil {
				return initial, err
			}
			continue
		}
		if source == int(target) {
			return initial, fmt.Errorf("invalid self contact for player %d", source)
		}
		if err := applyOriginalPlayerContact(&result, uint8(source), uint8(target), ctx.CourtID); err != nil {
			return initial, err
		}
	}
	return result, nil
}
