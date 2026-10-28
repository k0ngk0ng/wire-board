// Package contra contains the independent, deterministic Contra rules.
// Original ROM execution is used only by development capture tools.
package contra

// Buttons use the original controller latch order. Platform key bindings and
// turbo generation translate to these logical bits before each simulation tick.
type Buttons uint8

const (
	ButtonRight Buttons = 1 << iota
	ButtonLeft
	ButtonDown
	ButtonUp
	ButtonStart
	ButtonSelect
	ButtonB
	ButtonA
)

// Controls separates the current held state from newly pressed edges. It is
// part of the saved deterministic state, so replay preserves short presses.
type Controls struct {
	Held    [2]Buttons `json:"held"`
	Pressed [2]Buttons `json:"pressed"`
}

// Sample implements the logical, stable-input portion of original $C1DA..C1F8.
// Hardware double reads at $C1BD are unnecessary for already authenticated
// digital input. Single-player original play accepts either physical port;
// co-op keeps both players independent. Conflicting directions are preserved
// for the game's action-specific priority rules to resolve.
func (c *Controls) Sample(pads [2]Buttons, cooperative bool) {
	if !cooperative {
		pads[0] |= pads[1]
	}
	for actor, held := range pads {
		c.Pressed[actor] = held &^ c.Held[actor]
		c.Held[actor] = held
	}
}
