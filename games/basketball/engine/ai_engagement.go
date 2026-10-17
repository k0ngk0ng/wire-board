package engine

import "errors"

// TeammateLink is the other teammate's attachment selector ($0642 + actor^1).
// VerticalSpeed is the signed integer byte of the actor's height velocity.
type AIEngagementContext struct {
	TeammateLink  uint8
	VerticalSpeed int8
}

// These commands reproduce original AI writes before the actor action phase.
// They neither resolve an attack nor assign contact, damage, or score.
type AIEngagementCommand struct {
	StopPlanar       bool
	ChangeNextAction bool
	NextAction       Action
}

func advanceOriginalEngagement(result *AIControlDecision, view AIControlView, ctx AIEngagementContext) error {
	if view.Actor >= 4 {
		return errors.New("invalid original engagement actor")
	}
	s := &result.Control
	mode := func(m AIMode) { s.Mode, s.Entered = m, false }
	if !s.Entered {
		s.Entered = true
		s.Counter = 8
		result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, 0
	}
	if view.HasBall {
		mode(1)
		return nil
	}
	if ctx.TeammateLink&128 != 0 && ctx.TeammateLink&127 == view.Actor {
		mode(28)
		return nil
	}
	return advanceOriginalEngagementPlan(result, view, ctx)
}

// AE96 is shared by mode7 and mode35; only the caller performs mode entry.
func advanceOriginalEngagementPlan(result *AIControlDecision, view AIControlView, ctx AIEngagementContext) error {
	s := &result.Control
	mode := func(m AIMode) { s.Mode, s.Entered = m, false }
	parameter := s.PlanParameter
	if parameter >= 12 {
		return errors.New("original engagement action table parameter outside recovered domain")
	}
	attack := [12]Action{0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x68, 0x61, 0x65}[parameter]
	face := func() error {
		if s.TrackedObject < 0 {
			return errors.New("original engagement target has no recovered coordinate source")
		}
		// $B1E9 reads the selected index's two integer X bytes, including
		// hoop7 and the canonical aliases already audited for $B100.
		targetX := uint16(view.BallPosition.X >> 8)
		if s.TrackedObject != 6 {
			var err error
			targetX, err = originalPursuitX(view, uint8(s.TrackedObject))
			if err != nil {
				return err
			}
		}
		result.FacingLeft = uint16(view.Position.X>>8) >= targetX
		result.Heading = HeadingRight
		if result.FacingLeft {
			result.Heading = HeadingLeft
		}
		return nil
	}
	startAttack := func(m AIMode) { result.Action, result.ActionEntered = attack, false; mode(m) }
	switch parameter {
	case 0, 4:
		if err := face(); err != nil {
			return err
		}
		result.Engagement.StopPlanar = true
		startAttack(0)
	case 2, 6:
		startAttack(20)
	case 1, 5:
		changed := result.Action != ActionJump
		chooseAIAction(result, ActionJump)
		if changed {
			result.Heading = HeadingNone
			return nil
		}
		if err := face(); err != nil {
			return err
		}
		if view.Airborne && uint8(ctx.VerticalSpeed) < 3 {
			startAttack(20)
		}
	case 3, 7:
		if result.Action == ActionLand {
			if !result.ActionEntered {
				return nil
			}
			if err := face(); err != nil {
				return err
			}
			result.Engagement.ChangeNextAction, result.Engagement.NextAction = true, attack
			mode(20)
		} else if result.Action != ActionJump {
			result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, -1
			result.Heading = HeadingNone
			result.Action, result.ActionEntered = ActionJump, false
		}
	case 8, 9:
		startAttack(0)
	case 10, 11:
		changed := result.Action != ActionRunningJump
		chooseAIAction(result, ActionRunningJump)
		if changed {
			return face()
		}
		if view.Airborne && uint8(view.Position.Height>>8) >= 16 {
			startAttack(20)
		}
	}
	return nil
}
