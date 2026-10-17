package engine

import (
	"errors"
	"fmt"
)

// AIShotContext is the actor/court input read by the original shot planner.
// JumpPhase is $0576; it is distinct from the shot-style cycle at $0572.
type AIShotContext struct {
	MountFlags      *[6]uint8
	JumpVelocity    Fixed
	Character       uint8
	Court           uint8
	JumpPhase       int8
	VerticalSpeed   int8
	HorizontalSpeed int8
	ShotSkill       uint8
}

type AIShotCommand struct {
	ChangeNextAction   bool
	NextAction         Action
	ClearDepthVelocity bool
	ChangeJumpPhase    bool
	JumpPhase          int8
	ChangeShotCycle    bool
	ShotCycle          int8
}

func originalShotJumpGrade(velocity Fixed) (uint8, error) {
	delta := velocity - 7*Pixel
	if delta < 0 || delta > 7*128 || delta%128 != 0 {
		return 0, errors.New("original shot planning requires a valid original jump profile")
	}
	return uint8(delta / 128), nil
}

// Bank 5 $B72D/$B77D: cumulative probabilities for each team's ordinary and
// attacking-half shot choices. These are gameplay constants, not ROM opcodes.
var originalShotThresholds = [2][8][8]uint8{
	{{128, 192, 255, 255, 255, 255, 0, 0}, {16, 64, 144, 255, 255, 255, 255, 0}, {16, 64, 144, 192, 255, 255, 255, 0}, {64, 80, 96, 192, 255, 255, 255, 0}, {64, 80, 96, 255, 255, 255, 0, 0}, {48, 64, 96, 192, 255, 255, 255, 0}, {32, 64, 208, 224, 255, 255, 0, 0}, {128, 160, 176, 255, 255, 255, 255, 255}},
	{{128, 192, 255, 255, 255, 255, 255, 255}, {16, 16, 16, 176, 176, 208, 255, 255}, {16, 64, 80, 128, 128, 192, 255, 255}, {16, 32, 80, 144, 144, 192, 255, 255}, {64, 80, 112, 112, 112, 128, 255, 255}, {48, 64, 96, 160, 160, 192, 255, 255}, {32, 64, 128, 128, 128, 192, 255, 255}, {128, 160, 255, 255, 255, 255, 255, 255}},
}

func chooseOriginalShotCycle(result *AIControlDecision, view AIControlView, ctx AIShotContext) error {
	var threshold uint8
	if view.TeamHasHuman {
		if ctx.ShotSkill >= 8 {
			return errors.New("invalid original shot skill")
		}
		threshold = (ctx.ShotSkill + 1) * 8
	} else {
		index := view.Difficulty >> 2
		if index >= 8 {
			return errors.New("invalid original AI shot difficulty")
		}
		threshold = [8]uint8{25, 50, 85, 100, 125, 150, 175, 200}[index]
	}
	var value byte
	result.Random, value = NextRandom(result.Random)
	cycle := int8(3)
	if value > threshold {
		cycle = 8
		if value&3 == 0 {
			cycle = 4
		}
	}
	result.ShotPlan.ChangeShotCycle, result.ShotPlan.ShotCycle = true, cycle
	return nil
}

// Original $B25D moves toward the court's raised platform, then repeatedly
// requests the original jump at its apex. It does not teleport the actor.
func approachOriginalShotPlatform(result *AIControlDecision, view AIControlView, ctx AIShotContext, zone CourtZone) {
	target, band := CourtZone(0x65), CourtZone(0x60)
	if ctx.Court == 5 {
		target, band = 0x55, 0x50
	}
	if zone == target {
		return
	}
	if zone&0x70 != band {
		result.Heading = HeadingDownRight
		if zone&0x70 < band {
			result.Heading = HeadingDownLeft
		}
		action := ActionRun
		if view.HasBall {
			action = ActionDribbleRun
		}
		chooseAIAction(result, action)
		return
	}
	jump := ActionJump
	if view.HasBall {
		jump = ActionCarrierJump
	}
	if result.Action&15 != 4 {
		result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, 0
		result.Action, result.ActionEntered = jump, false
		result.Heading = HeadingUp
	} else if ctx.JumpPhase >= 0 && view.Airborne && uint8(ctx.VerticalSpeed) < 1 {
		result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, ctx.JumpPhase-1
		result.Action, result.ActionEntered = jump, false
		result.Heading = HeadingUp
	}
}

