package engine

import (
	"errors"
	"math"
)

// MainWorkOperation prices one fixed semantic work quantum. TailCycles follow
// the internal refresh skip (used by temporary-register saves/restores). This
// carries no opcode, source address, register file or executable program.
type MainWorkOperation struct {
	Phase           string
	CPUCycles       uint16
	InternalRefresh bool
	TailCycles      uint16
}

func (op MainWorkOperation) Advance(cycle, refresh uint64) (uint64, uint64, error) {
	if op.Phase == "" || op.CPUCycles < 4 || op.CPUCycles > 136 || op.CPUCycles&1 != 0 || op.TailCycles > 16 || op.TailCycles&1 != 0 || op.TailCycles != 0 && !op.InternalRefresh {
		return 0, 0, errors.New("invalid main work quantum")
	}
	if cycle >= refresh {
		if cycle > math.MaxUint64-896 {
			return 0, 0, errors.New("main work refresh overflow")
		}
		refresh = cycle + 896
		cycle += 14
	}
	delta := uint64(op.CPUCycles) * 7
	if cycle > math.MaxUint64-delta {
		return 0, 0, errors.New("main work clock overflow")
	}
	cycle += delta
	if op.InternalRefresh && cycle >= refresh {
		if refresh > math.MaxUint64-896 {
			return 0, 0, errors.New("main internal refresh overflow")
		}
		refresh += 896
	}
	delta = uint64(op.TailCycles) * 7
	if cycle > math.MaxUint64-delta {
		return 0, 0, errors.New("main work tail overflow")
	}
	cycle += delta
	return cycle, refresh, nil
}
