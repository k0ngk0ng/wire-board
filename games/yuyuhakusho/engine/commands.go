package engine

import "errors"

// CommandState retains the recognizers' two independent eight-byte arrays.
// They correspond to timeouts and sequence indices, not eight four-byte slots.
// RawHeld is the unretained base input; normal fighter latching is separate.
type CommandState struct {
	RawHeld  uint8    `json:"raw_held"`
	Facing   uint8    `json:"facing"`
	Request  uint8    `json:"request"`
	Timeouts [8]uint8 `json:"timeouts"`
	Indices  [8]uint8 `json:"indices"`
}

type CommandKind uint8

const (
	CommandDirection CommandKind = iota
	CommandButtons
)

// CommandProgram is immutable rule data. Steps includes the terminal 0xff;
// reaching that marker completes on the next invocation, not on the last input.
type CommandProgram struct {
	Window uint8   `json:"window"`
	Steps  []uint8 `json:"steps"`
}

// Program IDs preserve data provenance, never addresses executed by the engine.
type CommandLibrary map[uint32]CommandProgram

func RecognizeCommand(state CommandState, slot uint8, kind CommandKind, program CommandProgram) (CommandState, error) {
	if slot >= 8 {
		return state, errors.New("invalid command slot")
	}
	if kind != CommandDirection && kind != CommandButtons {
		return state, errors.New("unsupported command recognizer")
	}
	if len(program.Steps) == 0 || len(program.Steps) > 256 || program.Steps[len(program.Steps)-1] != 255 {
		return state, errors.New("invalid command termination")
	}
	for _, step := range program.Steps[:len(program.Steps)-1] {
		if step == 255 {
			return state, errors.New("early command termination")
		}
	}
	if int(state.Indices[slot]) >= len(program.Steps) {
		return state, errors.New("command index outside program")
	}
	expected := program.Steps[state.Indices[slot]]
	request := slot + 1
	if expected == 255 {
		state.Timeouts[slot], state.Indices[slot], state.Request = 0, 0, request
		return state, nil
	}
	mask := uint8(0xf0)
	if kind == CommandDirection {
		mask = 0x0f
		if state.Facing != 0 {
			expected = expected&^0x0c | (expected>>1)&4 | (expected<<1)&8
		}
	}
	if state.RawHeld&mask == expected&mask {
		state.Indices[slot]++
		state.Timeouts[slot] = 0
		return state, nil
	}
	state.Timeouts[slot]++ // Preserve the original byte wrap before comparison.
	if program.Window <= state.Timeouts[slot] {
		state.Timeouts[slot], state.Indices[slot] = 0, 0
		if state.Request == request {
			state.Request = 0
		}
	}
	return state, nil
}

// PrepareChuCommands recovers the complete order at 0x17d80. Later slots can
// replace earlier completed requests. Other characters require their own
// observed binding order and any additional preparation logic.
func PrepareChuCommands(state CommandState, library CommandLibrary) (CommandState, error) {
	before := state
	bindings := [...]struct {
		slot    uint8
		kind    CommandKind
		program uint32
	}{
		{6, CommandDirection, 0x1eaa4}, {7, CommandDirection, 0x1eaa9},
		{0, CommandButtons, 0x1eaf2}, {1, CommandButtons, 0x1eaeb},
		{2, CommandDirection, 0x1eab7}, {3, CommandButtons, 0x1eb00},
		{4, CommandDirection, 0x1ead9}, {5, CommandDirection, 0x1eacf},
	}
	for _, binding := range bindings {
		program, ok := library[binding.program]
		if !ok {
			return before, errors.New("command program is unavailable")
		}
		next, err := RecognizeCommand(state, binding.slot, binding.kind, program)
		if err != nil {
			return before, err
		}
		state = next
	}
	return state, nil
}