// B58E is the original low-jump approach/landing setup used by plan7.
// The caller overrides its continuation with plan22 after every invocation.
func advanceOriginalLowJumpShot(result *AIControlDecision, view AIControlView, ctx AIShotContext, zone CourtZone) error {
	s := &result.Control
	switch result.Action {
	case ActionCarrierLand:
		if !result.ActionEntered {
			return nil
		}
		next := Action(0x5a)
		if ctx.Character&1 != 0 {
			next = 0x5c
		}
		result.ShotPlan.ChangeNextAction, result.ShotPlan.NextAction = true, next
		s.Mode, s.Entered, s.Plan = AIPossessionWait, false, 17
		result.ShotPlan.ChangeShotCycle, result.ShotPlan.ShotCycle = true, 0
		if !view.TeamHasHuman {
			if err := chooseOriginalShotCycle(result, view, ctx); err != nil {
				return err
			}
			cycle := int8(8)
			if result.ShotPlan.ShotCycle == 8 {
				cycle = 0
			}
			result.ShotPlan.ShotCycle = cycle
		}
	case ActionCarrierJump:
	default:
		if zone == 0x62 {
			result.Action, result.ActionEntered = ActionCarrierJump, false
			result.Heading = HeadingNone
		} else {
			chooseAIAction(result, ActionDribbleRun)
			s.Destination = 0x62
			result.Heading = steerOriginalZone(zone, s.Destination, view.Actor)
		}
	}
	return nil
}

// AB87 selects a launch shot once, then continues through B383 on every call.
// It deliberately preserves Counter, Destination and TrackedObject.
func advanceOriginalLaunchShot(result *AIControlDecision, view AIControlView, ctx AIShotContext) error {
	s := &result.Control
	if !s.Entered {
		index := view.Difficulty >> 2
		if index >= 8 {
			return errors.New("invalid original launch shot difficulty")
		}
		// ABC0 has seven intended pairs. The eighth index reads the adjacent
		// instruction bytes BD/72; retain that original behavior for difficulty28–31.
		thresholds := [8][2]uint8{{112, 128}, {80, 128}, {48, 96}, {32, 80}, {16, 64}, {8, 32}, {8, 16}, {189, 114}}
		s.Entered, s.Plan, s.PlanParameter = true, 17, 8
		var value byte
		result.Random, value = NextRandom(result.Random)
		value >>= 1
		if value >= thresholds[index][0] {
			s.PlanParameter++
			if value >= thresholds[index][1] {
				s.PlanParameter++
			}
		}
	}
	return advanceOriginalShotPlan(result, view, ctx)
}

