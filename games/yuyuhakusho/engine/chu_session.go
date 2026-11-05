package engine

import "errors"

type ChuSessionPlayer struct {
	Slot         uint8
	CharacterID  uint8
	Color        uint8
	Computer     bool
	HumanChannel uint8
}

type ChuSessionOptions struct {
	Seed       uint32
	Difficulty uint16
	Players    []ChuSessionPlayer
	TeamBattle bool
	Arena      uint16 `json:",omitempty"`
}

type ChuSessionResource struct {
	Source   uint32
	Palette0 [16]uint16
}

// DefaultChuSessionResource is the32-byte palette part of the verified default
// arena resource. It is rule/fade setup data, not a texture or CPU image.
func DefaultChuSessionResource() ChuSessionResource {
	return ChuSessionResource{Source: 0x21720, Palette0: [16]uint16{2662, 512, 3822, 3786, 2150, 1126, 2728, 3240, 3240, 2184, 1092, 2694, 1058, 1604, 3786, 1090}}
}

var chuArenaBehaviors = [8]uint32{0x20918, 0x20ae0, 0x20cb2, 0x20e28, 0x20fb4, 0x21126, 0x212f8, 0x21412}

// ChuSessionResourceForArena returns immutable fade/setup metadata for the
// selected scene. Old rule bundles retain their verified default resource.
func ChuSessionResourceForArena(arena uint16, library ChuWorldLibrary) (ChuSessionResource, error) {
	if arena > 14 || arena&1 != 0 || library.MatchStart.ArenaBehaviors[arena/2] != chuArenaBehaviors[arena/2] {
		return ChuSessionResource{}, errors.New("selected arena rules are unavailable")
	}
	resource, found := library.ArenaResources[arena]
	if library.ArenaResources == nil && arena == 0 {
		resource, found = DefaultChuSessionResource(), true
	}
	if !found || resource.Source == 0 || resource.Source != library.MatchStart.ArenaResources[arena/2] {
		return ChuSessionResource{}, errors.New("selected arena resource metadata is unavailable")
	}
	for _, color := range resource.Palette0 {
		if color&^0x0eee != 0 {
			return ChuSessionResource{}, errors.New("selected arena resource palette is invalid")
		}
	}
	if arena == 0 {
		if resource != DefaultChuSessionResource() {
			return ChuSessionResource{}, errors.New("default arena resource metadata differs from its verified binding")
		}
		return resource, nil
	}
	scenes := library.ArenaScenes
	if arena == 2 || arena == 8 || arena == 12 {
		if len(scenes.WaveX) != 1024 {
			return ChuSessionResource{}, errors.New("selected arena waveform is unavailable")
		}
	}
	if arena == 2 || arena == 6 || arena == 8 || arena == 10 || arena == 14 {
		if len(scenes.LinearRows) != 241 {
			return ChuSessionResource{}, errors.New("selected arena linear rows are unavailable")
		}
		for x := int16(744); x <= 984; x++ {
			if len(scenes.LinearRows[x]) != 128 {
				return ChuSessionResource{}, errors.New("selected arena linear row is incomplete")
			}
		}
	}
	if arena == 4 {
		if len(scenes.Arena4Rows) != 64 {
			return ChuSessionResource{}, errors.New("selected arena scrolling rows are unavailable")
		}
		for x := uint8(0); x < 64; x++ {
			if len(scenes.Arena4Rows[x]) != 112 {
				return ChuSessionResource{}, errors.New("selected arena scrolling row is incomplete")
			}
		}
	}
	return resource, nil
}

