package conquest

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"slices"
	"sort"
)

func category(o Order) int {
	if o.Type == "move" {
		return 2
	}
	if o.Type == "disband" {
		return 1
	}
	return 0
}
func sortedOrders(orders []Order) []Order {
	result := append([]Order(nil), orders...)
	sort.SliceStable(result, func(i, j int) bool { return category(result[i]) < category(result[j]) })
	return result
}
func route(g *geography, from, to string) (bool, bool) {
	i, ok := g.index[from]
	if !ok || from == to {
		return false, false
	}
	p := g.data.Provinces[i]
	if slices.Contains(p.Neighbors, to) {
		return true, false
	}
	if slices.Contains(p.SeaNeighbors, to) {
		return true, true
	}
	return false, false
}
func buildCost(b string) (int64, int, bool) {
	switch b {
	case "fort":
		return 5000, 5, true
	case "tower":
		return 2500, 3, true
	case "town":
		return 75000, 50, true
	}
	return 0, 0, false
}
func (s *State) project(seat int, orders []Order) ([]Province, int64, int, string, error) {
	ps := append([]Province(nil), s.Provinces...)
	g, _ := mapFor(s.MapID)
	gold := s.Players[seat].Gold
	mp := s.Players[seat].MP10
	king := s.Players[seat].King
	fail := func(msg string) ([]Province, int64, int, string, error) { return ps, gold, mp, king, errors.New(msg) }
	for _, o := range sortedOrders(orders) {
		idx, ok := g.index[o.Province]
		if o.Type == "move" {
			idx, ok = g.index[o.From]
		}
		if o.Type != "tax" && (!ok || ps[idx].Owner != seat) {
			return fail("只能操作自己的地区")
		}
		cost := int64(0)
		points := 0
		switch o.Type {
		case "tax":
			if o.Value < 0 || o.Value > 100 || o.Value%10 != 0 {
				return fail("税率应为0～100的10倍数")
			}
			continue
		case "recruit":
			p := &ps[idx]
			if o.Amount <= 0 || o.Amount > p.Population/2 || p.Troops+o.Amount > maxTroops {
				return fail("征兵数量超过人口或驻军上限")
			}
			cost = int64(o.Amount)
			points = 10
			p.Troops += o.Amount
			p.Mobile += o.Amount
			p.Population -= o.Amount
		case "disband":
			p := &ps[idx]
			if o.Amount <= 0 || o.Amount > p.Mobile {
				return fail("解散数量超过可用兵力")
			}
			points = 1
			p.Troops -= o.Amount
			p.Mobile -= o.Amount
			p.Population = min(maxCivil, p.Population+o.Amount)
		case "build":
			var valid bool
			cost, points, valid = buildCost(o.Building)
			if !valid {
				return fail("无效建筑")
			}
			p := &ps[idx]
			switch o.Building {
			case "fort":
				if p.Fort {
					return fail("该地已有工事")
				}
				p.Fort = true
			case "tower":
				if !s.Options.Fog {
					return fail("公开地图不需要瞭望塔")
				}
				if p.Tower {
					return fail("该地已有瞭望塔")
				}
				p.Tower = true
			case "town":
				if p.Town {
					return fail("该地已有城镇")
				}
				p.Town = true
			}
		case "move":
			p := &ps[idx]
			valid, sea := route(g, o.From, o.To)
			if !valid {
				return fail("只能沿地图上相邻地区或海路行动")
			}
			if o.Amount <= 0 || o.Amount > p.Mobile {
				return fail("出兵数量超过可用兵力")
			}
			if o.King && king != o.From {
				return fail("君主不在该地区")
			}
			target := &ps[g.index[o.To]]
			if target.Owner == seat && target.Troops+o.Amount > maxTroops {
				return fail("目的地驻军已达上限")
			}
			points = 10
			if sea {
				cost = int64(o.Amount) * 2
			}
			p.Troops -= o.Amount
			p.Mobile -= o.Amount
			if target.Owner == seat {
				// Show expected arrivals without making them available for a
				// second move before this round has actually resolved.
				target.Troops += o.Amount
			}
			if o.King {
				king = "" // The ruler can accompany only one order this round.
			}
		default:
			return fail("未知计划类型")
		}
		if cost > 0 && gold < cost {
			return fail("金币不足")
		}
		if mp < points {
			return fail("行动点不足")
		}
		gold -= cost
		mp -= points
	}
	return ps, gold, mp, king, nil
}
func actionOrder(a Action, id int) (Order, error) {
	o := Order{ID: id, Type: a.Type, From: a.From, To: a.To, Province: a.Province, Amount: a.Amount, King: a.King, Building: a.Building}
	if a.Value != nil {
		o.Value = *a.Value
	}
	if a.Enabled != nil || a.ID != 0 {
		return o, errors.New("行动包含无关字段")
	}
	switch a.Type {
	case "move":
		if a.Province != "" || a.Building != "" || a.Value != nil {
			return o, errors.New("无效行军字段")
		}
	case "recruit", "disband":
		if a.From != "" || a.To != "" || a.King || a.Building != "" || a.Value != nil {
			return o, errors.New("无效征兵字段")
		}
	case "build":
		if a.From != "" || a.To != "" || a.Amount != 0 || a.King || a.Value != nil {
			return o, errors.New("无效建设字段")
		}
	case "tax":
		if a.Value == nil || a.From != "" || a.To != "" || a.Province != "" || a.Amount != 0 || a.King || a.Building != "" {
			return o, errors.New("无效税率字段")
		}
	default:
		return o, errors.New("未知行动")
	}
	return o, nil
}
func hasOrderFields(a Action) bool {
	return a.From != "" || a.To != "" || a.Province != "" || a.Amount != 0 || a.King || a.Building != "" || a.Value != nil
}
func apply(c gameapi.Context, m gameapi.Match, b json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	var a Action
	if strict(b, &a) != nil || c.Players != len(s.Players) || c.Actor < 0 || c.Actor >= len(s.Players) {
		return m, errors.New("无效行动")
	}
	if a.Session != s.Session || a.Epoch != s.Epoch {
		return m, errors.New("回合已更新，请重新操作")
	}
	seat := c.Actor
	p := &s.Players[seat]
	automated := seat < len(c.Automated) && c.Automated[seat]
	if s.Phase == "planning" && a.Type != "pause" && a.Type != "resign" && !automated && p.Deadline > 0 && c.Now.UnixMilli() >= p.Deadline {
		return m, errors.New("本轮行动时间已结束，等待托管接管")
	}
	if p.Eliminated || s.Phase == "finished" || s.Phase == "resolving" {
		return m, errors.New("当前不能操作")
	}
	switch a.Type {
	case "pause", "resume":
		if a.Enabled == nil || hasOrderFields(a) || a.ID != 0 {
			return m, errors.New("无效暂停投票")
		}
		if a.Type == "pause" {
			if s.Phase != "planning" {
				return m, errors.New("当前不能请求暂停")
			}
			s.PauseVotes[seat] = *a.Enabled
			if s.allVotes(s.PauseVotes, c.Automated) {
				for i := range s.Players {
					p := &s.Players[i]
					if !p.Eliminated {
						s.Remaining[i] = max(0, p.Deadline-c.Now.UnixMilli())
					}
					p.Deadline = 0
				}
				s.Phase = "paused"
				s.Epoch++
				s.ResumeVotes = make([]bool, len(s.Players))
				s.note("所有参与者同意暂停，计划与剩余时间已保存。")
			}
		} else {
			if s.Phase != "paused" {
				return m, errors.New("对局未暂停")
			}
			s.ResumeVotes[seat] = *a.Enabled
			if s.allVotes(s.ResumeVotes, c.Automated) {
				s.Phase = "planning"
				s.Epoch++
				for i := range s.Players {
					if !s.Players[i].Eliminated {
						s.Players[i].Deadline = c.Now.UnixMilli() + s.Remaining[i]
					}
				}
				s.PauseVotes = make([]bool, len(s.Players))
				s.ResumeVotes = make([]bool, len(s.Players))
				s.note("对局继续，沿用暂停前的行动时间。")
			}
		}
	case "submit", "unsubmit", "resign", "undo":
		if s.Phase != "planning" || hasOrderFields(a) || a.Enabled != nil {
			return m, errors.New("当前不能修改计划")
		}
		if a.Type != "undo" && a.ID != 0 {
			return m, errors.New("无效指令字段")
		}
		switch a.Type {
		case "submit":
			if p.Submitted {
				return m, errors.New("本轮已提交")
			}
			p.Submitted = true
			if s.allSubmitted() {
				s.beginResolution(c.Now)
			}
		case "unsubmit":
			if !p.Submitted {
				return m, errors.New("尚未提交")
			}
			p.Submitted = false
		case "resign":
			s.resign(seat)
			s.checkVictory()
			if s.Phase != "finished" && s.allSubmitted() {
				s.beginResolution(c.Now)
			}
		case "undo":
			if p.Submitted {
				return m, errors.New("请先撤回提交")
			}
			found := false
			kept := []Order{}
			for _, o := range s.Plans[seat] {
				if o.ID == a.ID {
					found = true
				} else {
					kept = append(kept, o)
				}
			}
			if !found {
				return m, errors.New("没有这个指令")
			}
			valid := []Order{}
			for _, o := range sortedOrders(kept) {
				candidate := append(append([]Order(nil), valid...), o)
				if _, _, _, _, e := s.project(seat, candidate); e == nil {
					valid = candidate
				}
			}
			sort.Slice(valid, func(i, j int) bool { return valid[i].ID < valid[j].ID })
			s.Plans[seat] = valid
		}
	default:
		if s.Phase != "planning" || p.Submitted {
			return m, errors.New("请先撤回提交再规划")
		}
		if len(s.Plans[seat]) >= maxOrders {
			return m, errors.New("本轮指令数量已达上限")
		}
		if s.NextOrder[seat] >= 100000 {
			return m, errors.New("本轮计划编辑次数已达上限，请提交当前计划")
		}
		o, e := actionOrder(a, s.NextOrder[seat]+1)
		if e != nil {
			return m, e
		}
		candidate := append(append([]Order(nil), s.Plans[seat]...), o)
		if _, _, _, _, e = s.project(seat, candidate); e != nil {
			return m, e
		}
		s.NextOrder[seat]++
		s.Plans[seat] = candidate
	}
	return s.envelope(), nil
}
func (s *State) allVotes(votes, automated []bool) bool {
	for i, p := range s.Players {
		if !p.Eliminated && !votes[i] && !(i < len(automated) && automated[i]) {
			return false
		}
	}
	return true
}
func (s *State) allSubmitted() bool {
	for _, p := range s.Players {
		if !p.Eliminated && !p.Submitted {
			return false
		}
	}
	return true
}
func (s *State) resign(seat int) {
	for i := range s.Provinces {
		if s.Provinces[i].Owner == seat {
			s.Provinces[i].Owner = -1
		}
	}
	p := &s.Players[seat]
	p.Eliminated = true
	p.King = ""
	p.LostRound = s.Round
	p.Deadline = 0
	p.Submitted = true
	s.Plans[seat] = nil
	s.note(fmt.Sprintf("势力%s退出争霸，其领地恢复中立。", p.Color))
	s.refresh()
}
func (s *State) checkVictory() {
	alive := []int{}
	for i, p := range s.Players {
		if !p.Eliminated {
			alive = append(alive, i)
		}
	}
	if len(alive) == 1 {
		s.Winner = alive[0]
		s.Phase = "finished"
		s.Epoch++
		s.ResolveAt = 0
		s.Pending = nil
		s.Cursor = 0
		for i := range s.Players {
			s.Players[i].Deadline = 0
		}
		s.note("最后一个存活势力统一天下。")
	}
}