// advanceOriginalShotPlan restores $A2AB and the observed $B383 branches.
// Unrestored choices fail explicitly; callers must not substitute their own AI.
func advanceOriginalShotPlan(result *AIControlDecision, view AIControlView, ctx AIShotContext) (err error) {
	initial := *result
	defer func() {
		if err != nil {
			*result = initial
		}
	}()
	zone, err := originalAIZone(view, view.Actor)
	if err != nil {
		return err
	}
	s := &result.Control
	if !s.Entered {
		s.Entered = true
		s.Counter = 0
		if view.TeamHasHuman {
			s.PlanParameter = 0
		} else if view.Team == 7 && view.Action == ActionAimAirShot {
			s.PlanParameter = 1
		} else {
			if view.Team >= 8 {
				return errors.New("invalid original AI shot team")
			}
			region := 0
			if zone >= 0x50 {
				region = 1
			}
			var value byte
			result.Random, value = NextRandom(result.Random)
			value <<= 1
			choice := 8
			for i, threshold := range originalShotThresholds[region][view.Team] {
				if value < threshold {
					choice = i
					break
				}
			}
			s.PlanParameter = [9]uint8{0, 1, 2, 14, 6, 3, 7, 0, 0}[choice]
		}
	}
	changeMode := func(mode AIMode) { s.Mode, s.Entered = mode, false }
	switch s.PlanParameter & 127 {
	case 0:
		if err = chooseOriginalShotCycle(result, view, ctx); err != nil {
			return err
		}
		result.Action, result.ActionEntered = ActionAimShot, false
		changeMode(AIRecover)
	case 1:
		switch result.Action {
		case ActionFalling:
		case ActionCarrierJump:
			if view.Airborne && (ctx.VerticalSpeed < 0 || uint8(ctx.VerticalSpeed) < 2) {
				result.Action, result.ActionEntered = ActionAimAirShot, false
				result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, -1
			}
		case ActionAimAirShot:
			if uint8(ctx.VerticalSpeed) < 2 {
				if err = chooseOriginalShotCycle(result, view, ctx); err != nil {
					return err
				}
				result.Action, result.ActionEntered = ActionAirShoot, false
				changeMode(AIPossessionWait)
				s.Plan = 17
				result.Heading = HeadingRight
				if view.Actor >= 2 {
					result.Heading = HeadingLeft
				}
			}
		default:
			result.Action, result.ActionEntered = ActionCarrierJump, false
			result.Heading = HeadingNone
		}
	case 2:
		// B413: a second jump at the first apex, then aim and release at
		// the second apex. The velocity tests are unsigned byte comparisons.
		switch result.Action {
		case ActionAimAirShot:
			if uint8(ctx.VerticalSpeed) == 0 {
				if err = chooseOriginalShotCycle(result, view, ctx); err != nil {
					return err
				}
				result.Action, result.ActionEntered = ActionAirShoot, false
				result.Heading = HeadingRight
				if view.Actor >= 2 {
					result.Heading = HeadingLeft
				}
				changeMode(AIPossessionWait)
				// Unlike plan1, B413 does not overwrite the existing Plan.
			}
		case ActionCarrierJump:
			if ctx.JumpPhase < 0 {
				if uint8(ctx.VerticalSpeed) < 3 {
					result.Action, result.ActionEntered = ActionAimAirShot, false
				}
			} else if view.Airborne && uint8(ctx.VerticalSpeed) < 2 {
				result.Action, result.ActionEntered = ActionCarrierJump, false
				result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, -1
			}
		default:
			result.Action, result.ActionEntered = ActionCarrierJump, false
			result.Heading = HeadingNone
			result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, 0
		}
	case 3:
		// B471 reads the opponent's low mount first. A missing mount
		// switches the next choice without consulting the jump grade.
		if ctx.MountFlags == nil {
			return errors.New("original shot plan3 requires current mount flags")
		}
		mount := uint8(1)
		if view.Actor >= 2 {
			mount = 0
		}
		if ctx.MountFlags[mount] == 0 {
			s.PlanParameter = 2
			break
		}
		grade, gradeErr := originalShotJumpGrade(ctx.JumpVelocity)
		if gradeErr != nil {
			return gradeErr
		}
		if grade < 3 {
			s.PlanParameter = 2
			break
		}
		chooseAIAction(result, ActionDribbleRun)
		result.Heading = HeadingRight
		if view.Actor >= 2 {
			result.Heading = HeadingLeft
		}
		band := zone & 0x70
		if band == 0x60 {
			result.Heading = HeadingNone
			changeMode(18)
			s.Plan = 17
		} else if band > 0x60 {
			result.Heading = HeadingLeft
			if view.Actor >= 2 {
				result.Heading = HeadingRight
			}
		}
	case 4:
		if err = chooseOriginalShotCycle(result, view, ctx); err != nil {
			return err
		}
		result.Action, result.ActionEntered = ActionAimShot, false
		s.Plan = 22
		changeMode(AIRecover)
		result.Heading = HeadingRight
		if view.Actor >= 2 {
			result.Heading = HeadingLeft
		}
	case 5:
		// B4D2 chooses plan2 for the next call; it does not jump now.
		s.PlanParameter = 2
		result.Heading = HeadingRight
		if view.Actor >= 2 {
			result.Heading = HeadingLeft
		}
		s.Plan = 22
	case 6:
		// B4E3 requests a fresh standing pass and resumes possession wait.
		// Heading, continuation plan and shot cycle are deliberately retained.
		result.Action, result.ActionEntered = ActionPass, false
		changeMode(AIPossessionWait)
	case 7:
		// 0619's grade and the existing profile's jump velocity correspond
		// exactly through the original 7*256 + grade*128 table.
		grade, gradeErr := originalShotJumpGrade(ctx.JumpVelocity)
		if gradeErr != nil {
			return gradeErr
		}
		if grade < 3 {
			if err = advanceOriginalLowJumpShot(result, view, ctx, zone); err != nil {
				return err
			}
			s.Plan = 22
		} else {
			chooseAIAction(result, ActionDribbleRun)
			s.Destination = 0x60
			result.Heading = steerOriginalZone(zone, s.Destination, view.Actor)
			if result.Heading == HeadingNone {
				changeMode(18)
				s.Plan = 22
			}
		}
	case 8:
		// B51D compares the high X-velocity sign against the actor's attack side.
		matching := (ctx.HorizontalSpeed < 0) == (view.Actor >= 2)
		if matching {
			if uint8(ctx.VerticalSpeed) == 0 {
				result.Heading = HeadingRight
				if view.Actor >= 2 {
					result.Heading = HeadingLeft
				}
				result.Action, result.ActionEntered = ActionAimAirShot, false
				changeMode(AIPossessionWait)
			}
			break
		}
		// B540 is a fallthrough, without first changing PlanParameter to 9.
		fallthrough
	case 9:
		if uint8(ctx.VerticalSpeed) == 0 {
			result.Action, result.ActionEntered = ActionCarrierJump, false
			result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, -1
			s.PlanParameter = 1
		}
	case 10:
		if uint8(ctx.VerticalSpeed) == 0 {
			s.PlanParameter = 2
		}

	case 14, 15:
		if s.PlanParameter&127 == 14 && ctx.Court == 1 {
			if zone == 0x65 && result.Action == ActionDribbleIdle {
				changeMode(18)
			} else {
				approachOriginalShotPlatform(result, view, ctx, zone)
			}
			return nil
		}
		switch ctx.Court {
		case 2, 3:
			if s.Counter == 0 {
				s.Destination = 0x55
				if ctx.Court == 3 {
					s.Destination = 0x45
				}
				result.Heading = steerOriginalZone(zone, s.Destination, view.Actor)
				if result.Heading != HeadingNone {
					chooseAIAction(result, ActionDribbleRun)
				} else {
					s.Counter++
				}
				return nil
			}
			if ctx.Court == 2 {
				if zone == 0x45 {
					s.Counter++
					if uint8(s.Counter) >= 16 {
						result.Action, result.ActionEntered = ActionCarrierRunningJump, false
						changeMode(AIPossessionWait)
						s.Plan = 17
						s.Counter++
					}
				} else {
					result.Heading = HeadingRight
					chooseAIAction(result, ActionDribbleRun)
				}
			} else if s.Counter == 1 {
				result.Heading = HeadingUpLeft
				chooseAIAction(result, ActionDribbleRun)
				if zone == 0x55 {
					result.Action, result.ActionEntered = ActionCarrierRunningJump, false
					changeMode(AIPossessionWait)
					s.Plan = 17
					s.Counter++
				}
			}
		case 5:
			if zone == 0x55 {
				changeMode(AIPossessionWait)
			}
			approachOriginalShotPlatform(result, view, ctx, zone)
		default:
			s.PlanParameter = 2
		}
	default:
		return fmt.Errorf("original AI shot-plan branch %d not reconstructed", s.PlanParameter&127)
	}
	return nil
}
