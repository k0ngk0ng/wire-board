package engine

import "fmt"

func runtimeConfiguredGrades(c RuntimeConfig) ([4]CharacterGrades, error) {
	var result [4]CharacterGrades
	for i := range result {
		g, e := OriginalCharacterGrades(c.Match.Teams[i/2], c.Match.Characters[i])
		if e != nil {
			return result, e
		}
		if i < 2 && c.Match.Mode&128 == 0 {
			g, e = ApplyOriginalEquipmentGrades(g, c.Match.Equipment[c.Match.Characters[i]])
			if e != nil {
				return result, e
			}
		}
		result[i] = g
	}
	return result, nil
}
func validateRuntimeOpeningConfig(c RuntimeConfig, s OpeningState) error {
	if s.Equipment != c.Match.Equipment || s.Transition.Mode != c.Match.Mode {
		return fmt.Errorf("opening equipment or mode differs from configuration")
	}
	grades, e := runtimeConfiguredGrades(c)
	if e != nil {
		return e
	}
	if grades != s.Grades {
		return fmt.Errorf("opening grades differ from configured characters")
	}
	return nil
}
func validateRuntimePlayingConfig(c RuntimeConfig, s *RuntimePlayingState) error {
	if c.Campaign != nil {
		r := s.Context.PursuitReserved
		if r == nil || r.XHigh88 == nil || *r.XHigh88 != uint8(c.Campaign.Inventory[7]) {
			return fmt.Errorf("playing inventory alias differs from campaign preparation")
		}
	}
	if c.Match.Court == 5 && (s.World.LaunchColumn == nil || s.World.LaunchColumn.Column >= 64 || s.World.LaunchColumn.Phase > 1) {
		return fmt.Errorf("court5 has an invalid or missing generated column source")
	}
	if c.Match.Court == 4 {
		for i, auxiliary := range s.World.Preparation.Actions.Auxiliary {
			if auxiliary.Kind < 10 || auxiliary.Kind > 14 {
				return fmt.Errorf("court4 auxiliary %d has an invalid or missing dynamic kind", i)
			}
		}
	}
	a := s.Context.Actions
	if a.RecoverUnsupportedBall != c.RecoverUnsupportedBall {
		return fmt.Errorf("playing ball recovery policy differs from configuration")
	}
	if a.EnhancementEnabled != (c.Match.Mode&128 == 0) || a.Equipment != uint8(c.Match.Equipment[0][0]) {
		return fmt.Errorf("playing equipment mode differs from configuration")
	}
	grades, e := runtimeConfiguredGrades(c)
	if e != nil {
		return e
	}
	for i, g := range grades {
		control := uint8(128)
		if c.Match.ControllerAssignments[i] >= 0 {
			control = uint8(c.Match.ControllerAssignments[i])
		}
		if a.Controls[i] != control || s.Context.AttackGrades[i] != g.Attack || a.RecoveryGrades[i] != g.Recovery {
			return fmt.Errorf("playing actor %d controls or grades differ from configuration", i)
		}
		run, _ := RunSpeedForGrade(g.Run)
		jump, _ := JumpVelocityForGrade(g.Jump)
		if a.Profiles[i].RunSpeed != run || a.Profiles[i].JumpVelocity != jump || a.Profiles[i].ShotSkill != g.Shot || a.Profiles[i].Animations == nil {
			return fmt.Errorf("playing actor %d has an invalid character profile", i)
		}
		if a.Profiles[i].ScoringBonus != (i < 2 && c.Match.Equipment[c.Match.Characters[i]][2] == 27) {
			return fmt.Errorf("playing actor %d scoring bonus differs from configured shoes", i)
		}
		if a.ShotEquipment[i] != uint8(c.Match.Equipment[c.Match.Characters[i]][0]) || a.Enhancements[i] != uint8(c.Match.Equipment[c.Match.Characters[i]][1]) || s.Context.JumpEnhancements[i] != uint8(c.Match.Equipment[c.Match.Characters[i]][2]) {
			return fmt.Errorf("playing actor equipment differs from configuration")
		}
	}
	for i, v := range a.OtherObjectArcFlags {
		flat := i + 5
		if v != uint8(c.Match.Equipment[flat/3][flat%3]) {
			return fmt.Errorf("playing arc equipment alias differs from configuration")
		}
	}
	if s.Context.Difficulty != s.Match.Period {
		return fmt.Errorf("playing difficulty differs from original period")
	}
	return validateProjection(s.World.Projection, ProjectionContext{LayoutID: s.Context.LayoutID, CameraY: s.World.Camera.Y})
}
