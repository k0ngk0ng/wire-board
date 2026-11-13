package starfort

import (
	"fmt"
	"math"
	"slices"
)

type KnockbackGuard struct {
	TowerID int   `json:"towerId"`
	Until   int64 `json:"until"`
}

func (s *State) rebalanced() bool { return s.BalanceRules == 1 }

func (s *State) balancedStats(t Tower) (float64, float64, int) {
	plain := t
	plain.Branch = ""
	damage, radius, _ := stats(plain)
	c, _ := spec(t.Kind)
	rate := 1 + .04*float64(t.Level-1)
	switch t.Kind + "/" + t.Branch {
	case "arrow/volley":
		rate *= .7
	case "arrow/heart":
		rate *= 1.7
	case "cannon/power":
		damage *= 1.5
	case "cannon/speed":
		damage *= .5
		if !s.growing() {
			rate *= 1.5
		}
	case "frost/freeze":
		rate *= .5
	case "frost/speed":
		rate *= 1.5
	case "arcane/power":
		damage *= 1.2
	}
	return damage, radius, max(StepMS, int(float64(c.Interval)/rate))
}

func (s *State) iceAttack(t Tower) (float64, int64) {
	if s.revised() && t.Branch == "speed" {
		n := max(1, t.Level) - 1
		slow := .1 + .02*float64(n/2)
		if s.World.Event.ID == "frost" {
			slow += .15
		}
		return 1 - math.Min(.9, slow), int64(math.Ceil(float64(1000+500*((n+1)/2)) / StepMS))
	}
	if s.weatherV3() && t.Branch == "speed" {
		slow := .1 + .05*float64((max(1, t.Level)-1)/2)
		if s.World.Event.ID == "frost" {
			slow += .15
		}
		duration := int64(math.Ceil(float64(1000+500*(max(1, t.Level)/2)) / StepMS))
		return 1 - math.Min(.9, slow), duration
	}
	slow := math.Min(.9, .1*(1+.2*float64((t.Level-1)/2)))
	if s.World.Event.ID == "frost" && (!s.weathered() || t.Branch == "speed") {
		slow = math.Min(.9, slow+.15)
	}
	duration := int64(1000/StepMS) + int64(t.Level/2)
	return 1 - slow, duration
}

func (s *State) balancedChoices(t Tower) (choices []BranchChoice) {
	if s.weathered() {
		return s.weatherChoices(t)
	}
	defer func() {
		if s.growing() {
			for i := range choices {
				preview := t
				preview.Branch = choices[i].ID
				preview.Level = max(4, t.Level)
				choices[i].Description = s.growthDescription(preview, choices[i].Description)
			}
		}
	}()
	switch t.Kind {
	case "arrow":
		return []BranchChoice{
			{"volley", "风羽箭雨", "攻速 ×0.7；25% 概率对半径 75 内其他敌人造成 70% 伤害；独立 15% 概率击退 40，精英／首领减半且同塔 8 秒内不再击退"},
			{"heart", "一箭穿心", "攻速 ×1.7；12% 概率秒杀等级不高于塔等级两倍的普通敌人，超等级敌人、精英和首领承受 200% 伤害"},
		}
	case "cannon":
		return []BranchChoice{
			{"power", "威力专精", "伤害 ×1.5；20% 概率在主目标半径 110 内追加 75% 伤害"},
			{"speed", "阻滞专精", "伤害 ×0.5，攻速 ×1.5；25% 概率生成工作量 6 的弹坑，冷却 10 秒；精英／首领填坑时间为普通单位的 25%"},
		}
	case "frost":
		return []BranchChoice{
			{"freeze", "冰封专精", "攻速 ×0.5；15% 概率冰封塔射程内敌人 2 秒，随后 10 秒逐渐恢复移动速度；冷却 10 秒，精英／首领完全受影响"},
			{"speed", "攻速专精", "攻速 ×1.5；减速比例及持续时间随等级提高，寒潮额外减速 15 个百分点；无冰墙及永久减速"},
		}
	case "arcane":
		return []BranchChoice{
			{"power", "威力专精", "伤害 ×1.2；忽略护甲，基础破盾 ×1.5；额外破盾普通敌人从 ×1.2、精英／首领从 ×1.1 开始，每级增加 0.1"},
			{"magic", "魔法专精", "每 8 秒星链对附近最多两个敌人造成 50% 伤害；15% 概率对塔射程内敌人追加 100% 魔法伤害并沉默 3 秒，冷却 8 秒"},
		}
	}
	return nil
}

