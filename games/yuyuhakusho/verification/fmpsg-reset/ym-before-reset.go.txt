// Package chips contains independently expressed typed sound-chip arithmetic.
// It does not interpret a CPU, ROM, firmware, RAM image or native core context.
package chips

import "errors"

const MaxYMBlockSamples = 65536

// YMOperator is one of the four logical operators, in 1,2,3,4 order. The
// register parameters are canonical; phase increments, attenuation outputs and
// rate-table indexes are temporary views. LatchedKeyScale is a separate delayed
// circuit stage: writes and key-on can occur before the next sample refresh.
type YMOperator struct {
	Detune, Multiple, Level, KeyScale                 uint8
	Attack, Decay, SustainRate, Release, SustainLevel uint8
	SSG                                               uint8
	AMEnabled                                         bool
	Phase                                             uint32
	Envelope                                          uint16
	EnvelopePhase                                     uint8
	Negated, Key                                      bool
	LatchedKeyScale                                   uint8
}

type YMChannel struct {
	Frequency                                  uint16
	Algorithm, Feedback, Pan, AMDepth, PMDepth uint8
	FeedbackHistory                            [2]int32
	Memory                                     int32
	Operators                                  [4]YMOperator
}

// YMState owns all dynamic chip state. No register-image shadow, derived route
// pointers, phase-increment cache or host-dependent serialized context is kept.
type YMState struct {
	Channels                                   [6]YMChannel
	Address                                    uint16
	Status, Mode, FrequencyLatch, SpecialLatch uint8
	SpecialFrequency                           [3]uint16
	TimerAValue                                uint16
	TimerACounter                              int32
	TimerBValue                                uint8
	TimerBCounter                              int32
	CSM                                        uint8
	EnvelopeCount                              uint16
	EnvelopeDivider, LFOCycle                  uint8
	LFODivider                                 uint16
	LFOEnabled                                 bool
	LFOSpeed, DACValue                         uint8
	DACEnabled                                 bool
}

type YM struct{ state YMState }

func ResetYMState() YMState {
	var s YMState
	s.Mode = 0x30
	s.DACValue = 128
	for i := range s.Channels {
		s.Channels[i].Pan = 0xc0
		for j := range s.Channels[i].Operators {
			s.Channels[i].Operators[j].Envelope = 1023
		}
	}
	return s
}

func validYM(s YMState) error {
	if s.Address > 511 || s.Status > 3 || s.FrequencyLatch > 63 || s.SpecialLatch > 63 || s.TimerAValue > 1023 || s.TimerACounter < 0 || s.TimerACounter > 1024 || s.TimerBCounter < 0 || s.TimerBCounter > 4096 || s.CSM > 1 || s.EnvelopeCount > 4095 || s.EnvelopeDivider > 2 || s.LFOCycle > 127 || s.LFODivider > 107 || s.LFOSpeed > 7 {
		return errors.New("invalid YM clock, latch or timer state")
	}
	if !s.LFOEnabled && (s.LFOSpeed != 0 || s.LFODivider != 0 || s.LFOCycle != 0) {
		return errors.New("disabled YM LFO must be in reset")
	}
	for _, frequency := range s.SpecialFrequency {
		if frequency > 16383 {
			return errors.New("invalid YM special frequency")
		}
	}
	for _, ch := range s.Channels {
		if ch.Frequency > 16383 || ch.Algorithm > 7 || ch.Feedback > 7 || ch.Pan&^uint8(0xc0) != 0 || ch.AMDepth > 3 || ch.PMDepth > 7 || ch.Memory < -16384 || ch.Memory > 16382 {
			return errors.New("invalid YM channel parameter")
		}
		for _, value := range ch.FeedbackHistory {
			if value < -8192 || value > 8191 {
				return errors.New("invalid YM feedback history")
			}
		}
		for _, op := range ch.Operators {
			if op.Detune > 7 || op.Multiple > 15 || op.Level > 127 || op.KeyScale > 3 || op.Attack > 31 || op.Decay > 31 || op.SustainRate > 31 || op.Release > 15 || op.SustainLevel > 15 || op.SSG > 15 || op.Envelope > 1031 || op.EnvelopePhase > 4 || op.LatchedKeyScale > 31 {
				return errors.New("invalid YM operator state")
			}
		}
	}
	return nil
}

