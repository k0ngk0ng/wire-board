package scoreclock

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
)

type PauseChip interface {
	Chip
	PSGChip
}

type PauseResult struct {
	Clock  Clock
	Levels rootcontrol.PauseLevels
}

func pauseService(clock Clock, bus Bus, chip Chip, limits Limits) (*service, error) {
	if bus == nil || chip == nil {
		return nil, errors.New("missing pause bus/chip owner")
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return nil, err
	}
	s.bus, s.chip = bus, chip
	return s, nil
}

func (s *service) pauseLoopEnd(last bool) error {
	if last {
		return s.spend(14)
	}
	return s.spend(10)
}

// pauseLevels is one fixed three-channel/four-operator/both-bank caller.
// All argument setup, calls, memory loads, final DBF paths and one return are
// accounted here; the canonical YM leaf contributes its own work exactly once.
func (s *service) pauseLevels(writes [24]rootcontrol.LevelWrite, restore bool) error {
	if !restore {
		if err := s.spend(4); err != nil {
			return err
		} // select mute level
	}
	if err := s.spend(4); err != nil {
		return err
	} // three channels
	index := 0
	for channel := 0; channel < 3; channel++ {
		for n := 0; n < 3; n++ {
			if err := s.spend(4); err != nil {
				return err
			} // base, channel, operator count
		}
		for operator := 0; operator < 4; operator++ {
			for bank := 0; bank < 2; bank++ {
				if restore {
					if err := s.spend(14); err != nil {
						return err
					} // live retained level
				}
				if err := s.spend(18); err != nil {
					return err
				} // PC-relative caller
				write := writes[index]
				index++
				if err := s.serveYM(Request{Bank: write.Bank, Register: write.Register, Value: write.Value}); err != nil {
					return err
				}
			}
			if err := s.spend(4); err != nil {
				return err
			} // next physical operator
			if err := s.pauseLoopEnd(operator == 3); err != nil {
				return err
			}
		}
		if err := s.pauseLoopEnd(channel == 2); err != nil {
			return err
		}
	}
	return s.spend(16)
}

// StartPause begins immediately after EnterTick's actual pause-ack store. It
// acquires the real hardware bus before capturing visible levels, then mutes
// FM and PSG in source order. It does not create the later world/IRQ frame
// hold. External owners must be detached; any failure discards the whole call.
func StartPause(clock Clock, bus Bus, chip PauseChip, capture func(uint64) (rootcontrol.PauseLevels, error), limits Limits) (PauseResult, error) {
	if capture == nil {
		return PauseResult{}, errors.New("missing live pause level capture")
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return PauseResult{}, err
	}
	for _, cost := range [...]uint64{14, 20} {
		if err = s.spend(cost); err != nil {
			return PauseResult{}, err
		}
	}
	if err = s.acquire(); err != nil {
		return PauseResult{}, err
	}
	if err = s.spend(16); err != nil {
		return PauseResult{}, err
	} // restore mask
	levels, err := capture(s.clock.Cycle)
	if err != nil {
		return PauseResult{}, err
	}
	if _, err = rootcontrol.PauseRestoreWrites(levels); err != nil {
		return PauseResult{}, err
	}
	if err = s.spend(8); err != nil {
		return PauseResult{}, err
	} // select retained levels
	for n := 0; n < 8; n++ {
		if err = s.spend(20); err != nil {
			return PauseResult{}, err
		} // preserve fixed 32-byte source group
	}
	if err = s.spend(18); err != nil {
		return PauseResult{}, err
	}
	if err = s.pauseLevels(rootcontrol.PauseMuteWrites(), false); err != nil {
		return PauseResult{}, err
	}
	if err = s.spend(8); err != nil {
		return PauseResult{}, err
	}
	for n := 0; n < 8; n++ {
		if err = s.spend(20); err != nil {
			return PauseResult{}, err
		}
	}
	if err = s.spend(10); err != nil {
		return PauseResult{}, err
	} // fixed tail branch
	if err = s.servePSG(PSGRequest{Kind: PSGMuteAll}, chip); err != nil {
		return PauseResult{}, err
	}
	return PauseResult{Clock: s.clock, Levels: levels}, nil
}

// ResumePause begins after EnterTick's actual resume-clear store. It releases
// the same bus and restores only the retained FM levels; PSG remains muted
// until later score work writes it. The payload is read-only and not consumed
// on failure. The caller releases its pending Root payload only after commit.
func ResumePause(clock Clock, levels rootcontrol.PauseLevels, bus Bus, chip Chip, limits Limits) (Clock, error) {
	writes, err := rootcontrol.PauseRestoreWrites(levels)
	if err != nil {
		return Clock{}, err
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return Clock{}, err
	}
	for _, cost := range [...]uint64{14, 20} {
		if err = s.spend(cost); err != nil {
			return Clock{}, err
		}
	}
	if err = s.request(20, false); err != nil {
		return Clock{}, err
	}
	if err = s.spend(16); err != nil {
		return Clock{}, err
	}
	if err = s.spend(8); err != nil {
		return Clock{}, err
	}
	if err = s.pauseLevels(writes, true); err != nil {
		return Clock{}, err
	}
	return s.clock, nil
}
