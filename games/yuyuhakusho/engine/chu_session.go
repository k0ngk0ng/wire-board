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
	state := ChuWorldState{Objects: WorldObjects{
		0x8000: {ObjectID: 0x8000, Header: MatchObjectHeader{Behavior: 0x9ad0, Size: 64, Stage: 20}},
		0x8040: {ObjectID: 0x8040, Header: MatchObjectHeader{Behavior: 0xf2a4, Size: 64}},
	}, RandomSeed: options.Seed, Difficulty: options.Difficulty, ResetEnabled: 1, SceneMode: 52,
		Clock: WorldClockState{ControllerCount: 4}, Throws: ThrowContactBuffer{Records: make([]ThrowContactRecord, 16)},
		Registration: FighterRegistrationState{Owners: make([]uint16, 64)}, Stream: make([]byte, 1024)}
	state.Match.Rule, state.Match.Layout, state.Match.Grouping = 2, uint16(len(options.Players)), 2
	if options.TeamBattle {
		state.Match.Grouping = 4
	}
	var slots, humanChannels uint8
	for _, player := range options.Players {
		if player.Slot >= 4 || (player.CharacterID != 9 && player.CharacterID != 10) || player.Color >= 12 || slots&(1<<player.Slot) != 0 || !player.Computer && (player.HumanChannel >= 4 || humanChannels&(1<<player.HumanChannel) != 0) {
			return ChuWorldState{}, nil, errors.New("unsupported or repeated Chu player slot, character, palette or input channel")
		}
		if spec, ok := library.Fighters[player.CharacterID]; !ok || spec.CharacterID != player.CharacterID {
			return ChuWorldState{}, nil, errors.New("selected fighter initialization rules are unavailable")
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

// CompleteChuSessionResource acknowledges exactly the pending default resource.
// Code/assets/canvas readiness remains the host's real loading gate. Duplicate
// or mismatched completions cannot reset a running match or its colors.
func CompleteChuSessionResource(state ChuWorldState, resource ChuSessionResource) (ChuWorldState, error) {
	manager, err := LookupWorldObject(state.Objects, 0x8040)
	if err != nil || state.ResourceBusy == 0 || manager.Header.Behavior != 0xf2a4 || manager.Header.Stage != 2 || state.Match.Arena != 0 || state.ResourceList != resource.Source || resource != DefaultChuSessionResource() {
		return state, errors.New("resource completion does not match the waiting Chu world")
	}
	copy(state.Palette.Primary[:16], resource.Palette0[:])
	state.ResourceBusy = 0
	state.Palette.Flags |= 0x80 // The verified default scene's display-ready flag.
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
