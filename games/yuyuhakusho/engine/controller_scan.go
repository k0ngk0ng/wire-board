package engine

import "errors"

// ControllerScanInput is a transient virtual hardware sample. Count is one
// MD6 pad or a four-MD6 TeamPlayer. ColdIO selects the distinct first scan after
// IO reset; ordinary match scans start with TH high after the scanline gap.
type ControllerScanPort struct {
	Count uint8
	Held  [4]Buttons
}
type ControllerScanInput struct {
	ColdIO bool
	Ports  [2]ControllerScanPort
}
type ControllerSample struct {
	Device ControllerDevice
	Held   Buttons
}
type ControllerScanResult struct {
	Count   uint8
	Samples [8]ControllerSample
}
type ControllerScanIO struct {
	Port           uint8
	Control, Write bool
	Value          uint8
}
type ControllerScanOperation struct {
	Work MainWorkOperation
	IO   *ControllerScanIO
}
type ControllerScanHook func(ControllerScanOperation) error

var ErrControllerScanProfile = errors.New("unsupported physical controller scan input")

func ValidateControllerScanInput(input ControllerScanInput) error {
	for _, port := range input.Ports {
		if port.Count != 1 && port.Count != 4 {
			return ErrControllerScanProfile
		}
		for i, held := range port.Held {
			if ValidateControllerButtons(held) != nil || held&(ButtonUp|ButtonDown) == ButtonUp|ButtonDown || held&(ButtonLeft|ButtonRight) == ButtonLeft|ButtonRight || i >= int(port.Count) && held != 0 {
				return ErrControllerScanProfile
			}
		}
	}
	return nil
}

// ScanBattleControllers recovers the full two-port sample rule. The virtual
// device signal sequence is derived from held controls and declared topology;
// no raw controller packet, CPU, ROM or port history is persisted. A work error
// suppresses all following hooks and discards the complete returned sample.
func ScanBattleControllers(input ControllerScanInput, hook ControllerScanHook) (ControllerScanResult, error) {
	if err := ValidateControllerScanInput(input); err != nil {
		return ControllerScanResult{}, err
	}
	work := controllerScanWork{hook: hook}
	var result ControllerScanResult
	work.spend("scan-clear-setup", 8, 8, 4)
	for i := 0; i < 5; i++ {
		work.spend("scan-clear", 12, 12, 12, 12)
		work.loop("scan-clear-loop", i < 4)
	}
	work.spend("scan-first-port", 8, 12, 18)
	for index, port := range input.Ports {
		if index != 0 {
			work.spend("scan-second-port", 8, 12)
		}
		bus := controllerSignal{port: uint8(index), count: port.Count, held: port.Held, th: 0x40, state: 0x60}
		if !input.ColdIO {
			bus.control, bus.data = 0x40, 0x40
			if port.Count == 4 {
				bus.control, bus.data = 0x60, 0x60
			}
		}
		work.control(&bus, 16, 0x60)
		work.write(&bus, 16, 0x60)
		work.nops()
		high := work.read(&bus, 12)
		work.write(&bus, 16, 0x20)
		work.nops()
		low := work.read(&bus, 12)
		work.spend("scan-identification", 8, 8, 18, 4)
		selector := uint8(0)
		for pair, signal := range []uint8{high & 15, high & 15, low & 15, low & 15} {
			first, second := uint8(3), uint8(2)
			if pair&1 != 0 {
				first, second = 1, 0
			}
			work.spend("scan-identification-bit", 10)
			set := signal&(1<<first) != 0
			work.branch("scan-identification-shortcut", set, false)
			if !set {
				work.spend("scan-identification-bit", 10)
				set = signal&(1<<second) != 0
				work.branch("scan-identification-zero", !set, false)
			}
			if set {
				work.spend("scan-identification-result", 10)
				selector |= 1 << uint(3-pair)
			}
		}
		work.spend("scan-identification-return", 16, 10, 18, 8)
		if port.Count == 1 {
			if selector != 13 {
				return ControllerScanResult{}, ErrControllerScanProfile
			}
			work.direct(&bus, &result)
		} else {
			if selector != 7 {
				return ControllerScanResult{}, ErrControllerScanProfile
			}
			work.team(&bus, &result)
		}
	}
	if work.err != nil {
		return ControllerScanResult{}, work.err
	}
	return result, nil
}

