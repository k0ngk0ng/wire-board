package starfort

import (
	"math"
	"slices"
)

// The flag is frozen at Start. Imported matches with no flag retain their
// original branches, damage, control and objective cadence.
func (s *State) specialized() bool             { return s.SpecializationRules == 1 }
func (s *State) frozen(e Enemy) bool           { return e.FrozenUntil > s.Tick }
func (s *State) silenced(e Enemy) bool         { return e.SilencedUntil > s.Tick }
func (s *State) abilitiesBlocked(e Enemy) bool { return s.frozen(e) || s.silenced(e) || s.sleeping(e) }
func heavy(e Enemy) bool                       { return e.Kind == "elite" || e.Kind == "boss" }

func (s *State) specializationChoices(t Tower) []BranchChoice {
	if !s.specialized() {
		return nil
	}
	switch t.Kind {
	case "frost":
		return []BranchChoice{
			{"freeze", "冰封专精", "每次攻击有 15% 概率冰封塔射程内敌人 2 秒；精英与首领有效，冷却 8 秒；寒潮额外增强减速 15 个百分点"},
			{"speed", "攻速专精", "攻速提高 65%，单次伤害降低 10%；保留寒霜冰墙，寒潮额外增强减速 15 个百分点"},
		}
	case "cannon":
		return []BranchChoice{
			{"power", "威力专精", "伤害提高 50%，攻速降低 10%；20% 概率追加半径 110、75% 攻击伤害的爆破，精英与首领有效"},
			{"speed", "攻速专精", "攻速提高 65%，单次伤害降低 10%；25% 概率留下弹坑，冷却 6 秒；精英与首领直接通过"},
		}
	case "arcane":
		return []BranchChoice{
			{"power", "威力专精", "伤害提高 50%，攻速降低 10%；完全忽略护甲，破盾效率再提高 15%，额外破盾不作用于精英与首领"},
			{"magic", "魔法专精", "15% 概率召唤流星雨，对塔射程内敌人造成 100% 攻击伤害并沉默 3 秒；精英与首领有效，冷却 8 秒"},
		}
	}
	return nil
}

func (s *State) specializationView(t Tower) *PassiveView {
	if !s.specialized() {
		return nil
	}
	p := &PassiveView{ReadyInMS: max(0, t.PassiveReadyAt-s.ElapsedMS)}
	switch {
	case t.Kind == "arrow" && t.Branch == "volley":
		p.Name = "风羽箭雨"
		p.Variant = "volley"
		p.TriggerChance = 25
		p.Description = "25% 概率对主目标周围 75 范围造成 70% 攻击伤害；每箭独立有 15% 概率将该范围敌人击退 60，精英与首领有效"
	case t.Kind == "frost" && t.Branch == "freeze":
		p.Name = "极寒冰封"
		p.Variant = "freeze"
		p.TriggerChance = 15
		p.CooldownMS = 8000
		p.Description = "每次攻击有 15% 概率使塔当前射程内的存活敌人冰封 2 秒，停止移动、施法、填坑及攻击任务；精英与首领有效。冷却 8 秒；寒潮下冰塔减速额外增强 15 个百分点"
	case t.Kind == "cannon" && t.Branch == "power":
		p.Name = "震荡爆破"
		p.Variant = "blast"
		p.TriggerChance = 20
		p.Description = "每次攻击有 20% 概率，在主目标周围半径 110 内追加 75% 攻击伤害，精英与首领有效；替换基础震地弹"
	case t.Kind == "cannon" && t.Branch == "speed":
		p.Name = "阻滞弹坑"
		p.Variant = "crater"
		p.TriggerChance = 25
		p.CooldownMS = 6000
		p.Description = "每次攻击有 25% 概率留下总工作量为 6 的弹坑，后续普通敌人停下填补；精英与首领直接通过。冷却 6 秒"
	case t.Kind == "arcane" && t.Branch == "magic":
		p.Name = "星陨流星雨"
		p.Variant = "meteor"
		p.TriggerChance = 15
		p.CooldownMS = 8000
		p.Description = "每次攻击有 15% 概率，对塔当前射程内敌人造成 100% 魔法攻击伤害并沉默 3 秒；暂停治疗、旗手加速、工兵能力、首领阶段触发和死亡分裂，精英与首领有效。冷却 8 秒；替换星链"
	case t.Kind == "arcane" && t.Branch == "power":
		p.Name = "破盾星链"
		p.CooldownMS = 8000
		p.Description = "攻击完全忽略所有敌人的护甲，对普通敌人的破盾效率在基础 1.5 倍上再提高 15%；额外破盾不作用于精英与首领。保留每 8 秒一次、最多两名目标的半额星链"
	default:
		return nil
	}
	if p.CooldownMS == 0 {
		p.ReadyInMS = 0
	}
	return p
}