func NewYM(state YMState) (*YM, error) {
	if err := validYM(state); err != nil {
		return nil, err
	}
	return &YM{state: state}, nil
}
func (ym *YM) Snapshot() YMState { return ym.state }

func rateCode(raw uint8) int {
	if raw == 0 {
		return 0
	}
	return 32 + int(raw)*2
}
func sustainLevel(op *YMOperator) int32 {
	if op.SustainLevel == 15 {
		return 992
	}
	return int32(op.SustainLevel) * 32
}
func keyCode(frequency uint16) uint8 {
	f := frequency & 2047
	low := uint8(0)
	switch {
	case f >= 1152:
		low = 3
	case f >= 1024:
		low = 2
	case f >= 896:
		low = 1
	}
	return uint8(frequency>>11)*4 + low
}
func (ym *YM) frequency(channel, operator int) uint16 {
	if channel == 2 && ym.state.Mode&0xc0 != 0 && operator != 3 {
		return ym.state.SpecialFrequency[[3]int{1, 2, 0}[operator]]
	}
	return ym.state.Channels[channel].Frequency
}
func beginEnvelope(op *YMOperator) {
	op.Phase = 0
	op.Negated = false
	if rateCode(op.Attack)+int(op.LatchedKeyScale) >= 94 {
		op.Envelope = 0
	}
	if op.Envelope > 0 {
		op.EnvelopePhase = 4
	} else if sustainLevel(op) == 0 {
		op.EnvelopePhase = 2
	} else {
		op.EnvelopePhase = 3
	}
}
func releaseEnvelope(op *YMOperator) {
	if op.EnvelopePhase <= 1 {
		return
	}
	op.EnvelopePhase = 1
	if op.SSG&8 != 0 {
		if op.Negated != (op.SSG&4 != 0) {
			op.Envelope = uint16((512 - int32(op.Envelope)) & 1023)
		}
		if op.Envelope >= 512 {
			op.Envelope = 1023
			op.EnvelopePhase = 0
		}
	}
}
func (ym *YM) key(channel int, mask uint8) {
	for i := range ym.state.Channels[channel].Operators {
		op := &ym.state.Channels[channel].Operators[i]
		on := mask&(16<<i) != 0
		if ym.state.CSM == 0 {
			if on && !op.Key {
				beginEnvelope(op)
			}
			if !on && op.Key {
				releaseEnvelope(op)
			}
		}
		op.Key = on
	}
}
func (ym *YM) csmKey(on bool) {
	for i := range ym.state.Channels[2].Operators {
		op := &ym.state.Channels[2].Operators[i]
		if !op.Key {
			if on {
				if ym.state.CSM == 0 {
					beginEnvelope(op)
				}
			} else {
				releaseEnvelope(op)
			}
		}
	}
}
func (ym *YM) writeMode(reg, value uint8) {
	s := &ym.state
	switch reg {
	case 0x22:
		s.LFOEnabled = value&8 != 0
		if s.LFOEnabled {
			s.LFOSpeed = value & 7
		} else {
			s.LFOSpeed = 0
			s.LFOCycle = 0
			s.LFODivider = 0
		}
	case 0x24:
		s.TimerAValue = (s.TimerAValue & 3) | (uint16(value) << 2)
	case 0x25:
		s.TimerAValue = (s.TimerAValue & 1020) | uint16(value&3)
	case 0x26:
		s.TimerBValue = value
	case 0x27:
		if value&0xc0 != 0x80 && s.CSM != 0 {
			ym.csmKey(false)
			s.CSM = 0
		}
		if value&1 != 0 && s.Mode&1 == 0 {
			s.TimerACounter = 1024 - int32(s.TimerAValue)
		}
		if value&2 != 0 && s.Mode&2 == 0 {
			s.TimerBCounter = (256 - int32(s.TimerBValue)) * 16
		}
		s.Status &^= (value >> 4) & 3
		s.Mode = value
	case 0x28:
		if value&3 == 3 {
			return
		}
		channel := int(value & 3)
		if value&4 != 0 {
			channel += 3
		}
		ym.key(channel, value)
	case 0x2a:
		s.DACValue = value
	case 0x2b:
		s.DACEnabled = value&128 != 0
	}
}

