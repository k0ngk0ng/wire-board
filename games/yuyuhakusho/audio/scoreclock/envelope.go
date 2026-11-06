package scoreclock

import (
	"errors"
	"math/bits"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
)

var ErrEnvelopeWorkUnavailable = errors.New("envelope Work clock unavailable")
var ErrVolumeSequence = errors.New("invalid envelope volume sequence")

type PitchCalculationRequest struct {
	Family            string
	HasEnvelope       bool
	Work              score.EnvelopePathWork
	Tick              param.PitchTick
	Scale             int8
	ModulationEnabled bool
}
type PitchCalculationResult struct {
	Clock Clock
	Next  string
}

func (s *service) envelopeSampleRead() error { return s.ordinaryCosts(4, 12, 16, 14) }

func PricePitchCalculation(clock Clock, r PitchCalculationRequest, limits Limits) (PitchCalculationResult, error) {
	if r.Family != "fm" && r.Family != "psg" || r.Work.Restarts > 1 || r.Tick.ScaleAdd != 0 {
		return PitchCalculationResult{}, ErrEnvelopeWorkUnavailable
	}
	if r.HasEnvelope {
		if err := r.Tick.Validate(); err != nil {
			return PitchCalculationResult{}, err
		}
	} else if r.Work.Restarts != 0 || r.Tick.Offset != nil || r.Tick.KeyOff || r.Tick.FixedPitchWord != nil {
		return PitchCalculationResult{}, ErrEnvelopeWorkUnavailable
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return PitchCalculationResult{}, err
	}
	return pricePitchCalculation(s, r)
}

func pricePitchCalculation(s *service, r PitchCalculationRequest) (PitchCalculationResult, error) {
	var err error
	if err = s.ordinaryCosts(4, 12, 8); err != nil {
		return PitchCalculationResult{}, err
	}
	if err = s.branch(!r.HasEnvelope, false); err != nil {
		return PitchCalculationResult{}, err
	}
	if r.HasEnvelope {
		if err = s.ordinaryCosts(8, 4, 10, 18); err != nil {
			return PitchCalculationResult{}, err
		}
		for n := uint8(0); n < r.Work.Restarts; n++ {
			if err = s.envelopeSampleRead(); err != nil {
				return PitchCalculationResult{}, err
			}
			if err = s.ordinaryCosts(8, 8, 10, 16, 10); err != nil {
				return PitchCalculationResult{}, err
			}
		}
		if err = s.envelopeSampleRead(); err != nil {
			return PitchCalculationResult{}, err
		}
		positive := !r.Tick.KeyOff && *r.Tick.Offset >= 0
		if err = s.branch(positive, false); err != nil {
			return PitchCalculationResult{}, err
		}
		if !positive {
			if err = s.ordinaryCosts(8, 8, 8, 8, 8); err != nil {
				return PitchCalculationResult{}, err
			}
			if err = s.branch(r.Tick.KeyOff, false); err != nil {
				return PitchCalculationResult{}, err
			}
			if r.Tick.KeyOff {
				if err = s.ordinaryCosts(16, 12); err != nil {
					return PitchCalculationResult{}, err
				}
				if err = s.branch(r.Family == "psg", false); err != nil {
					return PitchCalculationResult{}, err
				}
				if err = s.spend(10); err != nil {
					return PitchCalculationResult{}, err
				}
				return PitchCalculationResult{Clock: s.clock, Next: "key-off"}, nil
			}
			if err = s.ordinaryCosts(8, 8, 8, 8); err != nil {
				return PitchCalculationResult{}, err
			}
		}
		if err = s.ordinaryCosts(4, 12, 4); err != nil {
			return PitchCalculationResult{}, err
		}
		// This core charges all MULU work internally with base zero, and no
		// internal refresh skip; signed-byte scale is extended to 16 bits.
		if err = s.spend(uint64(38 + 2*bits.OnesCount16(uint16(int16(r.Scale))))); err != nil {
			return PitchCalculationResult{}, err
		}
	}
	if err = s.ordinaryCosts(12, 4, 4, 12, 12); err != nil {
		return PitchCalculationResult{}, err
	}
	if err = s.branch(!r.ModulationEnabled, false); err != nil {
		return PitchCalculationResult{}, err
	}
	if r.ModulationEnabled {
		if err = s.spend(12); err != nil {
			return PitchCalculationResult{}, err
		}
	}
	if err = s.spend(16); err != nil {
		return PitchCalculationResult{}, err
	}
	return PitchCalculationResult{Clock: s.clock, Next: "complete"}, nil
}

type EnvelopeVolumeRequest struct {
	HasEnvelope              bool
	Work                     score.EnvelopePathWork
	Tick                     param.VolumeTick
	BaseLevel, Channel       uint8
	NoteOff, Claimed, Tied   bool
	GateTicks, GateRemaining uint8
}
type VolumeTransfer struct {
	s                       *service
	chip                    PSGChip
	value                   uint8
	expected, applied, done bool
	failure                 error
}

func BeginEnvelopeVolume(clock Clock, r EnvelopeVolumeRequest, chip PSGChip, limits Limits) (*VolumeTransfer, error) {
	if r.Channel != 128 && r.Channel != 160 && r.Channel != 192 && r.Channel != 224 || r.Work.Restarts != 0 {
		return nil, ErrEnvelopeWorkUnavailable
	}
	if r.HasEnvelope {
		if err := r.Tick.Validate(); err != nil {
			return nil, err
		}
		if r.Tick.Level != nil && *r.Tick.Level >= 128 && *r.Tick.Level <= 131 {
			return nil, ErrEnvelopeWorkUnavailable
		}
	} else if r.Tick.Level != nil || r.Tick.WritesLevel || r.Tick.KeyOff {
		return nil, ErrEnvelopeWorkUnavailable
	}
	s, err := newPSGService(clock, chip, limits)
	if err != nil {
		return nil, err
	}
	return beginEnvelopeVolume(s, r, chip)
}

