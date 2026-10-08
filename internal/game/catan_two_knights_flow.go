package game

import (
	"errors"
	"slices"
)

// These checks belong to the marked two-player recipe. Legacy ordinary and
// multiplayer city games keep their existing restoration contract.
func (s *State) validateTwoCityFlow() error {
	g, k := s.Catan, s.Catan.CitiesKnights
	if k.Layout != "variable" || (!g.twoSeafarersKnights() && !g.riverKnights() && k.RobberStart != g.twoDesert()) || k.EventDie < -1 || k.EventDie > 5 || g.RollID == 0 && k.EventDie != -1 || g.RollID > 0 && k.EventDie < 0 && g.CardEvent == nil || k.Chase != "" && k.Chase != "robber" && !(g.twoSeafarersKnights() && k.Chase == "pirate") {
		return errors.New("双人城市事件或强盗状态无效")
	}
	if g.twoSeafarersKnights() && (k.RobberStart < -1 || k.RobberStart >= len(g.Tiles) || k.PirateStart < -1 || k.PirateStart >= len(g.Tiles) || g.wonders() != nil && k.PirateStart != -1) {
		return errors.New("双人骑士强盗海盗起点无效")
	}
	for _, p := range g.Players {
		if len(p.Resources) != 8 || sum(p.Dev) != 0 || sum(p.NewDev) != 0 || p.Knights != 0 {
			return errors.New("双人城市玩家组件无效")
		}
	}
	if err := s.validateCityProgressInventory(); err != nil {
		return err
	}
	if err := s.validateExplorerCityDevelopment(); err != nil {
		return err
	}
	seen := map[int]bool{}
	for _, vertex := range k.FallenCities {
		if vertex < 0 || vertex >= len(g.Vertices) || g.Vertices[vertex].Owner < 0 || g.Vertices[vertex].Owner >= 2 || g.Vertices[vertex].Level != 1 || seen[vertex] {
			return errors.New("双人横置城市记录无效")
		}
		seen[vertex] = true
	}
	if m := k.Merchant; m != nil && (m.Owner < 0 || m.Owner >= 2 || !slices.Contains(g.merchantTiles(m.Owner), m.Tile)) {
		return errors.New("双人商人位置无效")
	}
	if powers := k.TradePowers; powers != nil {
		if powers.Player != s.Turn || len(powers.Fleets)+len(powers.Harbors) == 0 || len(powers.Fleets) > 2 || len(powers.Harbors) > 2 || len(g.Two.Rolls) != 2 {
			return errors.New("双人贸易能力行动段无效")
		}
		for i, c := range powers.Fleets {
			if c < 0 || c >= 8 || slices.Contains(powers.Fleets[:i], c) {
				return errors.New("商船队种类无效")
			}
		}
		for _, remaining := range powers.Harbors {
			if len(remaining) > 1 || len(remaining) == 1 && remaining[0] != 1-s.Turn {
				return errors.New("商业港剩余目标无效")
			}
		}
	}
	if e := k.Event; e != nil {
		if len(g.Two.Rolls) == 0 || e.Red < 1 || e.Red > 6 || e.Yellow > 6 || e.Yellow < 0 || (e.Production == 0 && e.Yellow == 0) || (e.Production != 0 && (g.EventDeck == nil || e.Yellow != 0 || e.Production < 2 || e.Production > 12)) || e.Face < 0 || e.Face > 5 || e.Face != k.EventDie {
			return errors.New("双人城市事件骰无效")
		}
		for _, task := range e.Tasks {
			if task.Player < 0 || task.Player >= 2 || !slices.Contains([]string{"draw", "pillage", "defender_reward"}, task.Kind) || task.Kind == "draw" && (task.Track < 0 || task.Track > 2) {
				return errors.New("双人城市事件队列无效")
			}
		}
	}
	if err := s.validateExplorerCityEventQueue(); err != nil {
		return err
	}
	q := k.Pending
	if q == nil {
		if slices.Contains([]string{"catan_pillage", "catan_defender_reward", "catan_progress_discard", "catan_progress_end", "catan_aqueduct", "catan_metropolis", "catan_knight_retreat", "catan_guild_dues", "catan_commercial_harbor", "catan_diplomacy", "catan_espionage", "catan_sabotage", "catan_wedding", "catan_treason_remove", "catan_treason_place"}, s.Phase) {
			return errors.New("双人城市回应缺失")
		}
		return nil
	}
	ending := q.Kind == "progress_discard" && s.Phase == "catan_progress_end"
	if !ending && s.Phase != "catan_"+q.Kind || len(q.Players) > 2 || len(q.Players) == 2 && q.Players[0] == q.Players[1] || g.Trade != nil || (q.Ship && !(g.twoSeafarersKnights() && q.Kind == "diplomacy")) || q.Warship {
		return errors.New("双人城市回应阶段无效")
	}
	actor := q.Players[0]
	if q.Source != "" && !(q.Kind == "knight_retreat" && q.Source == "intrigue") && !(q.Kind == "treason_remove" && q.Source == "two_neutral") {
		return errors.New("双人城市回应来源无效")
	}
	switch q.Kind {
	case "pillage", "defender_reward", "progress_discard":
		if ending {
			if k.Event != nil || len(q.Players) != 1 || actor != s.Turn || len(k.Players[actor].Progress) <= 4 || len(g.Two.Rolls) != 2 {
				return errors.New("双人回合末弃进步牌无效")
			}
		} else if k.Event == nil {
			return errors.New("双人城市生产事件缺失")
		}
	case "aqueduct":
		if k.Event != nil || len(g.Two.Rolls) == 0 || sum(g.Bank[:5]) == 0 {
			return errors.New("双人引水渠阶段无效")
		}
		for _, p := range q.Players {
			if k.Players[p].Improvements[CatanScience] < 3 {
				return errors.New("未取得引水渠能力")
			}
		}
	case "metropolis", "knight_retreat", "guild_dues", "commercial_harbor", "diplomacy", "espionage", "sabotage", "wedding", "treason_remove", "treason_place":
		if len(g.Two.Rolls) != 2 || k.Event != nil || len(q.Players) != 1 {
			return errors.New("双人城市行动回应无效")
		}
		return s.validateTwoCityActionResponse()
	default:
		return errors.New("未知双人城市回应")
	}
	return nil
}

