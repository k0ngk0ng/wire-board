package score

import "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"

// These queries derive a semantic tick and its bound work without advancing
// any voice. Returned scalar pointers are detached from the immutable catalog.
func (c *Catalog) PitchEnvelopeStep(id uint16, cursor param.EnvelopeCursor) (param.PitchTick, EnvelopePathWork, error) {
	work, err := c.EnvelopeWork("pitch", id, cursor)
	if err != nil {
		return param.PitchTick{}, EnvelopePathWork{}, err
	}
	tick, _, err := c.pitch[id].Next(cursor)
	if err != nil {
		return param.PitchTick{}, EnvelopePathWork{}, err
	}
	if tick.Offset != nil {
		value := *tick.Offset
		tick.Offset = &value
	}
	if tick.FixedPitchWord != nil {
		value := *tick.FixedPitchWord
		tick.FixedPitchWord = &value
	}
	return tick, work, nil
}

func (c *Catalog) VolumeEnvelopeStep(id uint16, cursor param.EnvelopeCursor) (param.VolumeTick, EnvelopePathWork, error) {
	work, err := c.EnvelopeWork("volume", id, cursor)
	if err != nil {
		return param.VolumeTick{}, EnvelopePathWork{}, err
	}
	tick, _, err := c.volume[id].Next(cursor)
	if err != nil {
		return param.VolumeTick{}, EnvelopePathWork{}, err
	}
	if tick.Level != nil {
		value := *tick.Level
		tick.Level = &value
	}
	return tick, work, nil
}
