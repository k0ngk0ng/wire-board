package engine

import "fmt"

// selectOriginalCommandOpponent restores AD17, including its direct mode41
// entry. The original compares only the low byte of half the absolute integer
// X distance. Ties select the second opponent; actions49/4A score255. It does
// not mark the mode entered, change actions, or consume random values.
func selectOriginalCommandOpponent(result *AIControlDecision, view AIControlView) error {
	if view.Actor >= 4 {
		return fmt.Errorf("invalid command opponent actor %d", view.Actor)
	}
	first := (view.Actor & 2) ^ 2
	distance := func(actor uint8) uint8 {
		action := view.PlayerActions[actor] & 127
		if action == 0x49 || action == 0x4a {
			return 255
		}
		dx := int32(int16(uint16(view.Position.X>>8) - uint16(view.Players[actor].X>>8)))
		return uint8(uint32(absPass(dx)) >> 1)
	}
	selected := first
	if distance(first) >= distance(first+1) {
		selected++
	}
	result.Control.TrackedObject = int8(selected)
	return nil
}

// advanceOriginalCommandAttack restores A63D for ordinary command modes36–38.
// Mode15 shares the ROM address but indexes beyond the intended three-choice
// selector tables. It is deliberately not treated as an alias of mode36.
func advanceOriginalCommandAttack(result *AIControlDecision, view AIControlView) (err error) {
	initial := *result
	defer func() {
		if err != nil {
			*result = initial
		}
	}()
	s := &result.Control
	if view.Actor >= 4 || s.Mode < 36 || s.Mode > 38 {
		return fmt.Errorf("command attack actor/mode outside restored domain: %d/%d", view.Actor, s.Mode)
	}
	if !s.Entered {
		s.Entered = true
		if err = selectOriginalCommandOpponent(result, view); err != nil {
			return err
		}
		s.Destination = 16
		index := s.Mode - 36
		var value uint8
		result.Random, value = NextRandom(result.Random)
		choice := (value & [3]uint8{1, 3, 7}[index]) + [3]uint8{0, 2, 0}[index]
		s.PlanParameter = [8]uint8{0, 4, 1, 5, 3, 7, 2, 6}[choice]
	}
	if s.TrackedObject < 0 || s.TrackedObject >= 4 {
		return fmt.Errorf("command attack tracked object %d has no recovered player source", s.TrackedObject)
	}
	target := view.Players[s.TrackedObject]
	// B05C always publishes its heading. A63D then checks its own 32-pixel
	// window, not B05C's carry based on the current Destination field.
	approachOriginalRiderHolder(result, view, target)
	dx := int32(int16(uint16(view.Position.X>>8) - uint16(target.X>>8)))
	dz := int32(int8(uint8(view.Position.Depth>>8) - uint8(target.Depth>>8)))
	if absPass(dx) < 32 && absPass(dz) < 8 {
		s.Mode, s.Entered, s.Plan = 7, false, 22
	} else {
		chooseAIAction(result, ActionRun)
	}
	return nil
}