func (s *State) balancedPassive(t Tower) *PassiveView {
	if s.weathered() {
		return s.weatherPassive(t)
	}
	if t.Branch == "" {
		return nil
	}
	p := &PassiveView{ReadyInMS: max(0, t.PassiveReadyAt-s.ElapsedMS)}
	for _, c := range s.balancedChoices(t) {
		if c.ID == t.Branch {
			p.Name, p.Description = c.Name, c.Description
		}
	}
	switch t.Kind + "/" + t.Branch {
	case "arrow/volley":
		p.Variant = "volley"
		p.TriggerChance = 25
	case "arrow/heart":
		p.Variant = "heart"
		p.TriggerChance = 12
		p.ExecuteMaxLevel = t.Level * 2
	case "cannon/power":
		p.Variant = "blast"
		p.TriggerChance = 20
	case "cannon/speed":
		p.Variant = "crater"
		p.TriggerChance = 25
		p.CooldownMS = 10000
	case "frost/freeze":
		p.Variant = "freeze"
		p.TriggerChance = 15
		p.CooldownMS = 10000
	case "frost/speed":
		factor, duration := s.iceAttack(t)
		p.Name = "霜晶减速"
		p.Description = fmt.Sprintf("攻速 ×1.5；命中减速 %.0f%%，持续 %.1f 秒；无冰墙及永久减速", (1-factor)*100, float64(duration)*StepMS/1000)
	case "arcane/power":
		p.Name = "辉光破盾"
	case "arcane/magic":
		p.Variant = "meteor"
		p.TriggerChance = 15
		p.CooldownMS = 8000
	}
	if s.growing() {
		p.Description = s.growthDescription(t, p.Description)
		switch t.Kind + "/" + t.Branch {
		case "arrow/volley":
			p.TriggerChance = volleyGrowth(t.Level).Chance
		case "cannon/power":
			p.TriggerChance = cannonGrowth(t.Level).Chance
		case "cannon/speed":
			chance, cd, _ := craterGrowth(t.Level)
			p.TriggerChance = chance
			p.CooldownMS = cd
		}
	}
	if p.CooldownMS == 0 {
		p.ReadyInMS = 0
	}
	return p
}

func (s *State) balancedHit(t *Tower, target Enemy, damage float64) {
	if s.weathered() {
		s.weatherHit(t, target, damage)
		return
	}
	if t.Branch == "" {
		return
	}
	// Star-chain and meteor have independent cooldowns in the magic branch.
	if t.Kind == "arcane" && t.Branch == "magic" && s.combatNow() >= t.ChainReadyAt {
		t.ChainReadyAt = s.combatNow() + 8000
		indices := []int{}
		for i, e := range s.Enemies {
			if e.ID != target.ID && e.HP > 0 && math.Hypot(e.X-target.X, e.Y-target.Y) <= 125 {
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
		from := Point{target.X, target.Y}
		for _, i := range indices[:min(2, len(indices))] {
			e := s.Enemies[i]
			s.event(Event{Type: "chain", TowerID: t.ID, TargetID: e.ID, X: from.X, Y: from.Y, ToX: e.X, ToY: e.Y, Owner: t.Owner, Kind: "arcane"})
			s.hit(i, damage*.5, *t)
			from = Point{e.X, e.Y}
		}
	}
	s.specializationHit(t, target, damage)
}

// One highground and six pads per new route; existing IDs never change.
func addRoutePads(m *Map, r Route) {
	count := min(6, 40-len(m.Pads))
	if count <= 0 {
		return
	}
	start := len(m.Pads)
	for i := 0; i < count; i++ {
		d := pathDistance(r.Points) * (.24 + .11*float64(i))
		p := pathPosition(r.Points, d)
		q := pathPosition(r.Points, d+1)
		dx, dy := q.X-p.X, q.Y-p.Y
		l := math.Hypot(dx, dy)
		side := 1.
		if i%2 == 1 {
			side = -1
		}
		p.X = math.Max(35, math.Min(float64(m.Width)-35, p.X-dy/l*side*70))
		p.Y = math.Max(35, math.Min(float64(m.Height)-35, p.Y+dx/l*side*70))
		terrain := "plain"
		if i == 0 {
			terrain = "highground"
		}
		m.Pads = append(m.Pads, Pad{ID: len(m.Pads), X: p.X, Y: p.Y, Terrain: terrain, Sector: r.Sector, Route: r.ID})
	}
	m.Pads = placePads(*m, start)
}