type controllerScanWork struct {
	hook ControllerScanHook
	err  error
}

func (w *controllerScanWork) emit(phase string, cost uint16, io *ControllerScanIO) {
	if w.hook != nil && w.err == nil {
		w.err = w.hook(ControllerScanOperation{Work: MainWorkOperation{Phase: phase, CPUCycles: cost}, IO: io})
	}
}
func (w *controllerScanWork) spend(phase string, costs ...uint16) {
	for _, cost := range costs {
		w.emit(phase, cost, nil)
	}
}
func (w *controllerScanWork) nops() { w.spend("scan-settle", 4, 4, 4, 4) }
func (w *controllerScanWork) branch(phase string, taken, word bool) {
	cost := uint16(10)
	if !taken {
		cost = 8
		if word {
			cost = 12
		}
	}
	w.emit(phase, cost, nil)
}
func (w *controllerScanWork) loop(phase string, more bool) {
	cost := uint16(14)
	if more {
		cost = 10
	}
	w.emit(phase, cost, nil)
}
func (w *controllerScanWork) control(bus *controllerSignal, cost uint16, value uint8) {
	if bus.control != value {
		bus.control = value
		bus.update()
	}
	w.emit("scan-control", cost, &ControllerScanIO{Port: bus.port, Control: true, Write: true, Value: value})
}
func (w *controllerScanWork) write(bus *controllerSignal, cost uint16, value uint8) {
	bus.data = value
	bus.update()
	w.emit("scan-write", cost, &ControllerScanIO{Port: bus.port, Write: true, Value: value})
}
func (w *controllerScanWork) read(bus *controllerSignal, cost uint16) uint8 {
	value := bus.read()
	w.emit("scan-read", cost, &ControllerScanIO{Port: bus.port, Value: value})
	return value
}
func (w *controllerScanWork) clearRemaining() { w.spend("scan-clear-unused", 16, 16, 16, 16, 16) }
func (w *controllerScanWork) direct(bus *controllerSignal, result *ControllerScanResult) {
	w.control(bus, 16, 0x40)
	w.write(bus, 16, 0x60)
	w.nops()
	w.read(bus, 12)
	w.write(bus, 16, 0x20)
	w.nops()
	w.read(bus, 12)
	w.spend("scan-base-buttons", 4, 8, 8, 14, 8, 10, 4, 12, 12)
	w.write(bus, 16, 0x60)
	w.nops()
	w.read(bus, 12)
	w.write(bus, 16, 0x20)
	w.nops()
	low := w.read(bus, 12)
	w.spend("scan-extension-test", 8)
	six := low&15 == 0
	w.branch("scan-extension-branch", six, false)
	w.spend("scan-device-kind", 16, 18)
	w.clearRemaining()
	device, held := MD3, bus.held[0]&0xff
	if six {
		device, held = MD6, bus.held[0]
		w.write(bus, 16, 0x60)
		w.nops()
		w.read(bus, 12)
		w.write(bus, 16, 0x20)
		w.nops()
		w.read(bus, 12)
		w.spend("scan-extension-store", 8, 12)
	}
	w.write(bus, 16, 0x40)
	w.spend("scan-return", 16)
	result.Samples[result.Count] = ControllerSample{Device: device, Held: held}
	result.Count++
}
func (w *controllerScanWork) token(bus *controllerSignal, toggle *uint8) uint8 {
	value := *toggle
	w.spend("scan-team-timeout", 8)
	w.write(bus, 12, value)
	w.branch("scan-team-polarity", value != 0, false)
	read := w.read(bus, 12)
	w.spend("scan-team-ack", 10, 12)
	if value == 0 {
		w.spend("scan-team-low-path", 10)
	}
	*toggle ^= 0x20
	w.spend("scan-team-nibble", 8, 8)
	return read & 15
}
func (w *controllerScanWork) team(bus *controllerSignal, result *ControllerScanResult) {
	toggle := uint8(0)
	w.spend("scan-team-begin", 4)
	for i := 0; i < 2; i++ {
		value := w.token(bus, &toggle)
		w.branch("scan-team-header", value != 0, true)
		if value != 0 {
			w.err = ErrControllerScanProfile
			return
		}
	}
	w.spend("scan-team-kind", 16, 4)
	for i := 0; i < 4; i++ {
		value := w.token(bus, &toggle)
		if value != 1 {
			w.err = ErrControllerScanProfile
			return
		}
		w.spend("scan-team-pad-kind", 18, 8, 4, 18, 16)
	}
	w.spend("scan-team-pads", 8)
	for i := 0; i < 4; i++ {
		w.spend("scan-team-pad-dispatch", 8, 18, 12, 18, 8)
		for group := 0; group < 3; group++ {
			w.token(bus, &toggle)
			w.spend("scan-team-pad-store", 12)
		}
		w.spend("scan-team-pad-return", 16)
		w.loop("scan-team-pad-loop", i < 3)
		result.Samples[result.Count] = ControllerSample{Device: MD6, Held: bus.held[i]}
		result.Count++
	}
	w.spend("scan-team-reset-timeout", 8)
	w.write(bus, 16, 0x60)
	w.nops()
	w.read(bus, 12)
	w.spend("scan-team-reset-ack", 10, 12, 16)
}

