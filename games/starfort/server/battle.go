package starfort

import (
	"fmt"
	"math"
	"sort"
)

const shieldRatio = 0.8

// Legacy helpers remain for schema-1 migration; live combat always uses the
// route saved on each enemy, never whichever entrances are active this wave.
func pathLength() float64             { return pathDistance(board.Path) }
func position(distance float64) Point { return pathPosition(board.Path, distance) }
func pathDistance(path []Point) float64 {
	length := 0.
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		length += math.Hypot(b.X-a.X, b.Y-a.Y)
	}
	return length
}
func pathPosition(path []Point, distance float64) Point {
	if len(path) == 0 {
		return Point{}
	}
	distance = math.Max(0, distance)
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		length := math.Hypot(b.X-a.X, b.Y-a.Y)
		if length == 0 {
			continue
		}
		if distance <= length {
			return Point{a.X + (b.X-a.X)*distance/length, a.Y + (b.Y-a.Y)*distance/length}
		}
		distance -= length
	}
	return path[len(path)-1]
}
func (s *State) describeWave() {
	rules := []string{"march", "rush", "armor", "swarm", "shield"}
	rule := rules[(s.Wave-1)%len(rules)]
	title, description := "林地来袭", "史莱姆沿道路前进，守住星核"
	switch rule {
	case "rush":
		title, description = "疾风突袭", "快速敌人增多，箭塔与减速可以有效拦截"
	case "armor":
		title, description = "铁甲编队", "重甲敌人增多，法师塔的攻击忽略护甲"
	case "swarm":
		title, description = "萤群涌动", "小怪密集来袭，炮塔的范围攻击更有效"
	case "shield":
		title, description = "护盾潮汐", "护盾先吸收伤害，法师塔更擅长击破护盾"
	}
	if s.Wave%5 == 0 {
		title = "精英 · " + title
		description += "；本波有精英压阵"
	}
	boss := s.Wave%10 == 0
	if boss {
		title = "星陨巨像"
		description = "首领半血后召唤护盾护卫并加速，集中火力守住星核"
	}
	if s.Wave >= 4 {
		description += "；治疗师可恢复周围敌人，建议优先支援集火"
	}
	if s.Wave >= 6 {
		description += "；旗手使附近敌人加速"
	}
	if s.Wave >= 7 {
		description += "；分裂怪会留下小怪"
	}
	if s.Wave >= 9 {
		if s.World.Layout == "central" {
			description += "；工兵填坑速度更快"
		} else {
			description += "；工兵存活 20 秒会预告下波捷径"
		}
	}
	s.WaveInfo = WaveInfo{Title: fmt.Sprintf("第 %d 波 · %s", s.Wave, title), Description: description, Rule: rule, Total: min(44, 9+s.Wave*2), Boss: boss}
}
func (s *State) prepare(now int64) {
	s.Phase = "prepare"
	s.Epoch++
	s.ServerAt = now
	s.NextAt = now + int64(s.Options.PrepSeconds)*1000
	for i := range s.Players {
		s.Players[i].Ready = false
		s.Players[i].PauseVote = false
		s.Players[i].ResumeVote = false
	}
	s.describeWave()
	s.worldPrepare(now)
}
func (s *State) startWave(now int64) {
	s.Phase = "battle"
	s.Epoch++
	s.ServerAt = now
	s.NextAt = 0
	s.SpawnClock = 0
	for i := range s.Players {
		s.Players[i].Ready = false
		s.Players[i].PauseVote = false
		s.Players[i].ResumeVote = false
	}
	s.event(Event{Type: "wave", Owner: -1, Amount: float64(s.Wave)})
	s.note("%s：%s", s.WaveInfo.Title, s.WaveInfo.Description)
}
func (s *State) spawn() {
	index := s.WaveInfo.Spawned
	kind := "slime"
	switch s.WaveInfo.Rule {
	case "rush":
		if index%3 != 0 {
			kind = "runner"
		}
	case "armor":
		if index%3 != 0 {
			kind = "armored"
		}
	case "swarm":
		kind = "swarm"
	case "shield":
		if index%3 != 0 {
			kind = "shield"
		}
	}
	// Mixed later waves keep all four tower roles relevant.
	if s.Wave > 5 && index%7 == 2 {
		kind = "runner"
	}
	if s.Wave > 8 && index%7 == 4 {
		kind = "armored"
	}
	// Support slots are deterministic and telegraphed by the wave preview.
	if s.Wave >= 4 && index%11 == 3 {
		kind = "healer"
	}
	if s.Wave >= 6 && index%13 == 6 {
		kind = "banner"
	}
	if s.Wave >= 7 && index%9 == 5 {
		kind = "splitter"
	}
	if s.Wave >= 9 && index == s.WaveInfo.Total/2 {
		kind = "engineer"
	}
	if s.Wave%5 == 0 && index == s.WaveInfo.Total-1 {
		kind = "elite"
	}
	if s.WaveInfo.Boss && index == s.WaveInfo.Total-1 {
		kind = "boss"
	}
	difficulty := 1.
	if s.Options.Difficulty == "easy" {
		difficulty = .72
	}
	if s.Options.Difficulty == "hard" {
		difficulty = 1.4
	}
	// Campaign growth stays unchanged. Once the map reaches its final size,
	// endless strength grows more gradually: tower investment is paid from a
	// bounded number of enemies, so the original 10% exponential otherwise
	// outruns every legal economy shortly after the third expansion.
	growth := math.Pow(1.10, float64(min(s.Wave-1, 19))) * math.Pow(1.055, float64(min(max(s.Wave-20, 0), 241))) * (1 + .065*float64(s.Wave-1))
	hp := math.Min(maxValue, 55*growth*math.Pow(float64(len(s.Players)), .78)*difficulty)
	speed, armor, reward, leak, shield := 58., 0., 8, 1, 0.
	switch kind {
	case "runner":
		speed = 90
		hp *= .68
	case "armored":
		speed = 43
		hp *= 1.6
		armor = .55
		reward = 11
	case "swarm":
		speed = 68
		hp *= .48
		reward = 5
	case "shield":
		shield = hp * shieldRatio
		reward = 11
	case "healer":
		hp *= .85
		speed = 48
		reward = 14
	case "banner":
		hp *= 1.1
		speed = 52
		reward = 14
	case "splitter":
		hp *= 1.2
		speed = 52
		reward = 14
	case "engineer":
		hp *= 1.35
		speed = 45
		reward = 18
	case "elite":
		hp *= 4
		speed = 42
		armor = .2
		reward = 32
		leak = 4
	case "boss":
		hp *= 15
		speed = 33
		armor = .15
		reward = 100
		leak = 10
	}
	// Endless pressure increases quality, never unbounded entities or movement speed.
	speed *= 1 + math.Min(.5, float64(s.Wave-1)*.006)
	s.EnemySerial++
	routes := s.activeRoutes()
	route := routes[index%len(routes)]
	p := pathPosition(s.routePath(route), 0)
	phase := 0
	if kind == "boss" {
		phase = 1
	}
	s.Enemies = append(s.Enemies, Enemy{ID: s.EnemySerial, Kind: kind, Route: route, Phase: phase, AbilityTick: s.Tick, X: p.X, Y: p.Y, HP: hp, MaxHP: hp, Shield: shield, Speed: speed, Armor: armor, Reward: reward, Leak: leak})
	s.WaveInfo.Spawned++
	s.event(Event{TargetID: s.EnemySerial, Type: "spawn", X: p.X, Y: p.Y, Kind: kind, Owner: -1})
}
func (s *State) step(now int64) {
	s.Tick++
	s.ElapsedMS += StepMS
	// Physics and abilities still advance once per 200 ms. Towers whose saved
	// remainder expires inside that step fire first, in actual due-time order.
	// New/idle towers become ready at its boundary, preserving the old first
	// shot, spawn and ability cadence without rounding later attack intervals.
	atMS := s.ElapsedMS - StepMS
	s.eventAtMS = &atMS
	defer func() { s.eventAtMS = nil }()
	type dueTower struct{ index, offset int }
	due := make([]dueTower, 0, len(s.Towers))
	for i := range s.Towers {
		offset := s.Towers[i].Cooldown
		if offset == 0 {
			offset = StepMS
		}
		if offset > StepMS {
			s.Towers[i].Cooldown = offset - StepMS
		} else {
			due = append(due, dueTower{i, offset})
		}
	}
	sort.Slice(due, func(i, j int) bool {
		if due[i].offset != due[j].offset {
			return due[i].offset < due[j].offset
		}
		return s.Towers[due[i].index].ID < s.Towers[due[j].index].ID
	})
	boundary := []int{}
	for _, item := range due {
		if item.offset == StepMS {
			boundary = append(boundary, item.index)
			continue
		}
		atMS = s.ElapsedMS - StepMS + int64(item.offset)
		if s.fireTower(item.index) {
			s.Towers[item.index].Cooldown -= StepMS - item.offset
		} else {
			// No target earns no attack credit. Retry at the boundary, where
			// a newly spawned enemy can now enter range.
			boundary = append(boundary, item.index)
		}
	}
	atMS = s.ElapsedMS
	s.SpawnClock = max(0, s.SpawnClock-StepMS)
	if s.WaveInfo.Spawned < s.WaveInfo.Total && s.SpawnClock <= 0 && len(s.Enemies) < MaxEnemies {
		s.spawn()
		s.SpawnClock = 800
		if s.WaveInfo.Rule == "swarm" {
			s.SpawnClock = 400
		}
	}
	s.enemyAbilities()
	sort.Slice(boundary, func(i, j int) bool { return s.Towers[boundary[i]].ID < s.Towers[boundary[j]].ID })
	for _, index := range boundary {
		s.fireTower(index)
	}
	worldMap := s.mapData()
	// Deaths grant the same amount to each seat. The kill's owner is only a
	// contribution statistic: no last-hit reward race exists.
	// Snapshot living banner positions before compaction: in-place filtering
	// must not overwrite aura sources while later enemies still query them.
	banners := []Point{}
	for _, e := range s.Enemies {
		if e.HP > 0 && e.Kind == "banner" {
			banners = append(banners, Point{e.X, e.Y})
		}
	}
	children := []Enemy{}
	alive := make([]Enemy, 0, len(s.Enemies))
	for _, e := range s.Enemies {
		if e.HP <= 0 {
			if e.Kind == "splitter" {
				for child := 0; child < 2; child++ {
					spawned := e
					spawned.ID = 0
					spawned.Kind = "swarm"
					spawned.HP = math.Max(1, e.MaxHP*.3)
					spawned.MaxHP = spawned.HP
					spawned.Shield, spawned.Armor = 0, 0
					spawned.Speed = math.Min(150, e.Speed*1.15)
					spawned.Reward, spawned.Leak = 0, 1
					spawned.Phase, spawned.AbilityTick = 0, s.Tick
					children = append(children, spawned)
				}
			}
			continue
		}
		speed := e.Speed
		if s.World.Event.ID == "frost" {
			speed *= .85
		}
		if e.Kind == "boss" && e.Phase == 2 {
			speed *= 1.18
		}
		for _, banner := range banners {
			if math.Hypot(e.X-banner.X, e.Y-banner.Y) <= 150 {
				speed *= 1.2
				break
			}
		}
		if e.SlowUntil > s.Tick {
			speed *= .55
		}
		nextProgress := e.Progress + speed*StepMS/1000
		if !s.blockAtCrater(&e, nextProgress) {
			e.Progress = nextProgress
		}
		if e.Progress >= pathDistance(s.routePath(e.Route)) {
			s.BaseHP = max(0, s.BaseHP-e.Leak)
			s.event(Event{TargetID: e.ID, Type: "base_hit", X: worldMap.Base.X, Y: worldMap.Base.Y, Kind: e.Kind, Owner: -1, Amount: float64(e.Leak)})
			continue
		}
		p := pathPosition(s.routePath(e.Route), e.Progress)
		e.X, e.Y = p.X, p.Y
		alive = append(alive, e)
	}
	for _, child := range children {
		if len(alive) >= MaxEnemies {
			break
		}
		s.EnemySerial++
		child.ID = s.EnemySerial
		alive = append(alive, child)
		target := viewEnemy(child)
		s.event(Event{TargetID: child.ID, Target: &target, Type: "summon", X: child.X, Y: child.Y, Kind: child.Kind, Owner: -1})
	}
	s.Enemies = alive
	s.repairCraters()
	s.stepObjectives()
	if s.BaseHP == 0 {
		s.Phase = "lost"
		s.Epoch++
		s.NextAt = 0
		s.event(Event{Type: "lose", X: worldMap.Base.X, Y: worldMap.Base.Y, Owner: -1})
		s.note("星核熄灭，共守住 %d 波，击败 %d 个敌人", s.CompletedWaves, s.Kills)
		return
	}
	if s.WaveInfo.Spawned == s.WaveInfo.Total && len(s.Enemies) == 0 {
		s.CompletedWaves = s.Wave
		s.onWaveCleared()
		reward := s.rewardGold(40 + min(1000, s.Wave*3))
		for i := range s.Players {
			s.Players[i].Gold = min(maxValue, s.Players[i].Gold+reward)
		}
		s.note("守住第 %d 波，每位玩家获得 %d 金币", s.Wave, reward)
		if s.Options.Mode == "campaign" && s.CompletedWaves >= s.Options.Waves {
			s.failActiveObjectives("战役结束前未完成护送")
			s.Phase = "won"
			s.Epoch++
			s.NextAt = 0
			s.event(Event{Type: "win", X: worldMap.Base.X, Y: worldMap.Base.Y, Owner: -1})
			s.note("全队获胜，星光长明")
			return
		}
		s.Wave++
		s.prepare(now)
	}
}

