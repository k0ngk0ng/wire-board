package conquest

import (
	"errors"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Validate queued commands structurally, without requiring their source still
// belongs to their author: an earlier attack can legitimately cancel a later
// queued movement. Never trust persisted indices before indexing a slice.
func validateSavedOrder(o Order, g *geography) error {
	if o.ID < 1 || o.Amount < 0 || o.Amount > maxTroops {
		return errors.New("无效存档指令数值")
	}
	a := Action{Type: o.Type, From: o.From, To: o.To, Province: o.Province, Amount: o.Amount, King: o.King, Building: o.Building}
	if o.Type == "tax" {
		value := o.Value
		a.Value = &value
	} else if o.Value != 0 {
		return errors.New("存档指令包含多余值")
	}
	if _, e := actionOrder(a, o.ID); e != nil {
		return e
	}
	switch o.Type {
	case "tax":
		if o.Value < 0 || o.Value > 100 || o.Value%10 != 0 {
			return errors.New("无效存档税率")
		}
	case "move":
		if ok, _ := route(g, o.From, o.To); !ok || o.Amount == 0 {
			return errors.New("无效存档行军")
		}
	case "recruit", "disband":
		if _, ok := g.index[o.Province]; !ok || o.Amount == 0 {
			return errors.New("无效存档征兵")
		}
	case "build":
		if _, ok := g.index[o.Province]; !ok {
			return errors.New("无效存档建造地区")
		}
		if _, _, ok := buildCost(o.Building); !ok {
			return errors.New("无效存档建筑")
		}
	default:
		return errors.New("未知存档指令")
	}
	return nil
}
func (s *State) validateExecution(m gameapi.Match, g *geography) error {
	if (s.Phase == "paused") != m.Paused {
		return errors.New("暂停状态与存档信封不一致")
	}
	if (s.Phase == "finished") != (m.Result != nil) {
		return errors.New("结算状态与存档信封不一致")
	}
	alive := 0
	for seat, p := range s.Players {
		if s.NextOrder[seat] < 0 || s.NextOrder[seat] > 100000 || s.Remaining[seat] < 0 || s.Remaining[seat] > int64(s.Options.TurnSeconds)*1000 || p.EconomyRate < 0 || p.EconomyRate > 100 || p.MilitaryRate < 0 || p.MilitaryRate > 100 || p.Tax%10 != 0 {
			return errors.New("无效存档资源或时间")
		}
		if !p.Eliminated {
			alive++
			if s.Phase == "planning" && p.Deadline <= 0 {
				return errors.New("存档缺少行动时限")
			}
		} else {
			if p.King != "" || p.Deadline != 0 {
				return errors.New("出局势力仍有君主或时限")
			}
			for _, province := range s.Provinces {
				if province.Owner == seat {
					return errors.New("出局势力仍有领土")
				}
			}
		}
		seen := map[int]bool{}
		for _, o := range s.Plans[seat] {
			if seen[o.ID] {
				return errors.New("重复存档指令")
			}
			seen[o.ID] = true
			if e := validateSavedOrder(o, g); e != nil {
				return e
			}
		}
	}
	if s.Phase == "finished" {
		if alive != 1 || s.Players[s.Winner].Eliminated || len(m.Result.Winners) != 1 || m.Result.Winners[0] != s.Winner {
			return errors.New("存档胜利结果不一致")
		}
	}
	if s.Phase == "resolving" {
		if s.Cursor >= len(s.Pending) || s.ResolveAt <= 0 {
			return errors.New("待执行队列没有当前事件")
		}
		seen := map[string]bool{}
		for _, q := range s.Pending {
			if q.Seat < 0 || q.Seat >= len(s.Players) {
				return errors.New("待执行队列包含无效席位")
			}
			if q.Order.ID > s.NextOrder[q.Seat] {
				return errors.New("待执行队列引用未来指令")
			}
			key := fmt.Sprintf("%d/%d", q.Seat, q.Order.ID)
			if seen[key] {
				return errors.New("待执行队列重复执行指令")
			}
			seen[key] = true
			if e := validateSavedOrder(q.Order, g); e != nil {
				return e
			}
		}
		if s.Pending[s.Cursor].Order.Type != "move" {
			return errors.New("结算计时事件不是行军")
		}
	} else if len(s.Pending) != 0 || s.Cursor != 0 || s.ResolveAt != 0 {
		return errors.New("非结算阶段仍有待执行队列")
	}
	previous := 0
	for _, e := range s.Events {
		if e.ID <= previous || e.ID > s.NextEvent || e.Seat < 0 || e.Seat >= len(s.Players) || e.At < 0 || e.Until < e.At {
			return errors.New("无效存档事件时间线")
		}
		previous = e.ID
	}
	return nil
}

// Planning snapshots must be usable immediately after restart. Structural
// queue validation alone cannot detect a stale owner or overcommitted budget.
func (s *State) validateRestoredPlans() error {
	if s.Phase != "planning" && s.Phase != "paused" {
		return nil
	}
	for seat, p := range s.Players {
		if p.Eliminated {
			continue
		}
		if _, _, _, _, err := s.project(seat, s.Plans[seat]); err != nil {
			return fmt.Errorf("无法恢复势力计划: %w", err)
		}
	}
	return nil
}
