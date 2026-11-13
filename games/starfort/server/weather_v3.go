package starfort

import (
	"fmt"
	"math"
)

func (s *State) weatherV3() bool { return s.WeatherRules == 3 }

// Sleep magnifies newly applied control, without extending source cooldowns
// or per-tower immunity. Save the resulting status deadlines as usual.
func (s *State) controlMultiplier(e Enemy) int64 {
	if s.weatherV3() && s.sleeping(e) {
		return 2
	}
	return 1
}
func (s *State) sleepDamageFactor(e Enemy) float64 {
	if !s.sleeping(e) {
		return 1
	}
	if s.weatherV3() {
		return .5
	}
	return 0
}
func (s *State) freezeChance(level int) int {
	if s.revised() {
		return min(100, 15+(max(1, level)+1)/3)
	}
	if s.weatherV3() {
		return min(100, 15+5*((max(1, level)+1)/3))
	}
	return 15
}
func (s *State) meteorChance() int {
	chance := 15
	if s.weatherV3() {
		chance = 20
	}
	if s.World.Event.ID == "starlight" {
		chance += 10
	}
	return chance
}
func (s *State) meteorCooldown() int64 {
	if s.weatherV3() {
		return 5000
	}
	return 10000
}
func (s *State) meteorProtection() int64 {
	if s.revised() {
		return 5000 / StepMS
	}
	if s.weatherV3() {
		return 8000 / StepMS
	}
	return 10000 / StepMS
}
func (s *State) volleyPush() float64 {
	push := 40.
	if s.weatherV3() {
		push = 60
	}
	if s.revised() {
		push = 50
	}
	if s.World.Event.ID == "fog" {
		push += 10
	}
	return push
}
func (s *State) heartFogBonus() float64 {
	if s.weatherV3() {
		return 1.1
	}
	return 1.05
}
func (s *State) heartArea(level int) (int, float64) {
	if s.revised() {
		n := max(1, level) - 1
		return min(100, 2+n/2), math.Min(1, .05+.01*float64((n+1)/2))
	}
	if !s.weatherV3() {
		return heartArea(level)
	}
	n := max(1, level) - 1
	return min(100, 10+5*((n+1)/2)), math.Min(1, .1+.05*float64(n/2))
}
func (s *State) meteorArea(level int) (float64, float64) {
	if s.revised() {
		n := max(1, level) - 1
		return 200 + 10*float64((n+1)/2), .3 + .02*float64(n/2)
	}
	if !s.weatherV3() {
		return meteorArea(level)
	}
	n := max(1, level) - 1
	return 200 + 20*float64((n+1)/2), .3 + .05*float64(n/2)
}
func (s *State) weatherV3Choices(t Tower) []BranchChoice {
	level := max(4, t.Level)
	switch t.Kind {
	case "cannon":
		g := s.cannonArea(level)
		chance, cd, work := s.craterArea(level)
		return []BranchChoice{
			{"power", "威力专精", fmt.Sprintf("伤害 ×1.2；%d%% 概率在主目标半径 %.0f 内追加 %.0f%% 伤害；精英／首领完全有效，填坑中的敌人额外受到 50%% 伤害；升级依次增加概率、半径、伤害", g.Chance, g.Radius, g.Damage*100)},
			{"speed", "阻滞专精", fmt.Sprintf("攻速 ×0.8；%d%% 概率生成工作量 %.0f 的弹坑，冷却 %.1f 秒；同敌人 4 秒内不受同塔其他弹坑影响，精英／首领填坑时间减半；5 的倍数等级只加工作量，其余升级交替增加概率、减少冷却", chance, work, float64(cd)/1000)},
		}
	case "arrow":
		g := s.volleyArea(level)
		chance, percent := s.heartArea(level)
		return []BranchChoice{
			{"volley", "风羽箭雨", fmt.Sprintf("攻速 ×0.7；%d%% 概率对主目标半径 %.0f 内最多 %d 个其他敌人造成 %.0f%% 伤害；独立 15%% 概率击退存活主目标 %.0f，精英／首领减半，同塔 8 秒免疫；升级依次增加概率、半径、伤害，10 的倍数等级只加目标", g.Chance, g.Radius, g.Targets, g.Damage*100, s.volleyPush())},
			{"heart", "一箭残废", fmt.Sprintf("%d%% 概率追加目标剩余生命 %.0f%% 的伤害，同塔对同敌人 5 秒免疫；升级交替增加概率、伤害；薄雾下本塔伤害额外增加 10%%", chance, percent*100)},
		}
	case "frost":
		duration, cd := s.freezeTiming(level)
		preview := t
		preview.Level = level
		preview.Branch = "speed"
		factor, slowDuration := s.iceAttack(preview)
		return []BranchChoice{
			{"freeze", "冰封专精", fmt.Sprintf("攻速 ×0.5；%d%% 概率冰封射程内敌人 %.1f 秒，冷却 %.1f 秒，随后 8 秒逐渐恢复速度；同塔 8 秒免疫，精英／首领完全有效；升级依次增加概率、时长、减少冷却；寒潮额外延长 2 秒、冷却减少 2 秒", s.freezeChance(level), float64(duration)/1000, float64(cd)/1000)},
			{"speed", "攻速专精", fmt.Sprintf("攻速 ×1.5；命中减速 %.0f%%，持续 %.1f 秒，精英／首领完全有效；升级交替延长 0.5 秒、增加减速 5 个百分点；寒潮额外减速 15 个百分点", (1-factor)*100, float64(slowDuration)*StepMS/1000)},
		}
	case "arcane":
		radius, percent := s.meteorArea(level)
		return []BranchChoice{
			{"power", "威力专精", fmt.Sprintf("攻速 ×0.8；忽略护甲，有护甲目标基础伤害 ×1.1；有护甲普通敌人额外 ×%.1f，精英／首领额外 ×%.1f，每级各加 0.1；星辉下本塔伤害 ×1.2", 1.2+.1*float64(level-1), 1.1+.1*float64(level-1))},
			{"magic", "魔法专精", fmt.Sprintf("%d%% 概率对主目标半径 %.0f 与塔射程交集内敌人追加 %.0f%% 无视护甲伤害并沉默 3 秒；精英／首领另受 100%% 伤害；冷却 5 秒，同塔对同敌人 8 秒免疫；升级交替增加半径、伤害", s.meteorChance(), radius, percent*100)},
		}
	}
	return nil
}
