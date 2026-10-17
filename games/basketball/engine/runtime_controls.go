package engine

import "fmt"

// SetRuntimeAutomation adapts platform seats to the original human/AI input
// selector. Platform seat i owns logical controller i; local shared-controller
// layouts must use their own host and are never silently remapped here.
// This is a platform handoff, not a new AI strategy or a simulation step.
func SetRuntimeAutomation(original RuntimeState, automated [4]bool) (RuntimeState, error) {
	if err := ValidateRuntimeState(original); err != nil {
		return original, err
	}
	var assignments [4]int8
	var changed [4]bool
	anyChange := false
	for actor, prior := range original.Config.Match.ControllerAssignments {
		if prior >= 0 && prior != int8(actor) {
			return original, fmt.Errorf("platform automation requires one controller per seat")
		}
		assignments[actor] = int8(actor)
		if automated[actor] {
			assignments[actor] = -1
		}
		changed[actor] = assignments[actor] != prior
		anyChange = anyChange || changed[actor]
	}
	if !anyChange {
		return original, nil
	}
	if original.Outcome != nil {
		return original, fmt.Errorf("finished match cannot change controllers")
	}
	s := cloneRuntime(original)
	s.Config.Match.ControllerAssignments = assignments
	clearSeatInput := func(memory *WorldLoopMemory, actor int) {
		// A newly selected input source starts without physical tap/chord
		// history. Already accepted actions and all AI memory remain intact.
		memory.Held[actor], memory.Pressed[actor] = 0, 0
		memory.Memory[actor] = InputMemory{}
	}
	setScene := func(scene *OpeningState, actor int, control uint8) {
		scene.Transition.ControllerAssignments[actor] = assignments[actor]
		scene.Transition.PlayerControls[actor] = control
		scene.InputDelays[actor], scene.InputDelays[actor+4], scene.InputDelays[actor+8] = 0, 0, 0
	}
	for actor, update := range changed {
		if !update {
			continue
		}
		control := uint8(actor)
		if automated[actor] {
			control = 128
		}
		s.Held[actor], s.PendingPressed[actor] = 0, 0
		if s.Playing != nil {
			s.Playing.Context.Actions.Controls[actor] = control
			clearSeatInput(&s.Playing.World.Input, actor)
		}
		if s.Opening != nil {
			setScene(s.Opening, actor, control)
		}
		if s.Transition != nil {
			setScene(&s.Transition.Scene, actor, control)
		}
		if s.Carry != nil {
			clearSeatInput(&s.Carry.Input, actor)
		}
		s.Display.Controllers[actor] = assignments[actor]
		s.Display.Submitted.Controllers[actor] = assignments[actor]
		s.Display.Visible.Controllers[actor] = assignments[actor]
	}
	if err := ValidateRuntimeState(s); err != nil {
		return original, fmt.Errorf("controller handoff: %w", err)
	}
	return s, nil
}
