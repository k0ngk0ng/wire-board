package engine

import "errors"

const MatchDrawCode int16 = -32768

type MatchObjectHeader struct {
	Behavior uint32
	Size     uint16
	Stage    uint16
}

type MatchFighter struct {
	Behavior uint32
	Health   int16
	BaseTeam uint8
}

// MatchLifecycleState covers the battle-manager phases after scene/actor
// creation. Team codes are original game identities, never platform seats.
// The three statistics arrays retain their actual contiguous storage meaning;
// LossesByCharacter[60:64] aliases input-recording EndMarkers at FFFC..FFFF.
type MatchLifecycleState struct {
	Manager                                 MatchObjectHeader
	Fighters                                [4]MatchFighter
	Presentation                            [3]MatchObjectHeader
	PauseFlags, PauseMask                   uint8
	ModeFlag, Disabled                      uint8
	EligibleChannels                        uint8
	Pressed                                 [4]uint8
	PendingSound                            uint8
	SoundQueue                              [4]uint8
	OpeningTicks, ResultTicks               uint16
	WinnerCode                              int16
	Rule                                    uint16
	RoundIndex                              int16
	Grouping                                uint16
	TeamResult                              uint8
	OptionFF14, OptionFF16                  uint8
	OptionA097                              uint8
	Characters, Controllers                 [4]uint8
	PaletteFlags, PaletteStatus             uint8
	PaletteMode, PaletteStep, PaletteTarget uint16
	VideoMode                               uint16
	CurrentColors, SecondaryColors          [64]uint16
	HeadToHead                              [16]uint8
	WinsByCharacter, LossesByCharacter      [64]uint8
}

type MatchLifecycleOutcome struct {
	Intro          bool
	Results        bool
	ClearWorldFrom uint16
}

// StepMatchLifecycle recovers complete manager stages 4..26 at 0xf2a4,
// including their fallthroughs. Stages 0/2 still require separate scene/actor
// initialization. ClearWorldFrom requests clearing every object from 8040 to
// A000, including fields outside this typed view; the caller must apply it.
func StepMatchLifecycle(state MatchLifecycleState) (MatchLifecycleState, MatchLifecycleOutcome, error) {
	before := state
	outcome := MatchLifecycleOutcome{}
	if state.PauseFlags&0x80 != 0 || state.ModeFlag == 2 {
		return state, outcome, nil
	}
	abort := func() {
		queueMatchSound(&state, 1)
		state.Manager.Stage = 24
	}
	switch state.Manager.Stage {
	case 4:
		state.PaletteStatus, state.PaletteFlags = 0x80, state.PaletteFlags|1
		state.PaletteMode, state.PaletteTarget, state.PaletteStep = 0, 0x0fff, 0x100
		state.SecondaryColors = state.CurrentColors
		state.Manager.Stage += 2
		// The following stage's signed palette test sees the newly set 0x80.
	case 6:
		if int8(state.PaletteStatus) >= 0 {
			state.Manager.Stage += 2
		}
	case 8:
		configureMatchPause(&state)
		state.Presentation[0] = MatchObjectHeader{Behavior: 0x101f2, Size: 64}
		outcome.Intro = true
		state.OpeningTicks = 0
		state.Manager.Stage += 2
		fallthrough
	case 10:
		flushMatchSound(&state)
		if matchStartRequest(&state) {
			abort()
			break
		}
		state.OpeningTicks++
		if int16(state.OpeningTicks) >= 256 {
			state.Manager.Stage += 2
		}
	case 12:
		state.OpeningTicks, state.Disabled = 0, 0
		state.Manager.Stage += 2
		fallthrough
	case 14:
		flushMatchSound(&state)
		if matchSurvivorTeam(state.Fighters, false) != 0 {
			if matchSurvivorTeam(state.Fighters, true) != 0 {
				state.Manager.Stage += 2
			}
		} else if matchStartRequest(&state) {
			abort()
		}
	case 16:
		state.ModeFlag, state.Disabled, state.ResultTicks = 0, 1, 0
		state.Manager.Stage += 2
		fallthrough
	case 18:
		flushMatchSound(&state)
		if int16(state.ResultTicks) < 64 {
			state.ResultTicks++
			break
		}
		for _, fighter := range state.Fighters {
			if fighter.Behavior == 0x12668 && fighter.Health < 0 && fighter.Health != -1024 {
				return state, outcome, nil
			}
		}
		queueMatchSound(&state, 5)
		state.Manager.Stage += 2
	case 20:
		state.PauseFlags &^= 1
		state.WinnerCode = MatchDrawCode
		deriveMatchWinner(&state)
		if err := recordMatchStatistics(&state); err != nil {
			return before, MatchLifecycleOutcome{}, err
		}
		spawnMatchResults(&state)
		outcome.Results = true
		if state.WinnerCode < 0 {
			state.ResultTicks = 256
		}
		state.Manager.Stage += 2
		fallthrough
	case 22:
		flushMatchSound(&state)
		state.ResultTicks++
		if int16(state.ResultTicks) >= 224 {
			if matchPressed(state.EligibleChannels, state.Pressed)&0x80 != 0 || matchStartRequest(&state) {
				abort()
				break
			}
		}
		if int16(state.ResultTicks) >= 512 {
			state.Manager.Stage += 2
			break
		}
		if err := maybeMatchTaunt(&state); err != nil {
			return before, MatchLifecycleOutcome{}, err
		}
	case 24:
		state.PaletteStatus = 0x81
		state.PaletteMode, state.PaletteTarget, state.PaletteStep = 0, 0x0fff, 0x100
		state.SecondaryColors = [64]uint16{}
		state.Manager.Stage += 2
	case 26:
		if int8(state.PaletteStatus) >= 0 {
			state.VideoMode = 0
			state.Manager = MatchObjectHeader{}
			state.Fighters = [4]MatchFighter{}
			state.Presentation = [3]MatchObjectHeader{}
			outcome.ClearWorldFrom = 0x8040
		}
	default:
		return before, MatchLifecycleOutcome{}, errors.New("match initialization or stage is not recovered")
	}
	return state, outcome, nil
}