// fireTower consumes one ready attack only when a living legal target exists.
// The next cooldown is retained as a millisecond remainder by the caller.
func (s *State) fireTower(index int) bool {
	t := &s.Towers[index]
	t.Cooldown = 0
	damage, radius, interval := s.towerStats(*t)
	pad := s.World.Map.Pads[t.Pad]
	target := -1
	for j, enemy := range s.Enemies {
		if enemy.HP > 0 && math.Hypot(enemy.X-pad.X, enemy.Y-pad.Y) <= radius && (target < 0 || s.preferTarget(enemy, s.Enemies[target], t.Priority)) {
			target = j
		}
	}
	if target < 0 {
		return false
	}
	enemy := s.Enemies[target]
	t.Angle = math.Atan2(enemy.Y-pad.Y, enemy.X-pad.X)
	t.Cooldown = interval
	damage, skill := s.focusDamage(t, enemy, damage)
	s.event(Event{Skill: skill, TowerID: t.ID, TargetID: enemy.ID, Type: "shot", X: pad.X, Y: pad.Y, ToX: enemy.X, ToY: enemy.Y, Kind: t.Kind, Owner: t.Owner})
	if t.Kind == "cannon" {
		for j := range s.Enemies {
			if s.Enemies[j].HP > 0 && math.Hypot(s.Enemies[j].X-enemy.X, s.Enemies[j].Y-enemy.Y) <= 78 {
				s.hit(j, damage, *t)
			}
		}
	} else {
		s.hit(target, damage, *t)
	}
	s.onTowerHit(t, enemy, damage)
	return true
}

