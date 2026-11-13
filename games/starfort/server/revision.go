package starfort

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

// RevisionRules is independent of weather/geology: archived saves retain their
// original rules, RNG streams and deadlines when opened by a newer server.
func (s *State) revised() bool { return s.RevisionRules == 1 }

func (s *State) revisionStats(t Tower) (damage, radius float64, interval int) {
	plain := t
	plain.Branch = ""
	damage, radius, _ = stats(plain)
	c, _ := spec(t.Kind)
	if t.Kind == "arrow" {
		radius -= c.Range - 120
	}
	if t.Kind == "cannon" {
		radius -= c.Range - 200
	}
	rate := 1 + .04*float64(t.Level-1)
	switch t.Kind + "/" + t.Branch {
	case "arrow/volley", "arrow/heart":
		damage *= .5
		if t.Branch == "heart" && s.World.Event.ID == "fog" {
			damage *= 1.1
		}
	case "cannon/speed":
		rate *= .8
	case "frost/freeze":
		damage *= .6
		rate *= .5
	case "frost/speed":
		damage *= .4
		rate *= .8
	case "arcane/power":
		rate *= .8
	}
	if s.World.Event.ID == "starlight" && t.Kind != "arcane" {
		rate *= .9
	}
	return damage, radius, max(StepMS, int(float64(c.Interval)/rate))
}

func (s *State) revisionChoices(t Tower) []BranchChoice {
	l := max(4, t.Level)
	switch t.Kind {
	case "cannon":
		g := s.cannonArea(l)
		chance, cd, work := s.craterArea(l)
		return []BranchChoice{
			{"power", "威力专精", fmt.Sprintf("伤害 ×1.0；%d%% 概率在主目标半径 %.0f 内追加 %.0f%% 伤害；精英／首领完全有效；填坑中的敌人额外受伤 50%%；升级依次增加概率 2、半径 10、伤害 2 个百分点", g.Chance, g.Radius, g.Damage*100)},
			{"speed", "阻滞专精", fmt.Sprintf("攻速 ×0.8；%d%% 概率生成工作量 %.0f 的弹坑，冷却 %.1f 秒；同塔同敌人保护 4 秒，精英／首领填坑减半；5 的倍数等级只加工作量 1，其余升级交替增加概率 2、减少冷却 0.5 秒；地震区新弹坑工作量增加 30%%", chance, work, float64(cd)/1000)},
		}
	case "arrow":
		g := s.volleyArea(l)
		chance, percent := s.heartArea(l)
		return []BranchChoice{
			{"volley", "风羽箭雨", fmt.Sprintf("伤害 ×0.5；%d%% 概率对主目标半径 %.0f 内最多 %d 个其他敌人追加 %.0f%% 伤害，箭雨持续 2 秒；独立 5%% 概率击退存活主目标 %.0f，精英／首领减半，同塔保护 5 秒；升级依次增加概率 1、半径 20、伤害 2 个百分点，10 的倍数等级只加目标 1", g.Chance, g.Radius, g.Targets, g.Damage*100, s.volleyPush())},
			{"heart", "一箭残废", fmt.Sprintf("伤害 ×0.5；%d%% 概率追加目标剩余生命 %.0f%% 的伤害，同塔同敌人保护 5 秒；升级交替增加伤害 1、概率 1 个百分点；薄雾下本塔伤害增加 10%%", chance, percent*100)},
		}
	case "frost":
		duration, cd := s.freezeTiming(l)
		preview := t
		preview.Level = l
		preview.Branch = "speed"
		factor, slow := s.iceAttack(preview)
		return []BranchChoice{
			{"freeze", "冰封专精", fmt.Sprintf("伤害 ×0.6，攻速 ×0.5；%d%% 概率冰封射程内敌人 %.1f 秒，冷却 %.1f 秒；随后 8 秒逐渐恢复速度，同塔同敌人保护 8 秒；精英／首领完全有效；升级依次增加概率 1、时长 0.5 秒、减少冷却 0.1 秒；寒潮时长额外增加 2 秒、冷却减少 2 秒；湿润额外冰封 1 秒", s.freezeChance(l), float64(duration)/1000, float64(cd)/1000)},
			{"speed", "攻速专精", fmt.Sprintf("伤害 ×0.4，攻速 ×0.8；命中减速 %.0f%%，持续 %.1f 秒，精英／首领完全有效；升级交替延长 0.5 秒、增加减速 2 个百分点；寒潮额外减速 15、湿润额外减速 20 个百分点", (1-factor)*100, float64(slow)*StepMS/1000)},
		}
	case "arcane":
		radius, percent := s.meteorArea(l)
		return []BranchChoice{
			{"power", "威力专精", fmt.Sprintf("攻速 ×0.8；无视护甲、穿透护盾直接扣生命；有护甲或护盾时基础伤害 ×1.2；有护甲的普通敌人额外 ×%.1f，精英／首领额外 ×%.1f，每级各加 0.1；星辉伤害 ×1.2", 1.2+.1*float64(l-1), 1.1+.1*float64(l-1))},
			{"magic", "魔法专精", fmt.Sprintf("无视护甲、穿透护盾直接扣生命；有护甲或护盾时基础伤害 ×1.2；%d%% 概率对主目标半径 %.0f 与塔射程交集内敌人追加 %.0f%% 伤害并沉默 3 秒，流星雨持续 3 秒；精英／首领另受 100%% 伤害；冷却 5 秒，同塔同敌人保护 5 秒；升级交替增加半径 10、伤害 2 个百分点", s.meteorChance(), radius, percent*100)},
		}
	}
	return nil
}

