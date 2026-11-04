package engine

import "errors"

type FighterInputEnvelopeRoute uint8

const (
	FighterInputComplete FighterInputEnvelopeRoute = iota
	FighterInputNeedsAI
	FighterInputNeedsAIThenRecord
)

// StreamBase/Position preserve stream identity and cursor provenance. They are
// not executable addresses; the independent caller supplies the bounded bytes.
type FighterInputEnvelopeState struct {
	Controls   FighterControls
	Mode       int8
	Channel    uint8
	Disabled   uint8
	Stopped    uint8
	Position   uint32
	StreamBase uint32
	Stream     []byte
	Input      ControllerFrame
	EndMarkers [4]uint8
}

// PrepareFighterInputEnvelope recovers 0x15540 up to its actual return or the
// explicit AI call boundary. A recording AI must run its real rule first, then
// resume RecordFighterInputEnvelope; no expected AI result enters this API.
func PrepareFighterInputEnvelope(state FighterInputEnvelopeState) (FighterInputEnvelopeState, FighterInputEnvelopeRoute, error) {
	before := state
	clear := func() { state.Controls.Held, state.Controls.Pressed, state.Controls.Changed = 0, 0, 0 }
	if state.Disabled&1 != 0 {
		clear()
		return state, FighterInputComplete, nil
	}
	if state.Mode > 0 {
		if state.Stopped != 0 || state.Position == 0 {
			clear()
			return state, FighterInputComplete, nil
		}
		index, err := fighterStreamIndex(state)
		if err != nil {
			return before, FighterInputComplete, err
		}
		data := state.Stream[index : index+4]
		state.Controls.Held, state.Controls.Pressed, state.Controls.Changed = Buttons(data[0]), Buttons(data[1]), Buttons(data[2])
		state.Controls.RawHeld = data[3]
		if data[0]&0x80 != 0 {
			state.Stopped = 1
		}
		state.Position += 4
		return state, FighterInputComplete, nil
	}
	if state.Channel == 255 {
		clear()
		route := FighterInputNeedsAI
		if state.Mode < 0 {
			route = FighterInputNeedsAIThenRecord
		}
		return state, route, nil
	}
	if state.Channel >= 8 {
		return before, FighterInputComplete, errors.New("fighter input channel is unavailable")
	}
	state.Controls = LatchFighterInput(state.Controls, state.Input)
	if state.Mode < 0 {
		next, err := RecordFighterInputEnvelope(state)
		if err != nil {
			return before, FighterInputComplete, err
		}
		state = next
	}
	return state, FighterInputComplete, nil
}

// RecordFighterInputEnvelope recovers the continuation at 0x15594. The original
// end marker uses the channel-based A2 alias, not the stream's last four bytes.
// EndMarkers retain the four writable tail-RAM bytes; channel>=4 writes would
// instead address read-only ROM and leave these markers unchanged.
func RecordFighterInputEnvelope(state FighterInputEnvelopeState) (FighterInputEnvelopeState, error) {
	before := state
	if state.Stopped != 0 {
		return state, nil
	}
	index, err := fighterStreamIndex(state)
	if err != nil {
		return before, err
	}
	if state.Channel != 255 && state.Channel >= 8 {
		return before, errors.New("fighter record channel is unavailable")
	}
	state.Stream = append([]byte(nil), state.Stream...)
	copy(state.Stream[index:index+4], []byte{uint8(state.Controls.Held), uint8(state.Controls.Pressed), uint8(state.Controls.Changed), state.Controls.RawHeld})
	state.Position += 4
	if state.Channel != 255 && state.Input.Current&ButtonStart != 0 {
		state.Stopped = 1
		for i := int(state.Channel); i < len(state.EndMarkers); i++ {
			state.EndMarkers[i] |= 0x80
		}
	}
	return state, nil
}

