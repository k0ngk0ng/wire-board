package starfort

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
)

// Geology has its own persisted RNG: terrain rolls never perturb task/skill rolls.
type Geology struct {
	Wave       int                `json:"wave"`
	StartedMS  int64              `json:"startedMS"`
	Kind       string             `json:"kind"`
	TriggerMS  int64              `json:"triggerMS"`
	Triggered  bool               `json:"triggered,omitempty"`
	Zones      []GeoZone          `json:"zones,omitempty"`
	Disabled   []GeoDisabled      `json:"disabled,omitempty"`
	Recoveries int                `json:"recoveries,omitempty"`
	RoutePeaks map[string]float64 `json:"routePeaks"`
	Bonus      int                `json:"bonus,omitempty"`
	Rewarded   bool               `json:"rewarded,omitempty"`
}
type GeoDisabled struct {
	TowerID int    `json:"towerId"`
	Route   string `json:"route"`
	Floor   int    `json:"floor"`
}
type GeoZone struct {
	Sweep         bool             `json:"sweep,omitempty"`
	ID            int              `json:"id"`
	Kind          string           `json:"kind"`
	X             float64          `json:"x"`
	Y             float64          `json:"y"`
	Radius        float64          `json:"radius,omitempty"`
	Width         float64          `json:"width,omitempty"`
	Height        float64          `json:"height,omitempty"`
	Route         string           `json:"route,omitempty"`
	Progress      float64          `json:"progress,omitempty"`
	Speed         float64          `json:"speed,omitempty"`
	CreatedMS     int64            `json:"createdMS"`
	ExpiresMS     int64            `json:"expiresMS,omitempty"`
	FromBottom    bool             `json:"fromBottom,omitempty"`
	Activated     bool             `json:"activated,omitempty"`
	Skipped       bool             `json:"skipped,omitempty"`
	EnemyHits     []int            `json:"enemyHits,omitempty"`
	ObjectiveHits []string         `json:"objectiveHits,omitempty"`
	TowerHits     []int            `json:"towerHits,omitempty"`
	MissGuards    []KnockbackGuard `json:"missGuards,omitempty"`
}

