package starfort

import (
	"fmt"
	"math"
	"sync"
)

// New games use one fixed arena. Weather and wave growth never replace its
// routes, pads or core; saved schema-2 expanding worlds keep their own layout.
var centralCache [4]struct {
	once  sync.Once
	value Map
}

func centralMap(players int) Map {
	c := &centralCache[players-1]
	c.once.Do(func() { c.value = buildCentralMap(players) })
	return cloneMap(c.value)
}
func buildCentralMap(players int) Map {
	m := Map{Width: 1600, Height: 960, Base: Point{800, 480}, Regions: []Region{{ID: 1, Width: 1600, Height: 960, Biome: "forest", Label: "星核庭院"}}, Features: []Feature{}}
	angles := []float64{math.Pi}
	labels := []string{"林间入口"}
	switch players {
	case 2:
		angles = []float64{math.Pi, 0}
		labels = []string{"西林入口", "东林入口"}
	case 3:
		angles = []float64{-math.Pi / 2, math.Pi / 6, 5 * math.Pi / 6}
		labels = []string{"北林入口", "东南入口", "西南入口"}
	case 4:
		angles = []float64{-math.Pi / 2, 0, math.Pi / 2, math.Pi}
		labels = []string{"北林入口", "东林入口", "南林入口", "西林入口"}
	}
	for i, a := range angles {
		points := make([]Point, 0, 19)
		for j := 0; j <= 18; j++ {
			t := float64(j) / 18
			r := 1 - t
			theta := a + .48*math.Sin(t*math.Pi*4)*math.Min(1, r*3)
			points = append(points, Point{m.Base.X + 700*r*math.Cos(theta), m.Base.Y + 390*r*math.Sin(theta)})
		}
		id := fmt.Sprintf("lane-%d", i+1)
		if i == 0 {
			id = "main"
		}
		m.Routes = append(m.Routes, Route{ID: id, Label: labels[i], Sector: 1, Points: points, Active: true})
	}
	m.Path = append([]Point{}, m.Routes[0].Points...)
	origins := []Point{}
	for _, route := range m.Routes {
		length := pathDistance(route.Points)
		for _, f := range []float64{.2, .38, .56, .74} {
			p := pathPosition(route.Points, length*f)
			next := pathPosition(route.Points, length*f+1)
			dx, dy := next.X-p.X, next.Y-p.Y
			l := math.Hypot(dx, dy)
			for _, side := range []float64{-1, 1} {
				origins = append(origins, Point{p.X - dy/l*side*76, p.Y + dx/l*side*76})
			}
		}
	}
	for i := 0; i < 8; i++ {
		a := (float64(i) + .5) * math.Pi / 4
		origins = append(origins, Point{m.Base.X + 165*math.Cos(a), m.Base.Y + 165*math.Sin(a)})
	}
	for _, origin := range origins {
		best := math.Inf(1)
		chosen := origin
		for y := 55.; y < float64(m.Height)-55; y += 15 {
			for x := 55.; x < float64(m.Width)-55; x += 15 {
				p := Point{x, y}
				score := math.Hypot(x-origin.X, y-origin.Y)
				if score >= best || math.Hypot(x-m.Base.X, y-m.Base.Y) < 130 {
					continue
				}
				road := roadDistance(m, p)
				if road < 50 || road > 175 {
					continue
				}
				valid := true
				for _, other := range m.Pads {
					if math.Hypot(other.X-x, other.Y-y) < 65 {
						valid = false
						break
					}
				}
				if valid {
					best = score
					chosen = p
				}
			}
		}
		m.Pads = append(m.Pads, Pad{ID: len(m.Pads), X: chosen.X, Y: chosen.Y, Terrain: "plain", Sector: 1})
	}
	for i, kind := range []string{"mine", "beacon", "convoy"} {
		route := m.Routes[i%len(m.Routes)]
		p := objectiveLocation(route, kind)
		m.Features = append(m.Features, Feature{ID: kind + "-central", Kind: kind, X: p.X, Y: p.Y, Width: 70, Height: 70, Sector: 1})
	}
	return m
}
func (s *State) initCentralWorld(seed int64) {
	if seed < 0 {
		seed = -seed
	}
	if seed == 0 {
		seed = 1
	}
	s.World = World{Layout: "central", Seed: seed, Stage: 1, Map: centralMap(len(s.Players)), Objectives: []Objective{}, Branches: []string{}, Forecast: []WaveEvent{}}
	s.worldPrepare(s.ServerAt)
}
func (s *State) offerCentralObjectives() {
	for i, kind := range []string{"mine", "beacon", "convoy"} {
		if s.CompletedWaves < (i+1)*5 {
			continue
		}
		id := kind + "-central"
		exists := false
		for _, o := range s.World.Objectives {
			exists = exists || o.ID == id
		}
		if exists {
			continue
		}
		route := s.World.Map.Routes[i%len(s.World.Map.Routes)]
		p := objectiveLocation(route, kind)
		o := Objective{ID: id, Kind: kind, Sector: 1, X: p.X, Y: p.Y, Status: "available", Route: route.ID, HP: 120, MaxHP: 120}
		switch kind {
		case "mine":
			o.Label = "修复星砂矿场"
			o.Cost = min(120, 60*len(s.Players))
			o.Reward = 90
			o.Description = fmt.Sprintf("支付 %d 金币，守护矿场一波；成功后每人获得 90 金币", o.Cost)
		case "beacon":
			o.Label = "点亮瞭望塔"
			o.Cost = 160
			o.Reward = 1
			o.Description = "支付 160 金币，守护一波后全队射程增加 8%"
		case "convoy":
			o.Label = "护送星光补给车"
			o.Cost = 100
			o.Reward = 60
			o.HP = 100
			o.MaxHP = 100
			o.Progress = 80
			o.Description = "支付 100 金币，护送补给车抵达星核；修复 4 点生命，每人获得 60 金币"
		}
		s.World.Objectives = append(s.World.Objectives, o)
	}
}
