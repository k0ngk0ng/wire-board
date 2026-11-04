package engine

import "errors"

// FighterMovementSpec names only parameter offsets whose consumers have been
// recovered. The remaining source bytes are provenance, not guessed rules.
type FighterMovementSpec struct {
	WalkSpeed int32 `json:"walk_speed"`
	JumpParameters
	BurstSpeed  int32 `json:"burst_speed"`
	EnergyDrain int16 `json:"energy_drain"`
}

// The five source IDs preserve the identities used by immutable data catalogs.
// They are never executed as addresses or dereferenced by the independent game.
type FighterSpec struct {
	CharacterID    uint8               `json:"character_id"`
	GraphicsSet    uint32              `json:"graphics_set"`
	TilesBase      uint32              `json:"tiles_base"`
	AnimationSet   uint32              `json:"animation_set"`
	ShapeSet       uint32              `json:"shape_set"`
	MovementSource uint32              `json:"movement_source"`
	BaseStun       int16               `json:"base_stun"`
	AttackScale    uint16              `json:"attack_scale"`
	Movement       FighterMovementSpec `json:"movement"`
}

type FighterSpecLibrary map[uint8]FighterSpec

type FighterInitializationState struct {
	Body             FighterBodyState `json:"body"`
	CharacterID      uint8            `json:"character_id"`
	GraphicsSet      uint32           `json:"graphics_set"`
	TilesBase        uint32           `json:"tiles_base"`
	AnimationSet     uint32           `json:"animation_set"`
	ShapeSet         uint32           `json:"shape_set"`
	MovementSource   uint32           `json:"movement_source"`
	SkipGraphics     uint8            `json:"skip_graphics"`
	GraphicOffsetX   int16            `json:"graphic_offset_x"`
	GraphicOffsetY   int16            `json:"graphic_offset_y"`
	GraphicCache     [4]uint32        `json:"graphic_cache"`
	GraphicParameter uint16           `json:"graphic_parameter"`
	DisplayedHealth  int16            `json:"displayed_health"`
	Energy           int32            `json:"energy"`
	DisplayedEnergy  int16            `json:"displayed_energy"`
	AttackScale      uint16           `json:"attack_scale"`
}

// InitializeFighter recovers the complete 0x126c8 initializer. The caller owns
// spawning, seats, position and input state. In particular this is not a common
// action reset: completion, action flags, retained input and motion survive.
func InitializeFighter(state FighterInitializationState, attributeMask uint16, library FighterSpecLibrary) (FighterInitializationState, error) {
	spec, ok := library[state.CharacterID]
	if state.CharacterID >= 11 || !ok || spec.CharacterID != state.CharacterID {
		return state, errors.New("fighter initialization source is unavailable")
	}
	state.Body.Attributes |= attributeMask
	state.GraphicOffsetX, state.GraphicOffsetY, state.GraphicParameter = 0, 0, 0
	state.GraphicCache = [4]uint32{}
	state.SkipGraphics = 0
	state.Body.Clock.FrameIndex, state.Body.Clock.Duration, state.Body.Clock.Ticks, state.Body.Clock.Flags = 0, 8, 255, 0
	state.Body.Health, state.DisplayedHealth = 223, 0
	// MOVE.W to +b8 replaces only the high half of the energy long.
	state.Energy = int32(uint32(state.Energy)&0xffff | uint32(223)<<16)
	state.DisplayedEnergy = 0
	state.Body.Team = state.Body.BaseTeam
	state.GraphicsSet, state.TilesBase, state.AnimationSet, state.ShapeSet, state.MovementSource = spec.GraphicsSet, spec.TilesBase, spec.AnimationSet, spec.ShapeSet, spec.MovementSource
	state.Body.EnabledStages, state.Body.ActionKind, state.Body.Pose = 3, 0, 0
	state.Body.Auxiliary7E, state.Body.Auxiliary80 = spec.BaseStun, spec.BaseStun
	state.AttackScale = spec.AttackScale
	return state, nil
}
