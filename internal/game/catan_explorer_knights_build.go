package game

import (
	"errors"
	"slices"
)

// Medicine is enabled only by the shared owned-card dispatcher, never a
// client price field. Ordinary Explorer harbor upgrades use the same primitive.
func (c *catanExplorerCargo) upgradeSettlement(g *Catan, f *catanExplorerSailing, player int, sequence uint64, vertex int, kind string, medicine bool) error {
	if err := c.allowed(g, f, player, sequence, "action"); err != nil {
		return err
	}
	k := g.CitiesKnights
	if kind != "city" && kind != "harbor" || (kind == "city" || medicine) && k == nil || k != nil && (k.Pending != nil || k.Event != nil) {
		return errors.New("当前不能进行该城市或港口升级")
	}
	if vertex < 0 || vertex >= len(g.Vertices) || g.Vertices[vertex].Owner != player || g.Vertices[vertex].Level != 1 || !c.landVertex(g, player, vertex) {
		return errors.New("只能升级自己的已探索村庄，城市与港口不能互换")
	}
	cost := []int{0, 0, 0, 2, 3}
	if kind == "harbor" {
		count := 0
		for _, v := range g.Vertices {
			if v.Owner == player && catanExplorerHarborAt(g, v.ID) {
				count++
			}
		}
		if !catanExplorerCoast(g, vertex) || count >= 4 || k != nil && slices.Contains(k.FallenCities, vertex) {
			return errors.New("港口须由沿海村庄升级，横置城市不能变港口，且最多四座")
		}
		cost[4] = 2
	} else if !g.canCityUpgrade(player, vertex) {
		return errors.New("城市组件不足，或须优先修复横置城市")
	}
	if medicine {
		cost[3]--
		cost[4]--
	}
	if !catanExplorerCanPay(g, player, cost) {
		return errors.New("升级所需粮食或矿石不足")
	}
	catanExplorerPay(g, player, cost)
	g.Vertices[vertex].Level, g.Vertices[vertex].Harbor = 2, kind == "harbor"
	if kind == "city" {
		k.FallenCities = slices.DeleteFunc(k.FallenCities, func(v int) bool { return v == vertex })
	}
	g.Players[player].Score++
	return nil
}

// Private integration of the supported build/economy actions. The full
// combination dispatcher remains gated until all progress/knight rules work.
func (s *State) catanExplorerCityAction(player int, a Action) error {
	if err := s.validateExplorerCityProduction(); err != nil {
		return err
	}
	g, x := s.Catan, s.Catan.Explorer
	if s.Finished || s.Phase != "catan_turn" || player != s.Turn || a.Prompt < 1 || uint64(a.Prompt) != g.TurnSerial || g.CitiesKnights.Pending != nil || g.CitiesKnights.Event != nil || a.Skill != "" {
		return errors.New("不是当前组合行动玩家、阶段或序号")
	}
	if err := x.Economy.actionAllowed(g, x.Fleet, x.Cargo, player, g.TurnSerial); err != nil {
		return err
	}
	next := clone(*s)
	ng, nx := next.Catan, next.Catan.Explorer
	switch a.Type {
	case "catan_city", "catan_explorer_harbor":
		kind := "city"
		if a.Type == "catan_explorer_harbor" {
			kind = "harbor"
		}
		if a.Choice != "" {
			return errors.New("普通升级不能请求进步牌优惠")
		}
		if err := nx.Cargo.upgradeSettlement(ng, nx.Fleet, player, ng.TurnSerial, a.Vertex, kind, false); err != nil {
			return err
		}
		next.catanLog(player, "将村庄 #%d 升级为%s", a.Vertex+1, map[string]string{"city": "城市", "harbor": "港口"}[kind])
	case "catan_progress":
		if a.Card != 5 {
			return errors.New("本组合主动进步牌当前仅接入医学")
		}
		if err := next.catanPlayProgress(player, a); err != nil {
			return err
		}
	case "catan_explorer_spice_gold":
		if _, err := next.applyCatanExplorerSpice(player, a); err != nil {
			return err
		}
	default:
		return errors.New("尚未接入的组合行动")
	}
	next.catanScores()
	next.catanVictory()
	ng.Trade = nil
	if err := next.validateExplorerCityProduction(); err != nil {
		return err
	}
	*s = next
	return nil
}
