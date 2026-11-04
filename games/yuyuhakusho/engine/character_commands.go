package engine

import "errors"

type characterCommandBinding struct {
	slot    uint8
	kind    CommandKind
	program uint32
}

// Source IDs preserve the original eleven-entry preparation dispatch order.
// Slots not present in a character's list retain their state unchanged. Program
// identifiers are immutable data identities, never code addresses to execute.
var characterCommandBindings = [11][]characterCommandBinding{
	{
		{6, CommandDirection, 0x1eaa4}, {7, CommandDirection, 0x1eaa9},
		{0, CommandButtons, 0x1eaf2}, {1, CommandButtons, 0x1eaeb},
		{2, CommandDirection, 0x1ea9c}, {3, CommandDirection, 0x1ead9},
		{4, CommandDirection, 0x1eab7}, {5, CommandDirection, 0x1eb1d},
	},
	{
		{6, CommandDirection, 0x1eaa4}, {7, CommandDirection, 0x1eaa9},
		{0, CommandDirection, 0x1ea9c}, {1, CommandDirection, 0x1ead9},
		{2, CommandButtons, 0x1eb00}, {3, CommandDirection, 0x1eab7}, {5, CommandDirection, 0x1eb12},
	},
	{
		{6, CommandDirection, 0x1eaa4}, {7, CommandDirection, 0x1eaa9},
		{0, CommandButtons, 0x1eb00}, {2, CommandDirection, 0x1eab7},
		{3, CommandDirection, 0x1ea9c}, {4, CommandDirection, 0x1ead9}, {5, CommandDirection, 0x1eb27},
	},
	{
		{6, CommandDirection, 0x1eaa4}, {7, CommandDirection, 0x1eaa9},
		{0, CommandButtons, 0x1eaf2}, {1, CommandButtons, 0x1eaeb},
		{2, CommandDirection, 0x1eab7}, {3, CommandDirection, 0x1ead9}, {5, CommandDirection, 0x1eb1d},
	},
	{
		{6, CommandDirection, 0x1eaa4}, {7, CommandDirection, 0x1eaa9},
		{0, CommandDirection, 0x1eaae}, {1, CommandButtons, 0x1eb00},
		{2, CommandDirection, 0x1ead9}, {3, CommandDirection, 0x1eab7}, {5, CommandDirection, 0x1eb0c},
	},
	{
		{6, CommandDirection, 0x1eaa4}, {7, CommandDirection, 0x1eaa9},
		{0, CommandButtons, 0x1eb00}, {2, CommandDirection, 0x1eab7},
		{3, CommandDirection, 0x1ead9}, {5, CommandDirection, 0x1eb27},
	},
	{
		{6, CommandDirection, 0x1eaa4}, {7, CommandDirection, 0x1eaa9},
		{0, CommandDirection, 0x1ead9}, {1, CommandDirection, 0x1ea9c},
		{2, CommandDirection, 0x1eabc}, {3, CommandDirection, 0x1eab7}, {5, CommandDirection, 0x1eab7},
	},
	{
		{6, CommandDirection, 0x1eaa4}, {7, CommandDirection, 0x1eaa9},
		{0, CommandDirection, 0x1ead9}, {1, CommandDirection, 0x1eab7},
		{2, CommandDirection, 0x1ea9c}, {5, CommandButtons, 0x1eb00},
	},
	{
		{6, CommandDirection, 0x1eaa4}, {7, CommandDirection, 0x1eaa9},
		{0, CommandButtons, 0x1eb00}, {1, CommandDirection, 0x1eab7},
		{2, CommandDirection, 0x1ead9}, {3, CommandDirection, 0x1ea9c}, {5, CommandDirection, 0x1eb17},
	},
	{
		{6, CommandDirection, 0x1eaa4}, {7, CommandDirection, 0x1eaa9},
		{0, CommandButtons, 0x1eaf2}, {1, CommandButtons, 0x1eaeb},
		{2, CommandDirection, 0x1eab7}, {3, CommandButtons, 0x1eb00},
		{4, CommandDirection, 0x1ead9}, {5, CommandDirection, 0x1eacf},
	},
	{
		{6, CommandDirection, 0x1eaa4}, {7, CommandDirection, 0x1eaa9},
		{0, CommandButtons, 0x1eb00}, {1, CommandDirection, 0x1eab7},
		{2, CommandDirection, 0x1ead9}, {5, CommandDirection, 0x1eb12},
	},
}

// PrepareCharacterCommands recovers the complete character-specific routines
// reached through 0x15c20. Later completed slots can replace earlier requests.
// It does not latch input, select an attack or run the shared preparation tail.
func PrepareCharacterCommands(characterID uint8, state CommandState, library CommandLibrary) (CommandState, error) {
	before := state
	if int(characterID) >= len(characterCommandBindings) {
		return before, errors.New("unsupported source character")
	}
	for _, binding := range characterCommandBindings[characterID] {
		program, ok := library[binding.program]
		if !ok {
			return before, errors.New("character command program is unavailable")
		}
		next, err := RecognizeCommand(state, binding.slot, binding.kind, program)
		if err != nil {
			return before, err
		}
		state = next
	}
	return state, nil
}
