// Package audiostate restores the original silent music/effect control state.
// It produces no PCM and owns no player. WorkByte preserves an original
// scratch-byte side effect observed by one gameplay compatibility path.
package audiostate

import "fmt"

type Channel struct {
	Priority, Pending, Flags                                           byte
	Cursor                                                             uint16
	Wait, Note, Fraction, Volume                                       byte
	LoopA, LoopB                                                       byte
	Return                                                             uint16
	Repeat, Gain                                                       byte
	Length, Extra, GateLength, GateWait, GateMax, GateScale, Transpose byte
	Envelope                                                           uint16
	EnvelopeValue, EnvelopeStage, EnvelopeWait                         byte
	VibratoValue, VibratoStep, VibratoDelay, VibratoWait, VibratoDepth byte
}

// State is serializable by value. Only explicit requests and sound IRQ ticks
// advance it; browser media clocks, mute, and device volume are unrelated.
type State struct {
	WorkByte                                             byte
	Music, SavedMusic, RetriggerDelay, RetriggerSound    byte
	ActiveChannel, Tempo, TempoPhase, Flags, LastRequest byte
	Channels                                             [9]Channel
	Percussion                                           uint16
	Registers                                            [8][4]byte
	RegisterAddress                                      byte
	Initialized                                          bool
}

func New(carriedWork byte) *State { return &State{WorkByte: carriedWork} }

// Request applies an original sound/control request synchronously. Invalid
// requests fail before mutation. Controls 251..255 retain their source meaning.
func (s *State) Request(id byte) error {
	if sourceError != nil {
		return sourceError
	}
	if id > 76 && id < 251 {
		return fmt.Errorf("unsupported original sound request %d", id)
	}
	return s.request(id, 0)
}

func (s *State) request(id byte, depth int) error {
	if depth > 32 {
		return fmt.Errorf("nested sound request limit")
	}
	s.LastRequest = id
	s.Flags &^= 0x80
	switch id {
	case 0:
		s.reset()
	case 251:
		s.Flags &^= 0x20
	case 252:
		saved := s.SavedMusic
		s.reset()
		s.SavedMusic = saved
	case 253:
		s.Flags |= 0x40
		s.SavedMusic = s.Music
	case 254:
		s.SavedMusic = s.Music
	case 255:
		s.Flags &^= 0x40
		if s.SavedMusic != 0 && s.SavedMusic != s.Music {
			address, ok := sources.Sounds[s.SavedMusic]
			if !ok {
				return fmt.Errorf("unsupported saved sound %d", s.SavedMusic)
			}
			if err := s.loadHeader(address, 0x80); err != nil {
				return err
			}
		} else {
			for ch := 7; ch >= 4; ch-- {
				s.ActiveChannel = byte(ch)
				s.stop(ch)
			}
		}
	default:
		address, ok := sources.Sounds[id]
		if !ok {
			return fmt.Errorf("unsupported original sound %d", id)
		}
		if err := s.loadHeader(address, 0x80); err != nil {
			return err
		}
	}
	s.Flags |= 0x80
	return nil
}

func (s *State) reset() { work := s.WorkByte; *s = State{WorkByte: work, Initialized: true} }

func (s *State) loadHeader(address uint16, gain byte) error {
	h, ok := sources.Headers[address]
	if !ok {
		return fmt.Errorf("unsupported sound header $%04x", address)
	}
	s.WorkByte = gain
	if h.Priority&0x80 != 0 {
		s.Music = s.LastRequest
		s.Tempo = 255
		s.TempoPhase = 255
		s.Flags = (s.Flags | 2) &^ 0x40
	} else {
		if h.Repeat != nil {
			s.RetriggerDelay = *h.Repeat
			s.RetriggerSound = s.LastRequest
			s.Flags |= 0x20
		}
		for _, entry := range h.Channels {
			if h.Priority < s.Channels[entry.Channel].Priority {
				return nil
			}
		}
	}
	for _, entry := range h.Channels {
		ch := int(entry.Channel)
		c := &s.Channels[ch]
		if ch >= 4 && ch < 8 {
			c.Priority = h.Priority
		}
		if ch != 3 && ch != 8 {
			if ch < 4 {
				if ch < 3 {
					c.VibratoStep = 30
					c.VibratoDepth = 150
				}
				c.GateScale = 250
				c.Transpose = 0
			} else if h.Priority == 0 {
				return fmt.Errorf("unsupported zero-priority effect header $%04x", address)
			}
			c.Gain = gain
			c.Fraction = 0
		}
		if ch != 3 {
			c.Pending = 0x80
		}
		c.LoopA = 0
		c.LoopB = 0
		c.Wait = 1
		c.Flags = 1
		c.Cursor = entry.Stream
	}
	return nil
}

// Tick advances one complete original sound-driver IRQ call. The owner must
// schedule it relative to gameplay requests; this package does not invent NMI
// ordering or silently tie it to a browser/audio wall clock.
func (s *State) Tick() error {
	if sourceError != nil {
		return sourceError
	}
	if s.Flags&0x80 == 0 {
		return nil
	}
	if s.Flags&0x20 != 0 && s.Flags&0x40 == 0 {
		s.RetriggerDelay--
		if s.RetriggerDelay == 0 {
			if err := s.request(s.RetriggerSound, 0); err != nil {
				return err
			}
		}
	}
	for ch := 8; ch >= 4; ch-- {
		s.ActiveChannel = byte(ch)
		if err := s.advance(ch, false); err != nil {
			return err
		}
		s.apply(ch)
	}
	if s.Flags&0x40 != 0 {
		return nil
	}
	for ch := 2; ch >= 0; ch-- {
		s.ActiveChannel = byte(ch)
		s.vibrato(ch)
		s.apply(ch)
		if err := s.envelope(ch); err != nil {
			return err
		}
	}
	sum := uint16(s.Tempo) + uint16(s.TempoPhase)
	s.TempoPhase = byte(sum)
	if sum < 256 {
		return nil
	}
	s.ActiveChannel = 3
	if err := s.advance(3, false); err != nil {
		return err
	}
	for ch := 2; ch >= 0; ch-- {
		if s.Channels[ch].Flags != 0 {
			s.ActiveChannel = byte(ch)
			if err := s.advance(ch, true); err != nil {
				return err
			}
		}
	}
	return nil
}

// scale is the driver's integer fraction multiply. A zero multiplier denotes
// 256/256. The scratch decrement is observable, except when value is zero.
func (s *State) scale(value, multiplier byte) byte {
	s.WorkByte = multiplier
	if value == 0 {
		return 0
	}
	s.WorkByte--
	return byte(uint16(value) * (uint16(s.WorkByte) + 1) >> 8)
}

func (s *State) writeRegister(ch, reg int, value byte) {
	s.Registers[ch][reg] = value
	if ch < 4 && s.Channels[ch+4].Flags != 0 {
		return
	}
	s.RegisterAddress = byte((ch*4 + reg) & 15)
}

func (s *State) stop(ch int) {
	c := &s.Channels[ch]
	c.Pending = 0x80
	c.Flags = 0
	if ch < 4 {
		if s.Channels[0].Flags|s.Channels[1].Flags|s.Channels[2].Flags|s.Channels[3].Flags == 0 {
			s.Flags &^= 2
		}
		return
	}
	if s.Flags&0x40 == 0 && ch != 7 {
		music := ch & 3
		if s.Channels[music].Flags != 0 {
			for reg := 0; reg < 4; reg++ {
				s.writeRegister(music, reg, s.Registers[music][reg])
			}
			c.Pending = 0
		}
	}
	c.Priority = 0
}
