package engine

import "errors"

// TeamStrategyState is the team-level strategy ($0663/$0668), separate from
// each actor's personal mode. A negative refresh counter re-arms entry.
type TeamStrategyState struct {
	Mode    uint8 `json:"mode"`
	Entered bool  `json:"entered"`
	Counter int8  `json:"counter"`
}

type TeamAIState struct {
	Teams   [2]TeamStrategyState `json:"teams"`
	Players [4]AIControlState    `json:"players"`
}

type TeamAIContext struct {
	Teams                  [2]uint8
	HasHuman               [2]bool
	HumanTactics           [2]uint8
	HumanCommands          [2]uint8
	HumanStatus            [2]int8
	Holder, PreviousHolder uint8
	ScoreDifference        uint8
	CourtID                uint8
	Objects                [7]Position
	Airborne               [4]bool
	Attachments            [4]uint8
	Hoops                  [6]AIHoopView
	// $B2F4 requires exact raw action $81, including the entered bit.
	HoopEntered [6]bool
}

type TeamAIStep struct{ State TeamAIState }

func originalTeamRoleOrder(team uint8, ctx TeamAIContext) (uint8, uint8) {
	first := team * 2
	second := first + 1
	if ctx.Holder == first {
		return first, second
	}
	if ctx.Holder == second {
		return second, first
	}
	// $9E2F compares the low byte of abs(horizontal integer distance)/2.
	// Its quantization and second-player tie break are intentional.
	distance := func(actor uint8) uint8 {
		delta := int16(uint16(ctx.Objects[actor].X>>8) - uint16(ctx.Objects[ctx.Holder].X>>8))
		magnitude := int32(delta)
		if magnitude < 0 {
			magnitude = -magnitude
		}
		return uint8(uint16(magnitude) >> 1)
	}
	if distance(first) < distance(second) {
		return first, second
	}
	return second, first
}

func originalTeamScoreBand(team uint8, difference uint8) (uint8, error) {
	if difference&0x7f > 3 {
		return 0, errors.New("invalid original score-difference tier")
	}
	if difference == 0 {
		return 0, nil
	}
	level := difference & 15
	negative := difference&128 != 0
	if (team == 0 && negative) || (team == 1 && !negative) {
		level += 3
	}
	return level, nil
}

func originalHumanTeamPlan(team uint8, ctx TeamAIContext) (AIMode, error) {
	profile := ctx.HumanTactics[team]
	if profile >= 6 {
		return 0, errors.New("invalid original human-team tactic profile")
	}
	index := uint8(0)
	if ctx.Holder != 6 {
		if ctx.Holder >= 4 {
			return 0, errors.New("original human-team prop-holder tactic is not reconstructed")
		}
		index = [8]uint8{3, 2, 1, 1, 1, 1, 3, 2}[team*4+ctx.Holder]
	}
	return originalTeamHumanPlans[profile][index], nil
}

func assignOriginalTeamRole(state *TeamAIState, actor uint8, mode AIMode, ctx TeamAIContext) {
	p := &state.Players[actor]
	if p.Mode == AICollectHoop {
		return
	}
	p.Entered = false
	if ctx.Airborne[actor] {
		p.Plan = mode
		p.Mode = AIRecover
		if actor == ctx.Holder {
			p.Mode = AIPossessionWait
		}
	} else {
		p.Mode = mode
	}
}

