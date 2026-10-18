package engine

import "fmt"

func acknowledgeOriginalTeamCommand(h *HumanCommandState, message uint8) {
	h.Messages[1], h.Timers[1] = message, 16
}

// selectOriginalHumanTeamStrategy executes the source command arbitration
// before either team's strategy handler. The -1 busy marker differs from the
// other negative status bytes consumed by the strategy lifecycle.
func selectOriginalHumanTeamStrategy(result *TeamAIStep, team uint8, ctx TeamAIContext) error {
	h := &result.Human[team]
	s := &result.State.Teams[team]
	if h.Request > 2 {
		return fmt.Errorf("human command request %d outside original input domain", h.Request)
	}
	teammate := team*2 + 1
	if ctx.Holder != ctx.PreviousHolder {
		if ctx.Holder == teammate {
			h.Status = 0
			h.Messages = [2]uint8{}
		} else if h.Status == -1 {
			return nil
		}
		s.Mode, s.Entered = originalHumanTeamModes[team*8+ctx.Holder], false
		return nil
	}
	if h.Status == -1 {
		return nil
	}
	if h.Request == 0 {
		desired := originalHumanTeamModes[team*8+ctx.Holder]
		if s.Mode != desired {
			s.Mode, s.Entered = desired, false
		}
		return nil
	}
	mode := result.State.Players[teammate].Mode
	if int(mode) >= len(originalCommandModeFlags) {
		return fmt.Errorf("command eligibility mode %d outside original table", mode)
	}
	if originalCommandModeFlags[mode]&1 == 0 || ctx.CourtID == 1 && ctx.CachedZones[teammate]&15 == 5 {
		acknowledgeOriginalTeamCommand(h, 2)
		h.Status, h.Request = 0, 0
		return nil
	}
	base := uint8(5)
	if ctx.Holder == teammate {
		base = 3
	}
	s.Mode, s.Entered = base+h.Request, false
	return nil
}

// advanceOriginalTeamCommand preserves entry and lifecycle writes for the
// original strategies4/5 and6/7, including cancellation after possession loss.
func advanceOriginalTeamCommand(result *TeamAIStep, team uint8, ctx TeamAIContext) error {
	if team >= 2 {
		return fmt.Errorf("invalid command team %d", team)
	}
	s := &result.State.Teams[team]
	h := &result.Human[team]
	teammate := team*2 + 1
	if s.Mode < 4 || s.Mode > 7 {
		return fmt.Errorf("invalid cooperation strategy %d", s.Mode)
	}
	if s.Entered {
		if s.Mode < 6 {
			if ctx.Holder == teammate && h.Status < 0 {
				return nil
			}
			h.Request, h.Status, h.ButtonWindows[0] = 0, 0, 0
			s.Mode, s.Entered = 13, false
		} else if h.Status >= 0 {
			s.Mode, s.Entered = 13, false
		}
		return nil
	}
	s.Entered = true
	action := ctx.PlayerActions[teammate]
	if action >= 0x20 || action&15 >= 4 {
		h.Status = 0
		acknowledgeOriginalTeamCommand(h, 2)
		h.Request, h.ButtonWindows[0] = 0, 0
		return nil
	}
	if h.Request > 2 || ctx.HumanTactics[team] >= 6 {
		return fmt.Errorf("command tactic/request outside original table")
	}
	h.Accepted = h.Request
	h.Status = -1
	acknowledgeOriginalTeamCommand(h, 1)
	index := uint8(0)
	if ctx.Holder != 6 {
		if ctx.Holder >= 4 {
			return fmt.Errorf("command prop-holder plan not reconstructed")
		}
		index = [8]uint8{3, 2, 1, 1, 1, 1, 3, 2}[team*4+ctx.Holder]
	}
	mode := originalHumanCommandPlans[ctx.HumanTactics[team]][h.Request*4+index]
	p := &result.State.Players[teammate]
	if ctx.Holder != teammate && ctx.Airborne[teammate] {
		p.Plan = mode
		p.Mode, p.Entered = AIRecover, false
	} else {
		p.Mode, p.Entered = mode, false
	}
	if p.Mode == 13 {
		if _, ok := originalAvailableAuxiliary(ctx.Auxiliary); !ok {
			h.Status = 0
			acknowledgeOriginalTeamCommand(h, 2)
		}
	}
	h.Request, h.ButtonWindows[0] = 0, 0
	return nil
}