func (s *State) rewardFor(owner int, kind string) *PadReward {
	for _, r := range s.World.PadRewards {
		if r.Owner == owner && (kind == "" || r.Kind == kind) {
			if r.Kind == "add" && len(s.World.Map.Pads) >= 40 {
				r.Kind = "move"
			}
			return &r
		}
	}
	return nil
}
func (s *State) addPadReward(owner int, kind string) {
	for i := range s.World.PadRewards {
		r := &s.World.PadRewards[i]
		if r.Owner == owner && r.Kind == kind {
			r.Count = min(maxValue, r.Count+1)
			return
		}
	}
	s.World.PadRewards = append(s.World.PadRewards, PadReward{Owner: owner, Kind: kind, Count: 1})
}
func (s *State) claimRevisionReward(owner int, p Point, pad, tower int, kind string) error {
	r := s.rewardFor(owner, kind)
	sourceKind := kind
	if sourceKind == "" {
		for _, q := range s.World.PadRewards {
			if q.Owner == owner {
				sourceKind = q.Kind
				break
			}
		}
	}
	if r == nil {
		return errors.New("当前没有可领取的奖励")
	}
	if r.Kind != "highground" && roadDistance(s.World.Map, p) < 45 {
		return errors.New("放置位置不能位于道路上")
	}
	// Reuse the archived placement validation, then debit one saved bag item.
	s.World.PadReward = r
	err := s.claimPadReward(owner, p, pad, tower)
	s.World.PadReward = nil
	if err != nil {
		return err
	}
	for i := range s.World.PadRewards {
		q := &s.World.PadRewards[i]
		if q.Owner == owner && q.Kind == sourceKind {
			q.Count--
			if q.Count == 0 {
				s.World.PadRewards = slices.Delete(s.World.PadRewards, i, i+1)
			}
			break
		}
	}
	return nil
}
func (s *State) automaticRevisionReward(owner int) *Action {
	copy := *s
	copy.World.PadReward = s.rewardFor(owner, "")
	return copy.automaticPadReward()
}
func (s *State) useInvulnerability(owner int) error {
	if !s.revised() || owner < 0 || owner >= len(s.Players) || (s.Phase != "prepare" && s.Phase != "battle") || s.InvulnerabilityPacks != 1 {
		return errors.New("当前没有可使用的星核无敌道具")
	}
	s.InvulnerabilityPacks = 0
	s.InvulnerableUntil = s.Tick + 10000/StepMS
	s.event(Event{Type: "objective", Kind: "invulnerability", Owner: owner, X: s.World.Map.Base.X, Y: s.World.Map.Base.Y})
	return nil
}
func (s *State) validRevision() error {
	if s.RevisionRules < 0 || s.RevisionRules > 1 || s.revised() && !s.geological() || s.InvulnerabilityPacks < 0 || s.InvulnerabilityPacks > 1 || s.InvulnerableUntil < 0 || (!s.revised() && (s.InvulnerabilityPacks != 0 || s.InvulnerableUntil != 0 || len(s.World.PadRewards) > 0)) {
		return errors.New("无效修订规则或无敌状态")
	}
	if len(s.World.PadRewards) > 12 || s.revised() && s.World.PadReward != nil {
		return errors.New("无效待领奖励")
	}
	seen := map[string]bool{}
	for _, r := range s.World.PadRewards {
		key := fmt.Sprintf("%d/%s", r.Owner, r.Kind)
		if r.Owner < 0 || r.Owner >= len(s.Players) || !slices.Contains([]string{"add", "highground", "move"}, r.Kind) || r.Count < 1 || r.Count > maxValue || seen[key] {
			return errors.New("无效累计奖励")
		}
		seen[key] = true
	}
	for _, e := range s.Enemies {
		if e.WetUntil < 0 || !s.revised() && e.WetUntil != 0 {
			return errors.New("无效湿润期限")
		}
	}
	return nil
}

