package starfort

import (
	"fmt"
	"math"
)

func pathLength() float64 {
	length := 0.
	for i := 1; i < len(board.Path); i++ {
		a, b := board.Path[i-1], board.Path[i]
		length += math.Hypot(b.X-a.X, b.Y-a.Y)
	}
	return length
}
func position(distance float64) Point {
	for i := 1; i < len(board.Path); i++ {
		a, b := board.Path[i-1], board.Path[i]
		length := math.Hypot(b.X-a.X, b.Y-a.Y)
		if distance <= length {
			return Point{a.X + (b.X-a.X)*distance/length, a.Y + (b.Y-a.Y)*distance/length}
		}
		distance -= length
	}
	return board.Path[len(board.Path)-1]
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
		description = "首领携护盾护卫来袭，集中火力守住星核"
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
	growth := math.Pow(1.10, float64(min(s.Wave-1, 260))) * (1 + .065*float64(s.Wave-1))
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
		shield = hp * .8
		reward = 11
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
	p := position(0)
	s.Enemies = append(s.Enemies, Enemy{ID: s.EnemySerial, Kind: kind, X: p.X, Y: p.Y, HP: hp, MaxHP: hp, Shield: shield, Speed: speed, Armor: armor, Reward: reward, Leak: leak})
	s.WaveInfo.Spawned++
}
func (s *State) step(now int64) {
	s.Tick++
	s.ElapsedMS += StepMS
	s.SpawnClock = max(0, s.SpawnClock-StepMS)
	if s.WaveInfo.Spawned < s.WaveInfo.Total && s.SpawnClock <= 0 && len(s.Enemies) < MaxEnemies {
		s.spawn()
		s.SpawnClock = 800
		if s.WaveInfo.Rule == "swarm" {
			s.SpawnClock = 400
		}
	}
	for i := range s.Towers {
		t := &s.Towers[i]
		t.Cooldown = max(0, t.Cooldown-StepMS)
		if t.Cooldown > 0 {
			continue
		}
		damage, radius, interval := stats(*t)
		pad := board.Pads[t.Pad]
		target := -1
		for j, e := range s.Enemies {
			if e.HP > 0 && math.Hypot(e.X-pad.X, e.Y-pad.Y) <= radius && (target < 0 || e.Progress > s.Enemies[target].Progress) {
				target = j
			}
		}
		if target < 0 {
			continue
		}
		e := s.Enemies[target]
		t.Angle = math.Atan2(e.Y-pad.Y, e.X-pad.X)
		t.Cooldown = interval
		s.event(Event{Type: "shot", X: pad.X, Y: pad.Y, ToX: e.X, ToY: e.Y, Kind: t.Kind, Owner: t.Owner})
		if t.Kind == "cannon" {
			for j := range s.Enemies {
				if s.Enemies[j].HP > 0 && math.Hypot(s.Enemies[j].X-e.X, s.Enemies[j].Y-e.Y) <= 78 {
					s.hit(j, damage, *t)
				}
			}
		} else {
			s.hit(target, damage, *t)
		}
	}
	// Deaths grant the same amount to each seat. The kill's owner is only a
	// contribution statistic: no last-hit reward race exists.
	alive := s.Enemies[:0]
	for _, e := range s.Enemies {
		if e.HP <= 0 {
			continue
		}
		speed := e.Speed
		if e.SlowUntil > s.Tick {
			speed *= .55
		}
		e.Progress += speed * StepMS / 1000
		if e.Progress >= pathLength() {
			s.BaseHP = max(0, s.BaseHP-e.Leak)
			s.event(Event{Type: "base_hit", X: 930, Y: 500, Kind: e.Kind, Owner: -1, Amount: float64(e.Leak)})
			continue
		}
		p := position(e.Progress)
		e.X, e.Y = p.X, p.Y
		alive = append(alive, e)
	}
	s.Enemies = alive
	if s.BaseHP == 0 {
		s.Phase = "lost"
		s.Epoch++
		s.NextAt = 0
		s.event(Event{Type: "lose", X: 930, Y: 500, Owner: -1})
		s.note("星核熄灭，共守住 %d 波，击败 %d 个敌人", s.CompletedWaves, s.Kills)
		return
	}
	if s.WaveInfo.Spawned == s.WaveInfo.Total && len(s.Enemies) == 0 {
		s.CompletedWaves = s.Wave
		reward := 40 + min(1000, s.Wave*3)
		for i := range s.Players {
			s.Players[i].Gold = min(maxValue, s.Players[i].Gold+reward)
		}
		s.note("守住第 %d 波，每位玩家获得 %d 金币", s.Wave, reward)
		if s.Options.Mode == "campaign" && s.CompletedWaves >= s.Options.Waves {
			s.Phase = "won"
			s.Epoch++
			s.NextAt = 0
			s.event(Event{Type: "win", X: 930, Y: 500, Owner: -1})
			s.note("全队获胜，星光长明")
			return
		}
		s.Wave++
		s.prepare(now)
	}
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
		e.SlowUntil = s.Tick + 8
	}
	s.event(Event{Type: "hit", X: e.X, Y: e.Y, Kind: e.Kind, Owner: t.Owner, Amount: actual})
	if e.HP <= 0 {
		s.Kills++
		s.Players[t.Owner].Kills++
		for i := range s.Players {
			s.Players[i].Gold = min(maxValue, s.Players[i].Gold+e.Reward)
		}
		s.event(Event{Type: "death", X: e.X, Y: e.Y, Kind: e.Kind, Owner: t.Owner, Amount: float64(e.Reward)})
	}
}
