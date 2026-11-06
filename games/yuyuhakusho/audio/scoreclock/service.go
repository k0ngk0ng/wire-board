// Package scoreclock accounts for bounded semantic sound-service costs.
// It does not execute source CPUs or infer an entire driver tick's timing.
package scoreclock

import (
	"errors"
	"fmt"
	"math"
)

type Clock struct {
	Cycle   uint64 `json:"cycle"`
	Refresh uint64 `json:"refresh"`
}
type Request struct{ Bank, Register, Value uint8 }
type Limits struct{ MaxOperations uint32 }
type LevelEffect struct {
	At       uint64 `json:"at"`
	Bank     uint8  `json:"bank"`
	Register uint8  `json:"register"`
	Value    uint8  `json:"value"`
}
type Result struct {
	Clock Clock
	Level *LevelEffect
}

// Bus owns the actual auxiliary player/hold/mailbox state. RequestBus may
// return a later main-clock cursor when synchronizing that independent owner.
type Bus interface {
	RequestBus(at uint64, asserted bool) (after uint64, err error)
	BusGranted(at uint64) (bool, error)
	Mailbox(at uint64) (uint8, error)
}

// Chip owns busy and all synthesis state. The service only queries real status
// and issues ordered writes; no busy deadline or register shadow lives here.
type Chip interface {
	ReadStatus(cycle uint64, port uint8) (uint8, error)
	WritePort(cycle uint64, port, value uint8) error
}

var ErrBudget = errors.New("sound service operation budget exhausted")

type service struct {
	clock      Clock
	bus        Bus
	chip       Chip
	operations uint32
	limit      uint32
	level      *LevelEffect
	before     func() error // Borrowed main-batch coordinator for an enclosing IRQ.
}

func (s *service) add(delta uint64) error {
	if s.clock.Cycle > math.MaxUint64-delta {
		return errors.New("sound service clock overflow")
	}
	s.clock.Cycle += delta
	return nil
}

// cost begins one fixed semantic operation at the reference refresh boundary.
// The fixed service is written below; no opcode or caller-supplied program is
// decoded. Its architecture costs are bound to the native source evidence.
func (s *service) cost(cpu uint64, io func() error) error {
	if s.operations >= s.limit {
		return ErrBudget
	}
	if s.before != nil {
		if err := s.before(); err != nil {
			return err
		}
	}
	s.operations++
	if s.clock.Cycle >= s.clock.Refresh {
		if s.clock.Cycle > math.MaxUint64-896 {
			return errors.New("sound service refresh overflow")
		}
		s.clock.Refresh = s.clock.Cycle + 896
		if err := s.add(14); err != nil {
			return err
		}
	}
	if io != nil {
		if err := io(); err != nil {
			return err
		}
	}
	return s.add(cpu * 7)
}
func (s *service) spend(cpu uint64) error { return s.cost(cpu, nil) }
func (s *service) request(cpu uint64, asserted bool) error {
	return s.cost(cpu, func() error {
		after, err := s.bus.RequestBus(s.clock.Cycle, asserted)
		if err != nil {
			return err
		}
		if after < s.clock.Cycle {
			return errors.New("auxiliary bus returned a backwards main cursor")
		}
		s.clock.Cycle = after
		return nil
	})
}
func (s *service) branch(taken bool, long bool) error {
	if taken {
		return s.spend(10)
	}
	if long {
		return s.spend(12)
	}
	return s.spend(8)
}

func (s *service) registerPrefix(r Request) error {
	if err := s.spend(8); err != nil { // register range comparison
		return err
	}
	if err := s.branch(r.Register >= 0x50, true); err != nil {
		return err
	}
	if r.Register < 0x50 {
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.branch(r.Register < 0x40, true); err != nil {
			return err
		}
		if r.Register >= 0x40 {
			if err := s.spend(8); err != nil { // bounded register normalization
				return err
			}
			if err := s.spend(8); err != nil { // level cache selection
				return err
			}
			if err := s.cost(14, func() error {
				s.level = &LevelEffect{At: s.clock.Cycle, Bank: r.Bank, Register: r.Register, Value: r.Value}
				return nil
			}); err != nil {
				return err
			}
		}
	}
	// Port selection, saved interrupt mask and temporary interrupt masking.
	if err := s.spend(12); err != nil {
		return err
	}
	if err := s.spend(14); err != nil {
		return err
	}
	return s.spend(20)
}

