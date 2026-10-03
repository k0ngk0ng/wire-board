package game

import (
	"fmt"
	"slices"
)

func dotaCardName(p DotaPlayer, card int) string {
	if card < 5 {
		return dotaCatalog.Common[card].Name
	}
	return dotaHero(p.Hero).Skills[card-5].Name
}
func dotaLaneName(l, n int) string {
	if l < 0 {
		return "泉水"
	}
	if n == 1 {
		return "中路"
	}
	if n == 2 {
		return []string{"上路", "下路"}[l]
	}
	return []string{"上路", "中路", "下路"}[l]
}
func dotaTeamName(t int) string { return []string{"近卫", "天灾"}[t] }

// Orders have already been validated as a complete simultaneous batch. Search
// trials use this same resolver without recording animation/history payloads.
func (s *State) dotaResolve(orders []DotaOrder, record bool) {
	g := s.Dota
	hs := g.Players
	size := len(hs)
	event := DotaEvent{Round: s.Round, Messages: []string{}}
	note := func(format string, args ...any) {
		if record {
			event.Messages = append(event.Messages, fmt.Sprintf(format, args...))
		}
	}
	if record {
		event.Before = clone(hs)
		event.Orders = slices.Clone(orders)
	}
	shield, pressure, slow, heal := make([]int, size), make([]int, size), make([]int, size), make([]int, size)
	silence, immune := make([]bool, size), make([]bool, size)
	reflect := make([]int, size)
	taunt := map[[2]int]int{}
	type hitEvent struct {
		from, to, amount int
		magic            bool
	}
	hits := []hitEvent{}
	enemies := func(p int) []int {
		out := []int{}
		for j, x := range hs {
			if x.Team != hs[p].Team && x.Lane == hs[p].Lane && x.Lane >= 0 {
				out = append(out, j)
			}
		}
		return out
	}
	allies := func(p int) []int {
		out := []int{}
		for j, x := range hs {
			if x.Team == hs[p].Team && x.Lane == hs[p].Lane && x.Lane >= 0 {
				out = append(out, j)
			}
		}
		return out
	}
	hit := func(p, target, damage int, magic, anywhere bool) {
		if target < 0 || hs[target].Lane < 0 || !anywhere && hs[target].Lane != hs[p].Lane {
			return
		}
		damage += dotaBool(magic && hs[p].has("null_talisman")) + 2*dotaBool(orders[p].Card == 7 && hs[p].has("aghanim"))
		hits = append(hits, hitEvent{p, target, damage, magic})
	}
	note("第 %d 轮揭牌 · %s先手", s.Round, dotaTeamName((g.First+s.Round-1)%2))
	for p, a := range orders {
		h := &hs[p]
		oldLane := h.Lane
		h.Wounded = false
		note("玩家 %d（%s）使用%s", p+1, dotaHero(h.Hero).Name, dotaCardName(*h, a.Card))
		if a.Card != 0 && a.Card != 4 && a.Lane >= 0 {
			h.Lane = a.Lane
		}
		switch a.Card {
		case 0:
			h.Lane = a.Lane
			pressure[p] = 2
			if oldLane == h.Lane {
				pressure[p] = 3
			} else if oldLane < 0 {
				pressure[p] = 1
			}
			pressure[p] += dotaBool(h.has("quelling_blade")) + dotaBool(oldLane != h.Lane && (h.has("boots") || h.has("phase_boots") || h.has("arcane_boots")))
			shield[p] += 2 * dotaBool(h.has("phase_boots"))
		case 1:
			pressure[p] = 1
		case 2:
			shield[p] = 3
			pressure[p] = 2
			heal[p] += 2 * dotaBool(h.has("wand"))
		}
		if oldLane != h.Lane {
			note("玩家 %d 移动至%s", p+1, dotaLaneName(h.Lane, g.N))
		}
		if a.Card != 4 {
			h.Spent |= 1 << a.Card
		}
		if a.Card == 7 {
			h.Charge -= 3
		}
		immune[p] = h.has("bkb") && (a.Card == 1 || a.Card == 7) || h.Hero == "juggernaut" && a.Card == 5
		reflect[p] = dotaBool(h.has("blade_mail"))
		if a.Card == 2 {
			reflect[p] *= 2
		}
	}
	for p, a := range orders {
		h := hs[p]
		if h.Lane >= 0 && h.Hero == "drow" && a.Card == 6 {
			for _, j := range enemies(p) {
				if !immune[j] {
					silence[j] = true
					note("玩家 %d 被沉默，本轮英雄技能失效", j+1)
				}
			}
			pressure[p] += 2
		}
	}
	for p, a := range orders {
		h := hs[p]
		if h.Lane < 0 {
			continue
		}
		if h.Hero == "axe" && a.Card == 5 && !silence[p] {
			taunt[[2]int{h.Team, h.Lane}] = p
			shield[p] += 4
			pressure[p] += 2
		}
		if h.Hero == "sven" && a.Card == 6 && !silence[p] {
			for _, j := range allies(p) {
				shield[j] += 3
			}
			pressure[p] += 2
		}
		if h.has("mekansm") && a.Card == 2 {
			for _, j := range allies(p) {
				heal[j] += 2
			}
		}
	}
	for p, a := range orders {
		h := hs[p]
		if h.Lane < 0 {
			continue
		}
		if a.Card == 1 {
			damage := 3 + dotaBool(h.has("wraith_band")) + dotaBool(h.has("blades_attack")) + 2*dotaBool(h.has("crystalys")) + dotaBool(h.Hero == "juggernaut" && h.HP == h.maximum())
			hit(p, a.Target, damage, false, false)
			if h.Hero == "sven" {
				for _, j := range enemies(p) {
					if j != a.Target {
						hit(p, j, 1, false, false)
					}
				}
			}
		}
		if a.Card < 5 || silence[p] {
			continue
		}
		target := a.Target
		first := a.Card == 5
		if a.Card == 7 {
			pressure[p]++
			if h.Hero == "crystal_maiden" || h.Hero == "earthshaker" || h.Hero == "juggernaut" {
				damage := 4
				if h.Hero == "earthshaker" {
					damage = min(5, 2+len(enemies(p)))
				}
				for _, j := range enemies(p) {
					hit(p, j, damage, true, false)
				}
			} else {
				damage := 6
				if h.Hero == "axe" && target >= 0 && hs[target].HP <= 4 {
					damage = 7
				}
				hit(p, target, damage, h.Hero != "sven" && h.Hero != "drow", h.Hero == "sniper")
			}
			continue
		}
		switch h.Hero {
		case "axe":
			if !first {
				hit(p, target, 3, true, false)
				pressure[p]++
			}
		case "crystal_maiden":
			targets := enemies(p)
			damage := 2
			if !first {
				damage = 4
				if slices.Contains(targets, target) {
					targets = []int{target}
				} else {
					targets = nil
				}
			}
			for _, j := range targets {
				hit(p, j, damage, true, false)
				slow[j] += dotaBool(!immune[j])
			}
			pressure[p]++
		case "earthshaker":
			damage := 4
			if first {
				damage = 3
			}
			hit(p, target, damage, first, false)
			pressure[p]++
			if first && slices.Contains(enemies(p), target) && !immune[target] {
				slow[target] += 2
			}
		case "sven":
			if first {
				hit(p, target, 3, true, false)
				if slices.Contains(enemies(p), target) && !immune[target] {
					slow[target] += 2
				}
			}
		case "juggernaut":
			if first {
				for _, j := range enemies(p) {
					hit(p, j, 3, true, false)
				}
			} else {
				for _, j := range allies(p) {
					heal[j] += 3
				}
			}
			pressure[p]++
		case "drow":
			if first {
				hit(p, target, 3, false, false)
				if slices.Contains(enemies(p), target) && !immune[target] {
					slow[target] += 2
				}
			}
		case "lina":
			damage := 2
			if first {
				damage = 3
			}
			for _, j := range enemies(p) {
				hit(p, j, damage, true, false)
				if !first && !immune[j] {
					slow[j]++
				}
			}
			pressure[p]++
		case "sniper":
			if first {
				for _, j := range enemies(p) {
					hit(p, j, 1, true, false)
				}
				pressure[p] += 2
			} else {
				hit(p, target, 4, false, false)
			}
		}
	}
	sources := make([]map[int]bool, size)
	for p := range hs {
		sources[p] = map[int]bool{}
		h := &hs[p]
		restored := min(h.maximum()-h.HP, heal[p])
		h.HP += restored
		if restored > 0 {
			note("玩家 %d 恢复 %d 点生命", p+1, restored)
		}
	}
	firstTeam := (g.First + s.Round - 1) % 2
	attackOrder := []int{}
	for offset := 0; offset < g.N; offset++ {
		for _, t := range []int{firstTeam, 1 - firstTeam} {
			attackOrder = append(attackOrder, g.seats(t)[(offset+s.Round-1)%g.N])
		}
	}
	for _, actor := range attackOrder {
		if hs[actor].HP <= 0 {
			note("玩家 %d 先被击倒，未发动的伤害取消", actor+1)
			continue
		}
		damage, reflected := make([]int, size), make([]int, size)
		for _, e := range hits {
			if e.from != actor || hs[e.to].HP <= 0 {
				continue
			}
			p, j, amount := e.from, e.to, e.amount
			if orders[p].Card == 1 {
				if protector, ok := taunt[[2]int{hs[j].Team, hs[j].Lane}]; ok && hs[protector].HP > 0 {
					j = protector
				}
			}
			if e.magic && immune[j] {
				note("玩家 %d 魔免，抵消%s的魔法伤害", j+1, dotaHero(hs[p].Hero).Name)
				continue
			}
			amount -= dotaBool(!e.magic && hs[j].has("vanguard"))
			damage[j] += max(0, amount)
			sources[j][p] = true
			retaliation := reflect[j] + dotaBool(hs[j].Hero == "axe" && orders[p].Card == 1 && hs[p].Lane == hs[j].Lane)
			reflected[p] += retaliation
			if retaliation > 0 {
				sources[p][j] = true
			}
		}
		for p := range hs {
			h := &hs[p]
			absorbed := min(shield[p], damage[p])
			shield[p] -= absorbed
			wounds := damage[p] - absorbed + reflected[p]
			h.HP -= wounds
			if absorbed > 0 {
				note("玩家 %d 的护盾抵消 %d 点伤害", p+1, absorbed)
			}
			if wounds > 0 {
				pressure[p] = 0
				h.Wounded = true
				note("玩家 %d 受到 %d 点伤害，本轮推进中断", p+1, wounds)
			}
		}
	}
	dead := make([]bool, size)
	goldChanges := make([]int, size)
	siege := [2][]int{make([]int, g.N), make([]int, g.N)}
	for p := range hs {
		h := &hs[p]
		if h.Lane >= 0 && h.HP <= 0 {
			dead[p] = true
			g.Kills[1-h.Team]++
			siege[1-h.Team][h.Lane] += 3
			goldChanges[p] -= 2
			for attacker := range sources[p] {
				goldChanges[attacker]++
			}
			note("玩家 %d 阵亡回泉水，损失 2 金；%s在%s获得 3 点援军推进", p+1, dotaTeamName(1-h.Team), dotaLaneName(h.Lane, g.N))
			h.Lane = -1
			h.HP = h.maximum()
			h.Spent = 0
			pressure[p] = 0
		}
	}
	for p := range hs {
		hs[p].Gold = min(12, max(0, hs[p].Gold+goldChanges[p]))
	}
	for p, a := range orders {
		h := &hs[p]
		if a.Card == 4 && !dead[p] {
			if a.Buy != "" {
				h.Gold -= dotaPrice(*h, a.Buy)
				if (a.Buy == "phase_boots" || a.Buy == "arcane_boots") && h.has("boots") {
					h.Gear = slices.Delete(h.Gear, slices.Index(h.Gear, "boots"), slices.Index(h.Gear, "boots")+1)
				} else if len(h.Gear) >= 2 {
					h.Gear = h.Gear[1:]
				}
				h.Gear = append(h.Gear, a.Buy)
				note("玩家 %d 购买%s", p+1, dotaItem(a.Buy).Name)
			}
			h.Lane = -1
			h.Spent = 0
			h.HP = h.maximum()
			note("玩家 %d 完成整备：回满生命、收回行动牌，下轮出征", p+1)
		}
	}
	for p, h := range hs {
		if h.Lane >= 0 {
			siege[h.Team][h.Lane] += max(0, pressure[p]-slow[p])
		}
	}
	for l := 0; l < g.N; l++ {
		difference := siege[0][l] - siege[1][l]
		if difference == 0 {
			continue
		}
		direction := 1
		if difference < 0 {
			direction = -1
			difference = -difference
		}
		step := 1
		if difference >= 3 {
			step = 2
		}
		g.Track[l] += direction * step
		note("%s推进：近卫 %d / 天灾 %d，兵线向%s移动 %d 格", dotaLaneName(l, g.N), siege[0][l], siege[1][l], dotaTeamName(dotaBool(direction > 0)), step)
		if g.Track[l] >= 2 || g.Track[l] <= -2 {
			defending := dotaBool(g.Track[l] > 0)
			amount := 1
			if s.Round >= 21 {
				amount = 3
			} else if s.Round >= 13 {
				amount = 2
			}
			if g.Towers[defending][l] > 0 {
				g.Towers[defending][l] = max(0, g.Towers[defending][l]-amount)
				note("%s的%s防御塔受到 %d 攻城伤害，剩余 %d", dotaTeamName(defending), dotaLaneName(l, g.N), amount, g.Towers[defending][l])
			} else {
				g.Core[defending] = max(0, g.Core[defending]-amount)
				note("%s遗迹受到 %d 攻城伤害，剩余 %d", dotaTeamName(defending), amount, g.Core[defending])
			}
			g.Track[l] = 0
		}
	}
	for p, a := range orders {
		h := &hs[p]
		h.Gold = min(12, h.Gold+dotaBool(s.Round%2 == 0)+3*dotaBool(a.Card == 3 && h.Lane >= 0))
		h.Charge = min(3, h.Charge+1)
		if a.Card == 4 && !dead[p] && h.has("arcane_boots") {
			for j := range hs {
				if hs[j].Team == h.Team {
					hs[j].Charge = min(3, hs[j].Charge+1)
				}
			}
		}
	}
	if min(g.Core[0], g.Core[1]) == 0 {
		s.Finished = true
		s.Phase = "finished"
		winner := dotaBool(g.Core[1] > g.Core[0])
		if g.Core[0] == g.Core[1] {
			winner = firstTeam
		}
		s.Winners = g.seats(winner)
		note("%s摧毁敌方遗迹，全队共同获胜", dotaTeamName(winner))
	}
	if record {
		event.After = clone(hs)
		event.Core = g.Core
		event.Towers = clone(g.Towers)
		g.History = append(g.History, event)
		if len(g.History) > 5 {
			g.History = g.History[len(g.History)-5:]
		}
		s.Log = append(s.Log, event.Messages...)
		if len(s.Log) > 100 {
			s.Log = s.Log[len(s.Log)-100:]
		}
	}
	s.Round++
}
