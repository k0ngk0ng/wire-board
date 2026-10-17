// Development-only comparison adapter; never used to initialize product state.
package main

import "github.com/k0ngk0ng/wire-board/games/basketball/audiostate"

func sourceState(s *audiostate.State) []byte {
	b := make([]byte, 0xe7)
	put := func(address int, value byte) { b[address-0x700] = value }
	put(0x700, s.Music)
	put(0x701, s.SavedMusic)
	put(0x702, s.RetriggerDelay)
	put(0x703, s.RetriggerSound)
	put(0x704, s.ActiveChannel)
	put(0x705, s.Tempo)
	put(0x706, s.TempoPhase)
	for i, c := range s.Channels {
		if i >= 4 && i < 8 {
			put(0x703+i, c.Priority)
		}
		put(0x70c+i, c.Pending)
		put(0x715+i, c.Flags)
		put(0x71e+i, byte(c.Cursor))
		put(0x727+i, byte(c.Cursor>>8))
		put(0x730+i, c.Wait)
		put(0x739+i, c.Note)
		put(0x742+i, c.Fraction)
		put(0x74b+i, c.Volume)
		put(0x754+i, c.LoopA)
		put(0x75d+i, c.LoopB)
		put(0x766+i, byte(c.Return))
		put(0x76f+i, byte(c.Return>>8))
		put(0x778+i, c.Repeat)
		put(0x781+i, c.Gain)
		if i < 4 {
			put(0x78a+i, c.Length)
			put(0x78e+i, c.Extra)
			put(0x792+i, c.GateLength)
			put(0x796+i, c.GateWait)
			put(0x79a+i, c.GateMax)
			put(0x79e+i, c.GateScale)
			put(0x7a2+i, c.Transpose)
		}
		if i < 2 {
			put(0x7a6+i, c.EnvelopeValue)
			put(0x7a8+i, byte(c.Envelope))
			put(0x7aa+i, byte(c.Envelope>>8))
			put(0x7ac+i, c.EnvelopeStage)
			put(0x7ae+i, c.EnvelopeWait)
		}
		if i < 3 {
			put(0x7b0+i, c.VibratoValue)
			put(0x7b3+i, c.VibratoStep)
			put(0x7b6+i, c.VibratoDelay)
			put(0x7b9+i, c.VibratoWait)
			put(0x7bc+i, c.VibratoDepth)
		}
	}
	put(0x7bf, byte(s.Percussion))
	put(0x7c0, byte(s.Percussion>>8))
	for channel, values := range s.Registers {
		for register, value := range values {
			put(0x7c1+channel*4+register, value)
		}
	}
	if s.Initialized {
		put(0x7e1, 0x8d)
		put(0x7e3, 0x40)
		put(0x7e4, 0x60)
	}
	put(0x7e2, s.RegisterAddress)
	put(0x7e5, s.Flags)
	put(0x7e6, s.LastRequest)
	return b
}
