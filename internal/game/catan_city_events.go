package game

import (
	"errors"
	"fmt"
	"slices"
)

const catanBarbarianDistance = 7

type CatanCityEventTask struct {
	Kind   string `json:"kind"`
	Player int    `json:"player"`
	Track  int    `json:"track"`
}
type CatanCityEvent struct {
	Epidemic   bool                 `json:"epidemic,omitempty"`
	Production int                  `json:"production,omitempty"` // Event-card number; zero means legacy red + yellow dice.
	Red        int                  `json:"red"`
	Yellow     int                  `json:"yellow"`
	Face       int                  `json:"face"` // science/trade/politics 0–2; three ship faces 3–5
	Attack     bool                 `json:"attack"`
	Tasks      []CatanCityEventTask `json:"tasks"`
}

// Callers use server dice, never a client's preferred event result.
// This method implements the real
// event -> response -> production pipeline without substituting base rules.
func (s *State) catanCityRoll(red, yellow, face int) error {
	g := s.Catan
	k := g.CitiesKnights
	if k == nil || !g.citySeaSupported() || s.Phase != "catan_roll" || k.Event != nil || k.Pending != nil || red < 1 || red > 6 || yellow < 1 || yellow > 6 || face < 0 || face > 5 {
		return errors.New("无效城市与骑士掷骰状态")
	}
	if g.twoKnights() && (len(g.Two.Rolls) >= 2 || len(g.Two.Rolls) == 1 && g.Two.Rolls[0] == red+yellow) {
		return errors.New("双人第二次生产必须使用不同总点数")
	}
	if g.EventDeck != nil {
		if err := s.validateCatanEventSession(); err != nil {
			return err
		}
		if g.RollID == int(^uint(0)>>1) {
			return errors.New("生产次数溢出")
		}
		g.EventDeck.AlchemyRolls++
		g.EventDeck.LastAlchemyRoll = g.RollID + 1
		s.catanLog(s.Turn, "炼金术替代本次事件牌抽取，牌堆保持原状")
	}
	if g.twoKnights() {
		g.Two.Rolls = append(g.Two.Rolls, red+yellow)
	}
	g.RevealedEvent = nil
	g.Dice = []int{red, yellow}
	g.RollID++
	if p := g.pirateIslands(); p != nil && !g.attackSeaKnights() {
		f := &CatanEventFleet{RollID: g.RollID, Dice: [2]int{red, yellow}}
		if g.EventDeck != nil {
			g.EventDeck.Fleet = f
		} else {
			p.CityFleet = f
		}
	}
	return s.catanStartCityDiceEvent(red, yellow, face, 0, false)
}

