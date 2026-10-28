package engine

import "errors"

// EffectSlot contains the fields owned by effect allocation and movement-dust
// initialization. Behavior is a recovered dispatch identifier, never an address
// executed by this engine. Other fields remain unchanged when a slot is reused.
type EffectSlot struct {
	Behavior   uint32 `json:"behavior"`
	Size       uint16 `json:"size"`
	Stage      uint16 `json:"stage"`
	VelocityX  int32  `json:"velocity_x"`
	Attributes uint16 `json:"attributes"`
	Layer      uint8  `json:"layer"`
	Depth      uint16 `json:"depth"`
	X          int32  `json:"x"`
	Y          int32  `json:"y"`
	Type       uint8  `json:"type"`
}

type EffectPool struct {
	Cursor uint16         `json:"cursor"`
	Slots  [16]EffectSlot `json:"slots"`
}

type EffectSource struct {
	X          int32  `json:"x"`
	Y          int32  `json:"y"`
	Attributes uint16 `json:"attributes"`
	Layer      uint8  `json:"layer"`
	Depth      uint16 `json:"depth"`
}

const dustBehavior uint32 = 0x149ac

// AllocateEffect recovers 0x1fa96 / 0x1fab4. The current slot is deliberately
// not searched: the original advances first and checks at most fifteen slots.
// A failed search still advances the shared cursor. Only Size marks occupancy.
func AllocateEffect(pool EffectPool) (EffectPool, int, error) {
	if pool.Cursor&0x7f != 0 {
		return pool, -1, errors.New("effect cursor is not aligned to a slot")
	}
	for attempt := 0; attempt < 15; attempt++ {
		pool.Cursor = (pool.Cursor + 0x80) & 0x7ff
		index := int(pool.Cursor / 0x80)
		if pool.Slots[index].Size != 0 {
			continue
		}
		pool.Slots[index].Behavior = dustBehavior
		pool.Slots[index].Size = 0x80
		pool.Slots[index].Stage = 0
		return pool, index, nil
	}
	return pool, -1, nil
}

// SpawnMovementDust recovers the full paired initialization at 0x1fce8. Each
// side allocates independently, so one side can succeed after the other fails.
// It does not run the effects' subsequent animation or lifetime stage.
func SpawnMovementDust(pool EffectPool, source EffectSource) (EffectPool, [2]int, error) {
	selected := [2]int{-1, -1}
	for side := 0; side < 2; side++ {
		next, index, err := AllocateEffect(pool)
		if err != nil {
			return pool, selected, err
		}
		pool = next
		selected[side] = index
		if index < 0 {
			continue
		}
		slot := &pool.Slots[index]
		slot.Type = 0x10
		// A word add to the coordinate's high half leaves its fraction intact.
		slot.X = source.X + int32((side*8-4)<<16)
		slot.Y = source.Y
		slot.VelocityX = int32((side*2 - 1) << 16)
		// The original copies the source word's high byte, masks the word, then
		// ORs 0x20 into the high byte; the low byte is cleared by that mask.
		slot.Attributes = source.Attributes&0x9800 | 0x2000
		slot.Layer = source.Layer - 4
		slot.Depth = source.Depth
	}
	return pool, selected, nil
}
