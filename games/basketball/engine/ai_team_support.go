package engine

// advanceOriginalTeamSupport restores A95F. The AI only requests the original
// support actions; attachment changes and player physics remain action phases.
func advanceOriginalTeamSupport(result *AIControlDecision, view AIControlView) (err error) {
	initial := *result
	defer func() {
		if err != nil {
			*result = initial
		}
	}()
	s := &result.Control
	if !s.Entered {
		s.Entered = true
		chooseAIAction(result, ActionRun)
	}
	if view.TeamHasHuman && view.TeammateLink&128 == 0 {
		s.Mode, s.Entered = 22, false
		return nil
	}
	if result.Action != ActionIdle {
		zone, e := originalAIZone(view, view.Actor)
		if e != nil {
			return e
		}
		var buttons Buttons
		band := zone & 0xf0
		if band < 0x30 {
			buttons = ButtonRight
		} else if band >= 0x50 {
			buttons = ButtonLeft
		}
		if buttons != 0 && view.Actor >= 2 {
			buttons ^= ButtonLeft | ButtonRight
		}
		if zone&15 >= 5 {
			buttons |= ButtonDown
		}
		result.Heading = HeadingForButtons(buttons)
		if result.Heading == HeadingNone {
			result.Action, result.ActionEntered = ActionIdle, false
		}
		return nil
	}
	if view.TeammateLink&128 == 0 || view.TeammateLink&15 != view.Actor {
		return nil
	}
	if view.BallHolder == int8(view.Actor) {
		s.Mode, s.Entered = 10, false
		return nil
	}
	result.Action, result.ActionEntered = Action(7), false
	if view.BallHolder == int8(view.Actor^1) {
		zone, e := originalAIZone(view, view.Actor)
		if e != nil {
			return e
		}
		result.Action = Action(9)
		result.Heading = HeadingRight
		if zone>>4 == 7 {
			result.Heading = HeadingLeft
		}
		if view.Actor >= 2 {
			if result.Heading == HeadingRight {
				result.Heading = HeadingLeft
			} else {
				result.Heading = HeadingRight
			}
		}
	}
	if view.TeamHasHuman {
		s.Mode, s.Entered = 22, false
	}
	return nil
}