// Card text has already resolved before this shared event-die pipeline starts.
// A nonzero production number is independent of the separately rolled red die.
func (s *State) catanStartCityDiceEvent(red, yellow, face, production int, epidemic bool) error {
	g := s.Catan
	k := g.CitiesKnights
	k.EventDie = face
	k.Event = &CatanCityEvent{Red: red, Yellow: yellow, Face: face, Production: production, Epidemic: epidemic, Tasks: []CatanCityEventTask{}}
	if g.attackTransportKnights() && face < 3 && (production == 2 || production == 12 || production == 0 && (red+yellow == 2 || red+yellow == 12)) {
		number := production
		if number == 0 {
			number = red + yellow
		}
		if _, err := s.attackTransportCityLanding([2]int{max(1, number-6), number - max(1, number-6)}); err != nil {
			return err
		}
	}
	if face >= 3 && g.attackKnights() {
		dice := [2]int{red, yellow}
		if production > 0 {
			dice = [2]int{max(1, production-6), production - max(1, production-6)}
		}
		targets, err := s.catanAttackCityLanding(dice)
		if err != nil {
			return err
		}
		s.catanLog(s.Turn, "蛮族船面：沿海登陆%d块地，不移动蛮族船", len(targets))
	} else if face >= 3 {
		k.BarbarianPosition++
		if production > 0 {
			s.catanLog(s.Turn, "事件牌点数%d；红骰%d、蛮族船：船前进至%d/%d", production, red, k.BarbarianPosition, catanBarbarianDistance)
		} else {
			s.catanLog(s.Turn, "掷出红骰%d、普通骰%d、蛮族船：船前进至%d/%d", red, yellow, k.BarbarianPosition, catanBarbarianDistance)
		}
		if k.BarbarianPosition == catanBarbarianDistance {
			s.catanPrepareBarbarians()
		}
	} else {
		if production > 0 {
			s.catanLog(s.Turn, "事件牌点数%d；红骰%d、%s事件", production, red, catanCityTracks[face])
		} else {
			s.catanLog(s.Turn, "掷出红骰%d、普通骰%d、%s事件", red, yellow, catanCityTracks[face])
		}
		for offset := range len(g.Players) {
			player := (s.Turn + offset) % len(g.Players)
			level := k.Players[player].Improvements[face]
			if !g.Players[player].Eliminated && level > 0 && red <= level+1 {
				k.Event.Tasks = append(k.Event.Tasks, CatanCityEventTask{Kind: "draw", Player: player, Track: face})
			}
		}
	}
	return s.catanContinueCityEvent()
}
func (g *Catan) pillageSites(player int) []int {
	out := []int{}
	for _, v := range g.Vertices {
		if v.Owner == player && g.cityAt(v.ID) && !slices.Contains(g.CitiesKnights.Metropolises[:], v.ID) {
			out = append(out, v.ID)
		}
	}
	return out
}
func (g *Catan) fallenCities(player int) []int {
	out := []int{}
	if k := g.CitiesKnights; k != nil {
		for _, v := range k.FallenCities {
			if g.Vertices[v].Owner == player {
				out = append(out, v)
			}
		}
	}
	return out
}
func (g *Catan) settlementPiecesLeft(player int) int {
	_, settlements, _ := g.pieces(player)
	return 5 - settlements + len(g.fallenCities(player))
}
func (g *Catan) cityPiecesLeft(player int) int {
	_, _, cities := g.pieces(player)
	return 4 - cities - len(g.fallenCities(player))
}
func (g *Catan) canCityUpgrade(player, vertex int) bool {
	if g.Attack != nil && g.Attack.buildBlocked(g, -1, vertex) {
		return false
	}
	if vertex < 0 || vertex >= len(g.Vertices) || g.Vertices[vertex].Owner != player || g.Vertices[vertex].Level != 1 {
		return false
	}
	fallen := g.fallenCities(player)
	if len(fallen) > 0 {
		return slices.Contains(fallen, vertex)
	}
	return g.cityPiecesLeft(player) > 0
}
func (s *State) catanPrepareBarbarians() {
	g := s.Catan
	k := g.CitiesKnights
	k.Event.Attack = true
	cities := 0
	for _, v := range g.Vertices {
		if g.cityAt(v.ID) {
			cities++
		}
	}
	strength := make([]int, len(g.Players))
	for _, n := range k.Knights {
		if n.Owner >= 0 && n.Active && !g.Players[n.Owner].Eliminated {
			strength[n.Owner] += n.Strength
		}
	}
	defense := sum(strength)
	s.Log = append(s.Log, fmt.Sprintf("蛮族抵达：蛮族强度%d，总防御%d；先结算防御，再进行资源生产", cities, defense))
	if defense >= cities {
		highest := -1
		for p, n := range strength {
			if !g.Players[p].Eliminated {
				highest = max(highest, n)
			}
		}
		winners := []int{}
		for offset := range len(g.Players) {
			p := (s.Turn + offset) % len(g.Players)
			if !g.Players[p].Eliminated && strength[p] == highest {
				winners = append(winners, p)
			}
		}
		if len(winners) == 1 {
			k.Players[winners[0]].DefenderPoints++
			s.catanLog(winners[0], "以%d点最高贡献击退蛮族（总防御%d，蛮族%d），获得防御者1分", highest, defense, cities)
		} else {
			s.Log = append(s.Log, "防御成功，最高贡献并列：依次选择进步牌，不获得防御者分")
			for _, p := range winners {
				k.Event.Tasks = append(k.Event.Tasks, CatanCityEventTask{Kind: "defender_reward", Player: p})
			}
		}
	} else {
		s.Log = append(s.Log, "蛮族获胜，拥有可劫掠城市且防御贡献最低的玩家各降级一座城市")
		lowest := int(^uint(0) >> 1)
		for p, n := range strength {
			if !g.Players[p].Eliminated && len(g.pillageSites(p)) > 0 {
				lowest = min(lowest, n)
			}
		}
		for offset := range len(g.Players) {
			p := (s.Turn + offset) % len(g.Players)
			if !g.Players[p].Eliminated && strength[p] == lowest && len(g.pillageSites(p)) > 0 {
				k.Event.Tasks = append(k.Event.Tasks, CatanCityEventTask{Kind: "pillage", Player: p})
			}
		}
	}
}
func (s *State) catanFinishBarbarians() {
	g := s.Catan
	k := g.CitiesKnights
	k.BarbarianPosition = 0
	for i := range k.Knights {
		k.Knights[i].Active = false
	}
	k.Invasions++
	if k.Invasions == 1 && g.Explorer == nil && g.Transport == nil {
		g.Robber = k.RobberStart
		if g.Seafarers != nil && g.wonders() == nil && (g.pirateIslands() == nil || g.pirateFortressesRemain()) {
			g.Seafarers.Pirate = k.PirateStart
			s.Log = append(s.Log, "首次蛮族进攻结束，海盗进入本剧本规定的起始位置")
		}
		if g.pirateIslands() == nil {
			s.Log = append(s.Log, "首次蛮族进攻结束，强盗进入本局规定的起始位置；此后掷出7会移动强盗")
		} else {
			s.Log = append(s.Log, "本剧本不使用强盗；此后掷7弃牌后可选择偷牌对手")
		}
	}
	s.Log = append(s.Log, "蛮族船返回起点，所有骑士转为未激活")
}
func (s *State) catanContinueCityEvent() error {
	g := s.Catan
	k := g.CitiesKnights
	e := k.Event
	if e == nil {
		return errors.New("缺少事件后续状态")
	}
	for len(e.Tasks) > 0 && !s.Finished {
		task := e.Tasks[0]
		if g.Players[task.Player].Eliminated {
			e.Tasks = e.Tasks[1:]
			continue
		}
		if task.Kind == "draw" {
			e.Tasks = e.Tasks[1:]
			s.catanDrawProgress(task.Player, task.Track)
			if k.Pending != nil {
				return nil
			}
		} else {
			if task.Kind == "defender_reward" && len(k.ProgressDecks[0])+len(k.ProgressDecks[1])+len(k.ProgressDecks[2]) == 0 {
				e.Tasks = e.Tasks[1:]
				continue
			}
			k.Pending = &CatanCityPending{Kind: task.Kind, Players: []int{task.Player}}
			s.Phase = "catan_" + task.Kind
			return nil
		}
	}
	if e.Attack {
		s.catanFinishBarbarians()
	}
	k.Event = nil
	s.catanScores()
	s.catanVictory()
	if s.Finished {
		return nil
	}
	total := e.Production
	if total == 0 {
		total = e.Red + e.Yellow
	}
	if g.Explorer != nil {
		return s.catanExplorerCityProduction([2]int{e.Red, e.Yellow})
	}
	if g.pirateIslands() != nil {
		return s.catanCityFleetProduction(total, e.Epidemic)
	}
	return s.catanRollProductionEffect(total, e.Epidemic)
}
func (s *State) catanEventChoice(player int, a Action) error {
	g := s.Catan
	k := g.CitiesKnights
	q := k.Pending
	ending := s.Phase == "catan_progress_end"
	switch q.Kind {
	case "progress_discard":
		if err := s.catanDiscardProgress(player, a); err != nil {
			return err
		}
	case "pillage":
		if a.Type == "catan_pillage" && s.Phase == "catan_pillage" && a.Choice == "gold" {
			if !g.canRiverPillageGold(player) {
				return errors.New("保住城市需要5金币")
			}
			g.riverGold()[player] -= 5
			if g.riversTransport() {
				g.Transport.GoldBank += 5
			} else {
				g.Rivers.Bank += 5
			}
			k.Event.Tasks = k.Event.Tasks[1:]
			s.catanLog(player, "支付金币×5，免除本次城市劫掠")
			s.catanScores()
			break
		}
		if a.Choice != "" || a.Type != "catan_pillage" || s.Phase != "catan_pillage" || !slices.Contains(g.pillageSites(player), a.Vertex) {
			return errors.New("请选择自己没有大都会的城市降级")
		}
		if g.settlementPiecesLeft(player) == 0 {
			k.FallenCities = append(k.FallenCities, a.Vertex)
			s.catanLog(player, "村庄棋子已用尽，城市 #%d 横置为村庄，必须优先修复", a.Vertex+1)
		}
		g.Vertices[a.Vertex].Level = 1
		if slices.Contains(k.Walls, a.Vertex) {
			s.catanLog(player, "城市 #%d 的城墙返回库存", a.Vertex+1)
		}
		k.Walls = slices.DeleteFunc(k.Walls, func(v int) bool { return v == a.Vertex })
		s.catanLog(player, "城市 #%d 被劫掠为村庄", a.Vertex+1)
		s.catanScores()
		k.Event.Tasks = k.Event.Tasks[1:]
	case "defender_reward":
		if a.Type != "catan_defender_reward" || s.Phase != "catan_defender_reward" || a.Color < 0 || a.Color >= 3 || len(k.ProgressDecks[a.Color]) == 0 {
			return errors.New("请选择仍有牌的科学、贸易或政治牌堆")
		}
		k.Event.Tasks = k.Event.Tasks[1:]
		k.Pending = nil
		s.catanLog(player, "防御贡献并列最高，选择%s进步牌作为奖励", catanCityTracks[a.Color])
		s.catanDrawProgress(player, a.Color)
		if k.Pending != nil {
			return nil
		}
	default:
		return errors.New("未知事件选择")
	}
	k.Pending = nil
	if g.riverKnights() && q.Kind == "pillage" {
		s.catanVictory()
	}
	if ending {
		s.Phase = "catan_turn"
		return s.applyCatanStep(player, Action{Type: "catan_end"})
	}
	return s.catanContinueCityEvent()
}