func fighterStreamIndex(state FighterInputEnvelopeState) (int, error) {
	if state.Position < state.StreamBase || uint64(state.Position-state.StreamBase)+4 > uint64(len(state.Stream)) {
		return 0, errors.New("fighter input stream storage is unavailable")
	}
	return int(state.Position - state.StreamBase), nil
}

type FighterRegistrationState struct {
	Used   uint16 // Byte offset, not owner count.
	Owners []uint16
}

// RegisterFighter recovers 0x14922. The original has no capacity clamp; the
// caller must supply the addressed slot. Old unused entries survive intact.
func RegisterFighter(state FighterRegistrationState, owner uint16) (FighterRegistrationState, error) {
	index := int(int16(state.Used)) / 2
	if state.Used&1 != 0 || index < 0 || index >= len(state.Owners) {
		return state, errors.New("fighter registration storage is unavailable")
	}
	state.Owners = append([]uint16(nil), state.Owners...)
	state.Owners[index] = owner
	state.Used += 2
	return state, nil
}

type FighterHUDState struct {
	Health, DisplayedHealth int16
	Energy, DisplayedEnergy int16
	Offset                  uint16
	TargetBit, Blink        uint8
	Cells                   []uint16
	TransferCursor          uint16
}

// FighterBarTransfer identifies one 32-byte bar pattern and its destination
// tile. Presentation code consumes this short emission list; game state does
// not need the original CPU's entire DMA command buffer or ROM image.
type FighterBarTransfer struct {
	Pattern    uint8
	TargetTile uint16
}

// StepFighterHUD recovers the full 0x15772 update, including both resource bars.
// Actual resources remain distinct from their one-unit-per-call display values.
func StepFighterHUD(state FighterHUDState) (FighterHUDState, []FighterBarTransfer, error) {
	before := state
	if state.Offset&1 != 0 {
		return before, nil, errors.New("unaligned fighter HUD offset")
	}
	state.Cells = append([]uint16(nil), state.Cells...)
	base := int(int16(state.Offset)) / 2
	write := func(index int, value uint16) bool {
		if index < 0 || index >= len(state.Cells) {
			return false
		}
		state.Cells[index] = value
		return true
	}
	transfers := make([]FighterBarTransfer, 0, 2)
	for resource := 0; resource < 2; resource++ {
		actual, shown := state.Health, state.DisplayedHealth
		row, attribute, glyph, tile := base, uint16(0xc002), uint16(0xa000), uint16(state.TargetBit>>2)+0x10
		if resource == 0 {
			if actual < 112 && state.Blink&1 != 0 {
				attribute = 0xc00e
			}
		} else {
			actual, shown = state.Energy, state.DisplayedEnergy
			row, attribute, glyph, tile = base+0x40, 0xc003, 0xc000, uint16(state.TargetBit>>2)+0x14
			if actual >= 200 && state.Blink&1 != 0 {
				attribute = 0xc00f
			}
		}
		if !write(row+7, attribute) {
			return before, nil, errors.New("fighter HUD attribute storage is unavailable")
		}
		if shown == actual {
			continue
		}
		if shown < actual {
			shown++
		} else {
			shown--
			if shown < 0 {
				shown++
			}
		}
		group := int(uint16(shown) >> 5)
		if group != 0 && !write(row+group-1, glyph|12) {
			return before, nil, errors.New("fighter HUD fill storage is unavailable")
		}
		if group != 6 && !write(row+group+1, glyph|11) {
			return before, nil, errors.New("fighter HUD edge storage is unavailable")
		}
		if !write(row+group, glyph|tile) || state.TransferCursor&1 != 0 {
			return before, nil, errors.New("fighter HUD tile or transfer storage is unavailable")
		}
		transfers = append(transfers, FighterBarTransfer{Pattern: uint8(uint16(shown) & 31), TargetTile: tile})
		state.TransferCursor += 16
		if resource == 0 {
			state.DisplayedHealth = shown
		} else {
			state.DisplayedEnergy = shown
		}
	}
	return state, transfers, nil
}
