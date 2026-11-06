package score

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
)

// InstrumentParameters resolves the sole selected resource without changing
// any voice. FMPatch and its operator array contain only detached scalar values.
func (c *Catalog) InstrumentParameters(reference string) (InstrumentWork, param.FMPatch, error) {
	if c == nil {
		return InstrumentWork{}, param.FMPatch{}, errors.New("missing instrument catalog")
	}
	work, ok := c.instruments[reference]
	if !ok {
		return InstrumentWork{}, param.FMPatch{}, errors.New("instrument workload unavailable")
	}
	patch, ok := c.patch(reference)
	if !ok {
		return InstrumentWork{}, param.FMPatch{}, errors.New("instrument parameters unavailable")
	}
	return work, patch, nil
}
