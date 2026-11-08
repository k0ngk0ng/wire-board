package starfort

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

func (s *State) growing() bool { return s.GrowthRules == 1 }

type areaGrowth struct {
	Chance         int
	Radius, Damage float64
	Targets        int
}

func cannonGrowth(level int) areaGrowth {
	level = max(1, level)
	return areaGrowth{Chance: min(100, 10+2*((level+1)/3)), Radius: 100 + 2*float64(level/3), Damage: .2 + .02*float64((level-1)/3)}
}
func volleyGrowth(level int) areaGrowth {
	level = max(1, level)
	n := level - 1 - level/10
	return areaGrowth{Chance: min(100, 10+2*((n+1)/2)), Radius: 80 + 10*float64(level/10-level/20), Damage: .5 + .02*float64(n/2), Targets: 3 + level/20}
}
func craterGrowth(level int) (chance int, cooldown int64, work float64) {
	level = max(1, level)
	n := level - 1 - level/20
	return min(100, 10+(n+1)/2), int64(max(StepMS, (20-n/2)*1000)), 5 + float64(level/20)
}

// Both branch previews and the saved tower use actual tower level, including
// the level-four upgrade that grants the specialization.
func (s *State) growthDescription(t Tower, fallback string) string {
	switch t.Kind + "/" + t.Branch {
	case "cannon/power":
		g := cannonGrowth(t.Level)
		return fmt.Sprintf("伤害 ×1.5；%d%% 概率在主目标半径 %.0f 内追加 %.0f%% 伤害，精英／首领有效；升级依次增加概率、半径、伤害", g.Chance, g.Radius, g.Damage*100)
	case "cannon/speed":
		chance, cd, work := craterGrowth(t.Level)
		return fmt.Sprintf("伤害 ×0.5，攻速 ×1.0；%d%% 概率生成工作量 %.0f 的弹坑，冷却 %.1f 秒；同一敌人 4 秒内不受同塔其他弹坑影响，精英／首领填坑时间为普通单位的 25%%；20 的倍数等级只增加工作量，其余升级交替增加概率、减少冷却", chance, work, float64(cd)/1000)
	case "arrow/volley":
		g := volleyGrowth(t.Level)
		return fmt.Sprintf("攻速 ×0.7；%d%% 概率对主目标半径 %.0f 内最多 %d 个其他敌人造成 %.0f%% 伤害；独立 15%% 概率击退存活主目标 30，精英／首领减半，同塔 8 秒内再次击退再减半；升级交替增加概率、伤害，10 的倍数等级只增加半径、20 的倍数等级只增加目标数", g.Chance, g.Radius, g.Targets, g.Damage*100)
	case "arcane/power":
		return fmt.Sprintf("伤害 ×1.2；忽略护甲，基础破盾 ×1.5；普通敌人额外破盾 ×%.1f，每级增加 0.1；精英／首领额外破盾固定 ×1.1", 1.2+.1*float64(max(1, t.Level)-1))
	}
	return fallback
}

func (s *State) nearbyTargets(target Enemy, radius float64, count int) []int {
	indices := []int{}
	for i, e := range s.Enemies {
		if e.ID != target.ID && e.HP > 0 && math.Hypot(e.X-target.X, e.Y-target.Y) <= radius {
			indices = append(indices, i)
		}
	}
	slices.SortFunc(indices, func(a, b int) int {
		ea, eb := s.Enemies[a], s.Enemies[b]
		da, db := math.Hypot(ea.X-target.X, ea.Y-target.Y), math.Hypot(eb.X-target.X, eb.Y-target.Y)
		if da < db {
			return -1
		}
		if da > db {
			return 1
		}
		return ea.ID - eb.ID
	})
	return indices[:min(count, len(indices))]
}

func (s *State) freshGuards(guards []KnockbackGuard) []KnockbackGuard {
	return slices.DeleteFunc(guards, func(g KnockbackGuard) bool {
		return g.Until <= s.Tick || !slices.ContainsFunc(s.Towers, func(t Tower) bool { return t.ID == g.TowerID })
	})
}
func hasGuard(guards []KnockbackGuard, tower int, tick int64) bool {
	return slices.ContainsFunc(guards, func(g KnockbackGuard) bool { return g.TowerID == tower && g.Until > tick })
}
func putGuard(guards []KnockbackGuard, tower int, until int64) []KnockbackGuard {
	for i := range guards {
		if guards[i].TowerID == tower {
			guards[i].Until = until
			return guards
		}
	}
	return append(guards, KnockbackGuard{TowerID: tower, Until: until})
}

type RepairControl struct {
	Limit     int  `json:"limit"`
	Remaining int  `json:"remaining"`
	MaxAmount int  `json:"maxAmount"`
	UnitCost  int  `json:"unitCost"`
	Enabled   bool `json:"enabled"`
}

func (s *State) repairControl(player int) *RepairControl {
	if !s.growing() {
		return nil
	}
	c := &RepairControl{Limit: s.BaseMaxHP / 10, UnitCost: 100}
	c.Remaining = c.Limit
	if s.RepairWave == s.Wave {
		c.Remaining -= s.RepairedHP
	}
	if player >= 0 && player < len(s.Players) && s.Phase == "prepare" && s.World.Choice == nil && s.World.BoonChoice == nil {
		c.MaxAmount = min(c.Remaining, s.BaseMaxHP-s.BaseHP, s.Players[player].Gold/c.UnitCost)
		c.Enabled = c.MaxAmount > 0
	}
	return c
}
func (s *State) repairBase(player, amount int) error {
	c := s.repairControl(player)
	if c == nil || !c.Enabled || amount < 1 || amount > c.MaxAmount {
		return errors.New("当前无法修复：请检查波前阶段、缺失生命、金币及本波剩余额度")
	}
	if s.RepairWave != s.Wave {
		s.RepairWave = s.Wave
		s.RepairedHP = 0
	}
	s.Players[player].Gold -= amount * c.UnitCost
	s.BaseHP += amount
	s.RepairedHP += amount
	p := s.World.Map.Base
	s.event(Event{Type: "objective", Kind: "repair_base", Owner: player, X: p.X, Y: p.Y, Amount: float64(amount)})
	s.note("玩家 %d 花费 %d 金币修复星核 %d 点生命", player+1, amount*c.UnitCost, amount)
	return nil
}

func (s *State) validGrowth() error {
	if s.GrowthRules < 0 || s.GrowthRules > 1 || (s.growing() && !s.rebalanced()) || s.RepairWave < 0 || s.RepairWave > s.Wave || s.RepairedHP < 0 || s.RepairedHP > s.BaseMaxHP/10 || (s.RepairWave == 0 && s.RepairedHP != 0) || (!s.growing() && (s.RepairWave != 0 || s.RepairedHP != 0)) {
		return errors.New("无效成长规则或修复额度")
	}
	for _, e := range s.Enemies {
		if len(e.CraterGuards) > 40 || (!s.growing() && len(e.CraterGuards) > 0) {
			return errors.New("无效弹坑保护")
		}
		ids := map[int]bool{}
		for _, g := range e.CraterGuards {
			if g.TowerID < 1 || g.TowerID > s.TowerSerial || ids[g.TowerID] || g.Until < 0 {
				return errors.New("无效弹坑保护")
			}
			ids[g.TowerID] = true
		}
	}
	return nil
}