func beginEnvelopeVolume(s *service, r EnvelopeVolumeRequest, chip PSGChip) (*VolumeTransfer, error) {
	var err error
	t := &VolumeTransfer{s: s, chip: chip}
	if err = s.ordinaryCosts(12, 4, 12); err != nil {
		return nil, err
	}
	if err = s.branch(!r.HasEnvelope, false); err != nil {
		return nil, err
	}
	level := r.BaseLevel
	if r.HasEnvelope {
		if err = s.ordinaryCosts(8, 4, 10, 18); err != nil {
			return nil, err
		}
		if err = s.envelopeSampleRead(); err != nil {
			return nil, err
		}
		positive := r.Tick.WritesLevel && *r.Tick.Level < 128
		if err = s.branch(positive, false); err != nil {
			return nil, err
		}
		if !positive {
			if err = s.spend(8); err != nil {
				return nil, err
			}
			if err = s.branch(r.Tick.KeyOff, false); err != nil {
				return nil, err
			}
			if r.Tick.KeyOff {
				if err = s.ordinaryCosts(16, 16, 10, 12); err != nil {
					return nil, err
				}
				if err = s.branch(r.Claimed, false); err != nil {
					return nil, err
				}
				if r.Claimed {
					return t, nil
				}
				if err = s.ordinaryCosts(12, 8); err != nil {
					return nil, err
				}
				t.expected, t.value = true, r.Channel|31
				return t, nil
			}
			if err = s.spend(8); err != nil {
				return nil, err
			}
			hold := !r.Tick.WritesLevel
			if err = s.branch(hold, false); err != nil {
				return nil, err
			}
			if hold {
				if err = s.spend(16); err != nil {
					return nil, err
				}
				return t, nil
			}
			if err = s.ordinaryCosts(8, 8, 8, 8); err != nil {
				return nil, err
			}
		}
		if err = s.ordinaryCosts(4, 8); err != nil {
			return nil, err
		}
		level = r.BaseLevel + *r.Tick.Level
		if err = s.branch(level < 16, false); err != nil {
			return nil, err
		}
		if level >= 16 {
			if err = s.spend(4); err != nil {
				return nil, err
			}
			level = 15
		}
	}
	return beginBaseVolume(s, r, chip, level)
}

// beginBaseVolume starts at the shared immediate attenuation guards. Fade
// enters here directly; normal volume has already resolved its envelope.
func beginBaseVolume(s *service, r EnvelopeVolumeRequest, chip PSGChip, level uint8) (*VolumeTransfer, error) {
	var err error
	t := &VolumeTransfer{s: s, chip: chip}
	if err = s.spend(12); err != nil {
		return nil, err
	}
	if err = s.branch(r.NoteOff, false); err != nil {
		return nil, err
	}
	if r.NoteOff {
		return t, nil
	}
	if err = s.spend(12); err != nil {
		return nil, err
	}
	if err = s.branch(r.Claimed, false); err != nil {
		return nil, err
	}
	if r.Claimed {
		return t, nil
	}
	if err = s.spend(12); err != nil {
		return nil, err
	}
	if err = s.branch(r.Tied, false); err != nil {
		return nil, err
	}
	if r.Tied {
		if err = s.spend(12); err != nil {
			return nil, err
		}
		if err = s.branch(r.GateTicks == 0, false); err != nil {
			return nil, err
		}
		if r.GateTicks != 0 {
			if err = s.spend(12); err != nil {
				return nil, err
			}
			if err = s.branch(r.GateRemaining != 0, false); err != nil {
				return nil, err
			}
			if r.GateRemaining == 0 {
				return t, nil
			}
		}
	}
	if err = s.ordinaryCosts(12, 8); err != nil {
		return nil, err
	}
	t.expected, t.value = true, (level|r.Channel)+16
	return t, nil
}

func (t *VolumeTransfer) ExpectedWrite() (uint8, bool) { return t.value, t.expected }
func (t *VolumeTransfer) Clock() Clock                 { return t.s.clock }
func (t *VolumeTransfer) fail(err error) error         { t.failure = err; return err }
func (t *VolumeTransfer) Write(value uint8) error {
	if t == nil {
		return ErrVolumeSequence
	}
	if t.done || t.failure != nil || t.applied || !t.expected || value != t.value {
		return t.fail(ErrVolumeSequence)
	}
	if err := t.s.cost(16, func() error { return t.chip.WritePSG(t.s.clock.Cycle, value) }); err != nil {
		return t.fail(err)
	}
	t.applied = true
	return nil
}
func (t *VolumeTransfer) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrVolumeSequence
	}
	if t.done || t.failure != nil || t.expected && !t.applied {
		return Clock{}, t.fail(ErrVolumeSequence)
	}
	if err := t.s.spend(16); err != nil {
		return Clock{}, t.fail(err)
	}
	t.done = true
	return t.s.clock, nil
}
func ServeEnvelopeVolume(clock Clock, r EnvelopeVolumeRequest, chip PSGChip, limits Limits) (Clock, error) {
	t, err := BeginEnvelopeVolume(clock, r, chip, limits)
	if err != nil {
		return Clock{}, err
	}
	if value, ok := t.ExpectedWrite(); ok {
		if err = t.Write(value); err != nil {
			return Clock{}, err
		}
	}
	return t.Finish()
}
