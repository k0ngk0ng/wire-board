package starfort

import (
	"errors"
	"fmt"
	"math"
)

func (s *State) weathered() bool { return s.WeatherRules >= 1 }

func (s *State) weatherFor(wave int) WaveEvent {
	if !s.weathered() {
		return eventFor(wave, s.World.Seed)
	}
	id := []string{"calm", "fog", "frost", "starlight"}[(wave+int(s.World.Seed%4))%4]
	e := WaveEvent{ID: id, Wave: wave}
	switch id {
	case "fog":
		e.Label, e.Description = "薄雾", "防御塔射程 ×0.8；箭雨额外命中 1 个目标、击退增加 10；残废箭伤害增加 5%"
	case "frost":
		e.Label, e.Description = "寒潮", "敌人速度 ×0.85；冰封延长 2 秒、冷却减少 2 秒；攻速冰塔额外减速 15 个百分点"
	case "starlight":
		e.Label, e.Description = "星辉", "威力法师伤害 ×1.2；魔法法师流星雨概率增加 10 个百分点"
	default:
		e.Label, e.Description = "无特殊天气", "本波无额外天气效果"
	}
	if s.weatherV2() {
		switch id {
		case "fog":
			e.Description = "除箭塔外射程减少 10%；敌人每秒 10% 概率倒退 3 秒，触发后 15 秒保护；箭雨多命中 1 个目标、击退增加 10，残废箭伤害增加 5%"
		case "starlight":
			e.Description = "除法师塔外攻速减少 10%；敌人每秒 10% 概率睡眠免伤 3 秒，触发后 15 秒保护；威力法师伤害 ×1.2，流星雨概率增加 10 个百分点"
		case "frost":
			e.Description = "敌速减少 10%；除冰塔外攻击 10% 概率 MISS，同塔同敌人落空后 10 秒内不再 MISS；冰封延长 2 秒、冷却减少 2 秒，攻速冰塔额外减速 15 个百分点"
		}
	}
	if s.weatherV3() {
		switch id {
		case "fog":
			e.Description = "除箭塔外射程减少 10%；敌人每秒 10% 概率倒退 3 秒，触发后 15 秒保护；箭雨多命中 1 个目标、击退增加 10，残废箭伤害增加 10%"
		case "starlight":
			e.Description = "除法师塔外攻速减少 10%；敌人每秒 10% 概率睡眠 3 秒、承受半伤，其他负面效果翻倍，触发后 10 秒保护；威力法师伤害 ×1.2，流星雨概率增加 10 个百分点"
		}
	}
	return e
}

