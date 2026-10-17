package audiostate

import "fmt"

// The source's sequence cursor has an eight-bit operand offset. Keeping that
// music-format detail preserves repeat/jump semantics without CPU registers.
type streamReader struct {
	base   uint16
	offset byte
}

func (r *streamReader) position() uint16    { return r.base + uint16(r.offset) }
func (r *streamReader) skip(n byte)         { r.offset += n }
func (r *streamReader) jump(address uint16) { r.base = address; r.offset = 0 }
func pair(args []byte) uint16               { return uint16(args[0]) | uint16(args[1])<<8 }

func (s *State) advance(ch int, gated bool) error {
	c := &s.Channels[ch]
	if gated {
		c.GateWait--
		if c.GateWait == 0 {
			c.Pending |= 0x80
		}
	} else if c.Flags == 0 {
		return nil
	}
	c.Wait--
	if c.Wait != 0 {
		return nil
	}
	if gated {
		c.Flags &^= 4
	}
	r := streamReader{base: c.Cursor}
	for operations := 0; operations < 10000; operations++ {
		key := uint32(ch)<<16 | uint32(r.position())
		instruction, ok := sources.Instructions[key]
		if !ok {
			return fmt.Errorf("unknown sequence $%04x channel %d", r.position(), ch)
		}
		repeating := c.Flags&2 != 0
		offset := r.offset
		end, err := s.command(ch, &r, instruction)
		if err != nil {
			return fmt.Errorf("sequence $%04x channel %d: %w", instruction.Address, ch, err)
		}
		if repeating {
			c.Repeat--
			if c.Repeat != 0 {
				r.offset = offset
			} else {
				c.Flags &^= 2
			}
		}
		if end {
			c.Cursor = r.position()
			return nil
		}
	}
	return fmt.Errorf("sequence command limit channel %d", ch)
}

func (s *State) musicNote(ch int, note byte, attributes byte) error {
	c := &s.Channels[ch]
	if ch == 3 {
		s.WorkByte = byte(s.Percussion >> 8)
		address, ok := sources.Percussion[s.Percussion+uint16(note&63)*2]
		if !ok {
			return fmt.Errorf("unknown percussion table $%04x note %d", s.Percussion, note&63)
		}
		extra := byte(0)
		if note&128 != 0 {
			extra = c.Extra
		}
		if err := s.loadHeader(address, extra+c.Volume+1); err != nil {
			return err
		}
		c.Wait = c.Length
		return nil
	}
	c.Note = (note + c.Transpose) & 63
	if c.Flags&8 != 0 {
		c.GateWait = 255
	} else {
		c.GateWait = c.GateLength
	}
	c.Wait = c.Length
	if note&128 != 0 {
		if attributes&0x40 != 0 {
			c.GateWait = min(c.Length, c.GateMax)
		}
		if attributes&0x80 != 0 {
			c.GateWait = 255
		}
		if attributes&0x20 != 0 {
			c.Flags |= 4
		}
	}
	if ch < 3 && c.Flags&0x80 == 0 {
		s.resetVibrato(ch)
	}
	c.Pending |= 0x30
	return nil
}

