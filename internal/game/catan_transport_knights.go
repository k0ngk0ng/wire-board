package game

import (
	"errors"
	"slices"
)

const CatanTransportKnightsRules = "catan-transport-knights-2025"

func NewCatanTransportCitiesKnights(n int) (*State, error) {
	s, err := NewCatanTransport(n)
	if err != nil {
		return nil, err
	}
	s.enableCitiesKnights()
	g := s.Catan
	g.Transport.Knights = CatanTransportKnightsRules
	g.Transport.DeckRecipe = ""
	// The combination removes a forest instead of a field from the scenario.
	for i := range g.Tiles {
		if g.Tiles[i].Resource == 0 {
			g.Tiles[i].Resource = 3
			break
		}
	}
	if g.Two != nil {
		g.Two.Knights = CatanTwoKnightsRules
	}
	s.Log = []string{"运输＋城市与骑士：取消运输发展牌，使用进步牌；不使用强盗与最长道路，15分获胜", "蛮族船与三名道路蛮族分别运作；激活骑士可驱赶相邻道路蛮族"}
	s.catanScores()
	return s, s.validateCatanTransport()
}
func (g *Catan) transportKnights() bool {
	return g.Transport != nil && g.Transport.Knights == CatanTransportKnightsRules && g.CitiesKnights != nil
}
func (s *State) validateTransportKnights() error {
	g := s.Catan
	if g.attackTransportKnights() {
		return nil
	} // Road-knight validation is owned by Attack.
	if g.Transport.Knights == "" && g.CitiesKnights == nil {
		return nil
	}
	if !g.transportKnights() || g.CitiesKnights.Rules != catanCitiesKnightsRules(len(g.Players)) || g.CitiesKnights.RobberStart != -1 || g.CitiesKnights.Chase != "" || g.Transport.Swift || g.Transport.Moves > 1 {
		return errors.New("运输城市骑士组合状态无效")
	}
	if err := s.validateTransportCityFlow(); err != nil {
		return err
	}
	if err := s.validateCityProgressInventory(); err != nil {
		return err
	}
	return s.validateExplorerCityDevelopment()
}
func (g *Catan) transportKnightBarbarians(n *CatanKnight) []int {
	out := []int{}
	if !g.transportKnights() || !g.knightCanAct(n) {
		return out
	}
	for i, id := range g.Transport.Barbarians {
		e := g.Edges[id]
		if e.A == n.Vertex || e.B == n.Vertex {
			out = append(out, i)
		}
	}
	return out
}
func (s *State) catanTransportKnightChase(player int, a Action) error {
	g := s.Catan
	if g == nil || !g.transportKnights() {
		return errors.New("当前不是运输城市骑士组合")
	}
	n := g.knightAt(a.Vertex)
	if s.Phase != "catan_turn" || player != s.Turn || n == nil || n.Owner != player || !slices.Contains(g.transportKnightBarbarians(n), a.Card) || a.Edge < 0 || a.Edge >= len(g.Edges) || slices.Contains(g.Transport.Barbarians[:], a.Edge) {
		return errors.New("请选择此前已激活的己方骑士、相邻蛮族及一个没有蛮族的边")
	}
	n.Active = false
	g.Trade = nil
	g.ResumePhase = "catan_turn"
	s.catanTransportBeginBarbarian()
	s.catanLog(player, "交点 #%d 的骑士驱赶道路蛮族，骑士转为未激活", a.Vertex+1)
	return s.catanTransportBarbarian(player, Action{Type: "catan_transport_barbarian", Offer: int(g.Transport.BarbarianSequence), Card: a.Card, Edge: a.Edge})
}
func (s *State) catanTransportBeginTravel(player int) error {
	g, t := s.Catan, s.Catan.Transport
	if err := t.beginTravel(g, player); err != nil {
		return err
	}
	t.Moves = 1
	g.Trade = nil
	s.Phase = "catan_transport_move"
	s.catanLog(player, "结束建设与交易，开始移动马车（%d点）", t.Travel.Points)
	return nil
}

// Preserve the event face while rerolling unusable production totals. The
// ordinary transport and standard city pipelines retain their own rules.
func (s *State) catanTransportCityRoll(dice func() [2]int, face int) error {
	g := s.Catan
	pair := dice()
	for (len(g.Players) <= 4 && !g.riversTransport() && !g.caravansTransport() && (pair[0]+pair[1] == 2 || pair[0]+pair[1] == 12)) || (g.twoKnights() && len(g.Two.Rolls) == 1 && pair[0]+pair[1] == g.Two.Rolls[0]) {
		pair = dice()
	}
	return s.catanCityRoll(pair[0], pair[1], face)
}

