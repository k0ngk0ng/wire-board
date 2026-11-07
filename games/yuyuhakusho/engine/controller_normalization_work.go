package engine

import "errors"

// ControllerPort describes an already latched physical scan. A direct port
// uses Pads[0]; a TeamPlayer has four MD6 pads. This is transient input, not a
// saved raw packet or a second owner of the scanner's hardware state.
type ControllerPort struct {
	TeamPlayer bool                `json:"team_player"`
	Pads       [4]ControllerSample `json:"pads"`
}
type ControllerPorts [2]ControllerPort

// Devices preserves the last normalized device independently of held buttons:
// zero is inactive; MD3 and MD6 are active. Controllers retains the original
// trusted sixteen-bit storage semantics, including its four non-button bits.
type ControllerNormalizationState struct {
	Controllers [8]ControllerFrame  `json:"controllers"`
	Devices     [8]ControllerDevice `json:"devices"`
}

type ControllerNormalizationInput struct {
	Ports         ControllerPorts `json:"ports"`
	ChannelCount  uint8           `json:"channel_count"`
	SwapACMask    uint8           `json:"swap_ac_mask"`
	ReservedCarry uint8           `json:"reserved_carry"`
	// LivePorts is queried at each actual device/nibble read, before its work
	// hook. An intervening sampling IRQ can replace the latched ports. The
	// previously read device type and each completed nibble remain latched.
	LivePorts func() (ControllerPorts, error) `json:"-"`
}

type ControllerNormalizationHook func(MainWorkOperation) error

var ErrControllerNormalizationProfile = errors.New("unsupported controller normalization input")

func validateNormalizationPorts(ports ControllerPorts) error {
	for _, port := range ports {
		for index, pad := range port.Pads {
			if !port.TeamPlayer && index > 0 {
				if pad != (ControllerSample{}) {
					return ErrControllerNormalizationProfile
				}
				continue
			}
			if pad.Device != MD3 && pad.Device != MD6 || port.TeamPlayer && pad.Device != MD6 ||
				ValidateControllerButtons(pad.Held) != nil || pad.Device == MD3 && pad.Held&^0xff != 0 ||
				pad.Held&(ButtonUp|ButtonDown) == ButtonUp|ButtonDown ||
				pad.Held&(ButtonLeft|ButtonRight) == ButtonLeft|ButtonRight {
				return ErrControllerNormalizationProfile
			}
		}
	}
	return nil
}

// NormalizeControllersWork recovers the complete two-port normalization rule.
// Work is emitted synchronously at native batch boundaries; it is never stored
// or replayed. A failed hook/read discards all local normalized changes and
// suppresses subsequent hooks and reads.
//
// ReservedCarry is the trusted caller's four unused storage bits, expressed in
// their semantic nibble domain. The first processed MD6 pad consumes it into
// current bits12..15; MD3 and adapter/empty records do not consume it. Physical
// controls remain strictly twelve-bit. A caller must derive the nibble from its
// proved entry path, rather than inventing a default or persisting CPU registers.
func NormalizeControllersWork(state ControllerNormalizationState, input ControllerNormalizationInput, hook ControllerNormalizationHook) (ControllerNormalizationState, error) {
	original := state
	if input.ChannelCount > 8 || input.ReservedCarry > 15 || validateNormalizationPorts(input.Ports) != nil {
		return original, ErrControllerNormalizationProfile
	}
	for _, device := range state.Devices {
		if device != 0 && device != MD3 && device != MD6 {
			return original, ErrControllerNormalizationProfile
		}
	}
	w := normalizationWork{input: input, hook: hook}
	w.spend("normalize-setup", 16, 8, 8)
	channel, carry := 0, input.ReservedCarry
	for physical := 0; physical < 10; physical++ {
		w.spend("normalize-channel-limit", 12, 12)
		finished := channel >= int(input.ChannelCount)
		w.branch("normalize-channel-limit-branch", finished)
		if finished {
			return w.finish(original, state)
		}
		device := w.device(physical)
		w.spend("normalize-device-dispatch", 18, 16)
		switch device {
		case MD3:
			wasSix := state.Devices[channel] == MD6
			state.Devices[channel] = MD3
			w.spend("normalize-three-device", 20, 20)
			w.branch("normalize-three-history-branch", !wasSix)
			if wasSix {
				f := &state.Controllers[channel]
				f.Current &= 0xff
				f.Previous &= 0xff
				f.Changed &= 0xff
				f.Pressed &= 0xff
				f.Released &= 0xff
				w.spend("normalize-three-clear-extension", 16, 16, 16, 16, 16)
			}
			w.spend("normalize-three-buttons-call", 18)
			held := Buttons(w.nibble(physical, 0))
			held |= Buttons(w.nibble(physical, 1)) << 4
			w.spend("normalize-three-buttons", 8, 8, 14, 4, 18)
			held = w.swap(held, input.SwapACMask&(1<<uint(channel)) != 0)
			state.Controllers[channel] = w.baseEdges(state.Controllers[channel], held)
			w.spend("normalize-buttons-return", 16)
			channel++
			w.spend("normalize-next-channel", 8, 16, 16)
		case MD6:
			state.Devices[channel] = MD6
			w.spend("normalize-six-device", 20, 18)
			held := Buttons(w.nibble(physical, 0))
			held |= Buttons(w.nibble(physical, 1)) << 4
			held |= Buttons(w.nibble(physical, 2)) << 8
			held |= Buttons(carry) << 12
			carry = 0
			w.spend("normalize-six-buttons", 8, 8, 8, 14, 22, 4, 4, 18)
			held = w.swap(held, input.SwapACMask&(1<<uint(channel)) != 0)
			previous := state.Controllers[channel].Current
			state.Controllers[channel] = w.baseEdges(state.Controllers[channel], held)
			w.spend("normalize-extension-select", 22)
			changed := previous ^ held
			state.Controllers[channel] = ControllerFrame{Current: held, Previous: previous,
				Changed: changed, Pressed: held & changed, Released: previous & changed}
			w.edges("normalize-extension-edges")
			w.spend("normalize-buttons-return", 16)
			channel++
			w.spend("normalize-next-channel", 8, 16, 16)
		default:
			w.spend("normalize-empty-device-return", 16)
		}
		w.spend("normalize-next-record", 8, 10)
		w.branch("normalize-next-record-branch", physical < 9)
	}
	w.spend("normalize-scan-complete", 10)
	for {
		w.spend("normalize-unused-limit", 12, 12)
		more := channel < int(input.ChannelCount)
		w.branch("normalize-unused-limit-branch", more)
		if !more {
			break
		}
		active := state.Devices[channel] != 0
		w.spend("normalize-unused-device", 12)
		w.branch("normalize-unused-device-branch", !active)
		if active {
			state.Devices[channel] = 0
			state.Controllers[channel] = ControllerFrame{}
			w.spend("normalize-unused-clear-device", 20)
			w.spend("normalize-unused-clear-buttons", 16, 16, 16, 16, 16, 16, 16, 16, 16, 16)
		}
		channel++
		w.spend("normalize-unused-next-channel", 8, 16)
	}
	return w.finish(original, state)
}