// WritePort accepts only the four physical YM ports. Both data ports write the
// shared last address latch, including its bank bit; data-port parity does not
// choose a different register bank. No time is advanced by this primitive.
func (ym *YM) WritePort(port, value uint8) error {
	if port > 3 {
		return errors.New("invalid YM port")
	}
	if port&1 == 0 {
		ym.state.Address = uint16(value) | uint16(port/2)*256
		return nil
	}
	address := ym.state.Address
	if address >= 0x20 && address < 0x30 {
		ym.writeMode(uint8(address), value)
		return nil
	}
	if address&3 == 3 {
		return nil
	}
	channel := int(address&3) + int(address>>8)*3
	ch := &ym.state.Channels[channel]
	physical := int(address>>2) & 3
	logical := [4]int{0, 2, 1, 3}[physical]
	op := &ch.Operators[logical]
	switch uint8(address) & 0xf0 {
	case 0x30:
		op.Detune = (value >> 4) & 7
		op.Multiple = value & 15
	case 0x40:
		op.Level = value & 127
	case 0x50:
		op.KeyScale = value >> 6
		op.Attack = value & 31
	case 0x60:
		op.AMEnabled = value&128 != 0
		op.Decay = value & 31
	case 0x70:
		op.SustainRate = value & 31
	case 0x80:
		op.SustainLevel = value >> 4
		op.Release = value & 15
		if op.EnvelopePhase == 3 && int32(op.Envelope) >= sustainLevel(op) {
			op.EnvelopePhase = 2
		}
	case 0x90:
		op.SSG = value & 15
	case 0xa0:
		switch physical {
		case 0:
			ch.Frequency = (uint16(ym.state.FrequencyLatch>>3) << 11) | (uint16(ym.state.FrequencyLatch&7) << 8) | uint16(value)
		case 1:
			ym.state.FrequencyLatch = value & 63
		case 2:
			if address < 256 {
				ym.state.SpecialFrequency[channel] = (uint16(ym.state.SpecialLatch>>3) << 11) | (uint16(ym.state.SpecialLatch&7) << 8) | uint16(value)
			}
		case 3:
			if address < 256 {
				ym.state.SpecialLatch = value & 63
			}
		}
	case 0xb0:
		if physical == 0 {
			ch.Algorithm = value & 7
			ch.Feedback = (value >> 3) & 7
		}
		if physical == 1 {
			ch.Pan = value & 0xc0
			ch.AMDepth = (value >> 4) & 3
			ch.PMDepth = value & 7
		}
	}
	return nil
}

// WriteRegister is a convenience for one banked register intent. The two port
// writes remain ordered at one caller-supplied time in the stream layer.
func (ym *YM) WriteRegister(bank, register, value uint8) error {
	if bank > 1 {
		return errors.New("invalid YM register bank")
	}
	_ = ym.WritePort(bank*2, register)
	return ym.WritePort(bank*2+1, value)
}

