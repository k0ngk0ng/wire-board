package game

import (
	"errors"
	"slices"
)

func (g *Catan) politicsOpponent(player, target int) bool {
	return target >= 0 && target < len(g.Players) && target != player && !g.Players[target].Eliminated
}
func (g *Catan) intrigueTargets(player int) []int {
	out := []int{}
	for _, n := range g.CitiesKnights.Knights {
		if !g.politicsOpponent(player, n.Owner) {
			continue
		}
		for _, e := range g.touching(n.Vertex) {
			if g.Edges[e].Owner == player {
				out = append(out, n.Vertex)
				break
			}
		}
	}
	return out
}
func (s *State) catanPoliticsProgress(player int, a Action) error {
	g := s.Catan
	k := g.CitiesKnights
	switch a.Card {
	case 16:
		if g.Seafarers != nil {
			return errors.New("外交与航海家组合尚未接入")
		}
		if !slices.Contains(g.diplomacyRoads(), a.Edge) {
			return errors.New("外交只能移除开放道路，不能拆开建筑或骑士间的封闭路线")
		}
		owner := g.Edges[a.Edge].Owner
		g.Edges[a.Edge].Owner = -1
		s.catanLog(player, "外交：移除玩家 %d 的道路 #%d", owner+1, a.Edge+1)
		if owner == player && len(g.diplomacyPlacements(player)) > 0 {
			k.Pending = &CatanCityPending{Kind: "diplomacy", Players: []int{player}}
			s.Phase = "catan_diplomacy"
		}
	case 17:
		count := 0
		for i := range k.Knights {
			n := &k.Knights[i]
			if n.Owner == player && !n.Active {
				n.Active = true
				n.ActivatedAt = k.ActionSerial
				count++
			}
		}
		s.catanLog(player, "免费激活%d名骑士；原先已激活的骑士保留行动资格", count)
	case 18:
		if !g.politicsOpponent(player, a.Target) {
			return errors.New("请选择另一位在场玩家的进步手牌")
		}
		if len(k.Players[a.Target].Progress) > 0 {
			k.Pending = &CatanCityPending{Kind: "espionage", Players: []int{player}, Target: a.Target}
			s.Phase = "catan_espionage"
		}
	case 19:
		if !slices.Contains(g.intrigueTargets(player), a.Vertex) {
			return errors.New("阴谋只能驱逐位于自己路线端点的对手骑士")
		}
		displaced := *g.knightAt(a.Vertex)
		k.Knights = slices.DeleteFunc(k.Knights, func(n CatanKnight) bool { return n.Vertex == a.Vertex })
		s.catanLog(player, "驱逐交点 #%d 上玩家 %d 的%d级骑士", a.Vertex+1, displaced.Owner+1, displaced.Strength)
		if len(g.knightDestinations(displaced, true)) > 0 {
			k.Pending = &CatanCityPending{Kind: "knight_retreat", Players: []int{displaced.Owner}, Knight: &displaced}
			s.Phase = "catan_knight_retreat"
		} else {
			s.catanLog(displaced.Owner, "被驱逐的骑士没有合法退路，返回库存")
		}
	case 20, 24:
		kind := "wedding"
		if a.Card == 20 {
			kind = "sabotage"
		}
		q := &CatanCityPending{Kind: kind, Players: []int{}, Target: player}
		own := g.Players[player].Score - g.hiddenVictoryPoints(player)
		for offset := 1; offset < len(g.Players); offset++ {
			p := (player + offset) % len(g.Players)
			score := g.Players[p].Score - g.hiddenVictoryPoints(p)
			count := sum(g.Players[p].Resources)
			if g.Players[p].Eliminated {
				continue
			}
			if (a.Card == 24 && score > own && count > 0) || (a.Card == 20 && score >= own && count >= 2) {
				q.Players = append(q.Players, p)
			}
		}
		if len(q.Players) > 0 {
			k.Pending = q
			s.Phase = "catan_" + kind
		}
	case 21:
		if k.Invasions == 0 || !g.robberAllowed(a.Tile) {
			return errors.New("首次蛮族进攻后才能使用征税，并须将强盗移至另一块陆地")
		}
		g.Robber = a.Tile
		s.catanLog(player, "征税：将强盗移至地块 #%d", a.Tile+1)
		seen := map[int]bool{}
		for _, id := range g.Tiles[a.Tile].Vertices {
			v := g.Vertices[id]
			if v.Level > 0 && g.politicsOpponent(player, v.Owner) && !seen[v.Owner] && sum(g.Players[v.Owner].Resources) > 0 {
				seen[v.Owner] = true
				hand := g.Players[v.Owner].Resources
				pick := catanRandom(sum(hand))
				for c, n := range hand {
					if pick < n {
						hand[c]--
						g.Players[player].Resources[c]++
						break
					}
					pick -= n
				}
				s.catanLog(player, "从玩家 %d 随机偷取一张资源或商品", v.Owner+1)
			}
		}
	case 22:
		if !g.politicsOpponent(player, a.Target) {
			return errors.New("请选择另一位拥有骑士的在场玩家")
		}
		found := false
		for _, n := range k.Knights {
			if n.Owner == a.Target {
				found = true
			}
		}
		if !found {
			return errors.New("该玩家没有可移除的骑士")
		}
		k.Pending = &CatanCityPending{Kind: "treason_remove", Players: []int{a.Target}, Target: player}
		s.Phase = "catan_treason_remove"
	default:
		return errors.New("该政治进步牌效果尚未接入")
	}
	// A displaced/removed knight may temporarily change the longest route. Do
	// not award victory until its mandatory response and final placement finish.
	if k.Pending == nil {
		s.catanScores()
		s.catanVictory()
	}
	return nil
}
func (s *State) catanPoliticsChoice(player int, a Action) error {
	g := s.Catan
	k := g.CitiesKnights
	q := k.Pending
	if a.Type != "catan_"+q.Kind || s.Phase != a.Type {
		return errors.New("请完成当前政治进步牌回应")
	}
	switch q.Kind {
	case "diplomacy":
		if a.Choice != "skip" {
			if a.Choice != "" || !slices.Contains(g.diplomacyPlacements(player), a.Edge) {
				return errors.New("请选择合法位置重建道路，或放弃重建")
			}
			k.Pending = nil
			g.FreeRoads = 1
			g.ResumePhase = "catan_turn"
			s.Phase = "catan_roads"
			return s.catanBuild(player, Action{Type: "catan_road", Edge: a.Edge}, false)
		}
		s.catanLog(player, "放弃外交的免费重建道路")
	case "espionage":
		if a.Choice == "skip" {
			s.catanLog(player, "查看进步牌后放弃取牌")
		} else {
			hand := k.Players[q.Target].Progress
			at := slices.Index(hand, a.Card)
			if a.Choice != "" || at < 0 || catanProgressRules[a.Card].Victory {
				return errors.New("请选择被展示的一张非胜利点进步牌")
			}
			k.Players[q.Target].Progress = slices.Delete(hand, at, at+1)
			k.Players[player].Progress = append(k.Players[player].Progress, a.Card)
			s.catanLog(player, "从玩家 %d 的进步手牌中取得一张牌", q.Target+1)
		}
	case "wedding", "sabotage":
		hand := g.Players[player].Resources
		due := min(2, sum(hand))
		if q.Kind == "sabotage" {
			due = sum(hand) / 2
		}
		if a.Choice != "" || !g.cardBundle(a.Give) || sum(a.Give) != due || !catanHas(hand, a.Give) {
			return errors.New("请选择规定数量的资源或商品")
		}
		to := g.Bank
		if q.Kind == "wedding" {
			to = g.Players[q.Target].Resources
		}
		catanMove(hand, to, a.Give)
		if q.Kind == "wedding" {
			s.catanLog(player, "婚礼：向玩家 %d 交出%d张资源或商品", q.Target+1, due)
		} else {
			s.catanLog(player, "破坏：弃置%d张资源或商品", due)
		}
	case "treason_remove":
		n := g.knightAt(a.Vertex)
		if a.Choice != "" || n == nil || n.Owner != player {
			return errors.New("必须选择自己的一名骑士移除")
		}
		removed := *n
		k.Knights = slices.DeleteFunc(k.Knights, func(n CatanKnight) bool { return n.Vertex == a.Vertex })
		s.catanLog(player, "因叛变移除交点 #%d 的%d级骑士", a.Vertex+1, removed.Strength)
		if len(g.treasonPlacements(q.Target, removed.Strength)) > 0 {
			k.Pending = &CatanCityPending{Kind: "treason_place", Players: []int{q.Target}, Knight: &removed}
			s.Phase = "catan_treason_place"
			return nil
		}
		s.catanLog(q.Target, "没有可放置的骑士或位置，仍完成对手骑士移除")
	case "treason_place":
		if a.Choice == "skip" {
			s.catanLog(player, "放弃叛变后的免费骑士放置")
		} else {
			if a.Choice != "" || q.Knight == nil || a.Color < 1 || a.Color > q.Knight.Strength || g.knightCount(player, a.Color) >= 2 || !g.knightPlaceable(player, a.Vertex) {
				return errors.New("请选择有库存的同级或更低级骑士，放在己方路线相接的空交点")
			}
			k.Knights = append(k.Knights, CatanKnight{Owner: player, Vertex: a.Vertex, Strength: a.Color, Active: q.Knight.Active})
			s.catanLog(player, "叛变：在交点 #%d 免费放置%d级骑士，保留被移除骑士的激活状态", a.Vertex+1, a.Color)
		}
	default:
		return errors.New("未知政治进步牌回应")
	}
	q.Players = q.Players[1:]
	if len(q.Players) == 0 {
		k.Pending = nil
		s.Phase = "catan_turn"
		s.catanScores()
		s.catanVictory()
	}
	return nil
}
func (g *Catan) treasonPlacements(player, maxStrength int) []int {
	stock := false
	for strength := 1; strength <= maxStrength; strength++ {
		if g.knightCount(player, strength) < 2 {
			stock = true
		}
	}
	out := []int{}
	if stock {
		for _, v := range g.Vertices {
			if g.knightPlaceable(player, v.ID) {
				out = append(out, v.ID)
			}
		}
	}
	return out
}