func (s *State) hit(index int, damage float64, t Tower) {
	e := &s.Enemies[index]
	if e.HP <= 0 {
		return
	}
	if t.Kind != "arcane" {
		damage *= 1 - e.Armor
	}
	before := e.HP + e.Shield
	if e.Shield > 0 {
		factor := 1.
		if t.Kind == "arcane" {
			factor = 1.5
		}
		absorbed := math.Min(e.Shield, damage*factor)
		e.Shield -= absorbed
		damage -= absorbed / factor
	}
	e.HP = math.Max(0, e.HP-damage)
	actual := before - e.HP - e.Shield
	s.Players[t.Owner].Damage = math.Min(maxValue, s.Players[t.Owner].Damage+actual)
	if t.Kind == "frost" {
		e.SlowUntil = max(e.SlowUntil, s.Tick+8)
	}
	s.event(Event{TowerID: t.ID, TargetID: e.ID, Type: "hit", X: e.X, Y: e.Y, Kind: e.Kind, Owner: t.Owner, Amount: actual})
	if e.HP <= 0 {
		s.Kills++
		s.Players[t.Owner].Kills++
		for i := range s.Players {
			s.Players[i].Gold = min(maxValue, s.Players[i].Gold+s.rewardGold(e.Reward))
		}
		s.event(Event{TowerID: t.ID, TargetID: e.ID, Type: "death", X: e.X, Y: e.Y, Kind: e.Kind, Owner: t.Owner, Amount: float64(s.rewardGold(e.Reward))})
	}
}