func (s *State) upgradePrice(t Tower) int {
	base := upgradeCost(t)
	if !s.weathered() || t.Level < 10 {
		return base
	}
	extra := .1 + .05*float64(t.Level/10-1)
	return int(math.Min(maxValue, math.Round(float64(base)*(1+extra))))
}
func (s *State) cannonArea(level int) areaGrowth {
	if s.weatherV3() {
		n := max(1, level) - 1
		return areaGrowth{Chance: min(100, 10+5*((n+2)/3)), Radius: 100 + 10*float64((n+1)/3), Damage: .2 + .05*float64(n/3)}
	}
	g := cannonGrowth(level)
	if s.weathered() {
		g.Radius = 100 + 5*float64(max(1, level)/3)
	}
	return g
}
func (s *State) volleyArea(level int) areaGrowth {
	if s.weatherV3() {
		level = max(1, level)
		n := level - 1 - level/10
		g := areaGrowth{Chance: min(100, 10+5*((n+2)/3)), Radius: 200 + 50*float64((n+1)/3), Damage: .3 + .05*float64(n/3), Targets: 3 + level/10}
		if s.World.Event.ID == "fog" {
			g.Targets++
		}
		return g
	}
	if !s.weathered() {
		return volleyGrowth(level)
	}
	level = max(1, level)
	n := level - 1 - level/10
	g := areaGrowth{Chance: min(100, 10+2*((n+2)/3)), Radius: 80 + 5*float64((n+1)/3), Damage: .3 + .03*float64(n/3), Targets: 3 + level/10}
	if s.World.Event.ID == "fog" {
		g.Targets++
	}
	return g
}
func (s *State) craterArea(level int) (int, int64, float64) {
	if s.weatherV3() {
		level = max(1, level)
		n := level - 1 - level/5
		return min(100, 10+5*((n+1)/2)), int64(max(StepMS, (20-2*(n/2))*1000)), 5 + float64(level/5)
	}
	if !s.weathered() {
		return craterGrowth(level)
	}
	level = max(1, level)
	n := level - 1 - level/10
	return min(100, 10+2*((n+1)/2)), int64(max(StepMS, (20-n/2)*1000)), 5 + float64(level/10)
}
func heartArea(level int) (int, float64) {
	n := max(1, level) - 1
	return min(100, 10+(n+1)/2), math.Min(1, .1+.01*float64(n/2))
}
func meteorArea(level int) (float64, float64) {
	n := max(1, level) - 1
	return 30 + 3*float64((n+1)/2), .3 + .03*float64(n/2)
}
func (s *State) freezeTiming(level int) (int64, int64) {
	if s.weatherV3() {
		n := max(1, level) - 1
		duration, cd := int64(2000+1000*((n+1)/3)), int64(10000-2000*(n/3))
		if s.World.Event.ID == "frost" {
			duration += 2000
			cd -= 2000
		}
		return duration, max(StepMS, cd)
	}
	if s.World.Event.ID == "frost" {
		return 4000, 8000
	}
	return 2000, 10000
}
func (s *State) weatherStats(t Tower) (float64, float64, int) {
	plain := t
	plain.Branch = ""
	damage, radius, _ := stats(plain)
	c, _ := spec(t.Kind)
	rate := 1 + .04*float64(t.Level-1)
	switch t.Kind + "/" + t.Branch {
	case "arrow/volley":
		rate *= .7
	case "arrow/heart":
		if s.World.Event.ID == "fog" {
			damage *= s.heartFogBonus()
		}
	case "cannon/power":
		damage *= 1.2
	case "cannon/speed":
		rate *= .8
	case "frost/freeze":
		rate *= .5
	case "frost/speed":
		rate *= 1.5
	case "arcane/power":
		rate *= .8
	}
	if s.weatherV2() && s.World.Event.ID == "starlight" && t.Kind != "arcane" {
		rate *= .9
	}
	return damage, radius, max(StepMS, int(float64(c.Interval)/rate))
}
func (s *State) weatherChoices(t Tower) []BranchChoice {
	if s.weatherV3() {
		return s.weatherV3Choices(t)
	}
	level := max(4, t.Level)
	choices := []BranchChoice{}
	switch t.Kind {
	case "cannon":
		g := s.cannonArea(level)
		chance, cd, work := s.craterArea(level)
		choices = []BranchChoice{{"power", "威力专精", fmt.Sprintf("伤害 ×1.2；%d%% 概率在主目标半径 %.0f 内追加 %.0f%% 伤害；填坑中的敌人额外受到 50%% 伤害；升级依次增加概率、半径、伤害", g.Chance, g.Radius, g.Damage*100)}, {"speed", "阻滞专精", fmt.Sprintf("攻速 ×0.8；%d%% 概率生成工作量 %.0f 的弹坑，冷却 %.1f 秒；同敌人 4 秒内不受同塔其他弹坑影响，精英／星陨巨像填坑时间为 50%%；10 的倍数等级只加工作量，其余升级交替增加概率、减少冷却", chance, work, float64(cd)/1000)}}
	case "arrow":
		g := s.volleyArea(level)
		chance, percent := s.heartArea(level)
		push := 40
		if s.World.Event.ID == "fog" {
			push = 50
		}
		choices = []BranchChoice{{"volley", "风羽箭雨", fmt.Sprintf("攻速 ×0.7；%d%% 概率对主目标半径 %.0f 内最多 %d 个其他敌人造成 %.0f%% 伤害；独立 15%% 概率击退存活主目标 %d，精英／星陨巨像减半，同塔 8 秒免疫；升级依次增加概率、半径、伤害，10 的倍数等级只加目标", g.Chance, g.Radius, g.Targets, g.Damage*100, push)}, {"heart", "一箭残废", fmt.Sprintf("%d%% 概率追加目标剩余生命 %.0f%% 的伤害，同塔对同敌人 5 秒免疫；升级交替增加概率、伤害；薄雾下本塔伤害额外增加 5%%", chance, percent*100)}}
	case "frost":
		duration, cd := s.freezeTiming(level)
		choices = []BranchChoice{{"freeze", "冰封专精", fmt.Sprintf("攻速 ×0.5；15%% 概率冰封射程内敌人 %.0f 秒，冷却 %.0f 秒，随后 8 秒逐渐恢复速度；同塔对同敌人 8 秒免疫，精英／星陨巨像完全有效；寒潮延长冰封 2 秒、冷却减少 2 秒", float64(duration)/1000, float64(cd)/1000)}, {"speed", "攻速专精", "攻速 ×1.5；减速及持续时间随等级提高，寒潮额外减速 15 个百分点；无冰墙及永久减速"}}
	case "arcane":
		radius, percent := s.meteorArea(level)
		chance := 15
		if s.World.Event.ID == "starlight" {
			chance = 25
		}
		choices = []BranchChoice{{"power", "威力专精", fmt.Sprintf("攻速 ×0.8；忽略护甲，有护甲目标基础伤害 ×1.1；有护甲普通敌人额外 ×%.1f，精英／星陨巨像额外 ×%.1f，每级各加 0.1；星辉下本塔伤害 ×1.2", 1.1+.1*float64(level-1), 1+.1*float64(level-1))}, {"magic", "魔法专精", fmt.Sprintf("%d%% 概率对主目标半径 %.0f 与塔射程交集内敌人追加 %.0f%% 无视护甲伤害并沉默 3 秒；精英／星陨巨像另受 50%% 伤害；冷却 10 秒，同塔对同敌人 10 秒免疫；升级交替增加半径、伤害；无星链", chance, radius, percent*100)}}
	}
	return choices
}
func (s *State) weatherPassive(t Tower) *PassiveView {
	if t.Branch == "" {
		return nil
	}
	p := &PassiveView{ReadyInMS: max(0, t.PassiveReadyAt-s.ElapsedMS)}
	for _, c := range s.weatherChoices(t) {
		if c.ID == t.Branch {
			p.Name, p.Description = c.Name, c.Description
		}
	}
	switch t.Kind + "/" + t.Branch {
	case "cannon/power":
		p.Variant = "blast"
		p.TriggerChance = s.cannonArea(t.Level).Chance
	case "cannon/speed":
		p.Variant = "crater"
		p.TriggerChance, p.CooldownMS, _ = s.craterArea(t.Level)
	case "arrow/volley":
		p.Variant = "volley"
		p.TriggerChance = s.volleyArea(t.Level).Chance
	case "arrow/heart":
		p.Variant = "heart"
		p.TriggerChance, _ = s.heartArea(t.Level)
	case "frost/freeze":
		p.Variant = "freeze"
		p.TriggerChance = s.freezeChance(t.Level)
		_, p.CooldownMS = s.freezeTiming(t.Level)
	case "frost/speed":
		factor, duration := s.iceAttack(t)
		p.Description = fmt.Sprintf("攻速 ×1.5；命中减速 %.0f%%，持续 %.1f 秒；寒潮额外减速 15 个百分点；无冰墙及永久减速", (1-factor)*100, float64(duration)*StepMS/1000)
	case "arcane/magic":
		p.Variant = "meteor"
		p.TriggerChance = s.meteorChance()
		p.CooldownMS = s.meteorCooldown()
	}
	if p.CooldownMS == 0 {
		p.ReadyInMS = 0
	}
	return p
}

