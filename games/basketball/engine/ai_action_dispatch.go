package engine

import "errors"

// A05D gives Action3E a mode-based gate before the ordinary action flags.
// The action is shared by court/device states and is not proof that the actor
// is carrying another player. 95EF's bit10 is set for modes1/29/31/34.
func originalAIControlDispatch(original AIControlState, action Action, hasBall bool) (AIControlState, bool, error) {
	if action != Action(0x3e) {
		return original, originalAIActionAvailable(action), nil
	}
	if original.Mode >= 42 {
		return original, false, errors.New("original Action3E mode flag outside recovered table")
	}
	const permitted uint64 = 1<<1 | 1<<29 | 1<<31 | 1<<34
	if permitted&(uint64(1)<<original.Mode) != 0 {
		return original, true, nil
	}
	if !hasBall {
		return original, false, nil
	}
	result := original
	result.Mode, result.Entered = AIPossessionWait, false
	return result, true, nil
}
