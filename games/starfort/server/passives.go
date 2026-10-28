package starfort

import (
	"errors"
	"math"
	"slices"
	"sort"
)

const maxCraters = 32

type Crater struct {
	ID          int     `json:"id"`
	TowerID     int     `json:"towerId"`
	Owner       int     `json:"owner"`
	Route       string  `json:"route"`
	Progress    float64 `json:"progress"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Radius      float64 `json:"radius"`
	Total       float64 `json:"total"`
	Remaining   float64 `json:"remaining"`
	Workers     []int   `json:"workers"`
	ExemptEnemy int     `json:"exemptEnemy"`
	CreatedMS   int64   `json:"createdMS"`
	FilledMS    int64   `json:"filledMS,omitempty"`
}
type PassiveView struct {
	TriggerChance   int    `json:"triggerChance,omitempty"`
	ExecuteMaxLevel int    `json:"executeMaxLevel,omitempty"`
	Variant         string `json:"variant,omitempty"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	CooldownMS      int64  `json:"cooldownMS"`
	ReadyInMS       int64  `json:"readyInMS"`
	Stacks          int    `json:"stacks"`
	MaxStacks       int    `json:"maxStacks"`
}

func (s *State) passiveView(t Tower) *PassiveView {
	if s.World.Layout != "central" && s.SkillRules != 2 {
		return nil
	}
	if p := s.newSkillView(t); p != nil {
		return p
	}
	p := &PassiveView{ReadyInMS: max(0, t.PassiveReadyAt-s.ElapsedMS)}
	switch t.Kind {
	case "arrow":
		p.Name = "追猎重矢"
		p.Description = "连续命中同一目标三箭，第三箭造成 1.8 倍伤害；换目标重新蓄势"
		p.Stacks = t.FocusHits
		p.MaxStacks = 3
	case "cannon":
		p.Name = "震地弹"
		p.Description = "命中留下弹坑，后续敌人停下合力填平；修补者越多越快"
		p.CooldownMS = 6000
	case "frost":
		p.Name = "霜环"
		p.Description = "命中时释放霜环，使附近敌人减速 3 秒"
		p.CooldownMS = 8000
	case "arcane":
		p.Name = "星链"
		p.Description = "向主目标附近至多两个敌人连锁，分别造成半额魔法伤害"
		p.CooldownMS = 8000
	}
	return p
}
func (s *State) combatNow() int64 {
	if s.eventAtMS != nil {
		return *s.eventAtMS
	}
	return s.ElapsedMS
}
func (s *State) craterEvent(kind string, c *Crater) {
	copy := *c
	copy.Workers = append([]int{}, c.Workers...)
	s.event(Event{Type: kind, Owner: c.Owner, TowerID: c.TowerID, X: c.X, Y: c.Y, Crater: &copy})
}
func (s *State) makeCrater(t *Tower, target Enemy) bool {
	now := s.combatNow()
	if (s.World.Layout != "central" && s.SkillRules != 2) || now < t.PassiveReadyAt {
		return false
	}
	for _, c := range s.Craters {
		if c.Remaining > 0 && c.Route == target.Route && math.Abs(c.Progress-target.Progress) < 64 {
			return false
		}
	}
	active := 0
	for _, c := range s.Craters {
		if c.Remaining > 0 {
			active++
		}
	}
	if active >= maxCraters {
		return false
	}
	s.CraterSerial++
	p := pathPosition(s.routePath(target.Route), target.Progress)
	c := Crater{ID: s.CraterSerial, TowerID: t.ID, Owner: t.Owner, Route: target.Route, Progress: target.Progress, X: p.X, Y: p.Y, Radius: 28, Total: 6, Remaining: 6, Workers: []int{}, ExemptEnemy: target.ID, CreatedMS: now}
	s.Craters = append(s.Craters, c)
	t.PassiveReadyAt = now + 6000
	s.craterEvent("crater", &s.Craters[len(s.Craters)-1])
	return true
}

