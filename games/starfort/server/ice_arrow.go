package starfort

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

const maxIceWalls = 32

type IceWall struct {
	ID        int     `json:"id"`
	TowerID   int     `json:"towerId"`
	Owner     int     `json:"owner"`
	Route     string  `json:"route"`
	Progress  float64 `json:"progress"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Angle     float64 `json:"angle"`
	CreatedMS int64   `json:"createdMS"`
	ExpiresMS int64   `json:"expiresMS"`
}

// Rendering projection excludes collision and ownership bookkeeping.
type ViewIceWall struct {
	ID        int     `json:"id"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Angle     float64 `json:"angle"`
	CreatedMS int64   `json:"createdMS"`
	ExpiresMS int64   `json:"expiresMS"`
}

func viewIceWall(w IceWall) ViewIceWall {
	return ViewIceWall{w.ID, w.X, w.Y, w.Angle, w.CreatedMS, w.ExpiresMS}
}

type BranchChoice struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *State) branchChoices(t Tower) []BranchChoice {
	if s.SkillRules == 2 && t.Kind == "arrow" {
		level := t.Level
		if level < 4 {
			level = 4
		}
		return []BranchChoice{{"volley", "箭雨", "每箭有 25% 概率，使主目标周围敌人受到 70% 攻击伤害"}, {"heart", "一箭穿心", fmt.Sprintf("每箭有 12%% 概率，秒杀 Lv.%d 及以下普通敌人；高等级敌人、精英和首领承受 3 倍伤害", level*2)}}
	}
	return []BranchChoice{{"power", "威力专精", "提高单次伤害"}, {"speed", "攻速专精", "提高攻击频率"}}
}
func (s *State) validBranch(t Tower) bool {
	if t.Level <= 3 {
		return t.Branch == ""
	}
	for _, choice := range s.branchChoices(t) {
		if t.Branch == choice.ID {
			return true
		}
	}
	return false
}
func (s *State) branchName(t Tower) string {
	if t.Branch == "" {
		return ""
	}
	for _, b := range s.branchChoices(t) {
		if b.ID == t.Branch {
			return b.Name
		}
	}
	return ""
}
func (s *State) skillRoll(percent uint64) bool {
	x := s.SkillRNG
	if x == 0 {
		x = uint64(s.World.Seed) ^ 0x9e3779b97f4a7c15
		if x == 0 {
			x = 1
		}
	}
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	s.SkillRNG = x
	return x%100 < percent
}
func (s *State) makeIceWall(t *Tower, target Enemy) {
	now := s.combatNow()
	if s.SkillRules != 2 || now < t.PassiveReadyAt {
		return
	}
	s.IceWalls = slices.DeleteFunc(s.IceWalls, func(w IceWall) bool { return w.ExpiresMS <= now })
	if len(s.IceWalls) >= maxIceWalls {
		return
	}
	path := s.routePath(target.Route)
	progress := math.Min(pathDistance(path)-1, target.Progress+24)
	if progress <= target.Progress {
		return
	}
	p := pathPosition(path, progress)
	ahead := pathPosition(path, progress+1)
	s.IceWallSerial++
	wall := IceWall{ID: s.IceWallSerial, TowerID: t.ID, Owner: t.Owner, Route: target.Route, Progress: progress, X: p.X, Y: p.Y, Angle: math.Atan2(ahead.Y-p.Y, ahead.X-p.X) + math.Pi/2, CreatedMS: now, ExpiresMS: now + 6000}
	s.IceWalls = append(s.IceWalls, wall)
	t.PassiveReadyAt = now + 8000
	publicWall := viewIceWall(wall)
	s.event(Event{Type: "ice_wall", TowerID: t.ID, Owner: t.Owner, Kind: "frost", X: p.X, Y: p.Y, IceWall: &publicWall})
}
func (s *State) crossIceWalls(e *Enemy, from, to float64) {
	if s.SkillRules != 2 || e.Frostbound || e.HP <= 0 {
		return
	}
	for _, wall := range s.IceWalls {
		if wall.Route == e.Route && wall.ExpiresMS > s.ElapsedMS && from < wall.Progress && to >= wall.Progress && s.skillRoll(30) {
			e.Frostbound = true
			s.event(Event{IceWallID: wall.ID, Type: "frostbound", Kind: "frost", TowerID: wall.TowerID, TargetID: e.ID, Owner: wall.Owner, X: e.X, Y: e.Y})
			return
		}
	}
}
func (s *State) arrowSkillDamage(t *Tower, target Enemy, damage float64) (float64, string) {
	if s.SkillRules != 2 || t.Kind != "arrow" || t.Branch == "" {
		return s.focusDamage(t, target, damage)
	}
	switch t.Branch {
	case "volley":
		if s.skillRoll(25) {
			return damage, "volley"
		}
	case "heart":
		if s.skillRoll(12) {
			if target.Kind != "elite" && target.Kind != "boss" && target.Level > 0 && target.Level <= t.Level*2 {
				return target.HP + target.Shield + 1, "execute"
			}
			return damage * 3, "heart"
		}
	}
	return damage, ""
}
func (s *State) newSkillView(t Tower) *PassiveView {
	if s.SkillRules != 2 {
		return nil
	}
	if t.Kind == "frost" {
		return &PassiveView{Name: "寒霜冰墙", Description: "命中时在前方生成持续 6 秒的冰墙；经过时有 30% 概率永久减速 30%，直到死亡，不重复叠加", CooldownMS: 8000, ReadyInMS: max(0, t.PassiveReadyAt-s.ElapsedMS), Variant: "ice-wall"}
	}
	if t.Kind == "arrow" {
		switch t.Branch {
		case "volley":
			return &PassiveView{Name: "风羽箭雨", Description: "每箭有 25% 概率触发箭雨，主目标正常受伤，附近敌人受到 70% 的攻击伤害", TriggerChance: 25, Variant: "volley"}
		case "heart":
			return &PassiveView{Name: "一箭穿心", Description: fmt.Sprintf("每箭有 12%% 概率触发；秒杀 Lv.%d 及以下普通敌人。高等级敌人、精英和首领承受 3 倍伤害，不触发秒杀。敌人等级等于出场波次", t.Level*2), TriggerChance: 12, ExecuteMaxLevel: t.Level * 2, Variant: "heart"}
		}
	}
	return nil
}
func (s *State) validNewSkills() error {
	if s.SkillRules != 0 && s.SkillRules != 2 {
		return errors.New("无效技能规则")
	}
	if s.SkillRules == 0 && (len(s.IceWalls) > 0 || s.SkillRNG != 0) {
		return errors.New("旧规则包含新技能状态")
	}
	if len(s.IceWalls) > maxIceWalls || s.IceWallSerial < 0 {
		return errors.New("冰墙数量无效")
	}
	ids := map[int]bool{}
	for _, w := range s.IceWalls {
		if w.ID < 1 || w.ID > s.IceWallSerial || ids[w.ID] || w.TowerID < 1 || w.TowerID > s.TowerSerial || w.Owner < 0 || w.Owner >= len(s.Players) || len(s.routePath(w.Route)) < 2 || !finite(w.Progress) || !finite(w.X) || !finite(w.Y) || !finite(w.Angle) || w.Progress < 0 || w.Progress > pathDistance(s.routePath(w.Route)) || w.CreatedMS < 0 || w.ExpiresMS-w.CreatedMS != 6000 {
			return errors.New("无效冰墙")
		}
		ids[w.ID] = true
	}
	return nil
}