func (s *State) geological() bool { return s.GeologyRules == 1 }
func (s *State) geoRandom(n int) int {
	if s.GeoRNG == 0 {
		s.GeoRNG = uint64(s.World.Seed) ^ 0x9e3779b97f4a7c15
	}
	x := s.GeoRNG
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	s.GeoRNG = x
	return int(x % uint64(n))
}
func (s *State) startGeology() {
	if !s.geological() {
		return
	}
	g := &Geology{Wave: s.Wave, StartedMS: s.ElapsedMS, Kind: "none", RoutePeaks: map[string]float64{}}
	s.Geology = g
	chance := 30
	if s.revised() {
		chance = 50
	}
	if s.geoRandom(100) >= chance {
		return
	}
	g.Kind = []string{"earthquake", "flood", "typhoon"}[s.geoRandom(3)]
	seconds := 30
	if g.Kind == "earthquake" {
		seconds = 10
	}
	spawnInterval := 800
	if s.WaveInfo.Rule == "swarm" {
		spawnInterval = 400
	}
	// The last spawn is a lower bound on wave duration even with instant kills.
	// Keep the first effect inside that bound so a selected event cannot vanish
	// merely because the team cleared a short wave before its scheduled time.
	latest := min(seconds*1000/StepMS, max(5, (s.WaveInfo.Total-1)*spawnInterval/StepMS))
	g.TriggerMS = s.ElapsedMS + int64(5+s.geoRandom(latest-4))*StepMS
	m := s.World.Map
	switch g.Kind {
	case "earthquake":
		count := 1 + s.geoRandom(4)
		for tries := 0; tries < 512 && len(g.Zones) < count; tries++ {
			r := float64(75 + s.geoRandom(126))
			z := GeoZone{ID: len(g.Zones) + 1, Kind: g.Kind, Radius: r, X: r + float64(s.geoRandom(max(1, int(float64(m.Width)-2*r)))), Y: r + float64(s.geoRandom(max(1, int(float64(m.Height)-2*r)))), CreatedMS: g.TriggerMS}
			if slices.ContainsFunc(g.Zones, func(q GeoZone) bool { return math.Hypot(z.X-q.X, z.Y-q.Y) < z.Radius+q.Radius+8 }) {
				continue
			}
			g.Zones = append(g.Zones, z)
		}
	case "flood":
		if s.revised() {
			s.revisionFloods(g)
			break
		}
		count := 1 + s.geoRandom(5)
		fromBottom := s.geoRandom(2) == 1
		for tries := 0; tries < 1024 && len(g.Zones) < count; tries++ {
			width := float64(200 + s.geoRandom(max(1, m.Width-199)))
			height := float64(100 + s.geoRandom(min(301, m.Height-99)))
			z := GeoZone{ID: len(g.Zones) + 1, Kind: g.Kind, Width: width, Height: height, X: float64(s.geoRandom(max(1, m.Width-int(width)+1))), Y: float64(s.geoRandom(max(1, m.Height-int(height)+1))), CreatedMS: g.TriggerMS, ExpiresMS: g.TriggerMS + int64(10000+s.geoRandom(51)*200), FromBottom: fromBottom}
			if z.X < m.Base.X+60 && z.X+z.Width > m.Base.X-60 {
				if (!fromBottom && z.Y+z.Height >= m.Base.Y-60) || (fromBottom && z.Y <= m.Base.Y+60) {
					continue
				}
			}
			dx := math.Max(z.X, math.Min(m.Base.X, z.X+z.Width)) - m.Base.X
			dy := math.Max(z.Y, math.Min(m.Base.Y, z.Y+z.Height)) - m.Base.Y
			if math.Hypot(dx, dy) < 60 || slices.ContainsFunc(g.Zones, func(q GeoZone) bool {
				return z.X < q.X+q.Width && z.X+z.Width > q.X
			}) {
				continue
			}
			g.Zones = append(g.Zones, z)
		}
	case "typhoon":
		routes := s.activeRoutes()
		count, minSpeed, diameterSpan := 16, 40, 91
		if s.revised() {
			count, minSpeed, diameterSpan = 20, 20, 291
		}
		for i := 0; i < count && len(routes) > 0; i++ {
			route := routes[s.geoRandom(len(routes))]
			length := pathDistance(s.routePath(route))
			progress := length * (.1 + .3*float64(s.geoRandom(1001))/1000)
			p := pathPosition(s.routePath(route), progress)
			g.Zones = append(g.Zones, GeoZone{ID: i + 1, Kind: g.Kind, Route: route, Progress: progress, X: p.X, Y: p.Y, Radius: float64(60+s.geoRandom(diameterSpan)) / 2, Speed: float64(minSpeed + s.geoRandom(71-minSpeed)), CreatedMS: s.ElapsedMS + int64(5+s.geoRandom(146))*StepMS})
		}
		sort.SliceStable(g.Zones, func(i, j int) bool { return g.Zones[i].CreatedMS < g.Zones[j].CreatedMS })
		for i := range g.Zones {
			g.Zones[i].ID = i + 1
		}
		if len(g.Zones) > 0 {
			g.Zones[0].CreatedMS = min(g.Zones[0].CreatedMS, s.ElapsedMS+int64(latest)*StepMS)
			g.TriggerMS = g.Zones[0].CreatedMS
		}
	}
}
func (s *State) geoActive(z GeoZone) bool {
	return s.geological() && s.Geology != nil && s.Geology.Triggered && s.ElapsedMS >= z.CreatedMS && !z.Skipped && (z.ExpiresMS == 0 || s.ElapsedMS < z.ExpiresMS) && (z.Kind != "typhoon" || z.Activated)
}