func (s *State) command(ch int, r *streamReader, instruction sourceInstruction) (bool, error) {
	c := &s.Channels[ch]
	op, args := instruction.Opcode, instruction.Args
	if op < 0x40 {
		r.skip(1)
		if ch >= 4 {
			c.Wait = op
			return true, nil
		}
		c.Length = op
		length := s.scale(op, c.GateScale)
		if length == 0 {
			length = 1
		}
		c.GateLength = length
		return false, nil
	}
	if op < 0xc0 {
		r.skip(instruction.Size)
		if ch >= 4 {
			c.Note = op
			c.Pending |= 0x20
			return false, nil
		}
		attributes := byte(0)
		if len(args) > 0 {
			attributes = args[0]
		}
		return true, s.musicNote(ch, op, attributes)
	}
	if op >= 0xe0 {
		return false, fmt.Errorf("unsupported command $%02x", op)
	}
	s.WorkByte = sources.HandlerHigh[op-0xc0]
	r.skip(1)
	switch op {
	case 0xc0:
		c.Note = args[0]
		c.Pending |= 0x20
		r.skip(1)
	case 0xc1:
		c.Note += args[0]
		c.Pending |= 0x20
		r.skip(1)
	case 0xc2:
		c.Fraction = args[0]
		c.Pending |= 0x20
		r.skip(1)
	case 0xc3:
		sum := uint16(c.Fraction) + uint16(args[0])
		c.Fraction = byte(sum)
		if args[0] < 128 {
			c.Note += byte(sum >> 8)
		} else if sum < 256 {
			c.Note--
		}
		c.Pending |= 0x20
		r.skip(1)
	case 0xc4:
		c.Volume = args[0]
		c.Pending |= 8
		r.skip(1)
	case 0xc5:
		c.Volume += args[0]
		c.Pending |= 8
		r.skip(1)
	case 0xc6:
		if ch >= 4 {
			return false, fmt.Errorf("extra parameter on unsupported channel %d", ch)
		}
		c.Extra = args[0]
		r.skip(1)
	case 0xc7:
		if ch >= 4 {
			return false, fmt.Errorf("gate ratio on unsupported channel %d", ch)
		}
		c.GateScale = args[0]
		length := s.scale(c.Length, c.GateScale)
		if length == 0 {
			length = 1
		}
		c.GateLength = length
		r.skip(1)
	case 0xc8:
		if ch >= 4 {
			return false, fmt.Errorf("gate maximum on unsupported channel %d", ch)
		}
		c.GateMax = args[0]
		r.skip(1)
	case 0xc9:
		if ch >= 4 {
			return false, fmt.Errorf("music rest on unsupported channel %d", ch)
		}
		c.Wait = c.Length
		c.Pending |= 0x80
		return true, nil
	case 0xca:
		c.Pending |= 0x10
	case 0xcb:
		c.Pending |= 0x80
	case 0xcc:
		s.Tempo = args[0]
		r.skip(1)
	case 0xcd:
		pointer := pair(args)
		r.skip(2)
		if ch == 3 {
			s.Percussion = pointer
			break
		}
		if ch&3 == 2 {
			break
		}
		s.WorkByte = byte(pointer >> 8)
		instrument, ok := sources.Instruments[pointer]
		if !ok {
			return false, fmt.Errorf("unknown instrument $%04x", pointer)
		}
		if ch < 4 {
			c.Envelope = instrument.Envelope
		}
		s.Registers[ch][0] = instrument.Control | 0x30
		s.Registers[ch][1] = instrument.Sweep
		c.Pending |= 0x40
	case 0xce, 0xcf:
		counter := &c.LoopA
		if op == 0xcf {
			counter = &c.LoopB
		}
		if *counter == 0 {
			*counter = args[0]
		}
		r.skip(1)
		*counter--
		if *counter != 0 {
			r.jump(pair(args[1:]))
		} else {
			r.skip(2)
		}
	case 0xd0:
		r.jump(pair(args))
	case 0xd1:
		r.skip(2)
		c.Return = r.position()
		r.jump(pair(args))
	case 0xd2:
		r.jump(c.Return)
	case 0xd3:
		// Restoring a covered music channel leaves the source sequence's
		// operand offset at the final shadow-register slot. The now-inactive
		// channel retains that cursor until another request replaces it.
		if ch >= 4 && s.Flags&0x40 == 0 && ch != 7 && s.Channels[ch&3].Flags != 0 {
			r.offset = byte((ch&3)*4 + 3)
		}
		s.stop(ch)
		return true, nil
	case 0xd4:
		if ch >= 8 {
			return false, fmt.Errorf("sweep on unsupported channel %d", ch)
		}
		s.Registers[ch][1] = args[0]
		c.Pending |= 4
		r.skip(1)
	case 0xd5:
		s.Flags |= 0x40
	case 0xd6:
		c.Flags |= 8
		c.Pending |= 0x80
	case 0xd7:
		c.Flags &^= 8
		c.Pending |= 0x80
	case 0xd8:
		if ch >= 4 {
			return false, fmt.Errorf("transpose on unsupported channel %d", ch)
		}
		c.Transpose = args[0]
		r.skip(1)
	case 0xd9:
		if err := s.request(args[0], 1); err != nil {
			return false, err
		}
		r.skip(1)
	case 0xda:
		c.Repeat = args[0]
		c.Flags |= 2
		r.skip(1)
	case 0xdb:
		if ch >= 3 {
			return false, fmt.Errorf("vibrato parameters on unsupported channel %d", ch)
		}
		c.VibratoDelay = args[0]
		c.VibratoStep = args[1]
		c.VibratoDepth = args[2]
		c.Flags |= 0x10
		r.skip(3)
	case 0xdc:
		c.Flags |= 0x10
	case 0xdd:
		c.Flags &^= 0x10
	case 0xde:
		s.Channels[8].Note = args[0]
		s.Channels[8].Volume = args[1]
		s.Channels[8].Fraction = args[2]
		r.skip(3)
	case 0xdf:
		for index, notes := 0, 0; notes < 2; notes++ {
			note := args[index]
			index++
			r.skip(1)
			attributes := byte(0)
			if ch < 3 && note&128 != 0 {
				attributes = args[index]
				index++
				r.skip(1)
			}
			if err := s.musicNote(ch, note, attributes); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, nil
}