var catanTransportCityPhases = []string{"catan_pillage", "catan_defender_reward", "catan_progress_discard", "catan_progress_end", "catan_aqueduct", "catan_metropolis", "catan_knight_retreat", "catan_guild_dues", "catan_commercial_harbor", "catan_diplomacy", "catan_espionage", "catan_sabotage", "catan_wedding", "catan_treason_remove", "catan_treason_place"}

func (s *State) validateTransportCityFlow() error {
	g, k := s.Catan, s.Catan.CitiesKnights
	if len(k.Players) != len(g.Players) || k.Layout != "variable" || k.ActionSerial == 0 || k.ActionSerial < g.TurnSerial || k.Invasions < 0 || k.BarbarianPosition < 0 || k.BarbarianPosition > catanBarbarianDistance || k.EventDie < -1 || k.EventDie > 5 {
		return errors.New("运输城市组件、行动序号或蛮族轨道无效")
	}
	q := k.Pending
	if slices.Contains(catanTransportCityPhases, s.Phase) != (q != nil) && !s.Finished {
		return errors.New("运输城市回应缺失或阶段不匹配")
	}
	if q != nil {
		ending := q.Kind == "progress_discard" && s.Phase == "catan_progress_end"
		if len(q.Players) == 0 || !slices.Contains(catanTransportCityPhases, "catan_"+q.Kind) || !s.Finished && !ending && s.Phase != "catan_"+q.Kind || g.Trade != nil {
			return errors.New("运输城市回应类型或阶段无效")
		}
		for i, p := range q.Players {
			if p < 0 || p >= len(g.Players) || g.Players[p].Eliminated || slices.Contains(q.Players[:i], p) {
				return errors.New("运输城市回应玩家无效")
			}
		}
		if ending && (k.Event != nil || len(q.Players) != 1 || q.Players[0] != s.Turn || len(k.Players[s.Turn].Progress) <= 4) {
			return errors.New("运输回合结束弃牌无效")
		}
	}
	if e := k.Event; e != nil {
		if e.Red < 1 || e.Red > 6 || e.Yellow < 0 || e.Yellow > 6 || (e.Production == 0 && e.Yellow == 0) || e.Production != 0 && (g.EventDeck == nil || e.Yellow != 0 || e.Production < 2 || e.Production > 12) || e.Face < 0 || e.Face > 5 || e.Face != k.EventDie {
			return errors.New("运输城市事件骰无效")
		}
		for _, task := range e.Tasks {
			if task.Player < 0 || task.Player >= len(g.Players) || !slices.Contains([]string{"draw", "pillage", "defender_reward"}, task.Kind) || task.Kind == "draw" && (task.Track < 0 || task.Track > 2) {
				return errors.New("运输城市事件队列无效")
			}
		}
	}
	if err := s.validateExplorerCityEventQueue(); err != nil {
		return err
	}
	counts := map[int][3]int{}
	seen := map[int]bool{}
	check := func(n CatanKnight, retreat bool) error {
		neutral := g.twoNeutralKnightOwner(n.Owner)
		if (!neutral && (n.Owner < 0 || n.Owner >= len(g.Players) || g.Players[n.Owner].Eliminated)) || n.Vertex < 0 || n.Vertex >= len(g.Vertices) || n.Strength < 1 || n.Strength > 3 || n.ActivatedAt > k.ActionSerial || n.PromotedAt > k.ActionSerial {
			return errors.New("运输骑士记录无效")
		}
		if g.Transport.Map.siteAt(n.Vertex) >= 0 || g.Vertices[n.Vertex].Level != 0 || !retreat && seen[n.Vertex] || neutral && (n.Active || n.Strength > 2) {
			return errors.New("运输骑士交点或中立状态无效")
		}
		if !retreat {
			seen[n.Vertex] = true
		}
		stock := counts[n.Owner]
		stock[n.Strength-1]++
		counts[n.Owner] = stock
		if stock[n.Strength-1] > 2 {
			return errors.New("运输骑士库存超过两枚")
		}
		return nil
	}
	for _, n := range k.Knights {
		if err := check(n, false); err != nil {
			return err
		}
	}
	if q != nil && q.Kind == "knight_retreat" {
		if q.Knight == nil {
			return errors.New("运输撤退骑士缺失")
		}
		if err := check(*q.Knight, true); err != nil {
			return err
		}
	}
	return nil
}
