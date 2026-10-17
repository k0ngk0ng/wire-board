package engine

import "fmt"

// AITeammateControlCommand records AAC7's writes to the other seat. Applying
// the command is the input phase's job, so a failed decision remains atomic.
type AITeammateControlCommand struct {
	Change     bool
	Actor      uint8
	Mode       AIMode
	ChangePlan bool
	Plan       AIMode
}

// ABEF checks the character/height opportunity only on entry. It requests the
// original air aim and shot-plan continuation; it never launches the ball.
func advanceOriginalTeamMode33(result *AIControlDecision, view AIControlView) error {
	s := &result.Control
	if !s.Entered {
		s.Entered = true
		if view.ShotContext.Character&1 != 0 && view.Airborne {
			zone, err := originalAIZone(view, view.Actor)
			if err != nil {
				return err
			}
			if zone&15 < 5 && uint8(view.Position.Height>>16) < 2 && uint8(view.Position.Height>>8) >= 8 {
				result.Action, result.ActionEntered = Action(0x1a), false
				s.Mode, s.Entered, s.PlanParameter = 6, false, 1
				result.Heading = HeadingRight
				if view.Actor >= 2 {
					result.Heading = HeadingLeft
				}
				return nil
			}
		}
	}
	if !view.Airborne {
		s.Mode, s.Entered = 17, false
	}
	return nil
}

// AAC7 preserves the teammate's independent control state. A ground teammate
// receives only mode28; an airborne teammate receives mode0 and plan28.
func advanceOriginalTeamMode34(result *AIControlDecision, view AIControlView) error {
	s := &result.Control
	if !s.Entered {
		s.Entered = true
		var value byte
		result.Random, value = NextRandom(result.Random)
		if value&128 != 0 {
			s.Mode, s.Entered = 17, false
			return nil
		}
		s.Plan, s.Destination, s.TrackedObject = 17, 4, int8(view.Actor^1)
		if view.TeamAI == nil {
			return fmt.Errorf("mode34 requires current teammate AI state")
		}
		peer := view.Actor ^ 1
		if view.TeamAI.Players[peer].Mode == 40 {
			s.Mode, s.Entered = 17, false
			return nil
		}
		if view.PursuitWorld == nil {
			return fmt.Errorf("mode34 requires current teammate airborne state")
		}
		result.TeammateControl = AITeammateControlCommand{Change: true, Actor: peer, Mode: 28}
		if view.PursuitWorld.Actors[peer].Player.Airborne {
			result.TeammateControl.Mode = 0
			result.TeammateControl.ChangePlan = true
			result.TeammateControl.Plan = 28
		}
	}
	switch result.Action {
	case Action(0x3d):
		return nil
	case Action(0x3e):
		s.Mode, s.Entered = 31, false
		return nil
	case ActionCarrierJump:
		result.Heading = HeadingRight
		if view.Actor >= 2 {
			result.Heading = HeadingLeft
		}
		grade, err := originalShotJumpGrade(view.ShotContext.JumpVelocity)
		if err != nil {
			return err
		}
		if grade < 4 && view.ShotContext.JumpPhase != -1 && uint8(view.ShotContext.VerticalSpeed) < 1 {
			result.Action, result.ActionEntered = ActionCarrierJump, false
			result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, -1
			result.Heading = HeadingNone
		}
		return nil
	}
	if s.TrackedObject < 0 || s.TrackedObject >= 4 {
		return fmt.Errorf("mode34 tracked teammate %d has no recovered position", s.TrackedObject)
	}
	if steerOriginalPoint(result, view.Position, view.Players[s.TrackedObject], 16, 8, uint8(s.Destination)).Reached {
		result.Engagement.StopPlanar = true
		result.Heading = HeadingNone
		if view.ShotContext.JumpPhase >= 0 {
			result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, -2
			result.Action, result.ActionEntered = ActionCarrierJump, false
		}
	} else if !view.Airborne {
		chooseAIAction(result, ActionDribbleRun)
	}
	return nil
}

// AC3A enters AE96 directly, without mode7's possession/link handoffs or its
// counter/jump-phase entry initialization.
func advanceOriginalTeamMode35(result *AIControlDecision, view AIControlView) error {
	s := &result.Control
	if !s.Entered {
		s.Entered = true
		if view.ShotContext.Character&1 == 0 {
			s.Mode, s.Entered = 2, false
			return nil
		}
		s.Plan = 35
		if uint8(view.BallAction)&7 == 0 {
			s.Mode, s.Entered = 2, false
			return nil
		}
		index := view.Difficulty >> 2
		if index >= 8 {
			return fmt.Errorf("mode35 difficulty %d is outside the original threshold table", view.Difficulty)
		}
		var value byte
		result.Random, value = NextRandom(result.Random)
		if value >= [8]uint8{128, 144, 160, 176, 192, 208, 255, 255}[index] {
			s.Mode, s.Entered = 2, false
			return nil
		}
		s.PlanParameter, s.TrackedObject = 7, 6
	}
	return advanceOriginalEngagementPlan(result, view, AIEngagementContext{VerticalSpeed: view.ShotContext.VerticalSpeed})
}