// NewChuSession prepares an independent platform match from configuration.
// It performs manager setup before the gameplay clock starts and waits for an
// explicit one-time resource-ready delivery. It never loads a reference state.
func NewChuSession(options ChuSessionOptions, library ChuWorldLibrary) (ChuWorldState, []ChuWorldEvent, error) {
	if len(options.Players) < 2 || len(options.Players) > 4 || options.Difficulty > 6 || options.Difficulty&1 != 0 || options.TeamBattle && len(options.Players) != 4 {
		return ChuWorldState{}, nil, errors.New("unsupported Chu match size, difficulty or team configuration")
	}
	if len(library.Commands) == 0 || len(library.Animations) == 0 {
		return ChuWorldState{}, nil, errors.New("Chu match rules are unavailable")
	}
	if _, err := ChuSessionResourceForArena(options.Arena, library); err != nil {
		return ChuWorldState{}, nil, err
	}
	state := ChuWorldState{Objects: WorldObjects{
		0x8000: {ObjectID: 0x8000, Header: MatchObjectHeader{Behavior: 0x9ad0, Size: 64, Stage: 20}},
		0x8040: {ObjectID: 0x8040, Header: MatchObjectHeader{Behavior: 0xf2a4, Size: 64}},
	}, RandomSeed: options.Seed, Difficulty: options.Difficulty, ResetEnabled: 1, SceneMode: 52,
		Clock: WorldClockState{ControllerCount: 4}, Throws: ThrowContactBuffer{Records: make([]ThrowContactRecord, 16)},
		Registration: FighterRegistrationState{Owners: make([]uint16, 64)}, Stream: make([]byte, 1024)}
	state.Match.Rule, state.Match.Layout, state.Match.Grouping = 2, uint16(len(options.Players)), 2
	state.Match.Arena = options.Arena
	if options.TeamBattle {
		state.Match.Grouping = 4
	}
	var slots, humanChannels uint8
	for _, player := range options.Players {
		if player.Slot >= 4 || player.CharacterID > 10 || player.Color >= 12 || slots&(1<<player.Slot) != 0 || !player.Computer && (player.HumanChannel >= 4 || humanChannels&(1<<player.HumanChannel) != 0) {
			return ChuWorldState{}, nil, errors.New("unsupported or repeated Chu player slot, character, palette or input channel")
		}
		if spec, ok := library.Fighters[player.CharacterID]; !ok || spec.CharacterID != player.CharacterID {
			return ChuWorldState{}, nil, errors.New("selected fighter initialization rules are unavailable")
		}
		if (player.CharacterID == 0 || player.CharacterID == 3) && len(library.Fighter0Projectiles.Sequences) != 8 {
			return ChuWorldState{}, nil, errors.New("source0 projectile rules are unavailable")
		}
		if player.CharacterID == 4 {
			if _, found := library.MovementSources[0x15946]; !found || library.Fighter4Dragon.Curve.FirstIndex != -32 || len(library.Fighter4Dragon.Curve.Values) != 1344 {
				return ChuWorldState{}, nil, errors.New("source4 dragon and alternate movement rules are unavailable")
			}
		}
		slots |= 1 << player.Slot
		state.Match.Characters[player.Slot], state.Match.Colors[player.Slot] = player.CharacterID+1, player.Color
		state.Match.BaseTeams[player.Slot] = player.Slot + 1
		if options.TeamBattle {
			state.Match.BaseTeams[player.Slot] = 1
			if player.Slot >= 2 {
				state.Match.BaseTeams[player.Slot] = 4
			}
		}
		if !player.Computer {
			humanChannels |= 1 << player.HumanChannel
			state.Match.Controllers[player.Slot] = player.HumanChannel + 1
		}
	}
	return StepChuWorldObjects(state, ChuWorldView{}, library)
}

// CompleteChuSessionResource acknowledges exactly the pending selected resource.
// Code/assets/canvas readiness remains the host's real loading gate. Duplicate
// or mismatched completions cannot reset a running match or its colors.
// The optional library preserves the historical two-argument default API;
// additional arenas require the same immutable library used to construct them.
func CompleteChuSessionResource(state ChuWorldState, resource ChuSessionResource, libraries ...ChuWorldLibrary) (ChuWorldState, error) {
	expected := DefaultChuSessionResource()
	if len(libraries) > 1 || len(libraries) == 0 && state.Match.Arena != 0 {
		return state, errors.New("selected arena resource registry is unavailable")
	}
	if len(libraries) == 1 {
		var err error
		expected, err = ChuSessionResourceForArena(state.Match.Arena, libraries[0])
		if err != nil {
			return state, err
		}
	}
	manager, err := LookupWorldObject(state.Objects, 0x8040)
	if err != nil || state.ResourceBusy == 0 || manager.Header.Behavior != 0xf2a4 || manager.Header.Stage != 2 || state.ResourceList != resource.Source || resource != expected {
		return state, errors.New("resource completion does not match the waiting Chu world")
	}
	copy(state.Palette.Primary[:16], resource.Palette0[:])
	state.ResourceBusy = 0
	state.Palette.Flags |= 0x80 // The verified scene resource display-ready flag.
	state.InterruptMode = 4     // Retained context identity; no IRQ is executed.
	return state, nil
}

// StepChuSessionFrame runs one independent logical tick. Only physical logical
// button bits enter this boundary; both pressed/released edges are calculated
// locally. Internal world state is never replaced by a reference frame.
func StepChuSessionFrame(state ChuWorldState, held [8]Buttons, library ChuWorldLibrary) (ChuWorldState, []ChuWorldEvent, WorldFrameOutcome, error) {
	original := state
	if state.MatchComplete {
		return state, nil, WorldFrameOutcome{}, nil
	}
	if state.ResourceBusy != 0 {
		return original, nil, WorldFrameOutcome{}, errors.New("Chu match resources are not ready")
	}
	var samples [8]ControllerFrame
	for i, buttons := range held {
		if err := ValidateControllerButtons(buttons); err != nil {
			return original, nil, WorldFrameOutcome{}, err
		}
		samples[i], _ = SampleController(state.Controllers[i].Current, buttons, MD6, false)
	}
	begin, outcome, err := BeginWorldFrame(WorldFrameState{Clock: state.Clock, Controllers: state.Controllers, RandomSeed: state.RandomSeed,
		SlowGate: state.SlowGate, ResetEnabled: state.ResetEnabled}, samples)
	if err != nil {
		return original, nil, WorldFrameOutcome{}, err
	}
	state.Clock, state.Controllers, state.RandomSeed = begin.Clock, begin.Controllers, begin.RandomSeed
	if outcome.ResetRequested {
		return state, []ChuWorldEvent{{Kind: "reset-request"}}, outcome, nil
	}
	state, events, err := StepChuWorldObjects(state, ChuWorldView{}, library)
	if err != nil {
		return original, nil, WorldFrameOutcome{}, err
	}
	state.Palette, err = FinishWorldPalette(state.Palette, library.Arena)
	if err != nil {
		return original, nil, WorldFrameOutcome{}, err
	}
	return state, events, outcome, nil
}
