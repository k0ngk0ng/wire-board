package engine

// FighterPreparationState reads the retained base held/pressed channels and
// the separate extension pressed byte. It follows character command matching;
// RawHeld and the XOR Changed channel are not inputs to this routine.
type FighterPreparationState struct {
	Energy          int32 `json:"energy"`
	Held            uint8 `json:"held"`
	Pressed         uint8 `json:"pressed"`
	ExtendedPressed uint8 `json:"extended_pressed"`
	Facing          uint8 `json:"facing"`
	ChargeTicks     uint8 `json:"charge_ticks"`
	Request         uint8 `json:"request"`
}

// PrepareFighterInput recovers the complete 0x15c36 routine. Only the signed
// high word of Energy affects charging. Old charge values above 64 survive a
// held charge; releasing it or reaching the energy threshold clears them.
func PrepareFighterInput(state FighterPreparationState) FighterPreparationState {
	if int16(state.Energy>>16) < 0xdf && state.Held&0x50 != 0 {
		if state.ChargeTicks <= 0x40 {
			state.ChargeTicks++
		}
	} else {
		state.ChargeTicks = 0
	}
	first, second := uint8(1), uint8(4)
	if state.Facing&1 != 0 {
		first, second = second, first
	}
	if state.ExtendedPressed&first != 0 {
		state.Request = 7
		return state
	}
	if state.ExtendedPressed&second != 0 || state.Held&0x20 != 0 && state.Pressed&0x10 != 0 {
		state.Request = 8
	}
	return state
}

type AttackValueState struct {
	Seed  uint32 `json:"seed"`
	Scale uint16 `json:"scale"`
	Value uint8  `json:"value"`
}

// RandomizeAttackValue recovers 0x15bfe after animation shape submission. It
// always consumes the shared RNG, even for zero Scale or Value. MULU uses the
// random result's low word; ADD.B retains only the resulting byte sum.
func RandomizeAttackValue(state AttackValueState) AttackValueState {
	next, random := NextRandom(state.Seed)
	product := uint32(uint16(random)) * uint32(state.Scale)
	state.Seed = next
	state.Value += uint8(product >> 16)
	return state
}
