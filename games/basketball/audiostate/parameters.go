package audiostate

import "fmt"

func (s *State) apply(ch int) {
	c := &s.Channels[ch]
	if c.Pending == 0 {
		return
	}
	if ch == 8 {
		c.Pending = 0
		return
	} // DPCM writes only hardware; no feedback state.
	if c.Pending&0x80 != 0 {
		c.Flags &^= 0x80
		if ch&3 == 2 {
			s.writeRegister(ch, 0, 0x80)
		} else if ch < 4 {
			if ch < 2 {
				c.EnvelopeStage = 2
			}
		} else {
			s.writeRegister(ch, 0, s.Registers[ch][0]&0xf0)
		}
	}
	if c.Pending&0x40 != 0 {
		s.writeRegister(ch, 0, s.Registers[ch][0])
		s.writeRegister(ch, 1, s.Registers[ch][1])
	}
	if c.Pending&0x20 != 0 {
		s.pitch(ch)
	}
	if c.Pending&8 != 0 && ch&3 != 2 && ch >= 4 && c.Flags&0x80 != 0 {
		s.volume(ch, c.Volume)
	}
	if c.Pending&4 != 0 {
		s.writeRegister(ch, 1, s.Registers[ch][1])
	}
	if c.Pending&0x10 != 0 && c.Flags&0x80 == 0 {
		c.Flags |= 0x80
		s.writeRegister(ch, 2, s.Registers[ch][2])
		s.writeRegister(ch, 3, s.Registers[ch][3])
		if ch&3 == 2 {
			s.writeRegister(ch, 0, 0x81)
		} else if ch >= 4 {
			s.volume(ch, c.Volume)
		} else if ch < 2 {
			c.EnvelopeStage = 1
		}
	}
	c.Pending = 0
}

func (s *State) volume(ch int, value byte) {
	c := &s.Channels[ch]
	if c.Gain&0x80 == 0 {
		value = s.scale(value, c.Gain) >> 2
	} else {
		value >>= 3
	}
	s.writeRegister(ch, 0, (s.Registers[ch][0]&0xf0)|value|0x30)
}

func (s *State) pitch(ch int) {
	c := &s.Channels[ch]
	if ch&3 == 3 {
		s.writeRegister(ch, 2, c.Note)
		return
	}
	note, fraction := c.Note, c.Fraction
	if ch < 4 {
		sum := uint16(fraction) + uint16(c.VibratoValue)
		fraction = byte(sum)
		if c.VibratoValue < 128 {
			note += byte(sum >> 8)
		} else if sum < 256 {
			note--
		}
	}
	note &= 63
	low, high := sources.PitchLow[note], sources.PitchHigh[note]
	if fraction != 0 {
		amount := s.scale(fraction, low-sources.PitchLow[int(note)+1])
		if low < amount {
			high--
		}
		low -= amount
	}
	s.writeRegister(ch, 2, low)
	// $07bf+Y aliases this channel's sweep register, not a hardware read.
	if s.Registers[ch][1]&0x80 != 0 || high != s.Registers[ch][3] {
		s.writeRegister(ch, 3, high)
	}
}

func (s *State) vibrato(ch int) {
	c := &s.Channels[ch]
	if c.Flags&0x10 == 0 {
		return
	}
	c.VibratoWait--
	if c.VibratoWait < 128 {
		return
	}
	c.VibratoWait = 0
	half := c.VibratoDepth >> 1
	s.WorkByte = half
	base := half + c.VibratoValue
	sum := uint16(base) + uint16(c.VibratoStep)
	value := byte(sum)
	if c.VibratoStep < 128 {
		if value >= c.VibratoDepth {
			c.VibratoStep = -c.VibratoStep
			value = c.VibratoDepth
		}
	} else if sum < 256 {
		c.VibratoStep = -c.VibratoStep
		value = 0
	}
	c.VibratoValue = value - half
	c.Pending |= 0x20
}

func (s *State) resetVibrato(ch int) {
	c := &s.Channels[ch]
	c.VibratoWait = c.VibratoDelay
	c.VibratoValue = 0
	if c.VibratoStep >= 128 {
		c.VibratoStep = -c.VibratoStep
		c.Pending |= 0x20
	}
}

func (s *State) envelope(ch int) error {
	if ch >= 2 {
		return nil
	}
	c := &s.Channels[ch]
	if c.EnvelopeStage == 0 {
		return nil
	}
	e, ok := sources.Envelopes[c.Envelope]
	// On the first post-reset tick the source stops a zero-valued envelope
	// before loading an instrument. min(any byte at $0001, 0) is always zero.
	if !ok && !(c.Envelope == 0 && c.EnvelopeStage == 2 && c.EnvelopeValue == 0) {
		return fmt.Errorf("unsupported envelope $%04x stage %d", c.Envelope, c.EnvelopeStage)
	}
	loadWait := func(stage byte) error {
		c.EnvelopeStage = stage
		wait, err := e.value(stage + 1)
		if err != nil {
			return err
		}
		c.EnvelopeWait = wait
		if wait == 0 {
			delta, err := e.value(stage)
			if err != nil {
				return err
			}
			if delta == 0 {
				c.EnvelopeStage = 0
			} else {
				c.EnvelopeStage |= 0x80
			}
		}
		return nil
	}
	stage := c.EnvelopeStage
	switch {
	case stage == 2:
		if e.Gate < c.EnvelopeValue {
			c.EnvelopeValue = e.Gate
		}
		c.EnvelopeStage = 0x82
	case stage == 1:
		c.EnvelopeValue = e.Initial
		if err := loadWait(3); err != nil {
			return err
		}
	case stage < 128:
		delta, err := e.value(stage)
		if err != nil {
			return err
		}
		c.EnvelopeValue += delta
		c.EnvelopeWait--
		if c.EnvelopeWait == 0 {
			if err := loadWait(stage + 2); err != nil {
				return err
			}
		}
	default:
		index := stage & 127
		delta, err := e.value(index)
		if err != nil {
			return err
		}
		sum := uint16(c.EnvelopeValue) + uint16(delta)
		if delta < 128 && sum >= 256 {
			c.EnvelopeValue = 255
			c.EnvelopeStage = 0
		} else if delta >= 128 && sum < 256 {
			c.EnvelopeValue = 0
			c.EnvelopeStage = 0
		} else {
			c.EnvelopeValue = byte(sum)
			if sum < 256 {
				if err := loadWait(index); err != nil {
					return err
				}
			}
		}
	}
	extra := byte(0)
	if c.Flags&4 != 0 {
		extra = c.Extra
	}
	value := s.scale(c.EnvelopeValue, extra+c.Volume+1)
	s.volume(ch, value)
	return nil
}