func configureMatchPause(state *MatchLifecycleState) {
	if state.Rule == 2 && state.OptionFF16 != 0 {
		state.PauseMask = 15
		state.PauseFlags |= 1
		return
	}
	var mask uint32
	for _, channel := range state.Controllers {
		if channel != 0 {
			mask |= uint32(1) << ((channel - 1) & 31)
		}
	}
	state.PauseMask = uint8(mask)
	if state.PauseMask != 0 {
		state.PauseFlags |= 1
	}
}

// The byte minimum is the original unassigned sentinel. High-bit base-team
// values retain the original repeated-first-team behavior in isolated inputs.
func matchSurvivorTeam(fighters [4]MatchFighter, waitingForRetirement bool) uint8 {
	team := uint8(0x80)
	for _, fighter := range fighters {
		if fighter.Behavior != 0x12668 || !waitingForRetirement && fighter.Health < 0 || waitingForRetirement && fighter.Health == -1024 {
			continue
		}
		if int8(team) < 0 {
			team = fighter.BaseTeam
		} else if team != fighter.BaseTeam {
			return 0
		}
	}
	return team
}

func deriveMatchWinner(state *MatchLifecycleState) {
	var team uint16
	for _, fighter := range state.Fighters {
		if fighter.Behavior != 0x12668 || fighter.Health == -1024 {
			continue
		}
		if team == 0 {
			team = uint16(fighter.BaseTeam)
		} else if team != uint16(fighter.BaseTeam) {
			return
		}
	}
	if team != 0 {
		state.WinnerCode = int16(team)
	}
}

func matchPressed(mask uint8, pressed [4]uint8) uint8 {
	var buttons uint8
	for index, value := range pressed {
		if mask&(1<<index) != 0 {
			buttons |= value
		}
	}
	return buttons
}

func matchStartRequest(state *MatchLifecycleState) bool {
	if state.ModeFlag == 1 && matchPressed(^state.PauseMask, state.Pressed)&0x80 != 0 {
		state.ModeFlag = 2
		return true
	}
	return state.PauseMask == 0 && matchPressed(state.EligibleChannels, state.Pressed)&0x80 != 0
}

func queueMatchSound(state *MatchLifecycleState, request uint8) {
	for index, value := range state.SoundQueue {
		if value < 128 || value == request {
			state.SoundQueue[index] = request
			break
		}
	}
}

