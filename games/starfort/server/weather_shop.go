package starfort

import (
	"errors"
	"slices"
)

type MerchantShop struct {
	Wave         int   `json:"wave"`
	RepairUsed   int   `json:"repairUsed"`
	DamageTowers []int `json:"damageTowers"`
	RangeTowers  []int `json:"rangeTowers"`
	ShieldBought bool  `json:"shieldBought"`
}
type ShopTowerControl struct {
	TowerID       int  `json:"towerId"`
	DamageEnabled bool `json:"damageEnabled"`
	RangeEnabled  bool `json:"rangeEnabled"`
}
type ShopControl struct {
	Wave            int                `json:"wave"`
	Limit           int                `json:"limit"`
	Enabled         bool               `json:"enabled"`
	RepairRemaining int                `json:"repairRemaining"`
	RepairMax       int                `json:"repairMax"`
	DamageRemaining int                `json:"damageRemaining"`
	RangeRemaining  int                `json:"rangeRemaining"`
	ShieldEnabled   bool               `json:"shieldEnabled"`
	Towers          []ShopTowerControl `json:"towers"`
}

func (s *State) shopControl(player int) *ShopControl {
	shop := s.World.Shop
	if !s.weathered() || shop == nil {
		return nil
	}
	c := &ShopControl{Wave: shop.Wave, Limit: shop.Wave / 10, Towers: []ShopTowerControl{}}
	c.RepairRemaining = c.Limit - shop.RepairUsed
	c.DamageRemaining = c.Limit - len(shop.DamageTowers)
	c.RangeRemaining = c.Limit - len(shop.RangeTowers)
	c.Enabled = player >= 0 && player < len(s.Players) && s.Phase == "prepare" && s.World.Choice == nil && s.World.BoonChoice == nil
	if c.Enabled {
		gold := s.Players[player].Gold
		c.RepairMax = min(c.RepairRemaining, s.BaseMaxHP-s.BaseHP, gold/100)
		c.ShieldEnabled = !shop.ShieldBought && gold >= 1000
		for _, t := range s.Towers {
			if t.Owner == player {
				c.Towers = append(c.Towers, ShopTowerControl{TowerID: t.ID, DamageEnabled: gold >= 100 && c.DamageRemaining > 0 && !slices.Contains(shop.DamageTowers, t.ID), RangeEnabled: gold >= 100 && c.RangeRemaining > 0 && !slices.Contains(shop.RangeTowers, t.ID)})
			}
		}
	}
	return c
}
func (s *State) buyShop(player int, a Action) error {
	c := s.shopControl(player)
	if c == nil || !c.Enabled {
		return errors.New("商店仅在商人护送成功当波结束后的整备阶段开放")
	}
	shop := s.World.Shop
	cost := 100
	switch a.Choice {
	case "repair":
		if a.Amount < 1 || a.Amount > c.RepairMax {
			return errors.New("修复数量超过缺失生命、金币或商店剩余额度")
		}
		cost *= a.Amount
		s.BaseHP += a.Amount
		shop.RepairUsed += a.Amount
	case "damage", "range":
		index := -1
		for i, t := range s.Towers {
			if t.ID == a.TowerID && t.Owner == player {
				index = i
			}
		}
		if index < 0 {
			return errors.New("请选择自己的防御塔")
		}
		permitted := false
		for _, t := range c.Towers {
			if t.TowerID == a.TowerID {
				permitted = (a.Choice == "damage" && t.DamageEnabled) || (a.Choice == "range" && t.RangeEnabled)
			}
		}
		if !permitted {
			return errors.New("金币或商店额度不足，或本塔已购买该强化")
		}
		if a.Choice == "damage" {
			s.Towers[index].ShopDamage++
			shop.DamageTowers = append(shop.DamageTowers, a.TowerID)
		} else {
			s.Towers[index].ShopRange++
			shop.RangeTowers = append(shop.RangeTowers, a.TowerID)
		}
	case "shield":
		if !c.ShieldEnabled {
			return errors.New("金币不足或本店的星核庇护已售出")
		}
		cost = 1000
		shop.ShieldBought = true
		s.World.BossWards++
	default:
		return errors.New("未知商店商品")
	}
	s.Players[player].Gold -= cost
	s.event(Event{Type: "objective", Kind: "shop_" + a.Choice, Owner: player, X: s.World.Map.Base.X, Y: s.World.Map.Base.Y, Amount: float64(cost)})
	s.note("玩家 %d 在商店花费 %d 金币", player+1, cost)
	return nil
}
func (s *State) finishCampaign() {
	s.failActiveObjectives("战役结束前未完成护送")
	s.Phase = "won"
	s.Epoch++
	s.NextAt = 0
	p := s.World.Map.Base
	s.event(Event{Type: "win", X: p.X, Y: p.Y, Owner: -1})
	s.note("全队获胜，星光长明")
}
func (s *State) closeShop(now int64) error {
	if !s.weathered() || s.World.Shop == nil || s.Phase != "prepare" {
		return errors.New("当前没有可关闭的商店")
	}
	s.World.Shop = nil
	if s.Options.Mode == "campaign" && s.CompletedWaves >= s.Options.Waves {
		s.finishCampaign()
	} else {
		s.ServerAt = now
	}
	return nil
}
func (s *State) validShop() error {
	w := s.World
	if w.MerchantWave < 0 || w.MerchantWave > s.Wave || w.BossWards < 0 || w.BossWards > maxValue || !finite(w.BeaconPower) || !finite(w.BeaconReach) || w.BeaconPower < 0 || w.BeaconReach < 0 || (!s.weathered() && (w.Shop != nil || w.MerchantWave != 0 || w.BossWards != 0 || w.BeaconPower != 0 || w.BeaconReach != 0)) {
		return errors.New("无效商人或瞭望塔成长")
	}
	if shop := w.Shop; shop != nil {
		limit := shop.Wave / 10
		if shop.Wave < 10 || shop.Wave > s.CompletedWaves || shop.Wave != s.CompletedWaves || shop.RepairUsed < 0 || shop.RepairUsed > limit || len(shop.DamageTowers) > limit || len(shop.RangeTowers) > limit || s.Phase == "battle" || s.Phase == "lost" || s.Phase == "won" {
			return errors.New("无效商店状态")
		}
		for _, ids := range [][]int{shop.DamageTowers, shop.RangeTowers} {
			seen := map[int]bool{}
			for _, id := range ids {
				if id < 1 || id > s.TowerSerial || seen[id] {
					return errors.New("无效商店强化记录")
				}
				seen[id] = true
			}
		}
	}
	for _, o := range w.Objectives {
		if len(o.Attacked) > 256 || (!s.weathered() && len(o.Attacked) > 0) {
			return errors.New("无效任务攻击记录")
		}
		seen := map[int]bool{}
		for _, id := range o.Attacked {
			if id < 1 || id > s.EnemySerial || seen[id] {
				return errors.New("无效任务攻击记录")
			}
			seen[id] = true
		}
	}
	return nil
}
