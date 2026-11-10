package starfort

import (
	"encoding/json"
	"errors"
	"math"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Bots use public information only. A round-robin actor cursor means one bot
// cannot monopolize the host's one-action scheduling tick.
func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	p := ctx.Actor
	if p < 0 || p >= len(s.Players) || s.finished() {
		return nil, errors.New("当前不能托管")
	}
	a := Action{Epoch: s.Epoch}
	if s.Phase == "paused" {
		for _, v := range s.Players {
			if v.ResumeVote && !s.Players[p].ResumeVote {
				a.Type = "resume"
			}
		}
		return marshalBotAction(a)
	}
	if s.Phase == "prepare" {
		for _, v := range s.Players {
			if v.PauseVote {
				if !s.Players[p].PauseVote {
					a.Type = "pause"
				}
				return marshalBotAction(a)
			}
		}
	}
	if s.Phase == "prepare" {
		for _, kind := range []string{"route", "boon"} {
			c := s.World.Choice
			if kind == "boon" {
				c = s.World.BoonChoice
			}
			if c != nil && c.Votes[p] == "" {
				index := int((s.World.Seed + int64(c.Stage)) % int64(len(c.Options)))
				if kind == "boon" {
					index = 0
					if s.World.Boons.Power > 0 && s.World.Boons.Power%3 == 0 && s.World.Boons.Reach < s.World.Boons.Power/3 {
						index = 1
					}
				}
				return json.Marshal(Action{Type: kind + "_vote", Choice: c.Options[index].ID, Epoch: s.Epoch})
			}
		}
		if s.World.Choice != nil || s.World.BoonChoice != nil {
			return nil, nil
		}
		if s.rebalanced() && s.World.PadReward != nil {
			if s.World.PadReward.Owner == p {
				if a := s.automaticPadReward(); a != nil {
					a.Epoch = s.Epoch
					return json.Marshal(a)
				}
			}
			if s.World.PadReward.Kind == "move" && len(s.Towers) == 0 {
				if investment := s.investment(p); investment != nil {
					investment.Epoch = s.Epoch
					return json.Marshal(investment)
				}
			}
			return nil, nil
		}
		if quote := s.shopControl(p); quote != nil && quote.Enabled {
			if quote.RepairMax > 0 && s.BaseHP <= s.BaseMaxHP-4 {
				return json.Marshal(Action{Type: "shop", Choice: "repair", Amount: quote.RepairMax, Epoch: s.Epoch})
			}
			if s.Players[p].Gold >= 260 {
				for _, c := range quote.Towers {
					if c.DamageEnabled {
						return json.Marshal(Action{Type: "shop", Choice: "damage", TowerID: c.TowerID, Epoch: s.Epoch})
					}
				}
				for _, c := range quote.Towers {
					if c.RangeEnabled {
						return json.Marshal(Action{Type: "shop", Choice: "range", TowerID: c.TowerID, Epoch: s.Epoch})
					}
				}
			}
			if quote.ShieldEnabled && s.Players[p].Gold >= 1160 {
				return json.Marshal(Action{Type: "shop", Choice: "shield", Epoch: s.Epoch})
			}
		}
		if quote := s.repairControl(p); quote != nil && quote.Enabled && s.BaseHP <= s.BaseMaxHP/2 && s.Players[p].Gold >= quote.UnitCost*quote.MaxAmount+160 {
			return json.Marshal(Action{Type: "repair_base", Amount: quote.MaxAmount, Epoch: s.Epoch})
		}
		for _, o := range s.World.Objectives {
			if o.Status == "available" && s.Players[p].Gold >= o.Cost+160 && len(s.Towers) >= 6 {
				return json.Marshal(Action{Type: "objective", ObjectiveID: o.ID, Epoch: s.Epoch})
			}
		}
	}
	// One accurate interceptor in each sector prioritizes healers, banners
	// and engineers. Once assigned, leave the policy alone: bot ticks should
	// spend resources or yield, not repeatedly write equivalent target actions.
	if s.Wave >= 4 {
		support := map[int]bool{}
		for _, t := range s.Towers {
			if (t.Kind == "arrow" || t.Kind == "arcane") && t.Priority == "support" {
				support[s.World.Map.Pads[t.Pad].Sector] = true
			}
		}
		for _, t := range s.Towers {
			sector := s.World.Map.Pads[t.Pad].Sector
			if t.Owner == p && (t.Kind == "arrow" || t.Kind == "arcane") && !support[sector] {
				return json.Marshal(Action{Type: "target", TowerID: t.ID, Priority: "support", Epoch: s.Epoch})
			}
		}
	}
	if investment := s.investment(p); investment != nil {
		investment.Epoch = s.Epoch
		return json.Marshal(investment)
	}
	if s.Phase == "prepare" && !s.Players[p].Ready {
		a.Type = "ready"
	}
	return marshalBotAction(a)
}
func (s *State) investment(p int) *Action {
	if s.World.Layout == "central" {
		return s.centralInvestment(p)
	}
	owned := []Tower{}
	counts := map[string]int{}
	occupied := map[int]bool{}
	for _, t := range s.Towers {
		occupied[t.Pad] = true
		counts[t.Kind]++
		if t.Owner == p {
			owned = append(owned, t)
		}
	}
	// Every opened sector needs damage and slowing support. New roads can
	// bypass the original kill zone, so the team builds a mixed defense rather
	// than spending every reward on towers that can no longer cover that lane.
	for sector := 2; sector <= s.World.Stage; sector++ {
		count := 0
		for _, t := range s.Towers {
			if s.World.Map.Pads[t.Pad].Sector == sector {
				count++
			}
		}
		desired := 3
		if s.Wave >= 25 {
			desired = 4
		}
		if count >= desired {
			continue
		}
		kind := []string{"arcane", "cannon", "frost", "arrow"}[count]
		c, _ := spec(kind)
		if s.Players[p].Gold < c.Cost {
			continue
		}
		bestPad := -1
		bestCoverage := -1.
		for _, pad := range s.World.Map.Pads {
			if pad.Sector != sector || occupied[pad.ID] {
				continue
			}
			coverage := 0.
			for _, r := range s.World.Map.Routes {
				for d := 0.; d < pathDistance(r.Points); d += 50 {
					point := pathPosition(r.Points, d)
					if math.Hypot(point.X-pad.X, point.Y-pad.Y) <= c.Range {
						coverage++
					}
				}
			}
			if coverage > bestCoverage {
				bestCoverage = coverage
				bestPad = pad.ID
			}
		}
		if bestPad >= 0 {
			return &Action{Type: "build", Pad: bestPad, Tower: kind}
		}
	}
	// First establish a mixed central defense; later upgrades beat buying a
	// distant low-value tower. Shared frost support is limited to avoid a team
	// investing all money in slowing enemies without doing enough damage.
	desired := max(2, 10/len(s.Players))
	if len(owned) < desired && len(s.Towers) < 12 {
		kind := "arrow"
		if counts["cannon"] == 0 && len(s.Towers) > 0 {
			kind = "cannon"
		} else if counts["arcane"] == 0 && len(s.Towers) > 1 {
			kind = "arcane"
		} else if counts["frost"] == 0 && len(s.Towers) > 2 {
			kind = "frost"
		} else {
			kind = []string{"arcane", "arrow", "cannon"}[len(s.Towers)%3]
		}
		c, _ := spec(kind)
		if s.Players[p].Gold >= c.Cost {
			for _, pad := range []int{1, 5, 3, 0, 2, 4, 6, 7, 10, 14, 8, 9, 11, 12, 13, 15} {
				if pad < len(s.World.Map.Pads) && !occupied[pad] {
					return &Action{Type: "build", Pad: pad, Tower: kind}
				}
			}
		}
	}
	best := -1
	value := -1.
	for i, t := range owned {
		cost := s.upgradePrice(t)
		if cost > s.Players[p].Gold {
			continue
		}
		damage, _, interval := s.towerStats(t)
		score := (damage / float64(interval)) / float64(cost) / math.Sqrt(float64(t.Level))
		if t.Kind == "frost" {
			score *= .45
		}
		if score > value {
			value = score
			best = i
		}
	}
	if best >= 0 {
		t := owned[best]
		a := &Action{Type: "upgrade", TowerID: t.ID}
		if t.Level == 3 {
			a.Branch = "power"
			if t.Kind == "arrow" && s.SkillRules == 2 {
				a.Branch = "volley"
				if s.rebalanced() && t.Pad%3 != 0 {
					a.Branch = "heart"
				}
			}
			if t.Kind == "frost" || t.Kind == "cannon" {
				a.Branch = "speed"
			}
			if s.rebalanced() && t.Kind == "cannon" {
				a.Branch = "power"
			}
			if s.specialized() && t.Kind == "frost" && t.Pad%2 == 0 {
				a.Branch = "freeze"
			}
			if s.specialized() && t.Kind == "arcane" && t.Pad%2 == 0 {
				a.Branch = "magic"
			}
		}
		return a
	}
	return nil
}

// A nil action tells the host to try the next computer-controlled seat without
// storing a no-op snapshot or extending room activity.
func marshalBotAction(a Action) (json.RawMessage, error) {
	if a.Type == "" {
		return nil, nil
	}
	return json.Marshal(a)
}
