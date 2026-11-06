package score

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
)

// PanAutomationStep reads the canonical next tick without advancing a voice.
// The score driver remains the sole owner of its pan counters and stored pan.
func (c *Catalog) PanAutomationStep(definition param.PanDefinition, state param.PanState, onNote, noteOff bool) (param.PanTick, error) {
	if c == nil {
		return param.PanTick{}, errors.New("missing pan automation catalog")
	}
	return param.AdvancePan(definition, c.patterns, state, onNote, noteOff)
}
