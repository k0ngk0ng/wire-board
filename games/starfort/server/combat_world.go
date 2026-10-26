package starfort

import "math"

// All ability state is part of the server snapshot, so reconnecting, restoring
// a room or rendering at a different frame rate cannot replay an ability.
func (s *State) enemyAbilities() {
	initial := len(s.Enemies)
	for i := 0; i < initial; i++ {
		e := s.Enemies[i]
		if e.HP <= 0 {
			continue
		}
		switch e.Kind {
		case "healer":
			if s.Tick-e.AbilityTick < 2000/StepMS {
				continue
			}
			s.Enemies[i].AbilityTick = s.Tick
			for j := range s.Enemies {
				target := &s.Enemies[j]
				if j == i || target.HP <= 0 || target.HP >= target.MaxHP || math.Hypot(target.X-e.X, target.Y-e.Y) > 140 {
					continue
				}
				amount := math.Min(target.MaxHP-target.HP, target.MaxHP*.03)
				target.HP += amount
				s.event(Event{TargetID: target.ID, Type: "heal", X: target.X, Y: target.Y, ToX: e.X, ToY: e.Y, Kind: "healer", Owner: -1, Amount: amount})
			}
		case "engineer":
			if s.World.Layout == "central" {
				continue
			}
			if e.Phase > 0 || s.Tick-e.AbilityTick < 20000/StepMS {
				continue
			}
			s.Enemies[i].Phase = 1
			s.Enemies[i].AbilityTick = s.Tick
			if s.World.ShortcutWave != s.Wave+1 {
				s.World.ShortcutWave = s.Wave + 1
				s.event(Event{TargetID: e.ID, Type: "shortcut", X: e.X, Y: e.Y, Kind: "engineer", Owner: -1, Amount: float64(s.Wave + 1)})
				s.note("工兵打开了通道：第 %d 波将出现捷径，现有敌人的道路不变", s.Wave+1)
			}
		case "boss":
			if e.Phase >= 2 || e.HP > e.MaxHP*.5 {
				continue
			}
			s.Enemies[i].Phase = 2
			s.Enemies[i].AbilityTick = s.Tick
			s.event(Event{TargetID: e.ID, Type: "boss_phase", X: e.X, Y: e.Y, Kind: "boss", Owner: -1, Amount: 2})
			s.note("首领进入第二阶段：召唤护盾护卫，移动速度提高 18%%")
			for child := 0; child < 2 && len(s.Enemies) < MaxEnemies; child++ {
				guard := e
				s.EnemySerial++
				guard.ID = s.EnemySerial
				guard.Kind = "shield"
				guard.Phase = 0
				guard.AbilityTick = s.Tick
				guard.HP = math.Max(1, e.MaxHP*.06)
				guard.MaxHP = guard.HP
				guard.Shield = guard.HP * shieldRatio
				guard.Armor = 0
				guard.Speed = math.Min(150, e.Speed*1.3)
				guard.Reward = 0
				guard.Leak = 1
				s.Enemies = append(s.Enemies, guard)
				s.event(Event{TargetID: guard.ID, Type: "summon", X: guard.X, Y: guard.Y, Kind: "shield", Owner: -1})
			}
		}
	}
}

func (s *State) preferTarget(candidate, current Enemy, priority string) bool {
	switch priority {
	case "strong":
		a, b := candidate.HP+candidate.Shield, current.HP+current.Shield
		if a != b {
			return a > b
		}
	case "support":
		support := func(kind string) bool { return kind == "healer" || kind == "banner" || kind == "engineer" }
		a, b := support(candidate.Kind), support(current.Kind)
		if a != b {
			return a
		}
	}
	// Raw progress cannot compare roads of unequal length. Defend the enemy
	// nearest the shared base; persistent route IDs keep this stable on expansion.
	a := pathDistance(s.routePath(candidate.Route)) - candidate.Progress
	b := pathDistance(s.routePath(current.Route)) - current.Progress
	if a == b {
		return candidate.ID < current.ID
	}
	return a < b
}
