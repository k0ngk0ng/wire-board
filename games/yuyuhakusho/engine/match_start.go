package engine

import "errors"

type MatchStartObject struct {
	Header    MatchObjectHeader
	Parameter uint16
}
type MatchStartFighter struct {
	Header                                                   MatchObjectHeader
	TargetBit                                                uint8
	Attributes                                               uint16
	X, Ground                                                int32
	Depth                                                    uint16
	Facing, Layer, CharacterID, Channel, BaseTeam, InputMode uint8
	StreamPosition                                           uint32
	HUDOffset                                                uint16
}
type MatchStartState struct {
	Manager                                                        MatchObjectHeader
	Objects                                                        [10]MatchStartObject //8100/8140/8180/81c0/8200/8240/8280/9c00/9c80/9d00.
	Fighters                                                       [4]MatchStartFighter
	PauseFlags, ModeFlag, ResourceBusy                             uint8
	ResourceList                                                   uint32
	VideoMode, Arena, Rule, Layout, Round, RoundPair               uint16
	Characters, Controllers, Colors, BaseTeams                     [4]uint8
	Options                                                        [3]uint8 //FF14/FF15/FF16.
	OptionA097                                                     uint8
	Disabled, SlowGate, SlowFlag                                   uint8
	SlowTicks                                                      uint16
	PendingSound                                                   uint8
	SoundQueue                                                     [4]uint8
	OpeningTicks, ResultTicks, WinnerCode, AttributeMask, Selected uint16
	Camera                                                         [4]int16 //currentX/currentY/targetX/targetY.
	Bounds                                                         [2]int16
	PrimaryColors                                                  [64]uint16
	Feedback                                                       [7]uint16
	PlaneBases                                                     [5]uint16
	PlaneSize                                                      [2]uint16
	MaxSprites                                                     uint16
	ScrollSources                                                  [2]uint32
	VideoBytes                                                     [5]uint8  //F6df/e7/e9/f3/f5.
	ResetWords                                                     [4]uint16 //E800/EA00/E500/E528.
}
type MatchStartLibrary struct {
	InitialColors    [64]uint16        `json:"initial_colors"`
	ArenaResources   [8]uint32         `json:"arena_resources"`
	ArenaBehaviors   [8]uint32         `json:"arena_behaviors"`
	Palettes         [11][4][10]uint16 `json:"palettes"`
	SharedPalettes   [8][10]uint16     `json:"shared_palettes"`
	HUDOffsets       [4][4]uint16      `json:"hud_offsets"`
	RoundOpponents   [28]int16         `json:"round_opponents"`
	Names            [11][3]uint16     `json:"names"`
	ControllerLabels [5][2]uint16      `json:"controller_labels"`
	HealthTemplate   [9]uint16         `json:"health_template"`
	EnergyTemplate   [9]uint16         `json:"energy_template"`
}
type MatchStartEvent struct {
	Kind                        string    `json:"kind"`
	Source                      uint32    `json:"source"`
	ObjectID                    uint16    `json:"object_id"`
	Offset                      uint16    `json:"offset"`
	Words                       []uint16  `json:"words"`
	Glyphs                      [3]uint16 `json:"glyphs"`
	Attributes, X, Y, Parameter uint16
}
type MatchStartOutcome struct{ Events []MatchStartEvent }