// This temporary signal state exists only for one fixed scan transaction. It
// models device wire responses, not a processor or a serialized IO register bank.
type controllerSignal struct {
	port, count                       uint8
	held                              [4]Buttons
	control, data, th, state, counter uint8
}

func (b *controllerSignal) update() {
	if b.count == 1 {
		th := uint8(0x40)
		if b.control&0x40 != 0 {
			th = b.data & 0x40
		}
		if th != 0 && b.th == 0 && b.counter < 8 {
			b.counter += 2
		}
		b.th = th
		return
	}
	state := (b.state &^ b.control) | (b.data & b.control)
	if state&0x40 != 0 {
		b.counter = 0
	} else if (state^b.state)&0x60 != 0 {
		b.counter++
	}
	b.state = state
}
func (b *controllerSignal) read() uint8 {
	var signal uint8
	if b.count == 1 {
		held := uint16(b.held[0])
		signal = b.th | 0x3f
		step := b.counter | (b.th >> 6)
		var pressed uint8
		switch step {
		case 4:
			pressed = uint8((held>>2)&0x30) | 15
		case 6:
			pressed = uint8((held >> 2) & 0x30)
		case 7:
			pressed = uint8(held&0x30) | uint8((held>>8)&15)
		default:
			if step&1 != 0 {
				pressed = uint8(held & 0x3f)
			} else {
				pressed = uint8(held&3) | uint8((held>>2)&0x30) | 12
			}
		}
		signal &^= pressed
	} else {
		signal = (b.state & 0x20) >> 1
		switch {
		case b.counter == 0:
			signal |= 3
		case b.counter == 1:
			signal |= 15
		case b.counter < 4:
		case b.counter < 8:
			signal |= 1
		default:
			index := int(b.counter) - 8
			pad, group := index/3, index%3
			if pad < 4 {
				signal |= uint8(^(b.held[pad] >> uint(group*4))) & 15
			} else {
				signal |= 15
			}
		}
	}
	mask := b.control | 0x80
	return b.data&mask | signal&^mask
}