func (s *State) validateTwoCityActionResponse() error {
	g, k := s.Catan, s.Catan.CitiesKnights
	q := k.Pending
	actor := q.Players[0]
	switch q.Kind {
	case "diplomacy":
		if actor != s.Turn || len(g.diplomacyPlacements(actor)) == 0 {
			return errors.New("外交缺少合法路线重放位置")
		}
	case "knight_retreat":
		n := q.Knight
		if n == nil || n.Owner == s.Turn || actor != g.knightResponseActor(n.Owner, s.Turn) || len(g.knightDestinations(*n, true)) == 0 {
			return errors.New("双人骑士撤退玩家或退路无效")
		}
		attacker := g.knightAt(n.Vertex)
		valid := q.Source == "" && attacker != nil && attacker.Owner == s.Turn && !attacker.Active && attacker.Strength > n.Strength
		if q.Source == "intrigue" && attacker == nil {
			for _, edge := range g.touching(n.Vertex) {
				valid = valid || g.Edges[edge].Owner == s.Turn
			}
		}
		if !valid {
			return errors.New("双人骑士撤退缺少合法驱逐")
		}
	case "treason_remove":
		owner := actor
		if q.Source == "two_neutral" {
			owner = q.Color
			if !g.twoNeutralKnightOwner(owner) || actor != s.Turn {
				return errors.New("叛变中立颜色或控制者无效")
			}
		} else if !g.politicsOpponent(s.Turn, owner) {
			return errors.New("叛变对手无效")
		}
		if q.Target != s.Turn || g.twoWeakestKnight(owner) > 3 {
			return errors.New("叛变缺少可移除骑士")
		}
	case "treason_place":
		n := q.Knight
		if actor != s.Turn || n == nil || !(g.politicsOpponent(actor, n.Owner) || g.twoNeutralKnightOwner(n.Owner)) || n.Vertex < 0 || n.Vertex >= len(g.Vertices) || g.Vertices[n.Vertex].Level != 0 || g.knightAt(n.Vertex) != nil || n.Strength < 1 || n.Strength > 3 || n.ActivatedAt > k.ActionSerial || n.PromotedAt > k.ActionSerial || g.twoNeutralKnightOwner(n.Owner) && (n.Strength > 2 || n.Active || n.ActivatedAt != 0) || len(g.treasonPlacements(actor, n.Strength)) == 0 {
			return errors.New("叛变免费放置记录无效")
		}
	case "metropolis":
		if actor != s.Turn || q.Track < 0 || q.Track > 2 || k.Players[actor].Improvements[q.Track] < 4 || len(g.cityMetropolisSites(actor)) == 0 {
			return errors.New("双人大都会选择无效")
		}
	case "guild_dues":
		if actor != s.Turn || !slices.Contains(g.guildDuesTargets(actor), q.Target) || sum(g.Players[q.Target].Resources) == 0 {
			return errors.New("行会征费目标无效")
		}
	case "commercial_harbor":
		if q.Target != s.Turn || actor != 1-s.Turn || q.Color < 0 || q.Color >= 5 || g.Players[s.Turn].Resources[q.Color] == 0 || sum(g.Players[actor].Resources[5:]) == 0 || k.TradePowers == nil {
			return errors.New("商业港交换记录无效")
		}
	default:
		return s.validateExplorerPolitics()
	}
	return nil
}