// Returning true consumes the specialization's passive slot, even on a failed
// roll. Otherwise a failed proc could silently fall back to the old passive.
func (s *State) specializationHit(t *Tower, target Enemy, damage float64) bool {
	if !s.specialized() {
		return false
	}
	now := s.combatNow()
	switch {
	case t.Kind == "arrow" && t.Branch == "volley":
		if s.skillRoll(15) {
			for i := range s.Enemies {
				e := &s.Enemies[i]
				if e.HP <= 0 || (s.growing() && e.ID != target.ID) || (!s.growing() && math.Hypot(e.X-target.X, e.Y-target.Y) > 75) {
					continue
				}
				distance := 60.
				if s.growing() {
					distance = 30
					if heavy(*e) {
						distance /= 2
					}
					if hasGuard(e.KnockbackGuards, t.ID, s.Tick) {
						distance /= 2
					}
					e.KnockbackGuards = putGuard(s.freshGuards(e.KnockbackGuards), t.ID, s.Tick+8000/StepMS)
				} else if s.rebalanced() {
					distance = 40
					if heavy(*e) {
						if slices.ContainsFunc(e.KnockbackGuards, func(g KnockbackGuard) bool { return g.TowerID == t.ID && g.Until > s.Tick }) {
							continue
						}
						e.KnockbackGuards = slices.DeleteFunc(e.KnockbackGuards, func(g KnockbackGuard) bool {
							return g.Until <= s.Tick || !slices.ContainsFunc(s.Towers, func(t Tower) bool { return t.ID == g.TowerID })
						})
						e.KnockbackGuards = append(e.KnockbackGuards, KnockbackGuard{t.ID, s.Tick + 8000/StepMS})
						distance = 20
					}
				}
				e.Progress = math.Max(0, e.Progress-distance)
				p := pathPosition(s.routePath(e.Route), e.Progress)
				e.X, e.Y = p.X, p.Y
				e.Repairing, e.RepairSinceMS = 0, 0
				v := viewEnemy(*e)
				s.event(Event{Type: "knockback", Skill: "knockback", TowerID: t.ID, TargetID: e.ID, Target: &v, X: e.X, Y: e.Y, Owner: t.Owner, Kind: "arrow", Amount: distance})
			}
		}
		return true
	case t.Kind == "cannon" && t.Branch == "power":
		g := areaGrowth{Chance: 20, Radius: 110, Damage: .75}
		if s.growing() {
			g = cannonGrowth(t.Level)
		}
		if s.skillRoll(uint64(g.Chance)) {
			s.event(Event{Type: "blast", Skill: "blast", TowerID: t.ID, X: target.X, Y: target.Y, Owner: t.Owner, Kind: "cannon", Amount: g.Radius})
			for i, e := range s.Enemies {
				if e.HP > 0 && math.Hypot(e.X-target.X, e.Y-target.Y) <= g.Radius {
					s.hitWithSkill(i, damage*g.Damage, *t, "blast")
				}
			}
		}
		return true
	case t.Kind == "cannon" && t.Branch == "speed":
		chance := 25
		if s.growing() {
			chance, _, _ = craterGrowth(t.Level)
		}
		if now >= t.PassiveReadyAt && s.skillRoll(uint64(chance)) {
			s.makeCrater(t, target)
		}
		return true
	case t.Kind == "frost" && t.Branch == "freeze", t.Kind == "arcane" && t.Branch == "magic":
		if now < t.PassiveReadyAt || !s.skillRoll(15) {
			return true
		}
		t.PassiveReadyAt = now + 8000
		if s.rebalanced() && t.Kind == "frost" {
			t.PassiveReadyAt = now + 10000
		}
		_, radius, _ := s.towerStats(*t)
		pad := s.World.Map.Pads[t.Pad]
		indices := []int{}
		ids := []int{}
		for i, e := range s.Enemies {
			if e.HP > 0 && math.Hypot(e.X-pad.X, e.Y-pad.Y) <= radius {
				indices = append(indices, i)
				ids = append(ids, e.ID)
			}
		}
		event := Event{TowerID: t.ID, X: pad.X, Y: pad.Y, Owner: t.Owner, Kind: t.Kind, StatusTargets: ids, Amount: radius}
		if t.Kind == "frost" {
			event.Type, event.Skill, event.FrozenUntil = "freeze", "freeze", s.Tick+2000/StepMS
			for _, i := range indices {
				s.Enemies[i].FrozenUntil = max(s.Enemies[i].FrozenUntil, event.FrozenUntil)
				if s.rebalanced() {
					s.Enemies[i].RecoveryUntil = s.Enemies[i].FrozenUntil + 10000/StepMS
					event.RecoveryUntil = s.Enemies[i].RecoveryUntil
				}
			}
		} else {
			event.Type, event.Skill, event.SilencedUntil = "meteor", "meteor", s.Tick+3000/StepMS
			for _, i := range indices {
				s.Enemies[i].SilencedUntil = max(s.Enemies[i].SilencedUntil, event.SilencedUntil)
			}
		}
		s.event(event)
		if t.Kind == "arcane" {
			for _, i := range indices {
				s.hitWithSkill(i, damage, *t, "meteor")
			}
		}
		return true
	}
	return false
}

