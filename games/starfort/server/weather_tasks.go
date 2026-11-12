package starfort

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

func (s *State) recoveryMS() int64 {
	if s.weathered() {
		return 8000
	}
	return 10000
}

func (s *State) routePads(m *Map, r Route) {
	if !s.weathered() {
		addRoutePads(m, r)
		return
	}
	start := len(m.Pads)
	for i := 0; i < 6 && len(m.Pads) < 40; i++ {
		d := pathDistance(r.Points) * float64(i+1) / 7
		p, q := pathPosition(r.Points, d), pathPosition(r.Points, d+1)
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

func (s *State) offerWeatherTask() {
	if s.World.TaskOffersWave == s.Wave {
		return
	}
	for _, o := range s.World.Objectives {
		if o.Status == "active" {
			return
		}
	}
	s.World.TaskOffersWave = s.Wave
	s.World.Objectives = []Objective{}
	if s.World.TaskAttempts == nil {
		s.World.TaskAttempts = map[string]int{}
	}
	// One saved roll gates all task kinds; viewing/reloading cannot reroll it.
	chance := uint64(25)
	if s.weatherV3() {
		chance = 40
	}
	if !s.skillRoll(chance) {
		return
	}
	routes := s.activeRoutes()
	if len(routes) == 0 {
		return
	}
	routeID := routes[s.taskRandom(len(routes))]
	kinds := []string{"mine", "beacon", "convoy"}
	if s.Wave >= 10 {
		kinds = append(kinds, "merchant")
	}
	kind := kinds[s.taskRandom(len(kinds))]
	attempt := s.World.TaskAttempts[kind]
	baseHP, baseCost, growth, low, span, mult := 12, 80, 1.2, .3, .2, .8
	label := "修复星砂矿场"
	benefit := "成功后交替奖励自选建造点／高地，满 40 点改为移动一座防御塔"
	switch kind {
	case "beacon":
		baseHP, low, span, mult, label = 8, .4, .2, 1, "点亮瞭望塔"
		benefit = "守护期间半径 80 的防御塔攻击力增加 20%；成功后全队交替永久增加射程／攻击力"
	case "convoy":
		baseHP, baseCost, growth, low, span, mult, label = 5, 200, 1.15, .3, .1, 1.1, "护送星光补给车"
		benefit = "补给车速度 40，抵达后星核恢复 4 点生命"
	case "merchant":
		baseHP, baseCost, low, span, mult, label = 4, 200, .5, .1, 1.3, "护送商人"
		benefit = "商人速度 20，抵达星核后发放金币，当波过关后开放商店"
	}
	if s.weatherV2() {
		if kind == "mine" {
			low, span = .4, .1
		}
		if kind == "beacon" {
			low, span = .5, .1
		}
		if kind == "convoy" {
			benefit = fmt.Sprintf("补给车速度 40，开波后出发，抵达修复 %d 生命；星核已满血则获得医疗包，可在波间使用", 4+s.Wave/10)
		}
	}
	fraction := low + span*float64(s.taskRandom(1001))/1000
	path := s.routePath(routeID)
	progress := pathDistance(path) * fraction
	p := pathPosition(path, progress)
	hp := scaledTask(baseHP, 1.2, attempt)
	cost := int(math.Round(scaledTask(baseCost, growth, attempt)))
	reward := int(math.Min(maxValue, math.Round(float64(cost)*mult)))
	sector := 1
	for _, r := range s.World.Map.Routes {
		if r.ID == routeID {
			sector = r.Sector
		}
	}
	o := Objective{ID: fmt.Sprintf("task-%d-%s", s.Wave, kind), Kind: kind, Label: label, HP: hp, MaxHP: hp, Route: routeID, Progress: progress, Sector: sector, X: p.X, Y: p.Y, Cost: cost, Reward: reward, Status: "available", Owner: -1}
	o.Description = fmt.Sprintf("支付 %d 金币，该路敌人生命 ×1.1；仅受本路普通敌人经过时的一次攻击，每次扣 1 生命。%s；成功每人获得 %d 金币，失败退还付款人 %d 金币", cost, benefit, reward, s.refundTask(o))
	s.World.Objectives = []Objective{o}
}
func escort(kind string) bool { return kind == "convoy" || kind == "merchant" }
func (s *State) escortSpeed(kind string) float64 {
	if kind == "merchant" {
		return 20
	}
	return 40
}
func (s *State) refundTask(o Objective) int {
	if !s.weathered() {
		return o.Cost / 5
	}
	percent := 10
	switch o.Kind {
	case "beacon":
		percent = 20
	case "convoy":
		percent = 30
	case "merchant":
		percent = 0
	}
	return o.Cost * percent / 100
}

// Relative route progress is swept across the full simulation step, including
// the objective's motion, so a one-unit contact cannot be skipped at high speed.
func (s *State) taskContact(e Enemy, from, to float64) {
	if !s.weathered() || e.HP <= 0 || heavy(e) || s.frozen(e) || s.sleeping(e) {
		return
	}
	for i := range s.World.Objectives {
		o := &s.World.Objectives[i]
		if o.Status != "active" || o.Route != e.Route || slices.Contains(o.Attacked, e.ID) {
			continue
		}
		next := o.Progress
		if escort(o.Kind) {
			next = math.Min(pathDistance(s.convoyPath(o.Route)), next+s.escortSpeed(o.Kind)*StepMS/1000)
		}
		a, b := from-o.Progress, to-next
		if math.Min(a, b) > 1 || math.Max(a, b) < -1 {
			continue
		}
		o.Attacked = append(o.Attacked, e.ID)
		o.HP = math.Max(0, o.HP-1)
		s.event(Event{Type: "objective_hit", X: o.X, Y: o.Y, Owner: -1, Kind: o.Kind, Amount: 1})
	}
}
func (s *State) stepWeatherObjectives() {
	if s.BaseHP <= 0 {
		s.failActiveObjectives("基地失守")
		return
	}
	for i := range s.World.Objectives {
		o := &s.World.Objectives[i]
		if o.Status != "active" {
			continue
		}
		if o.HP <= 0 {
			s.finishObjective(i, false, "防守目标被摧毁")
			continue
		}
		if escort(o.Kind) {
			path := s.convoyPath(o.Route)
			length := pathDistance(path)
			o.Progress = math.Min(length, o.Progress+s.escortSpeed(o.Kind)*StepMS/1000)
			p := pathPosition(path, o.Progress)
			o.X, o.Y = p.X, p.Y
			if o.Progress >= length {
				s.finishObjective(i, true, "")
			}
		}
	}
}
func (s *State) finishWeatherObjective(index int, success bool, reason string) {
	o := &s.World.Objectives[index]
	if o.Status != "active" {
		return
	}
	o.Status = "failed"
	if success {
		o.Status = "completed"
		for i := range s.Players {
			s.Players[i].Gold = min(maxValue, s.Players[i].Gold+o.Reward)
		}
		switch o.Kind {
		case "mine":
			s.World.MineSuccesses++
			kind := "add"
			if s.World.MineSuccesses%2 == 0 {
				kind = "highground"
			}
			if len(s.World.Map.Pads) >= 40 {
				kind = "move"
			}
			if kind != "highground" || slices.ContainsFunc(s.World.Map.Pads, func(p Pad) bool { return p.Terrain == "plain" }) {
				s.World.PadReward = &PadReward{Owner: o.Owner, Kind: kind}
			}
		case "beacon":
			s.World.BeaconSuccesses++
			n := s.World.BeaconSuccesses
			if n%2 == 1 {
				s.World.BeaconReach += float64((n+1)/2+1) / 100
			} else {
				s.World.BeaconPower += float64(n/2+1) / 100
			}
		case "convoy":
			if s.weatherV2() {
				if s.BaseHP == s.BaseMaxHP {
					s.World.MedicalPacks = min(maxValue, s.World.MedicalPacks+1)
				} else {
					s.BaseHP = min(s.BaseMaxHP, s.BaseHP+4+s.Wave/10)
				}
			} else {
				s.BaseHP = min(s.BaseMaxHP, s.BaseHP+4)
			}
		case "merchant":
			s.World.MerchantWave = s.Wave
		}
		s.note("完成支线：%s", o.Label)
	} else {
		if o.Owner >= 0 && o.Owner < len(s.Players) {
			before := s.Players[o.Owner].Gold
			s.Players[o.Owner].Gold = min(maxValue, before+s.refundTask(*o))
			o.Refunded = s.Players[o.Owner].Gold - before
		}
		s.note("支线失败：%s，%s", o.Label, reason)
	}
	s.event(Event{Type: "objective", X: o.X, Y: o.Y, Owner: -1, Kind: o.Status})
}

func (s *State) claimPadReward(owner int, p Point, pad, tower int) error {
	r := s.World.PadReward
	if r == nil || r.Kind != "move" {
		return s.claimPad(owner, p, pad)
	}
	if !s.weathered() || s.Phase != "prepare" || s.World.Choice != nil || s.World.BoonChoice != nil || r.Owner != owner {
		return errors.New("当前没有可领取的移动奖励")
	}
	index := -1
	for i, t := range s.Towers {
		if t.ID == tower {
			index = i
		}
	}
	if index < 0 {
		return errors.New("请选择一座防御塔")
	}
	t := s.Towers[index]
	m := &s.World.Map
	if !finite(p.X) || !finite(p.Y) || p.X < 28 || p.Y < 28 || p.X > float64(m.Width)-28 || p.Y > float64(m.Height)-28 || math.Hypot(p.X-m.Base.X, p.Y-m.Base.Y) < 55 {
		return errors.New("请选择战场内不与星核重叠的位置")
	}
	for _, q := range m.Pads {
		if q.ID != t.Pad && math.Hypot(p.X-q.X, p.Y-q.Y) < 48 {
			return errors.New("与已有建造点距离过近")
		}
	}
	q := &m.Pads[t.Pad]
	q.X, q.Y, q.Custom, q.Route = p.X, p.Y, true, ""
	for _, region := range m.Regions {
		if p.X >= float64(region.X) && p.X <= float64(region.X+region.Width) && p.Y >= float64(region.Y) && p.Y <= float64(region.Y+region.Height) {
			q.Sector = region.ID
		}
	}
	s.World.PadReward = nil
	s.event(Event{Type: "objective", Owner: owner, Kind: "move_tower", TowerID: t.ID, X: p.X, Y: p.Y})
	return nil
}

func (s *State) taskKindLimit() int {
	if s.weathered() {
		return 4
	}
	return 3
}
