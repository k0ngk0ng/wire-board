package starfort

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

type Route struct {
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	Sector int     `json:"sector"`
	Points []Point `json:"points"`
	Active bool    `json:"active"`
}
type Region struct {
	ID     int    `json:"id"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Biome  string `json:"biome"`
	Label  string `json:"label"`
}
type Feature struct {
	ID     string  `json:"id"`
	Kind   string  `json:"kind"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Sector int     `json:"sector"`
}
type ChoiceOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Map         *Map   `json:"map,omitempty"`
}
type Choice struct {
	Kind     string         `json:"kind"`
	Stage    int            `json:"stage"`
	Title    string         `json:"title"`
	Options  []ChoiceOption `json:"options"`
	Votes    []string       `json:"votes"`
	Deadline int64          `json:"deadline"`
}
type Boons struct {
	Power   int `json:"power"`
	Reach   int `json:"reach"`
	Fortune int `json:"fortune"`
}
type WaveEvent struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Wave        int    `json:"wave"`
}
type Objective struct {
	HP          float64 `json:"hp"`
	MaxHP       float64 `json:"maxHP"`
	Route       string  `json:"route"`
	Progress    float64 `json:"progress"`
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	Label       string  `json:"label"`
	Description string  `json:"description"`
	Sector      int     `json:"sector"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Cost        int     `json:"cost"`
	Status      string  `json:"status"`
	StartWave   int     `json:"startWave"`
	EndWave     int     `json:"endWave"`
	Reward      int     `json:"reward"`
	BaseAtStart int     `json:"baseAtStart"`
}
type World struct {
	Seed         int64       `json:"seed"`
	Stage        int         `json:"stage"`
	PreparedWave int         `json:"preparedWave"`
	Map          Map         `json:"map"`
	Choice       *Choice     `json:"choice"`
	BoonChoice   *Choice     `json:"boonChoice"`
	Boons        Boons       `json:"boons"`
	Event        WaveEvent   `json:"event"`
	Forecast     []WaveEvent `json:"forecast"`
	Objectives   []Objective `json:"objectives"`
	Branches     []string    `json:"branches"`
	ShortcutWave int         `json:"shortcutWave"`
	BoonWave     int         `json:"boonWave"`
	Legacy       bool        `json:"legacy"`
}

func cloneMap(m Map) Map {
	m.Path = append([]Point{}, m.Path...)
	m.Pads = append([]Pad{}, m.Pads...)
	m.Routes = append([]Route{}, m.Routes...)
	for i := range m.Routes {
		m.Routes[i].Points = append([]Point{}, m.Routes[i].Points...)
	}
	m.Regions = append([]Region{}, m.Regions...)
	m.Features = append([]Feature{}, m.Features...)
	return m
}
func initialMap(seed int64, legacy bool) Map {
	m := cloneMap(board)
	m.Base = Point{930, 500}
	m.Regions = []Region{{ID: 1, Width: 960, Height: 640, Biome: "forest", Label: "星光林地"}}
	m.Features = []Feature{}
	if !legacy {
		m.Path[len(m.Path)-1] = m.Base
		switch seed % 3 {
		case 1:
			m.Path[0] = Point{250, -30}
		case 2:
			m.Path[0] = Point{-30, 500}
			m.Path[1] = Point{250, 500}
		}
		dy := float64((seed%5)-2) * 10
		m.Path[1].Y += dy
		m.Path[2].Y += dy
		m.Path[3].Y += dy
		for i := range m.Pads {
			if i != 12 && i != 13 {
				m.Pads[i].Y += dy / 2
			}
		}
	}
	for i := range m.Pads {
		m.Pads[i].Sector = 1
	}
	m.Routes = []Route{{ID: "main", Label: "林间小径", Sector: 1, Points: append([]Point{}, m.Path...), Active: true}}
	if !legacy {
		m.Pads = placePads(m, 0)
	}
	return m
}
func (s *State) initWorld(seed int64) {
	if seed < 0 {
		seed = -seed
	}
	if seed == 0 {
		seed = 1
	}
	s.World = World{Seed: seed, Stage: 1, Map: initialMap(seed, false), Objectives: []Objective{}, Branches: []string{}, Forecast: []WaveEvent{}}
	s.worldPrepare(s.ServerAt)
}
func (s *State) migrateWorld() {
	if s.World.Stage == 0 {
		seed := s.ServerAt % 2147483647
		if seed <= 0 {
			seed = 1
		}
		// Legacy matches had no weather. Preserve the current wave through
		// reconnects; worldPrepare enables seeded weather at the next boundary.
		event := eventFor(1, seed)
		event.Wave = s.Wave
		s.World = World{Seed: seed, Stage: 1, PreparedWave: s.Wave, Map: initialMap(seed, true), Legacy: true, Event: event, Forecast: []WaveEvent{}, Objectives: []Objective{}, Branches: []string{}}
		for i := range s.Enemies {
			s.Enemies[i].Route = "main"
		}
	}
	for i := range s.Towers {
		if s.Towers[i].Priority == "" {
			s.Towers[i].Priority = "first"
		}
	}
}
func (s *State) mapData() Map { return s.World.Map }
func (s *State) routePath(id string) []Point {
	for _, r := range s.World.Map.Routes {
		if r.ID == id {
			return r.Points
		}
	}
	return nil
}
func (s *State) activeRoutes() []string {
	ids := []string{}
	for _, r := range s.World.Map.Routes {
		if r.Active {
			ids = append(ids, r.ID)
		}
	}
	return ids
}
func (s *State) towerStats(t Tower) (damage, radius float64, interval int) {
	damage, radius, interval = stats(t)
	damage *= 1 + .12*float64(s.World.Boons.Power)
	radius *= 1 + .08*float64(s.World.Boons.Reach)
	if t.Pad >= 0 && t.Pad < len(s.World.Map.Pads) && s.World.Map.Pads[t.Pad].Terrain == "highground" {
		radius *= 1.15
	}
	if s.World.Event.ID == "fog" {
		radius *= .8
	}
	if s.World.Event.ID == "starlight" && t.Kind == "arcane" {
		damage *= 1.2
	}
	damage = math.Min(maxValue, damage)
	return
}
func (s *State) rewardGold(amount int) int {
	return int(math.Min(maxValue, float64(amount)*(1+.1*float64(s.World.Boons.Fortune))))
}
func eventFor(wave int, seed int64) WaveEvent {
	ids := []string{"calm", "fog", "frost", "surge", "starlight"}
	id := "calm"
	if wave > 2 {
		id = ids[(wave+int(seed%5))%len(ids)]
	}
	e := WaveEvent{ID: id, Wave: wave}
	switch id {
	case "fog":
		e.Label = "薄雾降临"
		e.Description = "本波防御塔射程降低 20%，优先守住汇合点"
	case "frost":
		e.Label = "寒潮来临"
		e.Description = "本波所有敌人移动速度降低 15%"
	case "surge":
		e.Label = "多路突袭"
		e.Description = "本波所有已开放入口同时来袭"
	case "starlight":
		e.Label = "星辉共鸣"
		e.Description = "本波法师塔伤害提高 20%"
	default:
		e.Label = "晴空守望"
		e.Description = "无额外天气影响，检查下一波入口"
	}
	return e
}
func (s *State) rotateRoutes() {
	w := &s.World
	for i := range w.Map.Routes {
		r := &w.Map.Routes[i]
		r.Active = true
		if r.ID == "shortcut" {
			r.Active = w.ShortcutWave == s.Wave
			continue
		}
		if s.Wave > 20 && w.Event.ID != "surge" && r.ID != "main" {
			r.Active = (i+s.Wave+int(w.Seed%3))%3 != 0
		}
	}
	if w.ShortcutWave == s.Wave {
		found := false
		for i := range w.Map.Routes {
			if w.Map.Routes[i].ID == "shortcut" {
				w.Map.Routes[i].Active = true
				found = true
			}
		}
		if !found {
			p := w.Map.Path
			points := []Point{{X: 0, Y: 520}, {X: 250, Y: 520}, p[2], p[3], p[4], p[5]}
			w.Map.Routes = append(w.Map.Routes, Route{ID: "shortcut", Label: "工兵开启的捷径", Sector: 1, Points: points, Active: true})
		}
	}
}
func (s *State) worldPrepare(now int64) {
	w := &s.World
	if w.Stage == 0 {
		return
	}
	if w.PreparedWave == s.Wave {
		if w.Choice != nil {
			w.Choice.Deadline = s.NextAt
		}
		if w.BoonChoice != nil {
			w.BoonChoice.Deadline = s.NextAt
		}
		return
	}
	w.PreparedWave = s.Wave
	w.Event = eventFor(s.Wave, w.Seed)
	w.Forecast = []WaveEvent{eventFor(s.Wave+1, w.Seed), eventFor(s.Wave+2, w.Seed)}
	s.rotateRoutes()
	s.offerExpansion(now)
	if s.CompletedWaves > 0 && s.CompletedWaves%5 == 0 && w.BoonWave < s.CompletedWaves {
		w.BoonWave = s.CompletedWaves
		w.BoonChoice = &Choice{Kind: "boon", Stage: w.Stage, Title: "星核赐福 · 全队选择成长", Votes: make([]string, len(s.Players)), Deadline: s.NextAt, Options: []ChoiceOption{{ID: "power", Label: "星火之力", Description: "全队防御塔伤害永久增加 12%"}, {ID: "reach", Label: "远望之眼", Description: "全队防御塔射程永久增加 8%"}, {ID: "fortune", Label: "丰收之光", Description: "击杀与过波金币永久增加 10%"}}}
	}
}
func (s *State) offerExpansion(now int64) {
	w := &s.World
	wanted := min(4, 1+(s.Wave-1)/5)
	if w.Stage >= wanted || w.Choice != nil {
		return
	}
	next := w.Stage + 1
	options := []ChoiceOption{}
	kinds := []string{"canyon", "river"}
	if next == 3 {
		kinds = []string{"snow", "marsh"}
	}
	if next == 4 {
		kinds = []string{"ruins", "forest"}
	}
	for _, kind := range kinds {
		m := s.expandedMap(next, kind)
		label, description := "峡谷窄道", "窄道利于集中火力；高台提供 15% 射程"
		if kind == "river" {
			label = "林河双桥"
			description = "两座桥梁延长敌人行程；更多沿河建造点"
		}
		if kind == "snow" {
			label = "霜原高地"
			description = "高台强化远程火力，折返雪路便于追击"
		}
		if kind == "marsh" {
			label = "萤泽浮桥"
			description = "弯曲栈道延长行程，防线沿沼泽展开"
		}
		if kind == "ruins" {
			label = "古垒废墟"
			description = "遗迹高台连接边境，集中火力控制长廊"
		}
		if kind == "forest" {
			label = "繁星密林"
			description = "林间曲径开辟最后战区，沿路布置交叉火力"
		}
		options = append(options, ChoiceOption{ID: kind, Label: label, Description: description, Map: &m})
	}
	w.Choice = &Choice{Kind: "route", Stage: next, Title: fmt.Sprintf("战场扩展 · 第 %d 战区", next), Options: options, Votes: make([]string, len(s.Players)), Deadline: now + int64(max(30, s.Options.PrepSeconds))*1000}
	s.NextAt = w.Choice.Deadline
}
func (s *State) expandedMap(stage int, kind string) Map {
	m := cloneMap(s.World.Map)
	variant := float64((s.World.Seed+int64(stage))%3) * 25
	points := []Point{}
	pads := []Point{}
	r := Region{ID: stage, Biome: kind}
	switch stage {
	case 2:
		m.Width = 1440
		r.X = 960
		r.Width = 480
		r.Height = 640
		r.Label = "东境峡谷"
		points = []Point{{1440, 410 + variant}, {1190, 410 + variant}, {1190, 50}, {700, 50}, m.Path[3]}
		pads = []Point{{1080, 100}, {1290, 230}, {1080, 230}, {1290, 70}, {970, 100}, {980, 240}, {1140, 390}, {1320, 390}}
	case 3:
		m.Height = 860
		r.Y = 640
		r.Width = 1440
		r.Height = 220
		r.Label = "南境河谷"
		points = []Point{{1440, 800 + variant/2}, {120, 800 + variant/2}, {120, 640}, {700, 640}, m.Path[4]}
		pads = []Point{{220, 730}, {55, 670}, {440, 680}, {550, 750}, {760, 720}, {960, 750}, {1120, 720}, {1320, 750}}
	case 4:
		m.Width = 1920
		m.Height = 960
		for i := range m.Regions {
			if m.Regions[i].ID == 3 {
				m.Regions[i].Height = 320
			}
		}
		r.X = 1440
		r.Width = 480
		r.Height = 960
		r.Label = "远星边境"
		points = []Point{{1920, 720 - variant}, {1690, 720 - variant}, {1690, 120}, {1370, 120}, {1370, 500}, m.Path[5]}
		pads = []Point{{1570, 220}, {1790, 280}, {1580, 460}, {1790, 550}, {1590, 710}, {1870, 820}, {1500, 80}, {1590, 890}}
	}
	if kind == "river" || kind == "marsh" || kind == "forest" {
		r.Label = "碧流 · " + r.Label
		bend := 90.
		if stage == 3 {
			bend = -90
		}
		points = append(points[:1], append([]Point{{points[1].X + 90, points[0].Y}, {points[1].X + 90, points[1].Y + bend}}, points[1:]...)...)

		if kind == "river" || kind == "marsh" {
			riverX := float64(r.X) + float64(r.Width)*.4
			riverTop, riverBottom := float64(r.Y), float64(r.Y+r.Height)
			m.Features = append(m.Features, Feature{ID: fmt.Sprintf("river-%d", stage), Kind: "river", X: riverX, Y: (riverTop + riverBottom) / 2, Width: 65, Height: float64(r.Height), Sector: stage})
			for i := 1; i < len(points); i++ {
				a, b := points[i-1], points[i]
				if a.X == b.X || riverX < math.Min(a.X, b.X) || riverX > math.Max(a.X, b.X) {
					continue
				}
				y := a.Y + (b.Y-a.Y)*(riverX-a.X)/(b.X-a.X)
				if y >= riverTop && y <= riverBottom {
					m.Features = append(m.Features, Feature{ID: fmt.Sprintf("bridge-%d-%d", stage, i), Kind: "bridge", X: riverX, Y: y, Width: 100, Height: 50, Sector: stage})
				}
			}
		}

	}
	join := 3
	if stage == 3 {
		join = 4
	}
	if stage == 4 {
		join = 5
	}
	points = append(points, m.Path[join+1:]...)
	m.Routes = append(m.Routes, Route{ID: fmt.Sprintf("sector-%d", stage), Label: r.Label + "入口", Sector: stage, Points: points, Active: true})
	m.Regions = append(m.Regions, r)
	for i, p := range pads {
		terrain := "plain"
		if (kind == "canyon" || kind == "snow" || kind == "ruins") && i%3 == 0 {
			terrain = "highground"
			m.Features = append(m.Features, Feature{ID: fmt.Sprintf("highground-%d-%d", stage, i), Kind: "highground", X: p.X, Y: p.Y, Width: 95, Height: 70, Sector: stage})
		}
		m.Pads = append(m.Pads, Pad{ID: len(m.Pads), X: p.X, Y: p.Y, Terrain: terrain, Sector: stage})
	}
	m.Features = bridgeCrossings(m)
	m.Pads = placePads(m, 16+(stage-2)*8)
	objectiveKind := "mine"
	if stage == 3 {
		objectiveKind = "beacon"
	}
	if stage == 4 {
		objectiveKind = "convoy"
	}
	loc := objectiveLocation(m.Routes[len(m.Routes)-1], objectiveKind)
	m.Features = append(m.Features, Feature{ID: fmt.Sprintf("%s-%d", objectiveKind, stage), Kind: objectiveKind, X: loc.X, Y: loc.Y, Width: 70, Height: 70, Sector: stage})
	return m
}
func choose(c *Choice) string {
	counts := map[string]int{}
	for _, v := range c.Votes {
		counts[v]++
	}
	best := ""
	score := -1
	for _, o := range c.Options {
		if counts[o.ID] > score {
			best = o.ID
			score = counts[o.ID]
		}
	}
	return best
}
func voted(c *Choice) bool {
	if c == nil {
		return false
	}
	for _, v := range c.Votes {
		if v == "" {
			return false
		}
	}
	return true
}
func (s *State) vote(kind, choice string, p int, now int64) error {
	if s.Phase != "prepare" {
		return errors.New("只能在波间投票")
	}
	c := s.World.Choice
	if kind == "boon" {
		c = s.World.BoonChoice
	}
	if c == nil {
		return errors.New("该投票已经结束")
	}
	if now >= c.Deadline {
		return errors.New("投票已到截止时间")
	}
	valid := false
	for _, o := range c.Options {
		valid = valid || o.ID == choice
	}
	if !valid {
		return errors.New("请选择本次提供的选项")
	}
	c.Votes[p] = choice
	if voted(c) {
		s.finishChoice(kind, now)
	}
	return nil
}
func (s *State) finishChoice(kind string, now int64) {
	w := &s.World
	c := w.Choice
	if kind == "boon" {
		c = w.BoonChoice
	}
	if c == nil {
		return
	}
	id := choose(c)
	if kind == "route" {
		for _, o := range c.Options {
			if o.ID == id && o.Map != nil {
				w.Map = cloneMap(*o.Map)
			}
		}
		w.Stage = c.Stage
		w.Branches = append(w.Branches, id)
		w.Choice = nil
		s.addObjectives()
		s.rotateRoutes()
		s.event(Event{Type: "expand", Owner: -1, Amount: float64(w.Stage), Kind: id})
		s.note("全队选择%s路线，第 %d 战区已开放", id, w.Stage)
		s.offerExpansion(now)
	} else {
		switch id {
		case "power":
			w.Boons.Power++
		case "reach":
			w.Boons.Reach++
		case "fortune":
			w.Boons.Fortune++
		}
		w.BoonChoice = nil
		s.event(Event{Type: "boon", Owner: -1, Kind: id})
		s.note("全队获得成长：%s", id)
	}
	s.Epoch++
	for i := range s.Players {
		s.Players[i].Ready = false
	}
	s.NextAt = now + int64(max(30, s.Options.PrepSeconds))*1000
	if w.Choice != nil {
		w.Choice.Deadline = s.NextAt
	}
	if w.BoonChoice != nil {
		w.BoonChoice.Deadline = s.NextAt
	}
}
func (s *State) resolveChoices(now int64) {
	routeDue := s.World.Choice != nil && now >= s.World.Choice.Deadline
	boonDue := s.World.BoonChoice != nil && now >= s.World.BoonChoice.Deadline
	if routeDue {
		s.finishChoice("route", now)
	}
	if boonDue {
		s.finishChoice("boon", now)
	}
}

func (s *State) validWorld() error {
	w := s.World
	m := w.Map
	if w.Seed < 1 || w.Stage < 1 || w.Stage > 4 || w.PreparedWave < 0 || w.PreparedWave > s.Wave || w.BoonWave < 0 || w.Boons.Power < 0 || w.Boons.Reach < 0 || w.Boons.Fortune < 0 || w.Boons.Power > 1000000 || w.Boons.Reach > 1000000 || w.Boons.Fortune > 1000000 || len(w.Forecast) > 2 || w.ShortcutWave < 0 || len(m.Pads) < 16 || len(m.Pads) > 40 || len(m.Routes) < 1 || len(m.Routes) > 8 || len(m.Regions) < 1 || len(m.Regions) > 4 || len(m.Features) > 32 || len(w.Objectives) > 3 || len(w.Branches) > 3 || m.Width < 960 || m.Width > 1920 || m.Height < 640 || m.Height > 960 {
		return errors.New("无效扩展战场")
	}
	if err := validateMap(m, w.Legacy); err != nil {
		return err
	}
	if !finite(m.Base.X) || !finite(m.Base.Y) || len(m.Path) < 2 || len(m.Path) > 20 {
		return errors.New("无效基地或主路")
	}
	for i, p := range m.Pads {
		if p.ID != i || !finite(p.X) || !finite(p.Y) || p.X < 0 || p.X > float64(m.Width) || p.Y < 0 || p.Y > float64(m.Height) || !slices.Contains([]string{"plain", "highground"}, p.Terrain) {
			return errors.New("无效建造点")
		}
	}
	seen := map[string]bool{}
	active := false
	for _, r := range m.Routes {
		if r.ID == "" || seen[r.ID] || len(r.Points) < 2 || len(r.Points) > 20 {
			return errors.New("无效路线")
		}
		seen[r.ID] = true
		active = active || r.Active
		for _, p := range r.Points {
			if !finite(p.X) || !finite(p.Y) || p.X < -40 || p.X > float64(m.Width)+40 || p.Y < -40 || p.Y > float64(m.Height)+40 {
				return errors.New("路线越界")
			}
		}
	}
	if !active {
		return errors.New("战场没有入口")
	}
	for _, c := range []*Choice{w.Choice, w.BoonChoice} {
		if c == nil {
			continue
		}
		if len(c.Votes) != len(s.Players) || len(c.Options) < 2 || len(c.Options) > 3 || c.Deadline < 0 || s.Phase == "battle" || s.finished() {
			return errors.New("无效战场选择")
		}
		ids := []string{""}
		for _, o := range c.Options {
			ids = append(ids, o.ID)
			if c.Kind == "route" {
				if o.Map == nil || validateMap(*o.Map, w.Legacy) != nil || len(o.Map.Pads) != len(m.Pads)+8 {
					return errors.New("无效扩展预览")
				}
				for i, p := range m.Pads {
					if o.Map.Pads[i] != p {
						return errors.New("扩展移动了已有建造点")
					}
				}
			}
		}
		for _, v := range c.Votes {
			if !slices.Contains(ids, v) {
				return errors.New("无效选票")
			}
		}
	}
	if !slices.Contains([]string{"calm", "fog", "frost", "surge", "starlight"}, w.Event.ID) {
		return errors.New("无效波次天气")
	}
	objectiveIDs := map[string]bool{}
	for _, o := range w.Objectives {
		if o.ID == "" || len(o.ID) > 80 || objectiveIDs[o.ID] || !finite(o.HP) || !finite(o.MaxHP) || !finite(o.X) || !finite(o.Y) || !finite(o.Progress) || o.HP < 0 || o.MaxHP <= 0 || o.MaxHP > 10000 || o.HP > o.MaxHP || o.X < 0 || o.X > float64(m.Width) || o.Y < 0 || o.Y > float64(m.Height) || o.Progress < 0 || len(s.routePath(o.Route)) < 2 || o.Progress > pathDistance(s.routePath(o.Route)) || o.StartWave < 0 || o.EndWave < 0 || o.Sector < 1 || o.Sector > w.Stage || (o.Status != "failed" && o.HP <= 0) {
			return errors.New("无效支线状态")
		}
		objectiveIDs[o.ID] = true
		if !slices.Contains([]string{"mine", "beacon", "convoy"}, o.Kind) || !slices.Contains([]string{"available", "active", "completed", "failed"}, o.Status) || o.Cost < 0 || o.Cost > 1000 || o.Reward < 0 || o.Reward > 1000 {
			return errors.New("无效支线目标")
		}
	}
	return nil
}

// Only newly unlocked pads are placed. Existing coordinates are immutable.
func roadDistance(m Map, p Point) float64 {
	nearest := math.Inf(1)
	for _, r := range m.Routes {
		for i := 1; i < len(r.Points); i++ {
			a, b := r.Points[i-1], r.Points[i]
			dx, dy := b.X-a.X, b.Y-a.Y
			ratio := 0.
			if dx*dx+dy*dy > 0 {
				ratio = math.Max(0, math.Min(1, ((p.X-a.X)*dx+(p.Y-a.Y)*dy)/(dx*dx+dy*dy)))
			}
			nearest = math.Min(nearest, math.Hypot(p.X-a.X-ratio*dx, p.Y-a.Y-ratio*dy))
		}
	}
	return nearest
}
func placePads(m Map, start int) []Pad {
	pads := append([]Pad{}, m.Pads...)
	for i := start; i < len(pads); i++ {
		origin := Point{pads[i].X, pads[i].Y}
		best := math.Inf(1)
		chosen := origin
		for dx := -6; dx <= 6; dx++ {
			for dy := -6; dy <= 6; dy++ {
				p := Point{origin.X + float64(dx)*30, origin.Y + float64(dy)*30}
				if p.X < 35 || p.Y < 35 || p.X > float64(m.Width)-35 || p.Y > float64(m.Height)-35 {
					continue
				}
				distance := roadDistance(m, p)
				if distance < 48 || distance > 185 {
					continue
				}
				clash := false
				for j := 0; j < i; j++ {
					if math.Hypot(pads[j].X-p.X, pads[j].Y-p.Y) < 60 {
						clash = true
						break
					}
				}
				if clash {
					continue
				}
				score := math.Hypot(float64(dx), float64(dy))
				if score < best {
					best = score
					chosen = p
				}
			}
		}
		pads[i].X, pads[i].Y = chosen.X, chosen.Y
		for j := range m.Features {
			f := &m.Features[j]
			if f.Kind == "highground" && f.X == origin.X && f.Y == origin.Y {
				f.X, f.Y = chosen.X, chosen.Y
			}
		}
	}
	return pads
}

func validateMap(m Map, legacy bool) error {
	if m.Width < 960 || m.Width > 1920 || m.Height < 640 || m.Height > 960 || len(m.Pads) < 16 || len(m.Pads) > 40 || len(m.Routes) < 1 || len(m.Routes) > 8 || len(m.Regions) < 1 || len(m.Regions) > 4 || len(m.Features) > 32 {
		return errors.New("无效地图范围")
	}
	inside := func(p Point) bool {
		return finite(p.X) && finite(p.Y) && p.X >= 0 && p.X <= float64(m.Width) && p.Y >= 0 && p.Y <= float64(m.Height)
	}
	if !inside(m.Base) || len(m.Path) < 2 || len(m.Path) > 20 {
		return errors.New("无效地图基地")
	}
	for _, r := range m.Routes {
		if len(r.ID) > 80 || len(r.Points) < 2 || len(r.Points) > 20 {
			return errors.New("无效路线长度")
		}
		for i, p := range r.Points {
			exception := (i == 0 && finite(p.X) && finite(p.Y) && p.X >= -40 && p.X <= float64(m.Width) && p.Y >= -40 && p.Y <= float64(m.Height)) || (legacy && i == len(r.Points)-1 && p == board.Path[len(board.Path)-1])
			if !inside(p) && !exception {
				return errors.New("路线超出战场")
			}
			if i > 0 && p == r.Points[i-1] {
				return errors.New("路线包含空路段")
			}
		}
	}
	regions := map[int]bool{}
	for _, r := range m.Regions {
		if r.ID < 1 || r.ID > 4 || regions[r.ID] || r.X < 0 || r.Y < 0 || r.Width <= 0 || r.Height <= 0 || r.X+r.Width > m.Width || r.Y+r.Height > m.Height || !slices.Contains([]string{"forest", "canyon", "river", "snow", "ruins", "marsh"}, r.Biome) {
			return errors.New("无效战区")
		}
		regions[r.ID] = true
	}
	features := map[string]bool{}
	for _, f := range m.Features {
		if f.ID == "" || len(f.ID) > 80 || features[f.ID] || !inside(Point{f.X, f.Y}) || !finite(f.Width) || !finite(f.Height) || f.Width <= 0 || f.Height <= 0 || f.Width > float64(m.Width) || f.Height > float64(m.Height) || !slices.Contains([]string{"bridge", "river", "highground", "mine", "beacon", "convoy"}, f.Kind) || !regions[f.Sector] {
			return errors.New("无效地图地物")
		}
		features[f.ID] = true
	}
	return nil
}

func bridgeCrossings(m Map) []Feature {
	features := append([]Feature{}, m.Features...)
	for _, river := range m.Features {
		if river.Kind != "river" {
			continue
		}
		for _, route := range m.Routes {
			for i := 1; i < len(route.Points); i++ {
				a, b := route.Points[i-1], route.Points[i]
				if a.X == b.X || river.X < math.Min(a.X, b.X) || river.X > math.Max(a.X, b.X) {
					continue
				}
				y := a.Y + (b.Y-a.Y)*(river.X-a.X)/(b.X-a.X)
				if y < river.Y-river.Height/2 || y > river.Y+river.Height/2 {
					continue
				}
				exists := false
				for _, f := range features {
					if f.Kind == "bridge" && math.Hypot(f.X-river.X, f.Y-y) < 2 {
						exists = true
						break
					}
				}
				if !exists {
					features = append(features, Feature{ID: fmt.Sprintf("crossing-%s-%s-%d", river.ID, route.ID, i), Kind: "bridge", X: river.X, Y: y, Width: 100, Height: 50, Sector: river.Sector})
				}
			}
		}
	}
	return features
}