// A common edge front reveals fixed rectangles. Fixed, disjoint footprints
// avoid overlapping floods during entry and preserve a clear core corridor.
func (s *State) revisionFloods(g *Geology) {
	m := s.World.Map
	count := 2 + s.geoRandom(5)
	bottom := s.geoRandom(2) == 1
	for pass := 0; pass < 32; pass++ {
		g.Zones = nil
		for tries := 0; tries < 512 && len(g.Zones) < count; tries++ {
			w := float64(200 + s.geoRandom(max(1, m.Width-199)))
			h := float64(100 + s.geoRandom(min(301, m.Height-99)))
			z := GeoZone{ID: len(g.Zones) + 1, Kind: "flood", X: float64(s.geoRandom(max(1, m.Width-int(w)+1))), Y: float64(s.geoRandom(max(1, m.Height-int(h)+1))), Width: w, Height: h, CreatedMS: g.TriggerMS, ExpiresMS: g.TriggerMS + int64(15000+s.geoRandom(126)*200), FromBottom: bottom, Sweep: true}
			dx := math.Max(z.X, math.Min(m.Base.X, z.X+w)) - m.Base.X
			dy := math.Max(z.Y, math.Min(m.Base.Y, z.Y+h)) - m.Base.Y
			if math.Hypot(dx, dy) < 60 || slices.ContainsFunc(g.Zones, func(q GeoZone) bool { return z.X < q.X+q.Width && z.X+w > q.X && z.Y < q.Y+q.Height && z.Y+h > q.Y }) {
				continue
			}
			g.Zones = append(g.Zones, z)
		}
		if len(g.Zones) == count {
			return
		}
	}
	// Small disjoint cells always fit the minimum map, even when random large
	// rectangles cannot pack the requested count. Each remains a valid flood.
	g.Zones = nil
	for y := 0.; y <= float64(m.Height)-100 && len(g.Zones) < count; y += 110 {
		for x := 0.; x <= float64(m.Width)-200 && len(g.Zones) < count; x += 210 {
			if x < m.Base.X+60 && x+200 > m.Base.X-60 && y < m.Base.Y+60 && y+100 > m.Base.Y-60 {
				continue
			}
			g.Zones = append(g.Zones, GeoZone{ID: len(g.Zones) + 1, Kind: "flood", X: x, Y: y, Width: 200, Height: 100, CreatedMS: g.TriggerMS, ExpiresMS: g.TriggerMS + int64(15000+s.geoRandom(126)*200), FromBottom: bottom, Sweep: true})
		}
	}
}

func (s *State) geoZoneLimit() int {
	if s.revised() {
		return 20
	}
	return 16
}
func (s *State) typhoonMaxRadius() float64 {
	if s.revised() {
		return 175
	}
	return 75
}
func (s *State) typhoonMinSpeed() float64 {
	if s.revised() {
		return 20
	}
	return 40
}

func (s *State) maxCraterWork() float64 {
	if s.revised() {
		return (5 + float64(maxValue)/5) * 1.3
	}
	return 5 + maxValue/10
}
