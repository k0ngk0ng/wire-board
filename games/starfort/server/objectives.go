package starfort

import (
	"errors"
	"fmt"
	"math"
)

// The preview and accepted world use the same objective position, so choosing
// a branch never moves a promised landmark after the team has voted.
func objectiveLocation(route Route, kind string) Point {
	distance := 90.
	if kind == "convoy" {
		distance = 80
	}
	p := pathPosition(route.Points, distance)
	if kind == "convoy" {
		return p
	}
	before := pathPosition(route.Points, math.Max(0, distance-1))
	dx, dy := p.X-before.X, p.Y-before.Y
	// Place stationary landmarks on the base-facing side of the road. A
	// constant positive normal puts southern-border landmarks off the map.
	end := route.Points[len(route.Points)-1]
	if math.Abs(dx) >= math.Abs(dy) {
		p.Y += math.Copysign(65, end.Y-p.Y)
	} else {
		p.X += math.Copysign(65, end.X-p.X)
	}
	return p
}

func (s *State) addObjectives() {
	stage := s.World.Stage
	kind := "mine"
	if stage == 3 {
		kind = "beacon"
	}
	if stage == 4 {
		kind = "convoy"
	}
	id := fmt.Sprintf("%s-%d", kind, stage)
	for _, o := range s.World.Objectives {
		if o.ID == id {
			return
		}
	}
	var route Route
	for _, r := range s.World.Map.Routes {
		if r.Sector == stage {
			route = r
			break
		}
	}
	if len(route.Points) < 2 {
		return
	}
	point := objectiveLocation(route, kind)
	o := Objective{ID: id, Kind: kind, Sector: stage, X: point.X, Y: point.Y, Status: "available", Route: route.ID, HP: 120, MaxHP: 120}
	switch kind {
	case "mine":
		o.Label = "修复星砂矿场"
		o.Cost, o.Reward = min(120, 60*len(s.Players)), 90
		o.Description = fmt.Sprintf("支付 %d 金币，保护矿场撑过一波；附近敌人会攻击矿场，成功后每人获得 %d 金币", o.Cost, o.Reward)
	case "beacon":
		o.Label = "点亮瞭望塔"
		o.Description = "支付 160 金币，保护瞭望塔撑过一波；附近敌人会攻击瞭望塔，成功后全队永久增加 8% 射程"
		o.Cost, o.Reward = 160, 1
	case "convoy":
		o.Label = "护送星光补给车"
		o.Description = "支付 100 金币，护送补给车沿道路抵达基地；附近敌人会攻击车辆，送达后修复 4 点基地生命，每人获得 60 金币"
		o.Cost, o.Reward = 100, 60
		o.HP, o.MaxHP = 100, 100
		o.Progress = 80
	}
	s.World.Objectives = append(s.World.Objectives, o)
}

func (s *State) acceptObjective(id string, p int) error {
	if p < 0 || p >= len(s.Players) || s.Phase != "prepare" || s.World.Choice != nil || s.World.BoonChoice != nil {
		return errors.New("请先完成波间选择")
	}
	for i := range s.World.Objectives {
		o := &s.World.Objectives[i]
		if o.ID != id {
			continue
		}
		if o.Status != "available" || s.Players[p].Gold < o.Cost {
			return errors.New("目标已承接或金币不足")
		}
		s.Players[p].Gold -= o.Cost
		o.Status = "active"
		o.StartWave, o.EndWave = s.Wave, s.Wave
		if o.Kind == "convoy" {
			o.EndWave = 0
		}
		o.BaseAtStart = s.BaseHP
		s.event(Event{Type: "objective", X: o.X, Y: o.Y, Owner: p, Kind: o.Kind})
		s.note("玩家 %d 承接支线：%s", p+1, o.Label)
		return nil
	}
	return errors.New("未知支线目标")
}

func (s *State) convoyPath(route string) []Point {
	path := append([]Point{}, s.routePath(route)...)
	if len(path) > 0 {
		path[len(path)-1] = s.World.Map.Base
	}
	return path
}

func (s *State) stepObjectives() {
	if s.BaseHP <= 0 {
		s.failActiveObjectives("基地失守")
		return
	}
	for i := range s.World.Objectives {
		o := &s.World.Objectives[i]
		if o.Status != "active" {
			continue
		}
		radius := 110.
		arrived := false
		if o.Kind == "convoy" {
			radius = 90
			path := s.convoyPath(o.Route)
			length := pathDistance(path)
			o.Progress = math.Min(length, o.Progress+90.*StepMS/1000)
			p := pathPosition(path, o.Progress)
			o.X, o.Y = p.X, p.Y
			arrived = o.Progress >= length
		}
		damage := 0.
		for _, e := range s.Enemies {
			if e.HP > 0 && math.Hypot(e.X-o.X, e.Y-o.Y) <= radius {
				damage += .8
			}
		}
		if damage > 0 {
			o.HP = math.Max(0, o.HP-damage)
			// Keep impact traffic bounded even when a large group surrounds a
			// landmark. Health is still simulated every 200 ms in the snapshot.
			if s.Tick%5 == 0 || o.HP == 0 {
				s.event(Event{Type: "objective_hit", X: o.X, Y: o.Y, Owner: -1, Kind: o.Kind, Amount: damage})
			}
		}
		if o.HP <= 0 {
			s.finishObjective(i, false, "防守目标被摧毁")
		} else if arrived {
			s.finishObjective(i, true, "")
		}
	}
}

func (s *State) finishObjective(index int, success bool, reason string) {
	o := &s.World.Objectives[index]
	if o.Status != "active" {
		return
	}
	o.Status = "failed"
	if success {
		o.Status = "completed"
		if o.Kind == "beacon" {
			s.World.Boons.Reach++
		} else {
			for p := range s.Players {
				s.Players[p].Gold = min(maxValue, s.Players[p].Gold+o.Reward)
			}
			if o.Kind == "convoy" {
				s.BaseHP = min(s.BaseMaxHP, s.BaseHP+4)
			}
		}
		s.note("完成支线：%s", o.Label)
	} else {
		s.note("支线失败：%s，%s", o.Label, reason)
	}
	s.event(Event{Type: "objective", X: o.X, Y: o.Y, Owner: -1, Kind: o.Status})
}

func (s *State) onWaveCleared() {
	for i, o := range s.World.Objectives {
		if o.Status == "active" && o.Kind != "convoy" && s.Wave >= o.EndWave {
			s.finishObjective(i, o.HP > 0, "防守目标被摧毁")
		}
	}
}
func (s *State) failActiveObjectives(reason string) {
	for i, o := range s.World.Objectives {
		if o.Status == "active" {
			s.finishObjective(i, false, reason)
		}
	}
}