func (s *service) acquire() error {
	for {
		if err := s.request(20, true); err != nil {
			return err
		}
		for {
			granted := false
			if err := s.cost(24, func() error {
				var err error
				granted, err = s.bus.BusGranted(s.clock.Cycle)
				if err != nil {
					return err
				}
				after, err := s.bus.RequestBus(s.clock.Cycle, true)
				if err != nil {
					return err
				}
				if after < s.clock.Cycle {
					return errors.New("auxiliary bus returned a backwards main cursor")
				}
				s.clock.Cycle = after
				return nil
			}); err != nil {
				return err
			}
			if err := s.branch(!granted, false); err != nil {
				return err
			}
			if granted {
				break
			}
		}
		mailbox := uint8(0)
		if err := s.cost(16, func() error {
			if err := s.add(7); err != nil {
				return err
			}
			var err error
			mailbox, err = s.bus.Mailbox(s.clock.Cycle)
			return err
		}); err != nil {
			return err
		}
		if err := s.branch(mailbox == 0, false); err != nil {
			return err
		}
		if mailbox == 0 {
			return nil
		}
		if err := s.request(20, false); err != nil {
			return err
		}
		if err := s.spend(18); err != nil { // retry delay invocation
			return err
		}
		for n := 0; n < 16; n++ {
			if err := s.spend(4); err != nil {
				return err
			}
		}
		if err := s.spend(16); err != nil {
			return err
		}
		if err := s.spend(10); err != nil {
			return err
		}
	}
}

func (s *service) waitChip() error {
	for {
		var status uint8
		if err := s.cost(8, func() error {
			if err := s.add(7); err != nil {
				return err
			}
			var err error
			status, err = s.chip.ReadStatus(s.clock.Cycle, 0)
			return err
		}); err != nil {
			return err
		}
		if err := s.branch(status&0x80 != 0, false); err != nil {
			return err
		}
		if status&0x80 == 0 {
			return nil
		}
	}
}
func (s *service) writePort(port, value uint8) error {
	return s.cost(12, func() error {
		if err := s.add(7); err != nil {
			return err
		}
		return s.chip.WritePort(s.clock.Cycle, port, value)
	})
}

// serveYM shares the caller's temporary clock and operation budget. The
// canonical leaf remains identical for standalone and composed invocations.
func (s *service) serveYM(request Request) error {
	if err := s.registerPrefix(request); err != nil {
		return err
	}
	if err := s.acquire(); err != nil {
		return err
	}
	if err := s.waitChip(); err != nil {
		return err
	}
	if err := s.writePort(request.Bank*2, request.Register); err != nil {
		return err
	}
	if err := s.spend(4); err != nil {
		return err
	}
	if err := s.waitChip(); err != nil {
		return err
	}
	if err := s.writePort(request.Bank*2+1, request.Value); err != nil {
		return err
	}
	if err := s.request(20, false); err != nil {
		return err
	}
	if err := s.spend(16); err != nil {
		return fmt.Errorf("restore service interrupt mask: %w", err)
	}
	if err := s.spend(16); err != nil {
		return err
	}
	return nil
}

// ServeYM performs one canonical register service from a supplied origin.
// Callers use detached transactional chip/bus owners and commit them only after
// success. Errors can follow prior external operations; this function does not
// duplicate or silently restore those owners. Frame flushing is the caller's
// explicit responsibility, not an implicit action of this service.
func ServeYM(clock Clock, request Request, bus Bus, chip Chip, limits Limits) (Result, error) {
	if request.Bank > 1 || bus == nil || chip == nil || limits.MaxOperations > 65536 || clock.Cycle > math.MaxUint64-896 {
		return Result{}, errors.New("invalid canonical YM service input")
	}
	limit := limits.MaxOperations
	if limit == 0 {
		limit = 4096
	}
	s := service{clock: clock, bus: bus, chip: chip, limit: limit}
	if err := s.serveYM(request); err != nil {
		return Result{}, err
	}
	return Result{Clock: s.clock, Level: s.level}, nil
}
