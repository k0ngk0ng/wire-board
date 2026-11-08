package starfort

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

type PadReward struct {
	Owner int    `json:"owner"`
	Kind  string `json:"kind"`
}

func scaledTask(base int, growth float64, attempt int) float64 {
	return math.Min(maxValue, float64(base)*math.Pow(growth, float64(min(attempt, 10000))))
}
func (s *State) taskRandom(n int) int {
	// Reuse the saved server RNG; all choices are made once during preparation.
	s.skillRoll(0)
	return int(s.SkillRNG % uint64(n))
}
func (s *State) offerWaveTask() {
	if !s.rebalanced() || s.World.TaskOffersWave == s.Wave {
		return
	}
	for _, o := range s.World.Objectives {
		if o.Status == "active" {
			return
		}
	}
	s.World.TaskOffersWave = s.Wave
	if s.World.TaskAttempts == nil {
		s.World.TaskAttempts = map[string]int{}
	}
	routes := s.activeRoutes()
	if len(routes) == 0 {
		return
	}
	routeID := routes[s.taskRandom(len(routes))]
	kind := []string{"mine", "beacon", "convoy"}[s.taskRandom(3)]
	attempt := s.World.TaskAttempts[kind]
	baseHP, baseCost, growth, fraction := 10, 100, 1.2, .7+float64(s.taskRandom(1001))/10000
	label := "修复星砂矿场"
	if kind == "beacon" {
		baseHP, growth, fraction, label = 4, 1.5, .5+float64(s.taskRandom(1001))/10000, "点亮瞭望塔"
	}
	if kind == "convoy" {
		baseHP, baseCost, growth, fraction, label = 6, 300, 2, 0, "护送星光补给车"
	}
	route := s.routePath(routeID)
	progress := pathDistance(route) * fraction
	p := pathPosition(route, progress)
	hp := scaledTask(baseHP, 1.1, attempt)
	cost := int(math.Round(scaledTask(baseCost, growth, attempt)))
	reward := int(math.Min(maxValue, math.Round(float64(cost)*1.2)))
	sector := 1
	for _, r := range s.World.Map.Routes {
		if r.ID == routeID {
			sector = r.Sector
		}
	}
	description := fmt.Sprintf("支付 %d 金币，保护目标撑过本波；该路敌人生命 ×1.2，成功后每人获得 %d 金币，并交替奖励自选建塔点／高地", cost, reward)
	if kind == "beacon" {
		reward = 0
		description = fmt.Sprintf("支付 %d 金币，保护瞭望塔撑过本波；该路敌人生命 ×1.2，成功后全队永久增加 5%% 射程", cost)
	}
	if kind == "convoy" {
		description = fmt.Sprintf("支付 %d 金币，护送速度 20 的补给车抵达星核；提前敌人 2 秒出发，该路敌人生命 ×1.2；送达修复 4 点生命，每人获得 %d 金币", cost, reward)
	}
	s.World.Objectives = []Objective{{ID: fmt.Sprintf("task-%d-%s", s.Wave, kind), Kind: kind, Label: label, Description: description, HP: hp, MaxHP: hp, Route: routeID, Progress: progress, Sector: sector, X: p.X, Y: p.Y, Cost: cost, Reward: reward, Status: "available", Owner: -1}}
}

