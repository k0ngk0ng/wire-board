package engine

import "fmt"

type ShotReleaseContext struct {
	Team, Character    uint8
	Zone               CourtZone
	EnhancementEnabled bool
	Equipment          uint8
	Random             RandomState
}
type ShotReleaseDecision struct {
	SpecialShot  uint8
	BallAction   BallAction
	ChangeCursor bool
	Cursor       uint8
	Random       RandomState
	UsedRandom   bool
}

var originalCharacterSpecialBalls = [8][4]BallAction{{23, 21, 17, 16}, {20, 20, 20, 20}, {15, 15, 15, 15}, {14, 14, 14, 14}, {13, 13, 13, 13}, {19, 19, 19, 19}, {22, 22, 22, 22}, {18, 18, 18, 18}}
var originalEquipmentShotModes = [32]uint8{255, 13, 2, 255, 4, 19, 20, 119, 255, 255, 255, 11, 255, 255, 255, 15, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 0, 255}

// ChooseOriginalShotRelease restores the distinct ground/air cycle tables and
// their original zone gate. A charged cycle outside the gate is ordinary play.
func ChooseOriginalShotRelease(action Action, cycle int8, ctx ShotReleaseContext) (ShotReleaseDecision, error) {
	result := ShotReleaseDecision{BallAction: BallShot, Random: ctx.Random}
	if action != ActionShoot && action != ActionAirShoot || ctx.Team >= 8 || ctx.Character >= 4 {
		return result, fmt.Errorf("invalid shot release profile")
	}
	if cycle < 0 {
		return result, nil
	}
	if cycle > 8 {
		return result, fmt.Errorf("shot cycle %d outside recovered release table", cycle)
	}
	// Cycle 8 is the run-start value. The original lookup reaches the adjacent
	// instruction byte (ground 0x60, air 0xbd); preserve that read explicitly.
	table := [9]uint8{128, 128, 128, 255, 255, 0, 0, 0, 0x60}
	if action == ActionAirShoot {
		table = [9]uint8{0, 0, 255, 255, 128, 128, 128, 0, 0xbd}
	}
	selected := table[cycle]
	if selected < 128 {
		return result, nil
	}
	if selected != 255 {
		result.SpecialShot = selected
		return result, nil
	}
	if ctx.Zone < 0x10 || ctx.Zone >= 0x60 || ctx.Zone&15 >= 5 {
		return result, nil
	}
	if action == ActionShoot {
		result.SpecialShot = 128
		result.Cursor = 2
	} else {
		result.Cursor = 3
	}
	result.ChangeCursor = true
	result.BallAction = originalCharacterSpecialBalls[ctx.Team][ctx.Character]
	if ctx.Team == 0 && ctx.EnhancementEnabled && ctx.Equipment < 128 {
		if ctx.Equipment >= 32 {
			return result, fmt.Errorf("shot equipment %d outside original catalog", ctx.Equipment)
		}
		mapped := originalEquipmentShotModes[ctx.Equipment]
		if mapped != ctx.Equipment && mapped < 128 {
			if mapped == 119 {
				var value byte
				result.Random, value = NextRandom(result.Random)
				result.UsedRandom = true
				value &= 0x16
				if value >= 13 {
					result.BallAction = BallAction(value)
				}
			} else if result.SpecialShot != 0 {
				result.BallAction = BallAction(mapped)
			}
		}
	}
	return result, nil
}

func animateOriginalShotRelease(p *Player, profile CharacterProfile, action Action) error {
	secondary := uint8(2)
	if action == ActionAirShoot {
		secondary = 3
	}
	if p.Animation.Cursor < secondary {
		return animatePlayer(p, profile, action)
	}
	spec := AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 20}, Ticks: 8}}, End: RepeatRemaining}
	if action == ActionAirShoot {
		spec.Frames = append(spec.Frames, AnimationFrame{Pose: Pose{Frame: 4}, Ticks: 255})
		spec.End = RepeatForever
	}
	state := p.Animation
	state.Cursor -= secondary
	next, err := AdvanceAnimation(spec, state, p.FacingLeft)
	if err != nil {
		return err
	}
	if next.State.Cursor != 0 || next.State.Elapsed != 0 {
		next.State.Cursor += secondary
	}
	p.Animation, p.Pose = next.State, next.Pose
	if next.ChangeAction {
		p.Action, p.Entered = next.Action, false
	}
	return nil
}