// StepMatchStart recovers the complete game-rule part of manager stages0/2.
// Presentation events describe the actual ordered reset/resource/HUD/portrait
// calls; hardware text raster, sprite cache, SAT and DMA encoding are external.
// Spawned fighters retain old fields not written here and initialize later in
// their own scheduled126c8 call, rather than being advanced prematurely.
func StepMatchStart(state MatchStartState, library MatchStartLibrary) (MatchStartState, MatchStartOutcome, error) {
	original := state
	out := MatchStartOutcome{}
	if state.PauseFlags&0x80 != 0 || state.ModeFlag == 2 {
		return state, out, nil
	}
	if state.Manager.Stage != 0 && state.Manager.Stage != 2 {
		return state, out, errors.New("unsupported match-start stage")
	}
	if state.Manager.Stage == 0 {
		if state.Arena&1 != 0 || state.Arena > 14 || library.ArenaResources[state.Arena/2] == 0 {
			return original, MatchStartOutcome{}, errors.New("match-start arena resource is unavailable")
		}
		state.VideoMode = 2
		startSound(&state, 4)
		state.VideoBytes = [5]uint8{0, 0, 0x81, 0, 0}
		state.PlaneBases = [5]uint16{0xc000, 0xe000, 0xd000, 0xf000, 0xf400}
		state.PlaneSize = [2]uint16{2, 0}
		state.MaxSprites = 0
		state.PauseFlags &^= 1
		state.ResetWords = [4]uint16{}
		state.ScrollSources = [2]uint32{0xffffe800, 0xffffea00}
		state.Feedback = [7]uint16{}
		state.SlowTicks, state.SlowFlag, state.SlowGate = 0, 0, 0
		state.PrimaryColors = library.InitialColors
		state.ResourceList, state.ResourceBusy = library.ArenaResources[state.Arena/2], 3
		out.Events = append(out.Events, MatchStartEvent{Kind: "reset"}, MatchStartEvent{Kind: "resource", Source: state.ResourceList})
		state.Manager.Stage += 2
	}
	if state.ResourceBusy != 0 {
		return state, out, nil
	}
	if state.Arena&1 != 0 || state.Arena > 14 || library.ArenaBehaviors[state.Arena/2] == 0 {
		return original, MatchStartOutcome{}, errors.New("match-start arena behavior is unavailable")
	}
	for i, cid := range state.Characters {
		if cid == 0 {
			continue
		}
		if cid > 11 || state.Controllers[i] > 4 || state.Layout < 1 || state.Layout > 4 {
			return original, MatchStartOutcome{}, errors.New("match-start character/controller/HUD configuration is unavailable")
		}
	}
	var opponent int16 = -1
	if state.Rule == 4 {
		index := (state.Round << 2) + state.RoundPair
		if index&1 != 0 || index >= 56 {
			return original, MatchStartOutcome{}, errors.New("match-start round table index is unavailable")
		}
		opponent = library.RoundOpponents[index/2]
	}
	state.Disabled, state.SlowGate, state.SlowFlag, state.SlowTicks = 1, 1, 0, 0
	state.PendingSound, state.OpeningTicks, state.ResultTicks, state.WinnerCode = 0, 0, 0, 0
	state.Camera = [4]int16{0x360, 0x20, 0x360, 0x20}
	state.Bounds = [2]int16{0x370, 0x490}
	state.VideoBytes[3], state.VideoBytes[4] = 0, 6
	state.Objects[7].Header = MatchObjectHeader{0x14882, 0x80, 0}
	state.Objects[0].Header = MatchObjectHeader{0x1edac, 0x40, 0}
	if state.ModeFlag == 1 {
		state.Objects[1].Header = MatchObjectHeader{0xfeb2, 0x40, 0}
	}
	if state.Options[2] != 0 {
		state.Objects[3].Header = MatchObjectHeader{0xff74, 0x40, 0}
	}
	if state.Options[1] == 0 {
		state.Objects[4].Header = MatchObjectHeader{0xfe78, 0x40, 0}
	}
	if state.Rule == 0 {
		state.Objects[5].Header = MatchObjectHeader{0x10088, 0x40, 0}
	}
	if state.Rule == 4 {
		state.Objects[5].Header = MatchObjectHeader{0x10116, 0x40, 0}
		state.Objects[5].Parameter = state.Round
		if opponent >= 0 {
			state.Objects[6].Header = MatchObjectHeader{0x10172, 0x40, 0}
			state.Objects[6].Parameter = uint16(opponent)
		}
	}
	arena := state.Arena / 2
	state.Objects[2].Header = MatchObjectHeader{library.ArenaBehaviors[arena], 0x40, 0}
	state.AttributeMask = 0
	if arena == 0 || arena == 2 {
		state.AttributeMask = 0x8000
	}
	if arena == 0 {
		state.Objects[8].Header = MatchObjectHeader{0x150ca, 0x80, 0}
		state.Objects[9].Header = MatchObjectHeader{0x1525e, 0x80, 0}
	}
	if state.Options[0] == 0 && state.OptionA097 == 0 {
		music := [8]uint8{0x82, 0x86, 0x83, 0x95, 0x85, 0x8a, 0x88, 0x8d}[arena]
		if state.Rule == 0 {
			if state.Characters[3] == 10 {
				music = 0x87
			} else if state.Characters[3] == 7 {
				music = 0x84
			}
		}
		startSound(&state, music)
	}
	state.Selected = 0
	for i, cid := range state.Characters {
		if cid == 0 {
			continue
		}
		f := &state.Fighters[i]
		f.Header = MatchObjectHeader{0x12668, 0x100, 0}
		f.TargetBit = uint8(i * 4)
		f.Attributes = [4]uint16{0x2200, 0x4280, 0x6300, 0x6380}[i]
		x := [4]uint16{0x3b0, 0x3d0, 0x430, 0x450}[i]
		f.X = int32(uint32(x)<<16 | uint32(f.X)&0xffff)
		f.Ground, f.Depth = 0x1400000, 0x100
		if i == 1 || i == 2 {
			f.Ground, f.Depth = 0x1300000, 0xd8
		}
		f.Facing = 0
		if i >= 2 {
			f.Facing = 1
		}
		f.Layer = [4]uint8{0x10, 0x94, 0x98, 0x14}[i]
		f.CharacterID, f.Channel, f.BaseTeam = cid-1, state.Controllers[i]-1, state.BaseTeams[i]
		f.InputMode, f.StreamPosition = 0, 0
		f.HUDOffset = library.HUDOffsets[state.Layout-1][state.Selected/2]
		state.Selected += 2
		controller := library.ControllerLabels[state.Controllers[i]]
		out.Events = append(out.Events, MatchStartEvent{Kind: "words", Offset: f.HUDOffset + 0x102, Words: append([]uint16(nil), controller[:]...)},
			MatchStartEvent{Kind: "words", Offset: f.HUDOffset, Words: append([]uint16(nil), library.HealthTemplate[:]...)},
			MatchStartEvent{Kind: "words", Offset: f.HUDOffset + 0x80, Words: append([]uint16(nil), library.EnergyTemplate[:]...)},
			MatchStartEvent{Kind: "name", ObjectID: uint16(0x8400 + i*0x400), Offset: f.HUDOffset, Glyphs: library.Names[cid-1]})
		if state.Layout != 4 {
			out.Events = append(out.Events, MatchStartEvent{Kind: "portrait", ObjectID: uint16(0x8400 + i*0x400), Attributes: f.Attributes&0x6000 | 0x8300 | uint16(f.TargetBit)<<2, X: (f.HUDOffset&0x7e)*4 + 0x78, Y: (f.HUDOffset&0xff80)>>4 + 0x90, Parameter: uint16(cid) * 2})
		}
		if i != 2 {
			color := state.Colors[i]
			index := color - 4
			tail := library.Palettes[cid-1][index&3]
			if int8(index) >= 0 {
				tail = library.SharedPalettes[index&7]
			}
			slot := i + 1
			if i == 3 {
				slot = 3
			}
			copy(state.PrimaryColors[slot*16+6:slot*16+16], tail[:])
		}
	}
	state.Manager.Stage += 2
	return state, out, nil
}
func startSound(state *MatchStartState, request uint8) {
	for i, value := range state.SoundQueue {
		if value < 128 || value == request {
			state.SoundQueue[i] = request
			break
		}
	}
}