func (s *State) weatherHit(t *Tower, target Enemy, damage float64) {
	now := s.combatNow()
	switch t.Kind + "/" + t.Branch {
	case "arrow/volley":
		if !s.skillRoll(15) {
			return
		}
		for i := range s.Enemies {
			e := &s.Enemies[i]
			if e.ID != target.ID || e.HP <= 0 || hasGuard(e.KnockbackGuards, t.ID, s.Tick) {
				continue
			}
			distance := s.volleyPush() * float64(s.controlMultiplier(*e))
			if heavy(*e) {
				distance /= 2
			}
			e.KnockbackGuards = putGuard(s.freshGuards(e.KnockbackGuards), t.ID, s.Tick+8000/StepMS)
			e.Progress = math.Max(0, e.Progress-distance)
			p := pathPosition(s.routePath(e.Route), e.Progress)
			e.X, e.Y = p.X, p.Y
			e.Repairing, e.RepairSinceMS = 0, 0
			v := viewEnemy(*e)
			s.event(Event{Type: "knockback", Skill: "knockback", TowerID: t.ID, TargetID: e.ID, Target: &v, X: e.X, Y: e.Y, Owner: t.Owner, Kind: t.Kind, Amount: distance})
		}
	case "cannon/power":
		g := s.cannonArea(t.Level)
		if !s.skillRoll(uint64(g.Chance)) {
			return
		}
		s.event(Event{Type: "blast", Skill: "blast", TowerID: t.ID, X: target.X, Y: target.Y, Owner: t.Owner, Kind: t.Kind, Amount: g.Radius})
		for i, e := range s.Enemies {
			if e.HP > 0 && math.Hypot(e.X-target.X, e.Y-target.Y) <= g.Radius {
				s.hitWithSkill(i, damage*g.Damage, *t, "blast")
			}
		}
	case "cannon/speed":
		chance, _, _ := s.craterArea(t.Level)
		if now >= t.PassiveReadyAt && s.skillRoll(uint64(chance)) {
			s.makeCrater(t, target)
		}
	case "frost/freeze", "arcane/magic":
		chance := s.freezeChance(t.Level)
		if t.Kind == "arcane" {
			chance = s.meteorChance()
		}
		if now < t.PassiveReadyAt || !s.skillRoll(uint64(chance)) {
			return
		}
		duration, cd := s.freezeTiming(t.Level)
		if t.Kind == "arcane" {
			cd = s.meteorCooldown()
		}
		t.PassiveReadyAt = now + cd
		_, rangePX, _ := s.towerStats(*t)
		pad := s.World.Map.Pads[t.Pad]
		radius, percent := s.meteorArea(t.Level)
		indices := []int{}
		for i, e := range s.Enemies {
			if e.HP <= 0 || math.Hypot(e.X-pad.X, e.Y-pad.Y) > rangePX {
				continue
			}
			if s.geological() && t.Kind == "frost" && (e.ConfusedUntil > s.Tick || s.frozen(e) || s.sleeping(e) || e.Repairing > 0 || s.iceSlowFactor(e) <= 0 || s.geoMotion(Point{e.X, e.Y}) == 0) {
				continue
			}
			guards := e.FreezeGuards
			if t.Kind == "arcane" {
				guards = e.MeteorGuards
				if math.Hypot(e.X-target.X, e.Y-target.Y) > radius {
					continue
				}
			}
			if hasGuard(guards, t.ID, s.Tick) {
				continue
			}
			indices = append(indices, i)
		}
		// Group equal control durations so presentation receives exact per-target
		// deadlines even when sleeping and awake enemies share the same attack.
		for _, multiplier := range []int64{1, 2} {
			group := []int{}
			for _, i := range indices {
				if s.controlMultiplier(s.Enemies[i]) == multiplier {
					group = append(group, i)
				}
			}
			if len(group) == 0 {
				continue
			}
			event := Event{TowerID: t.ID, X: pad.X, Y: pad.Y, Owner: t.Owner, Kind: t.Kind, Amount: rangePX}
			for _, i := range group {
				event.StatusTargets = append(event.StatusTargets, s.Enemies[i].ID)
			}
			if t.Kind == "frost" {
				event.Type, event.Skill, event.FrozenUntil = "freeze", "freeze", s.Tick+duration/StepMS*multiplier
				for _, i := range group {
					e := &s.Enemies[i]
					e.FrozenUntil = max(e.FrozenUntil, event.FrozenUntil)
					e.RecoveryUntil = e.FrozenUntil + 8000/StepMS*multiplier
					e.FreezeGuards = putGuard(s.freshGuards(e.FreezeGuards), t.ID, s.Tick+8000/StepMS)
					event.RecoveryUntil = e.RecoveryUntil
				}
			} else {
				event.Type, event.Skill, event.SilencedUntil = "meteor", "meteor", s.Tick+3000/StepMS*multiplier
				event.X, event.Y, event.Amount = target.X, target.Y, radius
				for _, i := range group {
					e := &s.Enemies[i]
					e.SilencedUntil = max(e.SilencedUntil, event.SilencedUntil)
					e.MeteorGuards = putGuard(s.freshGuards(e.MeteorGuards), t.ID, s.Tick+s.meteorProtection())
				}
			}
			s.event(event)
		}
		if t.Kind == "arcane" {
			for _, i := range indices {
				extra := percent
				if heavy(s.Enemies[i]) {
					if s.weatherV3() {
						extra += 1
					} else {
						extra += .5
					}
				}
				s.hitWithSkill(i, damage*extra, *t, "meteor")
			}
		}
	}
}
func (s *State) leakDamage(e Enemy) int {
	if s.weatherV2() && e.BossGuard {
		s.World.GuardLeakRemainder++
		damage := s.World.GuardLeakRemainder / 2
		s.World.GuardLeakRemainder %= 2
		return damage
	}
	if !s.weathered() || !heavy(e) {
		return e.Leak
	}
	base := 4
	if e.Kind == "boss" {
		if s.World.BossWards > 0 {
			s.World.BossWards--
			return 0
		}
		if s.weatherV2() {
			return 7 + int(math.Round(3*math.Max(0, math.Min(1, e.HP/e.MaxHP))))
		}
		base = 8
	}
	return base + int(math.Round(2*math.Max(0, math.Min(1, e.HP/e.MaxHP))))
}
func (s *State) validWeather() error {
	if s.WeatherRules < 0 || s.WeatherRules > 3 || (s.weathered() && !s.growing()) {
		return errors.New("无效天气规则")
	}
	if err := s.validWeatherV2(); err != nil {
		return err
	}
	for _, e := range s.Enemies {
		for _, guards := range [][]KnockbackGuard{e.FreezeGuards, e.HeartGuards, e.MeteorGuards, e.MissGuards} {
			if len(guards) > 40 || (!s.weathered() && len(guards) > 0) {
				return errors.New("无效技能保护")
			}
			seen := map[int]bool{}
			for _, g := range guards {
				if g.TowerID < 1 || g.TowerID > s.TowerSerial || g.Until < 0 || seen[g.TowerID] {
					return errors.New("无效技能保护")
				}
				seen[g.TowerID] = true
			}
		}
	}
	for _, t := range s.Towers {
		if t.ShopDamage < 0 || t.ShopRange < 0 || t.ShopDamage > maxValue || t.ShopRange > maxValue || (!s.weathered() && (t.ShopDamage != 0 || t.ShopRange != 0)) {
			return errors.New("无效商店强化")
		}
	}
	return s.validShop()
}
