package scoreclock

import "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"

// These fixed scans are shared by standalone leaves and the actual Root
// coordinator. Each canonical write advances the same service exactly once.
type levelScan struct {
	s       *service
	writes  [24]rootcontrol.LevelWrite
	restore bool
	index   int
	done    bool
}

func beginLevelScan(s *service, writes [24]rootcontrol.LevelWrite, restore bool) (*levelScan, error) {
	if !restore {
		if err := s.spend(4); err != nil {
			return nil, err
		}
	}
	if err := s.spend(4); err != nil {
		return nil, err
	}
	if err := s.ordinaryCosts(4, 4, 4); err != nil {
		return nil, err
	}
	return &levelScan{s: s, writes: writes, restore: restore}, nil
}
func (l *levelScan) next(write rootcontrol.LevelWrite) error {
	if l.done || l.index >= 24 || write != l.writes[l.index] {
		return ErrRunningSequence
	}
	if l.restore {
		if err := l.s.spend(14); err != nil {
			return err
		}
	}
	if err := l.s.spend(18); err != nil {
		return err
	}
	if err := l.s.serveYM(Request{Bank: write.Bank, Register: write.Register, Value: write.Value}); err != nil {
		return err
	}
	l.index++
	if l.index%2 == 0 {
		if err := l.s.spend(4); err != nil {
			return err
		}
		last := l.index%8 == 0
		if err := l.s.pauseLoopEnd(last); err != nil {
			return err
		}
		if last {
			if err := l.s.pauseLoopEnd(l.index == 24); err != nil {
				return err
			}
			if l.index < 24 {
				if err := l.s.ordinaryCosts(4, 4, 4); err != nil {
					return err
				}
			}
		}
	}
	if l.index == 24 {
		if err := l.s.spend(16); err != nil {
			return err
		}
		l.done = true
	}
	return nil
}

type mutePSGScan struct {
	s     *service
	chip  PSGChip
	index int
	done  bool
}

var mutePSGValues = [4]uint8{159, 191, 223, 255}

func beginMutePSG(s *service, chip PSGChip) (*mutePSGScan, error) {
	if chip == nil {
		return nil, ErrRunningProfile
	}
	if err := s.spend(12); err != nil {
		return nil, err
	}
	return &mutePSGScan{s: s, chip: chip}, nil
}
func (m *mutePSGScan) next(value uint8) error {
	if m.done || m.index >= 4 || value != mutePSGValues[m.index] {
		return ErrRunningSequence
	}
	if err := m.s.cost(12, func() error { return m.chip.WritePSG(m.s.clock.Cycle, value) }); err != nil {
		return err
	}
	m.index++
	if m.index == 4 {
		if err := m.s.spend(16); err != nil {
			return err
		}
		m.done = true
	}
	return nil
}