func envelopeOutput(op *YMOperator) uint32 {
	level := uint32(op.Envelope)
	if op.EnvelopePhase > 1 && op.SSG&8 != 0 && op.Negated != (op.SSG&4 != 0) {
		level = uint32((512 - int32(op.Envelope)) & 1023)
	}
	return level + uint32(op.Level)*8
}
func ssgTransition(op *YMOperator) {
	if op.SSG&8 == 0 || op.Envelope < 512 || op.EnvelopePhase <= 1 {
		return
	}
	if op.SSG&1 != 0 {
		if op.SSG&2 != 0 {
			op.Negated = true
		}
		if op.EnvelopePhase != 4 && op.Negated == (op.SSG&4 != 0) {
			op.Envelope = 1023
		}
		return
	}
	if op.SSG&2 != 0 {
		op.Negated = !op.Negated
	} else {
		op.Phase = 0
	}
	if op.EnvelopePhase != 4 {
		if rateCode(op.Attack)+int(op.LatchedKeyScale) >= 94 {
			op.Envelope = 0
		}
		if op.Envelope > 0 {
			op.EnvelopePhase = 4
		} else if sustainLevel(op) == 0 {
			op.EnvelopePhase = 2
		} else {
			op.EnvelopePhase = 3
		}
	}
}

// increment selects the small periodic bit pattern of the logarithmic rate
// circuit. Rates 0,1 of the lowest group and rate0 have no increment. The
// highest attack rates intentionally stop an already active attack.
func increment(code int, counter uint16, attack bool) int32 {
	if attack && code >= 94 {
		return 0
	}
	shift := 11
	if code >= 32 {
		shift = 11 - (code-32)/4
		if shift < 0 {
			shift = 0
		}
	}
	if uint32(counter)&((1<<shift)-1) != 0 {
		return 0
	}
	step := int(counter>>shift) & 7
	if code < 34 {
		return 0
	}
	group := (code - 32) / 4
	fraction := (code - 32) & 3
	if group >= 15 {
		return 8
	}
	// The four subrates add one pulse at fixed positions in an 8-step cycle.
	patterns := [4]uint8{0xaa, 0xba, 0xee, 0xfe}
	bit := int32((patterns[fraction] >> step) & 1)
	if group < 12 {
		return bit
	}
	// At rate12 the base is1, at13 it is2 and at14 it is4. Subrate0 has
	// no extra pulses; other subrates reuse the same high-rate positions.
	base := int32(1 << (group - 12))
	if fraction == 0 {
		return base
	}
	extra := [4]uint8{0, 0x88, 0xaa, 0xee}
	return base * (1 + int32((extra[fraction]>>step)&1))
}

func advanceEnvelope(op *YMOperator, counter uint16) {
	phase := op.EnvelopePhase
	if phase == 0 {
		return
	}
	code := int(op.LatchedKeyScale)
	switch phase {
	case 4:
		code += rateCode(op.Attack)
	case 3:
		code += rateCode(op.Decay)
	case 2:
		code += rateCode(op.SustainRate)
	case 1:
		code += 34 + int(op.Release)*4
	}
	inc := increment(code, counter, phase == 4)
	v := int32(op.Envelope)
	if phase == 4 {
		v += ((-v - 1) * inc) >> 4
		if v <= 0 {
			v = 0
			if sustainLevel(op) == 0 {
				op.EnvelopePhase = 2
			} else {
				op.EnvelopePhase = 3
			}
		}
	} else {
		if op.SSG&8 != 0 {
			if v < 512 {
				v += 4 * inc
			}
		} else {
			v += inc
		}
		if phase == 3 && v >= sustainLevel(op) {
			op.EnvelopePhase = 2
		}
		if phase == 2 && op.SSG&8 == 0 && v >= 1023 {
			v = 1023
		}
		limit := int32(1023)
		if op.SSG&8 != 0 {
			limit = 512
		}
		if phase == 1 && v >= limit {
			v = 1023
			op.EnvelopePhase = 0
		}
	}
	op.Envelope = uint16(v)
}

