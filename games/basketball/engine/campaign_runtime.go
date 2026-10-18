package engine

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

// CampaignMatchSelection contains committed roster/tactic choices and the
// caller's current original RNG. Enemy roster selection is not synthesized here.
type CampaignMatchSelection struct {
	Opponent     uint8
	Characters   [4]uint8
	HumanTactics [2]uint8
	Seed         RandomState
}

// NewCampaignRuntime starts the selected match from owned preparation state.
// It does not decode a password again, draw a reward or advance the random stream.
// The committed preparation remains in Config across saves. Explicit halftime
// confirmation can replace its loadout; live half-period and win masks belong
// to the existing match stages until the result is committed.
func NewCampaignRuntime(progress campaign.Progress, selection CampaignMatchSelection) (RuntimeState, error) {
	court, err := campaign.CourtForOpponent(progress, selection.Opponent)
	if err != nil {
		return RuntimeState{}, err
	}
	c := DefaultRuntimeConfig()
	c.Campaign = &progress
	c.Match.Court, c.Match.Mode = court, progress.Mode
	c.Match.Teams = [2]uint8{0, selection.Opponent}
	c.Match.Characters, c.Match.Seed = selection.Characters, selection.Seed
	c.HumanTactics = selection.HumanTactics
	if progress.Mode == 1 {
		c.Match.ControllerAssignments[1] = 1
	}
	for character, slots := range progress.Equipment {
		for slot, item := range slots {
			c.Match.Equipment[character][slot] = int8(item)
		}
	}
	return NewRuntime(c)
}

func validateCampaignConfiguration(c RuntimeConfig) error {
	if c.Campaign == nil {
		return nil
	}
	p := *c.Campaign
	court, err := campaign.CourtForOpponent(p, c.Match.Teams[1])
	if err != nil {
		return err
	}
	if c.Match.Teams[0] != 0 || c.Match.Mode != p.Mode || c.Match.Court != court {
		return fmt.Errorf("campaign mode, team or court differs from preparation")
	}
	for i, ch := range c.Match.Characters {
		if ch >= 4 || i%2 == 1 && ch == c.Match.Characters[i-1] {
			return fmt.Errorf("invalid campaign roster")
		}
	}
	for _, tactic := range c.HumanTactics {
		if tactic < 1 || tactic > 5 {
			return fmt.Errorf("campaign tactic is not an original menu choice")
		}
	}
	for character, slots := range p.Equipment {
		for slot, item := range slots {
			if c.Match.Equipment[character][slot] != int8(item) {
				return fmt.Errorf("campaign equipment differs from owned preparation")
			}
		}
	}
	return nil
}

func validateCampaignMatchProgress(s RuntimeState) error {
	if s.Config.Campaign == nil {
		return nil
	}
	p := *s.Config.Campaign
	m := runtimeMatch(&s)
	wins := p.WinBits
	var actual [2]uint8
	if s.Opening != nil {
		actual = s.Opening.Transition.TeamWinBits
	}
	if s.Playing != nil {
		actual = s.Playing.TeamWinBits
	}
	if s.Transition != nil {
		actual = s.Transition.Scene.Transition.TeamWinBits
	}
	if s.Outcome != nil {
		period := p.Period
		if s.Outcome.WinningTeam == 0 {
			period += 2
			wins[(p.Period>>1)&1] |= 1 << (s.Config.Match.Teams[1] - 1)
		}
		if m.Period != period {
			return fmt.Errorf("campaign result period differs from starting progress")
		}
	} else {
		limit := p.Period + 1
		// ResetOriginalPeriod increments before the result entry writes wins or
		// rolls back a defeat. This short unentered result stage is legitimate.
		if s.Transition != nil && m.Phase == 6 && !m.Entered {
			limit++
		}
		if m.Period < p.Period || m.Period > limit {
			return fmt.Errorf("campaign half-period differs from starting progress")
		}
	}
	if actual != wins {
		return fmt.Errorf("campaign win masks differ from match progress")
	}
	return nil
}
