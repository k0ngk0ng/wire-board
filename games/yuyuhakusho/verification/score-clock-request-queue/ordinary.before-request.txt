package scoreclock

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/resourcebridge"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

var ErrOrdinarySequence = errors.New("invalid ordinary PCM publication sequence")

type PCMChoiceField interface {
	ReadPCMChoice(at uint64) (uint8, error)
	WritePCMChoice(at uint64, value uint8) error
}

type OrdinaryBus interface {
	Bus
	OrdinaryDescriptor(id string) (pcmcommands.Descriptor, bool)
	ReadArbitration(at uint64, field pcmcommands.ArbitrationField) (uint8, error)
	PublishAcceptedAt(at uint64, input pcmcommands.Input) error
}

type OrdinaryResult struct {
	Clock    Clock
	Accepted bool
	Input    pcmcommands.Input
}

// OrdinaryPublication retains only a temporary accepted decision/work cursor.
// Its sampled StartDual survives the source's released arbitration interval.
// Root/PCM/resources each keep their sole canonical state and payload owner.
type OrdinaryPublication struct {
	s       *service
	bus     OrdinaryBus
	choice  PCMChoiceField
	input   pcmcommands.Input
	applied bool
	done    bool
	failure error
}

func (s *service) ordinaryCosts(values ...uint64) error {
	for _, value := range values {
		if err := s.spend(value); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) ordinaryCompare(taken bool) error {
	if err := s.spend(4); err != nil {
		return err
	}
	return s.branch(taken, true)
}

// ordinaryArbitration prices the source's fixed decision stages. The returned
// branch is checked against the canonical engine through PrepareOrdinary;
// this function neither mutates choice nor publishes a request.
func (s *service) ordinaryArbitration(d pcmcommands.Descriptor, v pcmcommands.Status, choice PCMChoiceField) (engine.PCMDestination, uint8, error) {
	priority := d.Flags & 192
	a, b := v.SlotFlags[0]&192, v.SlotFlags[1]&192
	if err := s.ordinaryCosts(4, 12, 8, 10); err != nil {
		return 0, 0, err
	}
	dualMode := v.ModeFlags&1 != 0
	if err := s.branch(dualMode, true); err != nil {
		return 0, 0, err
	}
	if d.Flags&1 == 0 {
		if !dualMode {
			if err := s.spend(8); err != nil {
				return 0, 0, err
			}
			accepted := priority >= v.ModeFlags&192
			if err := s.ordinaryCompare(accepted); err != nil {
				return 0, 0, err
			}
			if accepted {
				return engine.PCMSingle, 0, nil
			}
			return engine.PCMReject, 0, nil
		}
		if err := s.ordinaryCosts(8, 8); err != nil {
			return 0, 0, err
		}
		if err := s.ordinaryCompare(priority < a); err != nil {
			return 0, 0, err
		}
		if priority < a {
			return engine.PCMReject, 0, nil
		}
		if err := s.ordinaryCompare(priority >= b); err != nil {
			return 0, 0, err
		}
		if priority >= b {
			return engine.PCMSingle, 0, nil
		}
		return engine.PCMReject, 0, nil
	}
	if !dualMode {
		if err := s.spend(8); err != nil {
			return 0, 0, err
		}
		rejected := priority < v.ModeFlags&192
		if err := s.ordinaryCompare(rejected); err != nil {
			return 0, 0, err
		}
		if rejected {
			return engine.PCMReject, 0, nil
		}
	}
	if err := s.ordinaryCosts(8, 8, 12, 8); err != nil {
		return 0, 0, err
	}
	route := d.RoutingOrPan & 192
	if err := s.branch(route == 0, true); err != nil {
		return 0, 0, err
	}
	if route != 0 {
		if err := s.spend(8); err != nil {
			return 0, 0, err
		}
		if err := s.branch(route == 192, true); err != nil {
			return 0, 0, err
		}
		if route != 192 {
			if err := s.spend(4); err != nil {
				return 0, 0, err
			}
			toB := route&128 == 0
			if err := s.branch(toB, true); err != nil {
				return 0, 0, err
			}
			accepted := priority >= a
			if toB {
				accepted = priority >= b
			}
			if err := s.ordinaryCompare(accepted); err != nil {
				return 0, 0, err
			}
			if !accepted {
				return engine.PCMReject, 0, nil
			}
			if toB {
				return engine.PCMSlotB, 0, nil
			}
			return engine.PCMSlotA, 0, nil
		}
	}
	if err := s.spend(4); err != nil {
		return 0, 0, err
	}
	if err := s.branch(v.SlotBusy[0] == 0, true); err != nil {
		return 0, 0, err
	}
	if v.SlotBusy[0] == 0 {
		return engine.PCMSlotA, 0, nil
	}
	if err := s.spend(4); err != nil {
		return 0, 0, err
	}
	if err := s.branch(v.SlotBusy[1] == 0, true); err != nil {
		return 0, 0, err
	}
	if v.SlotBusy[1] == 0 {
		return engine.PCMSlotB, 0, nil
	}
	var bits uint8
	if err := s.cost(16, func() error {
		var err error
		bits, err = choice.ReadPCMChoice(s.clock.Cycle)
		if err != nil {
			return err
		}
		if bits > 1 {
			return ErrOrdinarySequence
		}
		return nil
	}); err != nil {
		return 0, 0, err
	}
	preferB := bits&1 != 0
	if err := s.branch(preferB, true); err != nil {
		return 0, 0, err
	}
	first, second := a, b
	firstDest, secondDest := engine.PCMSlotA, engine.PCMSlotB
	if preferB {
		first, second, firstDest, secondDest = b, a, engine.PCMSlotB, engine.PCMSlotA
	}
	if err := s.ordinaryCompare(priority >= first); err != nil {
		return 0, 0, err
	}
	if priority >= first {
		return firstDest, bits, nil
	}
	if err := s.ordinaryCompare(priority >= second); err != nil {
		return 0, 0, err
	}
	if priority >= second {
		return secondDest, bits, nil
	}
	return engine.PCMReject, bits, nil
}

func BeginOrdinaryPublication(clock Clock, code uint8, library *resourcebridge.Library, choice PCMChoiceField, bus OrdinaryBus, limits Limits) (*OrdinaryPublication, error) {
	if library == nil || choice == nil || bus == nil {
		return nil, ErrOrdinarySequence
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return nil, err
	}
	d, ok := library.OrdinaryDescriptor(code)
	actual, found := bus.OrdinaryDescriptor(d.ID)
	if !ok || !found || actual != d || d.Protocol != pcmcommands.OrdinaryProtocol || d.Flags&32 != 0 || d.Flags&1 == 0 && (d.NativeRate == 0 || d.NativeRate >= 128) {
		return nil, ErrOrdinarySequence
	}
	s.bus = bus
	t := &OrdinaryPublication{s: s, bus: bus, choice: choice}
	dual := d.Flags&1 != 0
	if err = s.spend(16); err != nil {
		return nil, err
	}
	if err = s.branch(dual, true); err != nil {
		return nil, err
	}
	if err = s.ordinaryCosts(14, 20); err != nil {
		return nil, err
	}
	if err = s.panAcquire(); err != nil {
		return nil, err
	}
	var view pcmcommands.Status
	fields := []pcmcommands.ArbitrationField{pcmcommands.ArbitrationMode, pcmcommands.ArbitrationAFlags, pcmcommands.ArbitrationBFlags}
	if dual {
		fields = append(fields, pcmcommands.ArbitrationABusy, pcmcommands.ArbitrationBBusy)
	}
	for _, field := range fields {
		if err = s.cost(16, func() error {
			if err := s.add(7); err != nil {
				return err
			}
			value, err := bus.ReadArbitration(s.clock.Cycle, field)
			if err != nil {
				return err
			}
			switch field {
			case pcmcommands.ArbitrationMode:
				view.ModeFlags = value
			case pcmcommands.ArbitrationAFlags:
				view.SlotFlags[0] = value
			case pcmcommands.ArbitrationBFlags:
				view.SlotFlags[1] = value
			case pcmcommands.ArbitrationABusy:
				view.SlotBusy[0] = value
			case pcmcommands.ArbitrationBBusy:
				view.SlotBusy[1] = value
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	if err = s.request(20, false); err != nil {
		return nil, err
	}
	if err = s.spend(16); err != nil {
		return nil, err
	}
	if view.ModeFlags&32 != 0 {
		return nil, ErrOrdinarySequence
	}
	destination, bits, err := s.ordinaryArbitration(d, view, choice)
	if err != nil {
		return nil, err
	}
	selection, err := library.PrepareOrdinary(code, view, bits)
	if err != nil {
		return nil, err
	}
	if selection.Accepted != (destination != engine.PCMReject) {
		return nil, ErrOrdinarySequence
	}
	if selection.Accepted {
		want := map[engine.PCMDestination]pcmcommands.Destination{engine.PCMSingle: pcmcommands.Single, engine.PCMSlotA: pcmcommands.SlotA, engine.PCMSlotB: pcmcommands.SlotB}[destination]
		if selection.Input.Destination != want {
			return nil, ErrOrdinarySequence
		}
		t.input = selection.Input
	}
	return t, nil
}

func (t *OrdinaryPublication) ExpectedInput() (pcmcommands.Input, bool) {
	return t.input, t.input.Kind == pcmcommands.Ordinary
}
func (t *OrdinaryPublication) Clock() Clock         { return t.s.clock }
func (t *OrdinaryPublication) fail(err error) error { t.failure = err; return err }

func (t *OrdinaryPublication) Apply(expected pcmcommands.Input) error {
	if t == nil {
		return ErrOrdinarySequence
	}
	if t.done || t.failure != nil || t.applied || t.input.Kind != pcmcommands.Ordinary || expected != t.input {
		return t.fail(ErrOrdinarySequence)
	}
	s := t.s
	dual := expected.Destination != pcmcommands.Single
	if dual {
		if err := s.cost(20, func() error {
			value := uint8(0)
			if expected.Destination == pcmcommands.SlotA {
				value = 1
			}
			return t.choice.WritePCMChoice(s.clock.Cycle, value)
		}); err != nil {
			return t.fail(err)
		}
		// Fixed immutable header resolution, not a source pointer/RAM replay.
		if err := s.ordinaryCosts(18, 4, 12, 24, 4, 12, 22, 12, 8, 14, 14, 14, 14, 16); err != nil {
			return t.fail(err)
		}
	}
	if err := s.ordinaryCosts(14, 20); err != nil {
		return t.fail(err)
	}
	if err := s.panAcquire(); err != nil {
		return t.fail(err)
	}
	input := expected
	began := false
	store := func(cpu uint64, last bool) error {
		return s.cost(cpu, func() error {
			if err := s.add(7); err != nil {
				return err
			}
			if !began {
				input.Begin, began = s.clock.Cycle, true
			}
			if last {
				input.End = s.clock.Cycle
				return t.bus.PublishAcceptedAt(s.clock.Cycle, input)
			}
			return nil
		})
	}
	if !dual {
		if err := store(20, false); err != nil {
			return t.fail(err)
		}
		for field := 0; field < 7; field++ {
			if err := store(24, field == 6); err != nil {
				return t.fail(err)
			}
		}
	} else {
		if err := s.spend(10); err != nil {
			return t.fail(err)
		}
		if err := s.branch(!input.StartDual, true); err != nil {
			return t.fail(err)
		}
		if input.StartDual {
			for field := 0; field < 2; field++ {
				if err := store(20, false); err != nil {
					return t.fail(err)
				}
			}
		}
		if err := store(20, false); err != nil {
			return t.fail(err)
		}
		for field := 0; field < 2; field++ {
			if err := store(24, false); err != nil {
				return t.fail(err)
			}
		}
		for field := 0; field < 4; field++ {
			if err := store(16, false); err != nil {
				return t.fail(err)
			}
		}
		for field := 0; field < 2; field++ {
			if err := store(24, false); err != nil {
				return t.fail(err)
			}
		}
		if err := store(20, true); err != nil {
			return t.fail(err)
		}
	}
	t.applied = true
	return nil
}

func (t *OrdinaryPublication) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrOrdinarySequence
	}
	if t.done || t.failure != nil || t.input.Kind == pcmcommands.Ordinary && !t.applied {
		return Clock{}, t.fail(ErrOrdinarySequence)
	}
	if t.applied {
		if err := t.s.request(20, false); err != nil {
			return Clock{}, t.fail(err)
		}
		if err := t.s.ordinaryCosts(16, 16); err != nil {
			return Clock{}, t.fail(err)
		}
		if err := t.s.branch(true, true); err != nil {
			return Clock{}, t.fail(err)
		} // normal descriptor has no duck flag
	}
	if err := t.s.spend(16); err != nil {
		return Clock{}, t.fail(err)
	}
	t.done = true
	return t.s.clock, nil
}

func ServeOrdinaryPublication(clock Clock, code uint8, library *resourcebridge.Library, choice PCMChoiceField, bus OrdinaryBus, limits Limits) (OrdinaryResult, error) {
	t, err := BeginOrdinaryPublication(clock, code, library, choice, bus, limits)
	if err != nil {
		return OrdinaryResult{}, err
	}
	input, accepted := t.ExpectedInput()
	if accepted {
		if err = t.Apply(input); err != nil {
			return OrdinaryResult{}, err
		}
	}
	end, err := t.Finish()
	if err != nil {
		return OrdinaryResult{}, err
	}
	return OrdinaryResult{Clock: end, Accepted: accepted, Input: input}, nil
}
