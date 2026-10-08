package game

import "errors"

// Site rule: event cards replace production only. Two independent server dice
// retain the original distribution of fleet movement/strength (their minimum).
const CatanEventFleetRules = "wire-board-events-fleet-v1"

type CatanEventFleet struct {
	RollID   int    `json:"rollId"`
	Dice     [2]int `json:"dice"`
	Resolved bool   `json:"resolved"`
}

func (s *State) validateEventFleet() error {
	g := s.Catan
	d := g.EventDeck
	p := g.pirateIslands()
	if p == nil {
		if d.FleetRules != "" || d.Fleet != nil {
			return errors.New("非海盗群岛事件存档包含舰队规则")
		}
		return nil
	}
	if d.FleetRules != CatanEventFleetRules || g.Robber != -1 {
		return errors.New("海盗群岛事件舰队规则版本或强盗状态无效")
	}
	f := d.Fleet
	if g.RollID == 0 {
		if f != nil || p.Raid != nil {
			return errors.New("尚未抽牌却有舰队行动")
		}
		return nil
	}
	if f == nil || f.RollID != g.RollID || f.Dice[0] < 1 || f.Dice[0] > 6 || f.Dice[1] < 1 || f.Dice[1] > 6 {
		return errors.New("舰队骰子与本次抽牌不一致")
	}
	beforeFleet := g.CardEvent != nil || g.CitiesKnights != nil && g.CitiesKnights.Event != nil
	if beforeFleet && (f.Resolved || p.Raid != nil) || !beforeFleet && !s.Finished && !f.Resolved {
		return errors.New("舰队与事件回应顺序不一致")
	}
	if q := p.Raid; q != nil {
		r := g.RevealedEvent
		if s.Phase != "catan_fleet_reward" || r == nil || r.ProductionStarted || q.Total != r.Production || q.Epidemic != (r.Kind == "epidemic") || len(q.Rewards) == 0 {
			return errors.New("舰队奖励与事件生产后续不一致")
		}
		seen := map[int]bool{}
		for _, player := range q.Rewards {
			if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || seen[player] {
				return errors.New("舰队奖励回应者无效")
			}
			seen[player] = true
		}
	} else if s.Phase == "catan_fleet_reward" {
		return errors.New("舰队奖励回应缺失")
	}
	return nil
}

func (s *State) catanEventProduction(total int, epidemic bool) error {
	g := s.Catan
	if g.pirateIslands() != nil {
		if g.EventDeck == nil || g.EventDeck.FleetRules != CatanEventFleetRules || g.EventDeck.Fleet == nil {
			return errors.New("缺少事件舰队骰子")
		}
		f := g.EventDeck.Fleet
		if f.RollID != g.RollID || f.Resolved {
			return errors.New("事件舰队已经行动")
		}
		f.Resolved = true
		if pending, err := s.catanRaidFleetDice(total, epidemic, f.Dice); pending || err != nil {
			return err
		}
	}
	return s.catanRollProductionEffect(total, epidemic)
}