func (s *State) iceSlowFactor(e Enemy) float64 {
	if s.rebalanced() {
		factor := 1.
		if e.SlowUntil > s.Tick {
			factor = e.SlowFactor
			if factor <= 0 {
				factor = 1
			}
		}
		if e.RecoveryUntil > s.Tick {
			duration := s.recoveryMS() / StepMS
			if s.weatherV3() {
				duration = max(1, e.RecoveryUntil-e.FrozenUntil)
			}
			factor = math.Min(factor, math.Max(0, float64(s.Tick-e.FrozenUntil)/float64(duration)))
		}
		return factor
	}
	factor := 1.
	if e.SlowUntil > s.Tick {
		factor = .55
	} else if e.Frostbound {
		factor = .7
	}
	if s.specialized() && s.World.Event.ID == "frost" && factor < 1 {
		factor -= .15
	}
	return factor
}

func (s *State) refreshObjectives() {
	if !s.specialized() {
		return
	}
	cycle := s.CompletedWaves / 15 * 15
	if cycle == 0 || cycle <= s.World.ObjectiveRefreshWave {
		return
	}
	s.World.ObjectiveRefreshWave = cycle
	for i := range s.World.Objectives {
		o := &s.World.Objectives[i]
		if o.Status == "active" {
			continue
		}
		o.Status = "available"
		o.HP = o.MaxHP
		o.StartWave, o.EndWave, o.BaseAtStart = 0, 0, 0
		o.Progress = 0
		if o.Kind == "convoy" {
			o.Progress = 80
			p := pathPosition(s.convoyPath(o.Route), 80)
			o.X, o.Y = p.X, p.Y
		}
	}
	s.note("已守过 %d 波，支线任务已刷新；进行中的任务继续保留", cycle)
}