func detune(op *YMOperator, kc uint8) int32 {
	value := int32(detuneMagnitude[op.Detune&3][kc])
	if op.Detune&4 != 0 {
		return -value
	}
	return value
}
func phaseStep(op *YMOperator, frequency uint16, kc uint8, offset int32) uint32 {
	f := uint32(frequency & 2047)
	block := uint32(frequency >> 11)
	if offset != 0 {
		f = uint32(int32(f*2)+offset) & 4095
		f = (f << block) >> 2
	} else {
		f = (f << block) >> 1
	}
	f = uint32(int32(f)+detune(op, kc)) & 131071
	multiple := uint32(op.Multiple) * 2
	if multiple == 0 {
		multiple = 1
	}
	return f * multiple / 2
}
func pmOffset(frequency uint16, depth, cycle uint8) int32 {
	if depth == 0 {
		return 0
	}
	step := cycle >> 2
	quarter := step & 7
	if step&8 != 0 {
		quarter = 7 - quarter
	}
	base := uint32(pmBase[depth][quarter])
	var result uint32
	for bit := uint(4); bit <= 10; bit++ {
		if frequency&(1<<bit) == 0 {
			continue
		}
		if bit < 8 {
			result += base >> (8 - bit)
		} else {
			result += base << (bit - 8)
		}
	}
	if step&16 != 0 {
		return -int32(result)
	}
	return int32(result)
}

func operatorOutput(op *YMOperator, modulation int32, feedback bool, am uint32, carrier bool) int32 {
	atten := envelopeOutput(op)
	if op.AMEnabled {
		atten += am
	}
	if atten >= 832 {
		return 0
	}
	if !feedback {
		modulation >>= 1
	}
	phase := (op.Phase >> 10) + uint32(modulation)
	i := phase & 1023
	q := i & 255
	if i&256 != 0 {
		q = 255 - q
	}
	index := atten*8 + uint32(logSine[q])*2 + (i >> 9)
	if index >= 6656 {
		return 0
	}
	value := int32(powerMantissa[(index>>1)&255] >> (index >> 9))
	if index&1 != 0 {
		value = -value
	}
	if carrier {
		value &^= 31
	}
	return value
}

// algorithmGraph is the small connection circuit: destinations are logical
// op1..4, delayed memory, and output. Op1 has a dedicated feedback input. The
// calculation order 1,3,2,4 keeps the hardware's delayed connection explicit.
var algorithmGraph = [8][4]uint8{
	{2, 16, 8, 32}, {16, 16, 8, 32}, {8, 16, 8, 32}, {2, 16, 8, 32},
	{2, 32, 8, 32}, {2 | 8 | 16, 32, 32, 32}, {2, 32, 32, 32}, {32, 32, 32, 32},
}

func (ym *YM) channelOutput(channel int) int32 {
	ch := &ym.state.Channels[channel]
	var input [4]int32
	var output int32
	memory := int32(0)
	switch ch.Algorithm {
	case 0, 1, 2, 5:
		input[2] = ch.Memory
	case 3:
		input[3] = ch.Memory
	default:
		memory = ch.Memory
	}
	am := uint32(0)
	cycle := ym.state.LFOCycle
	if cycle < 64 {
		am = uint32(cycle^63) * 2
	} else {
		am = uint32(cycle&63) * 2
	}
	am >>= [4]uint{8, 3, 1, 0}[ch.AMDepth]
	for _, opIndex := range [4]int{0, 2, 1, 3} {
		op := &ch.Operators[opIndex]
		modulation := input[opIndex]
		if opIndex == 0 {
			modulation = 0
			if ch.Feedback != 0 {
				modulation = (ch.FeedbackHistory[0] + ch.FeedbackHistory[1]) >> (10 - ch.Feedback)
			}
		}
		route := algorithmGraph[ch.Algorithm][opIndex]
		value := operatorOutput(op, modulation, opIndex == 0, am, route&32 != 0)
		if opIndex == 0 {
			ch.FeedbackHistory[0] = ch.FeedbackHistory[1]
			ch.FeedbackHistory[1] = value
		}
		for target := 0; target < 4; target++ {
			if route&(1<<target) != 0 {
				input[target] += value
			}
		}
		if route&16 != 0 {
			memory += value
		}
		if route&32 != 0 {
			output += value
		}
	}
	ch.Memory = memory
	for i := range ch.Operators {
		frequency := ym.frequency(channel, i)
		kc := keyCode(frequency)
		offset := pmOffset(frequency, ch.PMDepth, ym.state.LFOCycle)
		if offset != 0 && channel == 2 && ym.state.Mode&0xc0 != 0 {
			kc = keyCode(ch.Frequency)
		}
		ch.Operators[i].Phase += phaseStep(&ch.Operators[i], frequency, kc, offset)
	}
	return output
}