func flushMatchSound(state *MatchLifecycleState) {
	if state.PendingSound != 0 {
		queueMatchSound(state, state.PendingSound)
		state.PendingSound = 0
	}
}

func spawnMatchResults(state *MatchLifecycleState) {
	if state.WinnerCode < 0 {
		state.Presentation[0] = MatchObjectHeader{Behavior: 0x10436, Size: 64}
	} else if state.TeamResult == 0 {
		state.Presentation[0] = MatchObjectHeader{Behavior: 0x104ac, Size: 64}
		state.Presentation[1] = MatchObjectHeader{Behavior: 0x10522, Size: 64}
	} else {
		state.Presentation[0] = MatchObjectHeader{Behavior: 0x104ac, Size: 64}
		state.Presentation[1] = MatchObjectHeader{Behavior: 0x10616, Size: 64}
		state.Presentation[2] = MatchObjectHeader{Behavior: 0x106e8, Size: 64}
	}
}

func recordMatchStatistics(state *MatchLifecycleState) error {
	if state.Rule != 2 || state.WinnerCode <= 0 {
		return nil
	}
	if state.Grouping == 2 {
		winner := int(state.WinnerCode) - 1
		if winner < 0 || winner >= 4 {
			return errors.New("statistics winner is unavailable")
		}
		for loser := 0; loser < 4; loser++ {
			if loser != winner {
				if err := recordMatchPair(state, winner, loser); err != nil {
					return err
				}
			}
		}
		return nil
	}
	winners, losers := [2]int{0, 1}, [2]int{2, 3}
	if uint16(state.WinnerCode)>>1 != 0 {
		winners, losers = losers, winners
	}
	for _, winner := range winners {
		for _, loser := range losers {
			if err := recordMatchPair(state, winner, loser); err != nil {
				return err
			}
		}
	}
	return nil
}

func recordMatchPair(state *MatchLifecycleState, winner, loser int) error {
	a, b := state.Controllers[winner], state.Controllers[loser]
	if a == 0 || b == 0 {
		return nil
	}
	if a > 4 || b > 4 {
		return errors.New("statistics controller is unavailable")
	}
	row, column := int(a-1), int(b-1)
	index := row*4 + column
	if state.HeadToHead[index] >= 200 {
		state.HeadToHead[index] = 200
		return nil
	}
	state.HeadToHead[index]++
	ca, cb := state.Characters[winner], state.Characters[loser]
	if ca == 0 || ca > 11 || cb == 0 || cb > 11 {
		return errors.New("statistics character is unavailable")
	}
	winIndex, lossIndex := row*16+int(ca-1), column*16+int(cb-1)
	if state.WinsByCharacter[winIndex] >= 200 {
		state.WinsByCharacter[winIndex] = 200
	} else {
		state.WinsByCharacter[winIndex]++
	}
	if state.LossesByCharacter[lossIndex] >= 200 {
		state.LossesByCharacter[lossIndex] = 200
	} else {
		state.LossesByCharacter[lossIndex]++
	}
	return nil
}

func maybeMatchTaunt(state *MatchLifecycleState) error {
	if state.OptionFF14 != 0 || state.OptionA097 != 0 || state.WinnerCode <= 0 || state.Rule == 0 && state.RoundIndex >= 10 {
		return nil
	}
	frames := [11]int16{480, 320, 352, 352, 352, 384, 320, 320, 336, 352, 384}
	lookup := func(index int) (int16, error) {
		if index < 0 || index >= 4 || state.Characters[index] == 0 || state.Characters[index] > 11 {
			return 0, errors.New("taunt character is unavailable")
		}
		return frames[state.Characters[index]-1], nil
	}
	var frame int16
	var err error
	if state.Grouping == 4 {
		index := int(uint16(state.WinnerCode) >> 1)
		var other int16
		frame, err = lookup(index)
		if err == nil {
			other, err = lookup(index + 1)
			if other > frame {
				frame = other
			}
		}
	} else {
		frame, err = lookup(int(state.WinnerCode) - 1)
	}
	if err != nil {
		return err
	}
	if uint16(frame) == state.ResultTicks {
		queueMatchSound(state, 0x8f)
	}
	return nil
}