func advanceOriginalComputerTeam(state *TeamAIState, team uint8, ctx TeamAIContext) error {
	s := &state.Teams[team]
	if s.Entered {
		if ctx.Holder < 6 {
			if ctx.Holder >= 4 {
				return errors.New("original prop-holder team refresh is not reconstructed")
			}
			if ctx.Airborne[ctx.Holder] && ctx.Attachments[ctx.Holder]&128 == 0 {
				return nil
			}
			if s.Mode == 11 && !ctx.Airborne[ctx.Holder] {
				ready := true
				for i, h := range ctx.Hoops {
					ready = ready && h.Action == HoopReady && ctx.HoopEntered[i]
				}
				height := int16(uint16(ctx.Objects[ctx.Holder].Height >> 8))
				if ready && height > 0 {
					if ctx.CourtID >= 8 {
						return errors.New("invalid original elevated-defense court")
					}
					if [8]bool{false, true, true, false, false, true, false, true}[ctx.CourtID] {
						opponent := (ctx.Holder & 2) ^ 2
						a, b := state.Players[opponent].Mode, state.Players[opponent+1].Mode
						if !(a == 28 && b == 29 || a == 29 && b == 28) {
							first, second := originalTeamRoleOrder(team, ctx)
							assignOriginalTeamRole(state, first, 28, ctx)
							assignOriginalTeamRole(state, second, 29, ctx)
						}
						return nil
					}
				}
			}
		}
		s.Counter--
		if s.Counter < 0 {
			s.Entered = false
		}
		return nil
	}
	if ctx.Teams[team] >= 8 {
		return errors.New("invalid original team profile")
	}
	band, err := originalTeamScoreBand(team, ctx.ScoreDifference)
	if err != nil {
		return err
	}
	choice := 0
	if s.Mode == 10 {
		choice = 2
	} else if s.Mode == 11 {
		choice = 4
	}
	plans := originalTeamComputerPlans[ctx.Teams[team]][band]
	primary, secondary := plans[choice], plans[choice+1]
	for _, h := range ctx.Hoops {
		if h.Action == HoopInactive && h.Position.Height>>8 == 0 {
			secondary = AICollectHoop
			break
		}
	}
	first, second := originalTeamRoleOrder(team, ctx)
	s.Entered = true
	s.Counter = 48
	assignOriginalTeamRole(state, first, primary, ctx)
	assignOriginalTeamRole(state, second, secondary, ctx)
	return nil
}

// StepOriginalTeamAI restores the complete observed $99F0 phase: changes in
// possession select team strategies, then both teams assign personal modes.
// It does not advance physics, consume random numbers, or update PreviousHolder;
// the original caller commits that last field only after this phase returns.
func StepOriginalTeamAI(original TeamAIState, ctx TeamAIContext) (TeamAIStep, error) {
	initial := TeamAIStep{State: original}
	result := initial
	if ctx.Holder >= 7 || ctx.PreviousHolder >= 7 {
		return initial, errors.New("invalid original team ball holder")
	}
	for team := uint8(0); team < 2; team++ {
		s := &result.State.Teams[team]
		if ctx.HasHuman[team] {
			if ctx.HumanCommands[team] != 0 || ctx.HumanStatus[team] != 0 {
				return initial, errors.New("original human-team cooperation request is not reconstructed")
			}
			desired := originalHumanTeamModes[team*8+ctx.Holder]
			if ctx.Holder != ctx.PreviousHolder || s.Mode != desired {
				s.Mode, s.Entered = desired, false
			}
		} else if ctx.Holder != ctx.PreviousHolder {
			s.Mode, s.Entered = originalComputerTeamModes[team*8+ctx.Holder], false
		}
	}
	for team := uint8(0); team < 2; team++ {
		s := &result.State.Teams[team]
		switch s.Mode {
		case 0:
		case 1, 10, 11, 12:
			if err := advanceOriginalComputerTeam(&result.State, team, ctx); err != nil {
				return initial, err
			}
		case 2, 3, 8, 9, 13:
			if s.Entered {
				if s.Mode == 13 {
					s.Counter--
					if s.Counter < 0 {
						s.Entered = false
					}
				}
				continue
			}
			mode, err := originalHumanTeamPlan(team, ctx)
			if err != nil {
				return initial, err
			}
			s.Entered = true
			if s.Mode == 13 {
				s.Counter = 48
			}
			actor := team*2 + 1
			p := &result.State.Players[actor]
			p.Entered = false
			if ctx.Airborne[actor] {
				p.Plan = mode
				p.Mode = AIRecover
				if s.Mode == 2 || s.Mode == 3 || s.Mode == 8 {
					p.Mode = AIPossessionWait
				}
			} else {
				p.Mode = mode
			}
		default:
			return initial, errors.New("original team cooperation strategy is not reconstructed")
		}
	}
	return result, nil
}