// Generate returns signed 32-bit interleaved samples at one chip sample per
// 1008 master cycles. It preserves a caller's update boundary (including zero
// length) for the timer-B/reference semantics; resampling belongs to Stream.
func (ym *YM) Generate(count int) ([]int32, error) {
	if count < 0 || count > MaxYMBlockSamples {
		return nil, errors.New("invalid YM sample block length")
	}
	for channel := range ym.state.Channels {
		for operator := range ym.state.Channels[channel].Operators {
			op := &ym.state.Channels[channel].Operators[operator]
			op.LatchedKeyScale = keyCode(ym.frequency(channel, operator)) >> (3 - op.KeyScale)
		}
	}
	output := make([]int32, count*2)
	for sample := 0; sample < count; sample++ {
		for channel := range ym.state.Channels {
			for operator := range ym.state.Channels[channel].Operators {
				ssgTransition(&ym.state.Channels[channel].Operators[operator])
			}
		}
		var levels [6]int32
		for channel := range levels {
			if channel == 5 && ym.state.DACEnabled {
				levels[channel] = (int32(ym.state.DACValue) - 128) * 64
			} else {
				levels[channel] = ym.channelOutput(channel)
			}
		}
		if ym.state.LFOEnabled {
			ym.state.LFODivider++
			if ym.state.LFODivider >= lfoPeriod[ym.state.LFOSpeed] {
				ym.state.LFODivider = 0
				ym.state.LFOCycle = (ym.state.LFOCycle + 1) & 127
			}
		}
		ym.state.EnvelopeDivider++
		if ym.state.EnvelopeDivider == 3 {
			ym.state.EnvelopeDivider = 0
			ym.state.EnvelopeCount++
			if ym.state.EnvelopeCount == 4096 {
				ym.state.EnvelopeCount = 1
			}
			for channel := range ym.state.Channels {
				for operator := range ym.state.Channels[channel].Operators {
					advanceEnvelope(&ym.state.Channels[channel].Operators[operator], ym.state.EnvelopeCount)
				}
			}
		}
		for channel, value := range levels {
			if value > 8191 {
				value = 8191
			} else if value < -8192 {
				value = -8192
			}
			for side, bit := range [2]uint8{128, 64} {
				on := ym.state.Channels[channel].Pan&bit != 0
				if on {
					output[sample*2+side] += value
				}
				offset := int32(128)
				if value < 0 {
					offset = -128
					if on {
						offset = -96
					}
				}
				output[sample*2+side] += offset
			}
		}
		ym.state.CSM <<= 1
		if ym.state.Mode&1 != 0 {
			ym.state.TimerACounter--
			if ym.state.TimerACounter <= 0 {
				if ym.state.Mode&4 != 0 {
					ym.state.Status |= 1
				}
				ym.state.TimerACounter = 1024 - int32(ym.state.TimerAValue)
				if ym.state.Mode&0xc0 == 0x80 {
					ym.csmKey(true)
					ym.state.CSM = 1
				}
			}
		}
		if ym.state.CSM&2 != 0 {
			ym.csmKey(false)
			ym.state.CSM = 0
		}
	}
	if ym.state.Mode&2 != 0 {
		ym.state.TimerBCounter -= int32(count)
		if ym.state.TimerBCounter <= 0 {
			if ym.state.Mode&8 != 0 {
				ym.state.Status |= 2
			}
			period := (256 - int32(ym.state.TimerBValue)) * 16
			ym.state.TimerBCounter += ((-ym.state.TimerBCounter)/period + 1) * period
		}
	}
	return output, nil
}
