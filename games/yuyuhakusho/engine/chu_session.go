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
	OneVsThree bool   `json:",omitempty"`
	Arena      uint16 `json:",omitempty"`
}

type ChuSessionResource struct {
	Source   uint32
	Palette0 [16]uint16
}

func (options ChuSessionOptions) Grouping() uint16 {
	if options.OneVsThree {
		return 6
	}
	if options.TeamBattle {
		return 4
	}
	return 2
}

func (options ChuSessionOptions) Team(slot uint8) uint8 {
	if !options.TeamBattle {
		return slot + 1
	}
	if slot >= 2 || options.OneVsThree && slot >= 1 {
		return 4
	}
	return 1
}

func DefaultChuSessionResource() ChuSessionResource {
	return ChuSessionResource{Source: 0x21720, Palette0: [16]uint16{2662, 512, 3822, 3786, 2150, 1126, 2728, 3240, 3240, 2184, 1092, 2694, 1058, 1604, 3786, 1090}}
}

var chuArenaBehaviors = [8]uint32{0x20918, 0x20ae0, 0x20cb2, 0x20e28, 0x20fb4, 0x21126, 0x212f8, 0x21412}

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

func NewChuSession(options ChuSessionOptions, library ChuWorldLibrary) (ChuWorldState, []ChuWorldEvent, error) {
	if len(options.Players) < 2 || len(options.Players) > 4 || options.Difficulty > 6 || options.Difficulty&1 != 0 || options.TeamBattle && len(options.Players) != 4 || options.OneVsThree && !options.TeamBattle {
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
	state.Match.Rule, state.Match.Layout, state.Match.Grouping = 2, uint16(len(options.Players)), options.Grouping()
	state.Match.Arena = options.Arena
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
		state.Match.BaseTeams[player.Slot] = options.Team(player.Slot)
		if !player.Computer {
			humanChannels |= 1 << player.HumanChannel
			state.Match.Controllers[player.Slot] = player.HumanChannel + 1
		}
	}
	return StepChuWorldObjects(state, ChuWorldView{}, library)
}

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
	state.Palette.Flags |= 0x80
	state.InterruptMode = 4
	return state, nil
}

func StepChuSessionFrame(state ChuWorldState, held [8]Buttons, library ChuWorldLibrary) (ChuWorldState, []ChuWorldEvent, WorldFrameOutcome, error) {
	return StepChuSessionFrameObserved(state, held, library, nil)
}

func StepChuSessionFrameObserved(state ChuWorldState, held [8]Buttons, library ChuWorldLibrary, hook WorldSoundHook) (ChuWorldState, []ChuWorldEvent, WorldFrameOutcome, error) {
	return StepChuSessionFrameWithHooks(state, held, library, ChuWorldHooks{Sound: hook})
}

func StepChuSessionFrameWithHooks(state ChuWorldState, held [8]Buttons, library ChuWorldLibrary, hooks ChuWorldHooks) (ChuWorldState, []ChuWorldEvent, WorldFrameOutcome, error) {
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
		if hooks.Frame == nil {
			samples[i], _ = SampleController(state.Controllers[i].Current, buttons, MD6, false)
		}
	}
	if hooks.Owners != nil {
		if err := hooks.Owners(&state); err != nil {
			return original, nil, WorldFrameOutcome{}, err
		}
	}
	var outcome WorldFrameOutcome
	var err error
	if hooks.Frame != nil {
		outcome, err = beginWorldFrameLive(WorldFrameOwners{Clock: &state.Clock, Controllers: &state.Controllers, RandomSeed: &state.RandomSeed, SlowGate: &state.SlowGate, ResetEnabled: &state.ResetEnabled}, *hooks.Frame)
	} else {
		var begin WorldFrameState
		begin, outcome, err = BeginWorldFrame(WorldFrameState{Clock: state.Clock, Controllers: state.Controllers, RandomSeed: state.RandomSeed, SlowGate: state.SlowGate, ResetEnabled: state.ResetEnabled}, samples)
		if err == nil {
			state.Clock, state.Controllers, state.RandomSeed = begin.Clock, begin.Controllers, begin.RandomSeed
		}
	}
	if err != nil {
		return original, nil, WorldFrameOutcome{}, err
	}
	if outcome.ResetRequested {
		return state, []ChuWorldEvent{{Kind: "reset-request"}}, outcome, nil
	}
	state, events, err := StepChuWorldObjectsWithHooks(state, ChuWorldView{}, library, hooks)
	if err != nil {
		return original, nil, WorldFrameOutcome{}, err
	}
	if hooks.Owners != nil {
		if err := hooks.Owners(&state); err != nil {
			return original, nil, WorldFrameOutcome{}, err
		}
	}
	state.Palette, err = FinishWorldPalette(state.Palette, library.Arena)
	if err != nil {
		return original, nil, WorldFrameOutcome{}, err
	}
	return state, events, outcome, nil
}