type normalizationWork struct {
	input ControllerNormalizationInput
	hook  ControllerNormalizationHook
	err   error
}

func (w *normalizationWork) spend(phase string, cycles ...uint16) {
	if w.hook == nil || w.err != nil {
		return
	}
	for _, cost := range cycles {
		if w.err = w.hook(MainWorkOperation{Phase: phase, CPUCycles: cost}); w.err != nil {
			return
		}
	}
}

func (w *normalizationWork) branch(phase string, taken bool) {
	cost := uint16(8)
	if taken {
		cost = 10
	}
	w.spend(phase, cost)
}

func (w *normalizationWork) finish(original, state ControllerNormalizationState) (ControllerNormalizationState, error) {
	w.spend("normalize-return", 16)
	if w.err != nil {
		return original, w.err
	}
	return state, nil
}

func (w *normalizationWork) record(physical int) (ControllerSample, bool) {
	if w.err != nil {
		return ControllerSample{}, false
	}
	ports := w.input.Ports
	if w.input.LivePorts != nil {
		ports, w.err = w.input.LivePorts()
		if w.err != nil {
			return ControllerSample{}, false
		}
		if w.err = validateNormalizationPorts(ports); w.err != nil {
			return ControllerSample{}, false
		}
	}
	port, record := ports[physical/5], physical%5
	if port.TeamPlayer {
		if record == 0 {
			return ControllerSample{}, false
		}
		return port.Pads[record-1], true
	}
	if record != 0 {
		return ControllerSample{}, false
	}
	return port.Pads[0], true
}

func (w *normalizationWork) device(physical int) ControllerDevice {
	pad, _ := w.record(physical)
	w.spend("normalize-device-read", 8)
	return pad.Device
}

func (w *normalizationWork) nibble(physical, group int) uint8 {
	pad, exists := w.record(physical)
	// Cleared empty/header records and a three-button extension byte contain
	// zero in the original scan, which the normalizer inverts to fifteen.
	value := uint8(15)
	if exists && (group < 2 || pad.Device == MD6) {
		value = uint8(pad.Held>>uint(group*4)) & 15
	}
	phases := [3]string{"normalize-direction-read", "normalize-base-read", "normalize-extension-read"}
	w.spend(phases[group], 12)
	return value
}

func (w *normalizationWork) swap(held Buttons, enabled bool) Buttons {
	w.spend("normalize-swap-option", 12)
	w.branch("normalize-swap-option-branch", !enabled)
	if enabled {
		a, c := held&ButtonA != 0, held&ButtonC != 0
		held &^= ButtonA | ButtonC
		w.spend("normalize-swap-prepare", 4, 8, 10)
		w.branch("normalize-swap-a-branch", !a)
		if a {
			held |= ButtonC
			w.spend("normalize-swap-a", 10)
		}
		w.spend("normalize-swap-c-test", 10)
		w.branch("normalize-swap-c-branch", !c)
		if c {
			held |= ButtonA
			w.spend("normalize-swap-c", 10)
		}
	}
	w.spend("normalize-swap-return", 16)
	return held
}

func (w *normalizationWork) edges(phase string) {
	w.spend(phase, 12, 12, 12, 4, 4, 12, 4, 12, 4, 12)
}

func (w *normalizationWork) baseEdges(before ControllerFrame, held Buttons) ControllerFrame {
	previous, current := before.Current&0xff, held&0xff
	changed := previous ^ current
	before.Current = before.Current&^0xff | current
	before.Previous = before.Previous&^0xff | previous
	before.Changed = before.Changed&^0xff | changed
	before.Pressed = before.Pressed&^0xff | current&changed
	before.Released = before.Released&^0xff | previous&changed
	w.edges("normalize-base-edges")
	return before
}