func (s *State) taskRouteBoost(route string) float64 {
	if s.rebalanced() {
		for _, o := range s.World.Objectives {
			if (o.Status == "active" || o.Status == "completed" || o.Status == "failed") && o.Route == route && o.StartWave == s.Wave {
				return 1.2
			}
		}
	}
	return 1
}
func (s *State) taskConvoyActive() bool {
	if !s.rebalanced() {
		return false
	}
	for _, o := range s.World.Objectives {
		if o.Kind == "convoy" && o.Status == "active" {
			return true
		}
	}
	return false
}
func (s *State) skipTask(id string) error {
	if !s.rebalanced() || s.Phase != "prepare" {
		return errors.New("只能在波前跳过任务")
	}
	for i := range s.World.Objectives {
		o := &s.World.Objectives[i]
		if o.ID == id && o.Status == "available" {
			o.Status = "skipped"
			return nil
		}
	}
	return errors.New("任务不可跳过")
}
func (s *State) claimPad(owner int, p Point, pad int) error {
	r := s.World.PadReward
	if !s.rebalanced() || s.Phase != "prepare" || s.World.Choice != nil || s.World.BoonChoice != nil || r == nil || r.Owner != owner {
		return errors.New("当前没有可领取的建塔点奖励")
	}
	if r.Kind == "highground" {
		if pad < 0 || pad >= len(s.World.Map.Pads) || s.World.Map.Pads[pad].Terrain == "highground" {
			return errors.New("请选择普通建造点")
		}
		s.World.Map.Pads[pad].Terrain = "highground"
	} else {
		m := &s.World.Map
		if len(m.Pads) >= 40 || !finite(p.X) || !finite(p.Y) || p.X < 28 || p.Y < 28 || p.X > float64(m.Width)-28 || p.Y > float64(m.Height)-28 || math.Hypot(p.X-m.Base.X, p.Y-m.Base.Y) < 55 {
			return errors.New("请选择战场内不与星核重叠的位置；最多 40 个点")
		}
		for _, other := range m.Pads {
			if math.Hypot(p.X-other.X, p.Y-other.Y) < 48 {
				return errors.New("与已有建造点距离过近")
			}
		}
		sector := 1
		for _, region := range m.Regions {
			if p.X >= float64(region.X) && p.X <= float64(region.X+region.Width) && p.Y >= float64(region.Y) && p.Y <= float64(region.Y+region.Height) {
				sector = region.ID
			}
		}
		m.Pads = append(m.Pads, Pad{ID: len(m.Pads), X: p.X, Y: p.Y, Terrain: "plain", Sector: sector, Custom: true})
	}
	s.World.PadReward = nil
	s.event(Event{Type: "objective", Owner: owner, Kind: "pad_reward", X: p.X, Y: p.Y})
	return nil
}
func (s *State) automaticPadReward() *Action {
	r := s.World.PadReward
	if r == nil {
		return nil
	}
	if r.Kind == "highground" {
		for _, p := range s.World.Map.Pads {
			if p.Terrain == "plain" {
				return &Action{Type: "pad_reward", Pad: p.ID}
			}
		}
		return nil
	}
	// Prefer useful road-side positions, then scan remaining map coordinates.
	for _, route := range s.World.Map.Routes {
		for f := .2; f < .95; f += .05 {
			d := pathDistance(route.Points) * f
			p := pathPosition(route.Points, d)
			q := pathPosition(route.Points, d+1)
			dx, dy := q.X-p.X, q.Y-p.Y
			l := math.Hypot(dx, dy)
			for _, side := range []float64{-1, 1} {
				point := Point{p.X - dy/l*side*80, p.Y + dx/l*side*80}
				if s.validRewardPoint(point) {
					return &Action{Type: "pad_reward", X: point.X, Y: point.Y}
				}
			}
		}
	}
	for y := 32.; y < float64(s.World.Map.Height)-28; y += 32 {
		for x := 32.; x < float64(s.World.Map.Width)-28; x += 32 {
			if s.validRewardPoint(Point{x, y}) {
				return &Action{Type: "pad_reward", X: x, Y: y}
			}
		}
	}
	return nil
}
func (s *State) validRewardPoint(p Point) bool {
	m := s.World.Map
	return p.X >= 28 && p.Y >= 28 && p.X <= float64(m.Width)-28 && p.Y <= float64(m.Height)-28 && math.Hypot(p.X-m.Base.X, p.Y-m.Base.Y) >= 55 && !slices.ContainsFunc(m.Pads, func(q Pad) bool { return math.Hypot(p.X-q.X, p.Y-q.Y) < 48 })
}

func (s *State) minimumPads() int {
	if s.rebalanced() {
		return 6
	}
	return 16
}
func (s *State) expandedPadCount(n int) int {
	if s.rebalanced() {
		return min(40, n+6)
	}
	return n + 8
}
func (s *State) objectiveLimit() float64 {
	if s.rebalanced() {
		return maxValue
	}
	return 10000
}
func (s *State) validBalance() error {
	w := s.World
	if s.SpawnNotBefore < 0 || s.SpawnNotBefore > s.ElapsedMS+2000 || w.TaskOffersWave < 0 || w.TaskOffersWave > s.Wave || w.MineSuccesses < 0 || w.BeaconSuccesses < 0 || len(w.TaskAttempts) > 3 {
		return errors.New("无效任务进度")
	}
	for kind, n := range w.TaskAttempts {
		if !slices.Contains([]string{"mine", "beacon", "convoy"}, kind) || n < 0 || n > maxValue {
			return errors.New("无效任务次数")
		}
	}
	if r := w.PadReward; r != nil {
		if !s.rebalanced() || r.Owner < 0 || r.Owner >= len(s.Players) || (r.Kind != "add" && r.Kind != "highground") || (r.Kind == "add" && len(w.Map.Pads) >= 40) {
			return errors.New("无效建造点奖励")
		}
	}
	if s.rebalanced() && len(s.IceWalls) > 0 {
		return errors.New("新规则不允许冰墙")
	}
	for _, e := range s.Enemies {
		if !finite(e.SlowFactor) || e.SlowFactor < 0 || e.SlowFactor > 1 || e.RecoveryUntil < 0 || len(e.KnockbackGuards) > 40 || (s.rebalanced() && e.Frostbound) {
			return errors.New("无效减速状态")
		}
		ids := map[int]bool{}
		for _, g := range e.KnockbackGuards {
			if g.TowerID < 1 || g.TowerID > s.TowerSerial || ids[g.TowerID] || g.Until < 0 {
				return errors.New("无效击退保护")
			}
			ids[g.TowerID] = true
		}
	}
	for _, t := range s.Towers {
		if t.ChainReadyAt < 0 {
			return errors.New("无效星链冷却")
		}
	}
	return nil
}

func (s *State) objectiveMin(o Objective) float64 {
	if s.rebalanced() && o.Kind == "convoy" {
		return -40
	}
	return 0
}

func (s *State) objectiveGoldLimit() float64 {
	if s.rebalanced() {
		return maxValue
	}
	return 1000
}