// The growing flood rectangle is shared with the Canvas drawing calculation.
func (s *State) geoContains(z GeoZone, p Point) bool {
	if !s.geoActive(z) {
		return false
	}
	if z.Kind != "flood" {
		return math.Hypot(p.X-z.X, p.Y-z.Y) <= z.Radius
	}
	t := math.Max(0, math.Min(1, float64(s.ElapsedMS-z.CreatedMS)/4000))
	fromY := -z.Height
	if z.FromBottom {
		fromY = float64(s.World.Map.Height)
	}
	y := fromY + (z.Y-fromY)*t
	top, bottom := math.Max(0, y), math.Min(float64(s.World.Map.Height), y+z.Height)
	if z.Sweep {
		top, bottom = z.Y, math.Min(z.Y+z.Height, float64(s.World.Map.Height)*t)
		if z.FromBottom {
			top, bottom = math.Max(z.Y, float64(s.World.Map.Height)*(1-t)), z.Y+z.Height
		}
		if top >= bottom {
			return false
		}
	}

	return p.X >= z.X && p.X <= z.X+z.Width && p.Y >= top && p.Y <= bottom
}
func (s *State) geoMotion(p Point) float64 {
	factor := 1.
	if s.Geology != nil {
		for _, z := range s.Geology.Zones {
			if s.geoContains(z, p) {
				if z.Kind == "typhoon" {
					return 0
				}
				if z.Kind == "earthquake" {
					factor = .5
				}
			}
		}
	}
	return factor
}
func (s *State) geoTowerDisabled(id int) bool {
	return s.geological() && s.Geology != nil && !s.Geology.Rewarded && slices.ContainsFunc(s.Geology.Disabled, func(d GeoDisabled) bool { return d.TowerID == id })
}
func (s *State) geoTowerFactor(t Tower) float64 {
	if !s.geological() || t.FloodedAtMS == 0 {
		return 1
	}
	return .8 + .2*math.Max(0, math.Min(1, float64(s.ElapsedMS-t.FloodedAtMS)/10000))
}
func (s *State) geoRoad(p Pad) string {
	if p.Route != "" {
		return p.Route
	}
	best := math.Inf(1)
	id := ""
	for _, r := range s.World.Map.Routes {
		for i := 1; i < len(r.Points); i++ {
			a, b := r.Points[i-1], r.Points[i]
			dx, dy := b.X-a.X, b.Y-a.Y
			t := math.Max(0, math.Min(1, ((p.X-a.X)*dx+(p.Y-a.Y)*dy)/math.Max(1, dx*dx+dy*dy)))
			d := math.Hypot(p.X-a.X-t*dx, p.Y-a.Y-t*dy)
			if d < best {
				best, id = d, r.ID
			}
		}
	}
	return id
}
func (s *State) quakeDisable() {
	g := s.Geology
	byRoad := map[string][]int{}
	for _, t := range s.Towers {
		p := s.World.Map.Pads[t.Pad]
		if slices.ContainsFunc(g.Zones, func(z GeoZone) bool { return s.geoContains(z, Point{p.X, p.Y}) }) {
			r := s.geoRoad(p)
			byRoad[r] = append(byRoad[r], t.ID)
		}
	}
	roads := make([]string, 0, len(byRoad))
	for r := range byRoad {
		roads = append(roads, r)
	}
	sort.Strings(roads)
	for _, r := range roads {
		ids := byRoad[r]
		floor := int(math.Ceil(float64(len(ids)) * .1))
		count := (len(ids) + 1) / 2
		for j := 0; j < count; j++ {
			k := j + s.geoRandom(len(ids)-j)
			ids[j], ids[k] = ids[k], ids[j]
			g.Disabled = append(g.Disabled, GeoDisabled{ids[j], r, floor})
		}
	}
}
func (s *State) quakeRecover() {
	g := s.Geology
	for g.Recoveries < 3 && s.ElapsedMS >= g.TriggerMS+int64(g.Recoveries+1)*5000 {
		roads := []string{}
		for _, d := range g.Disabled {
			if !slices.Contains(roads, d.Route) {
				roads = append(roads, d.Route)
			}
		}
		sort.Strings(roads)
		for _, r := range roads {
			indices := []int{}
			floor := 0
			for i, d := range g.Disabled {
				if d.Route == r {
					indices = append(indices, i)
					floor = max(floor, d.Floor)
				}
			}
			if len(indices) > floor {
				index := indices[s.geoRandom(len(indices))]
				g.Disabled = slices.Delete(g.Disabled, index, index+1)
			}
		}
		g.Recoveries++
	}
}
func (s *State) geoEnemyDamage(i int, amount float64, kind string) {
	e := &s.Enemies[i]
	if e.HP <= 0 {
		return
	}
	amount = math.Min(e.HP, amount)
	e.HP -= amount
	s.event(Event{Type: "hit", Skill: kind, TargetID: e.ID, X: e.X, Y: e.Y, Owner: -1, Amount: amount})
	if e.HP <= 0 {
		s.Kills++
		for j := range s.Players {
			s.Players[j].Gold = min(maxValue, s.Players[j].Gold+s.rewardGold(e.Reward))
		}
		s.event(Event{Type: "death", Skill: kind, TargetID: e.ID, X: e.X, Y: e.Y, Owner: -1, Amount: float64(s.rewardGold(e.Reward))})
	}
}
func (s *State) stepGeology() {
	g := s.Geology
	if !s.geological() || g == nil || g.Rewarded || g.Kind == "none" {
		return
	}
	if !g.Triggered && s.ElapsedMS >= g.TriggerMS {
		g.Triggered = true
		if g.Kind == "earthquake" {
			s.quakeDisable()
		}
		s.event(Event{Type: "geology", Kind: g.Kind, Owner: -1})
		label := "地质效果"
		if s.revised() {
			label = "自然灾害"
		}
		s.note("本波%s：%s", label, map[string]string{"earthquake": "地震", "flood": "洪水", "typhoon": "台风"}[g.Kind])
	}
	if !g.Triggered {
		return
	}
	if g.Kind == "earthquake" {
		s.quakeRecover()
	}
	for j := range g.Zones {
		z := &g.Zones[j]
		if z.Kind == "typhoon" && s.ElapsedMS >= z.CreatedMS && !z.Activated && !z.Skipped {
			count := 0
			for k, q := range g.Zones {
				if k != j && q.Route == z.Route && s.geoActive(q) {
					count++
				}
			}
			if count >= 3 {
				z.Skipped = true
			} else {
				z.Activated = true
			}
		}
		if !s.geoActive(*z) {
			continue
		}
		if z.Kind == "typhoon" {
			if s.ElapsedMS > z.CreatedMS {
				z.Progress += z.Speed * StepMS / 1000
			}
			if z.Progress >= pathDistance(s.routePath(z.Route))*.9 {
				z.ExpiresMS = s.ElapsedMS
				continue
			}
			p := pathPosition(s.routePath(z.Route), z.Progress)
			z.X, z.Y = p.X, p.Y
		}
		if z.Kind == "earthquake" {
			continue
		}
		for i, e := range s.Enemies {
			if e.HP <= 0 || !s.geoContains(*z, Point{e.X, e.Y}) {
				continue
			}
			if s.revised() && z.Kind == "flood" {
				s.Enemies[i].WetUntil = s.Tick + 5000/StepMS
			}
			if slices.Contains(z.EnemyHits, e.ID) {
				continue
			}
			z.EnemyHits = append(z.EnemyHits, e.ID)
			amount := e.MaxHP * .1
			if z.Kind == "typhoon" {
				amount = e.HP * .1
			} else if heavy(e) {
				amount *= .5
			}
			s.geoEnemyDamage(i, amount, z.Kind)
		}
		for i := range s.World.Objectives {
			o := &s.World.Objectives[i]
			if !escort(o.Kind) || o.Status != "active" || o.HP <= 0 || slices.Contains(z.ObjectiveHits, o.ID) || !s.geoContains(*z, Point{o.X, o.Y}) {
				continue
			}
			z.ObjectiveHits = append(z.ObjectiveHits, o.ID)
			amount := o.MaxHP * .1
			if z.Kind == "typhoon" {
				amount = o.HP * .1
			}
			o.HP = math.Max(0, o.HP-amount)
			s.event(Event{Type: "objective_hit", Kind: z.Kind, X: o.X, Y: o.Y, Owner: -1, Amount: amount})
		}
		if z.Kind == "flood" {
			for i := range s.Towers {
				t := &s.Towers[i]
				p := s.World.Map.Pads[t.Pad]
				if !slices.Contains(z.TowerHits, t.ID) && s.geoContains(*z, Point{p.X, p.Y}) {
					z.TowerHits = append(z.TowerHits, t.ID)
					t.FloodedAtMS = s.ElapsedMS
				}
			}
		}
	}
}
func (s *State) geoMiss(t Tower, index int) bool {
	if !s.geological() || s.Geology == nil {
		return false
	}
	p := s.World.Map.Pads[t.Pad]
	for i := range s.Geology.Zones {
		z := &s.Geology.Zones[i]
		if z.Kind != "typhoon" || !s.geoContains(*z, Point{p.X, p.Y}) || hasGuard(z.MissGuards, t.ID, s.Tick) {
			continue
		}
		if s.geoRandom(100) < 20 {
			z.MissGuards = putGuard(s.freshGuards(z.MissGuards), t.ID, s.Tick+5000/StepMS)
			e := s.Enemies[index]
			s.event(Event{Type: "miss", Skill: "typhoon", TowerID: t.ID, TargetID: e.ID, X: e.X, Y: e.Y, Owner: t.Owner})
			return true
		}
	}
	return false
}
func (s *State) geoPeak(e Enemy) {
	g := s.Geology
	if !s.geological() || g == nil || g.Rewarded {
		return
	}
	if g.RoutePeaks == nil {
		g.RoutePeaks = map[string]float64{}
	}
	length := pathDistance(s.routePath(e.Route))
	if length > 0 {
		g.RoutePeaks[e.Route] = math.Max(g.RoutePeaks[e.Route], math.Min(1, e.Progress/length))
	}
}
func (s *State) finishGeology() int {
	g := s.Geology
	if !s.geological() || g == nil || g.Rewarded {
		return 0
	}
	peak := 0.
	for _, v := range g.RoutePeaks {
		peak = math.Max(peak, v)
	}
	bonus := 0
	if peak <= .5 {
		bonus = s.Wave * 10
	} else if peak <= .7 {
		bonus = s.Wave * 5
	}
	g.Bonus = bonus
	g.Rewarded = true
	g.Zones = nil
	g.Disabled = nil
	for i := range s.Towers {
		s.Towers[i].FloodedAtMS = 0
	}
	if bonus > 0 {
		s.note("防守奖励：每位玩家额外获得 %d 金币", bonus)
	}
	return bonus
}
func (s *State) validGeology() error {
	if s.GeologyRules < 0 || s.GeologyRules > 1 || s.GeoRNG != 0 && !s.geological() || s.geological() && !s.weatherV3() {
		return errors.New("无效地质规则")
	}
	for _, t := range s.Towers {
		if t.FloodedAtMS < 0 || t.FloodedAtMS > s.ElapsedMS || !s.geological() && t.FloodedAtMS != 0 {
			return errors.New("无效洪水恢复")
		}
	}
	g := s.Geology
	if g == nil {
		return nil
	}
	if !s.geological() || g.Wave < 1 || g.Wave > s.Wave || g.StartedMS < 0 || g.TriggerMS < 0 || g.Recoveries < 0 || g.Recoveries > 3 || g.Bonus < 0 || g.Bonus > maxValue || !slices.Contains([]string{"none", "earthquake", "flood", "typhoon"}, g.Kind) || len(g.Zones) > s.geoZoneLimit() || len(g.Disabled) > 40 || len(g.RoutePeaks) > len(s.World.Map.Routes) {
		return errors.New("无效地质状态")
	}
	for r, p := range g.RoutePeaks {
		if len(s.routePath(r)) < 2 || !finite(p) || p < 0 || p > 1 {
			return errors.New("无效最远距离")
		}
	}
	ids := map[int]bool{}
	for _, z := range g.Zones {
		if z.Sweep && (!s.revised() || z.Kind != "flood") {
			return errors.New("无效洪水进入规则")
		}
		if z.ID < 1 || ids[z.ID] || z.Kind != g.Kind || !finite(z.X) || !finite(z.Y) || !finite(z.Progress) || !finite(z.Speed) || z.CreatedMS < 0 || z.ExpiresMS < 0 || len(z.EnemyHits) > 512 || len(z.ObjectiveHits) > 4 || len(z.TowerHits) > 40 || len(z.MissGuards) > 40 {
			return errors.New("无效地质区域")
		}
		ids[z.ID] = true
		switch z.Kind {
		case "earthquake":
			if !finite(z.Radius) || z.Radius < 75 || z.Radius > 200 {
				return errors.New("无效地震直径")
			}
		case "flood":
			if !finite(z.Width) || !finite(z.Height) || z.Width < 200 || z.Width > float64(s.World.Map.Width) || z.Height < 100 || z.Height > 400 || z.ExpiresMS <= z.CreatedMS {
				return errors.New("无效洪水范围")
			}
		case "typhoon":
			if !finite(z.Radius) || z.Radius < 30 || z.Radius > s.typhoonMaxRadius() || z.Speed < s.typhoonMinSpeed() || z.Speed > 70 || z.Progress < 0 || len(s.routePath(z.Route)) < 2 {
				return errors.New("无效台风")
			}
		default:
			return errors.New("无效地质类型")
		}
		seen := map[int]bool{}
		for _, id := range z.EnemyHits {
			if id < 1 || id > s.EnemySerial || seen[id] {
				return errors.New("无效地质命中")
			}
			seen[id] = true
		}
		for _, guard := range z.MissGuards {
			if guard.TowerID < 1 || guard.TowerID > s.TowerSerial || guard.Until < 0 {
				return errors.New("无效台风保护")
			}
		}
	}
	seen := map[int]bool{}
	for _, d := range g.Disabled {
		if d.TowerID < 1 || d.TowerID > s.TowerSerial || seen[d.TowerID] || d.Floor < 0 || d.Floor > 40 || len(s.routePath(d.Route)) < 2 {
			return fmt.Errorf("无效地震停攻")
		}
		seen[d.TowerID] = true
	}
	return nil
}
func (s *State) viewGeology() *Geology {
	if s.Geology == nil {
		return nil
	}
	g := *s.Geology
	g.Zones = append([]GeoZone{}, g.Zones...)
	for i := range g.Zones {
		g.Zones[i].EnemyHits = nil
		g.Zones[i].ObjectiveHits = nil
		g.Zones[i].TowerHits = nil
		g.Zones[i].MissGuards = nil
	}
	return &g
}

func (s *State) geoVulnerability(e Enemy) float64 {
	if s.geological() && s.Geology != nil && slices.ContainsFunc(s.Geology.Zones, func(z GeoZone) bool { return z.Kind == "earthquake" && s.geoContains(z, Point{e.X, e.Y}) }) {
		return 1.2
	}
	return 1
}
