package param

import "fmt"

// EnvelopeCursor is the sole typed program position. Cycle means that Index
// indexes the cycle rather than the prefix; no absolute tick or raw cursor is
// needed. The zero value starts an envelope, including an empty-prefix cycle.
type EnvelopeCursor struct {
	Cycle bool   `json:"cycle"`
	Index uint16 `json:"index"`
}

type PitchTick struct {
	Offset         *int8   `json:"offset"`
	ScaleAdd       int8    `json:"scale_add"`
	KeyOff         bool    `json:"key_off"`
	FixedPitchWord *uint16 `json:"fixed_pitch_word"`
}

func (t PitchTick) Validate() error {
	if t.KeyOff {
		if t.Offset != nil || t.FixedPitchWord == nil || *t.FixedPitchWord != 131 {
			return fmt.Errorf("pitch keyoff requires its resolved bypass word")
		}
	} else if t.Offset == nil || t.FixedPitchWord != nil {
		return fmt.Errorf("pitch calculation requires an offset")
	}
	return nil
}

type PitchEnvelope struct {
	ID     uint16      `json:"id"`
	Prefix []PitchTick `json:"prefix"`
	Cycle  []PitchTick `json:"cycle"`
	Ended  bool        `json:"ended"`
}

func (e PitchEnvelope) Validate() error {
	if e.ID == 0 || len(e.Prefix) > 65535 || len(e.Cycle) > 65535 || len(e.Prefix)+len(e.Cycle) == 0 || (e.Ended && len(e.Cycle) != 0) || (!e.Ended && len(e.Cycle) == 0) {
		return fmt.Errorf("invalid pitch envelope shape")
	}
	for i, tick := range e.Prefix {
		if err := tick.Validate(); err != nil {
			return err
		}
		if tick.KeyOff != (e.Ended && i == len(e.Prefix)-1) {
			return fmt.Errorf("pitch keyoff must be the terminal prefix tick")
		}
	}
	for _, tick := range e.Cycle {
		if err := tick.Validate(); err != nil || tick.KeyOff {
			return fmt.Errorf("invalid pitch cycle tick")
		}
	}
	return nil
}

func position(c EnvelopeCursor, prefix, cycle int) (EnvelopeCursor, error) {
	if c.Cycle {
		if int(c.Index) >= cycle {
			return c, fmt.Errorf("envelope cycle cursor out of range")
		}
		return c, nil
	}
	if int(c.Index) < prefix {
		return c, nil
	}
	if int(c.Index) == prefix && cycle > 0 {
		return EnvelopeCursor{Cycle: true}, nil
	}
	return c, fmt.Errorf("envelope prefix cursor out of range or ended")
}

func advanced(c EnvelopeCursor, prefix, cycle int) EnvelopeCursor {
	c.Index++
	if c.Cycle {
		if int(c.Index) == cycle {
			c.Index = 0
		}
	} else if int(c.Index) == prefix && cycle > 0 {
		c = EnvelopeCursor{Cycle: true}
	}
	return c
}

func (e PitchEnvelope) Next(state EnvelopeCursor) (PitchTick, EnvelopeCursor, error) {
	if err := e.Validate(); err != nil {
		return PitchTick{}, state, err
	}
	c, err := position(state, len(e.Prefix), len(e.Cycle))
	if err != nil {
		return PitchTick{}, state, err
	}
	var tick PitchTick
	if c.Cycle {
		tick = e.Cycle[c.Index]
	} else {
		tick = e.Prefix[c.Index]
	}
	return tick, advanced(c, len(e.Prefix), len(e.Cycle)), nil
}

// PitchWord applies signed-byte scale wrap and low-word arithmetic. A terminal
// keyoff bypass returns its resolved literal without adding pitch or modulation.
func PitchWord(tick PitchTick, scale int8, base uint16, detune int8, modulation uint16) (uint16, int8, error) {
	if err := tick.Validate(); err != nil {
		return 0, scale, err
	}
	nextScale := int8(uint8(scale) + uint8(tick.ScaleAdd))
	if tick.FixedPitchWord != nil {
		return *tick.FixedPitchWord, nextScale, nil
	}
	value := int32(*tick.Offset)*int32(nextScale) + int32(base) + int32(detune) + int32(modulation)
	return uint16(value), nextScale, nil
}

type VolumeTick struct {
	Level       *uint8 `json:"level"`
	WritesLevel bool   `json:"writes_level"`
	KeyOff      bool   `json:"key_off"`
}

func (t VolumeTick) Validate() error {
	if t.WritesLevel != (t.Level != nil) || (t.KeyOff && t.WritesLevel) {
		return fmt.Errorf("invalid volume tick")
	}
	return nil
}

type VolumeEnvelope struct {
	ID     uint16       `json:"id"`
	Prefix []VolumeTick `json:"prefix"`
	Cycle  []VolumeTick `json:"cycle"`
}

func (e VolumeEnvelope) Validate() error {
	if e.ID == 0 || len(e.Prefix) > 65535 || len(e.Cycle) == 0 || len(e.Cycle) > 65535 {
		return fmt.Errorf("invalid volume envelope shape")
	}
	for _, group := range [][]VolumeTick{e.Prefix, e.Cycle} {
		for _, tick := range group {
			if err := tick.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e VolumeEnvelope) Next(state EnvelopeCursor) (VolumeTick, EnvelopeCursor, error) {
	if err := e.Validate(); err != nil {
		return VolumeTick{}, state, err
	}
	c, err := position(state, len(e.Prefix), len(e.Cycle))
	if err != nil {
		return VolumeTick{}, state, err
	}
	var tick VolumeTick
	if c.Cycle {
		tick = e.Cycle[c.Index]
	} else {
		tick = e.Prefix[c.Index]
	}
	return tick, advanced(c, len(e.Prefix), len(e.Cycle)), nil
}

// PSGAttenuation is evaluated before caller-owned note/claim/tie/gate suppression.
func PSGAttenuation(tick VolumeTick, base uint8) (uint8, bool, error) {
	if err := tick.Validate(); err != nil {
		return 0, false, err
	}
	if !tick.WritesLevel {
		return 0, false, nil
	}
	level := base + *tick.Level
	if level > 15 {
		level = 15
	}
	return level, true, nil
}