// Only enemies that subsequently reach the scar stop. The original impact
// target has already crossed it. All movement and worker membership are saved.
func (s *State) blockAtCrater(e *Enemy, nextProgress float64) bool {
	var chosen *Crater
	for i := range s.Craters {
		c := &s.Craters[i]
		if c.Remaining <= 0 || c.Route != e.Route || c.ExemptEnemy == e.ID {
			continue
		}
		if e.Repairing == c.ID || (e.Progress <= c.Progress+.001 && nextProgress >= c.Progress) {
			if chosen == nil || c.Progress < chosen.Progress {
				chosen = c
			}
		}
	}
	if chosen == nil {
		e.Repairing = 0
		return false
	}
	first := e.Repairing != chosen.ID
	e.Repairing = chosen.ID
	e.Progress = chosen.Progress
	e.X = chosen.X
	e.Y = chosen.Y
	if first {
		e.RepairSinceMS = s.ElapsedMS
		target := viewEnemy(*e)
		s.event(Event{Type: "repair_start", TargetID: e.ID, Target: &target, X: e.X, Y: e.Y, Kind: e.Kind, Owner: -1, CraterID: chosen.ID})
	}
	return true
}
func (s *State) repairCraters() {
	for i := range s.Craters {
		c := &s.Craters[i]
		if c.Remaining <= 0 {
			continue
		}
		workers := []int{}
		work := 0.
		for _, e := range s.Enemies {
			if e.HP > 0 && e.Repairing == c.ID {
				workers = append(workers, e.ID)
				rate := 1.
				if e.Kind == "engineer" {
					rate = 2
				}
				work += rate * float64(StepMS) / 1000
			}
		}
		if work == 0 {
			if len(c.Workers) > 0 {
				c.Workers = []int{}
				s.craterEvent("crater_repair", c)
			}
			continue
		}
		c.Workers = workers
		c.Remaining = math.Max(0, c.Remaining-work)
		if c.Remaining < 1e-8 {
			c.Remaining = 0
		}
		if c.Remaining == 0 {
			c.FilledMS = s.ElapsedMS
			for j := range s.Enemies {
				if s.Enemies[j].Repairing == c.ID {
					s.Enemies[j].Repairing = 0
				}
			}
			s.craterEvent("crater_filled", c)
		} else {
			s.craterEvent("crater_repair", c)
		}
	}
	s.Craters = slices.DeleteFunc(s.Craters, func(c Crater) bool { return c.FilledMS > 0 && s.ElapsedMS-c.FilledMS > 3000 })
}
func (s *State) focusDamage(t *Tower, target Enemy, damage float64) (float64, string) {
	if (s.World.Layout != "central" && s.SkillRules != 2) || t.Kind != "arrow" {
		return damage, ""
	}
	if t.FocusTarget != target.ID {
		t.FocusTarget = target.ID
		t.FocusHits = 0
	}
	t.FocusHits++
	if t.FocusHits == 3 {
		t.FocusHits = 0
		return damage * 1.8, "focus"
	}
	return damage, ""
}
func (s *State) onTowerHit(t *Tower, target Enemy, damage float64) {
	if s.World.Layout != "central" && s.SkillRules != 2 {
		return
	}
	now := s.combatNow()
	if t.Kind == "frost" && s.SkillRules == 2 {
		s.makeIceWall(t, target)
		return
	}
	if t.Kind == "cannon" {
		s.makeCrater(t, target)
		return
	}
	if now < t.PassiveReadyAt {
		return
	}
	switch t.Kind {
	case "frost":
		t.PassiveReadyAt = now + 8000
		slowTargets := []int{}
		for i := range s.Enemies {
			e := &s.Enemies[i]
			if e.HP > 0 && math.Hypot(e.X-target.X, e.Y-target.Y) <= 90 {
				e.SlowUntil = max(e.SlowUntil, s.Tick+15)
				slowTargets = append(slowTargets, e.ID)
			}
		}
		s.event(Event{Type: "frost_ring", TowerID: t.ID, X: target.X, Y: target.Y, Kind: "frost", Owner: t.Owner, SlowTargets: slowTargets, SlowUntil: s.Tick + 15})
	case "arcane":
		t.PassiveReadyAt = now + 8000
		indices := []int{}
		for i, e := range s.Enemies {
			if e.ID != target.ID && e.HP > 0 && math.Hypot(e.X-target.X, e.Y-target.Y) <= 125 {
				indices = append(indices, i)
			}
		}
		sort.Slice(indices, func(i, j int) bool {
			a, b := s.Enemies[indices[i]], s.Enemies[indices[j]]
			da, db := math.Hypot(a.X-target.X, a.Y-target.Y), math.Hypot(b.X-target.X, b.Y-target.Y)
			if da == db {
				return a.ID < b.ID
			}
			return da < db
		})
		from := Point{target.X, target.Y}
		for _, i := range indices[:min(2, len(indices))] {
			e := s.Enemies[i]
			s.event(Event{Type: "chain", TowerID: t.ID, TargetID: e.ID, X: from.X, Y: from.Y, ToX: e.X, ToY: e.Y, Owner: t.Owner, Kind: "arcane"})
			s.hit(i, damage*.5, *t)
			from = Point{e.X, e.Y}
		}
	}
}

func (s *State) validCraters() error {
	if len(s.Craters) > maxCraters*3 || s.CraterSerial < 0 {
		return errors.New("无效弹坑数量")
	}
	ids := map[int]Crater{}
	for _, c := range s.Craters {
		path := s.routePath(c.Route)
		p := pathPosition(path, c.Progress)
		if c.ID < 1 || c.ID > s.CraterSerial || ids[c.ID].ID != 0 || c.TowerID < 1 || c.TowerID > s.TowerSerial || c.Owner < 0 || c.Owner >= len(s.Players) || len(path) < 2 || !finite(c.Progress) || c.Progress < 0 || c.Progress > pathDistance(path) || !finite(c.X) || !finite(c.Y) || math.Hypot(p.X-c.X, p.Y-c.Y) > .01 || c.Radius != 28 || c.Total != 6 || !finite(c.Remaining) || c.Remaining < 0 || c.Remaining > c.Total || c.CreatedMS < 0 || c.CreatedMS > s.ElapsedMS || c.FilledMS < 0 || c.FilledMS > s.ElapsedMS || (c.Remaining == 0) != (c.FilledMS > 0) || len(c.Workers) > MaxEnemies {
			return errors.New("无效弹坑存档")
		}
		workers := map[int]bool{}
		for _, id := range c.Workers {
			if id < 1 || id > s.EnemySerial || workers[id] {
				return errors.New("无效修补者")
			}
			workers[id] = true
		}
		ids[c.ID] = c
	}
	for _, e := range s.Enemies {
		if e.Repairing < 0 || e.RepairSinceMS < 0 || e.RepairSinceMS > s.ElapsedMS {
			return errors.New("无效修补状态")
		}
		if e.Repairing > 0 {
			c, ok := ids[e.Repairing]
			if !ok || c.Remaining <= 0 || c.Route != e.Route || math.Abs(c.Progress-e.Progress) > .01 || c.ExemptEnemy == e.ID {
				return errors.New("敌人引用无效弹坑")
			}
		}
	}
	return nil
}
