package game

import (
	"errors"
	"slices"
)

func (g *Catan) progressResourceGain(player, color int) int {
	hexes := 0
	for _, tile := range g.Tiles {
		if tile.Resource != color {
			continue
		}
		for _, v := range tile.Vertices {
			if g.Vertices[v].Owner == player && g.Vertices[v].Level > 0 {
				hexes++
				break
			}
		}
	}
	return min(2*hexes, g.Bank[color])
}
func (g *Catan) inventionTiles() []int {
	out := []int{}
	for _, t := range g.Tiles {
		if t.Number > 0 && !slices.Contains([]int{2, 6, 8, 12}, t.Number) {
			out = append(out, t.ID)
		}
	}
	return out
}

func (s *State) catanPlayProgress(player int, a Action) error {
	return s.catanPlayProgressRandom(player, a, catanRandom)
}

// The random source belongs to the server, never to Action. Alchemy changes
// production dice only; keep the event roll independently testable.
func (s *State) catanPlayProgressRandom(player int, a Action, randN func(int) int) error {
	g := s.Catan
	k := g.CitiesKnights
	if k == nil || g.setup() || player != s.Turn || a.Card < 0 || a.Card >= len(catanProgressRules) {
		return errors.New("当前不能使用进步牌")
	}
	rule := catanProgressRules[a.Card]
	at := slices.Index(k.Players[player].Progress, a.Card)
	if at < 0 || rule.Victory {
		return errors.New("没有这张可使用的进步牌")
	}

	if (a.Card == 0 && (s.Phase != "catan_roll" || g.twoKnights() && len(g.Two.Rolls) > 0)) || (a.Card != 0 && s.Phase != "catan_turn") {
		return errors.New("炼金术只能在掷骰前使用，其他进步牌只能在行动阶段使用")
	}
	if a.Card == 0 && (len(a.Tokens) != 2 || a.Tokens[0] < 1 || a.Tokens[0] > 6 || a.Tokens[1] < 1 || a.Tokens[1] > 6 || a.Choice != "") {
		return errors.New("请选择红骰和普通骰各1至6的点数")
	}
	if a.Card == 21 && k.Invasions == 0 {
		return errors.New("首次蛮族进攻前不能使用征税")
	}
	// State.Apply clones C&K before dispatch. Any invalid target/cost below rolls
	// back the complete effect, including consumption and the bottom-of-deck return.
	k.Players[player].Progress = slices.Delete(k.Players[player].Progress, at, at+1)
	k.returnProgress([]int{a.Card})
	k.recordProgress("play", player, -1, rule.Track, 1, &a.Card)
	g.Trade = nil
	names := []string{"炼金术", "起重机", "工程学", "发明", "灌溉", "医学", "采矿", "道路建设", "锻造", "印刷术", "商业港", "行会征费", "商人", "商船队", "资源垄断", "商品垄断", "外交", "鼓舞", "间谍", "阴谋", "破坏", "征税", "叛变", "宪法", "婚礼"}
	s.catanLog(player, "使用进步牌「%s」", names[a.Card])
	if a.Card != 0 && a.Choice == "skip" {
		s.catanLog(player, "放弃此次卡牌收益，卡牌仍放回牌堆底部")
		return nil
	}
	if a.Choice != "" && !(g.Explorer != nil && a.Card == 5 && a.Choice == "harbor") {
		return errors.New("未知进步牌选项")
	}
	if a.Card >= 16 {
		return s.catanPoliticsProgress(player, a)
	}
	if a.Card >= 10 {
		return s.catanTradeProgress(player, a)
	}
	switch a.Card {
	case 0:
		// Only the production dice are chosen. The event die remains server-random.
		face := randN(6)
		if g.Explorer != nil {
			return s.catanExplorerCityRoll(a.Tokens[0], a.Tokens[1], face)
		}
		return s.catanCityRoll(a.Tokens[0], a.Tokens[1], face)
	case 1:
		return s.catanCityBuild(player, Action{Type: "catan_improvement", Color: a.Color}, 1)
	case 2:
		return s.catanCityBuild(player, Action{Type: "catan_wall", Vertex: a.Vertex}, 2)
	case 3:
		legal := g.inventionTiles()
		if a.Tile == a.Target || !slices.Contains(legal, a.Tile) || !slices.Contains(legal, a.Target) {
			return errors.New("请选择两个不同地块上的数字，不能交换2、6、8、12")
		}
		left, right := g.Tiles[a.Tile].Number, g.Tiles[a.Target].Number
		if g.Explorer != nil {
			if err := g.Explorer.Board.swapNumbers(g, a.Tile, a.Target); err != nil {
				return err
			}
		} else {
			g.Tiles[a.Tile].Number, g.Tiles[a.Target].Number = right, left
		}
		s.catanLog(player, "交换地块 #%d 的%d和地块 #%d 的%d，强盗位置不变", a.Tile+1, left, a.Target+1, right)
	case 4, 6:
		color := 3
		if a.Card == 6 {
			color = 4
		}
		gain := g.progressResourceGain(player, color)
		g.Players[player].Resources[color] += gain
		g.Bank[color] -= gain
		s.catanLog(player, "按相邻地块领取%s×%d（每个地块最多两张，受银行库存限制）", catanCardName(color), gain)
	case 5:
		if g.Explorer != nil {
			kind := "city"
			if a.Choice == "harbor" {
				kind = "harbor"
			}
			return g.Explorer.Cargo.upgradeSettlement(g, g.Explorer.Fleet, player, g.TurnSerial, a.Vertex, kind, true)
		}
		return s.catanBuild(player, Action{Type: "catan_city", Vertex: a.Vertex}, true)
	case 7:
		if g.Explorer != nil {
			return s.catanExplorerBeginFreeRoads(player)
		}
		g.ResumePhase = "catan_turn"
		if g.hasFreeRouteAction(player) {
			g.FreeRoads = 2
			s.Phase = "catan_roads"
		} else {
			s.catanLog(player, "没有可放置的免费路线，卡牌仍放回牌堆底部")
		}
	case 8:
		if len(a.Targets) > 2 {
			return errors.New("锻造最多升级两名骑士")
		}
		if len(a.Targets) == 2 && a.Targets[0] == a.Targets[1] {
			return errors.New("不能用锻造将同一骑士升级两次")
		}
		for _, v := range a.Targets {
			if err := s.catanKnightActionCost(player, Action{Type: "catan_knight_promote", Vertex: v}, true); err != nil {
				return err
			}
		}
	}
	s.catanScores()
	s.catanVictory()
	return nil
}
