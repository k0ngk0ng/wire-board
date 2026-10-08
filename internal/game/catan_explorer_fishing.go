package game

import (
	"errors"
	"slices"
)

// NewCatanExplorerFishing creates an Explorer game with separate fish tokens.
// Lakes are optional; original Explorer constructors retain their original maps.
func NewCatanExplorerFishing(players int, scenario string, cities, lakes bool) (*State, error) {
	var s *State
	var err error
	layout := "variable"
	switch {
	case cities:
		s, err = NewCatanExplorerCitiesKnights(players, scenario)
	case scenario == "land-ho":
		if players <= 4 {
			layout = "fixed"
		}
		s, err = NewCatanExplorerLandHo(players)
	case scenario == "spices-for-catan":
		s, err = NewCatanExplorerSpices(players)
	default:
		s, err = NewCatanExplorerMission(players, scenario)
	}
	if err != nil {
		return nil, err
	}
	board, recipe, fishing, err := newCatanExplorerFishingMap(players, scenario, layout, cities, lakes)
	if err != nil {
		return nil, err
	}
	g, x := s.Catan, s.Catan.Explorer
	// The new terrain recipe is chosen before any player placement/action;
	// topology, printed openings and mission component counts are unchanged.
	g.Tiles, x.Board = board.Tiles, recipe
	tokens, err := newCatanFishingTokens(players)
	if err != nil {
		return nil, err
	}
	g.Fishing = &CatanFishing{Explorer: recipe.Fishing, Map: *fishing, Tokens: *tokens, Started: make([]bool, players), LastRollID: -1}
	s.Log = append(s.Log, "探索者渔夫：鱼筹码独立于船运鱼群；村庄与港口产1枚鱼筹码，真实城市产2枚")
	s.Log = append(s.Log, "本站鱼行动适配：5鱼建设限交易建设阶段；2鱼和7鱼限航行阶段。2鱼免除本航行阶段所有船只的海盗通行费；7鱼每船最多再航行一次，原剩余移动点不保留，羊毛仍每船每回合限一次")
	if players > 4 {
		s.Log = append(s.Log, "本站五六人捕鱼地图：起始岛外框8个渔场；选湖时使用中央两湖，以12和2替换原位置数字后归还，移除两块山地")
	}
	if players == 2 {
		s.Log = append(s.Log, "本站双人组合：保留探索者轮序和静态中立建筑；渔夫每人起始5枚鱼筹码，不因起始建筑额外领取，分数落后者鱼行动费用减1")
	}
	if x.Setup == nil {
		vertices := make([]int, players)
		for p := range g.Players {
			vertices[p] = recipe.Opening[p].Settlement
			// Land Ho's printed resource icons correspond to the original
			// terrain. The optional lake replaces ore income, not bank stock.
			for r, n := range g.Players[p].Resources {
				g.Bank[r] += n
			}
			g.Players[p].Resources = slices.Clone(recipe.Opening[p].Resources)
			for r, n := range g.Players[p].Resources {
				g.Bank[r] -= n
			}
		}
		if err = s.catanExplorerStartingFish(vertices); err != nil {
			return nil, err
		}
	}
	return s, s.validateCatanExplorer()
}

func (x catanExplorer) validateFishing(g *Catan) error {
	if x.Board == nil || g == nil {
		return errors.New("探险捕鱼组件缺少地图")
	}
	f := g.Fishing
	if (x.Board.Fishing != "") != (f != nil) {
		return errors.New("探险捕鱼地图与筹码组件不一致")
	}
	if f == nil {
		return nil
	}
	n := len(g.Players)
	if f.Explorer != x.Board.Fishing || f.Explorer != catanExplorerFishingRule(n) || f.WorldSetup != nil || len(f.Started) != n || len(f.Tokens.Hands) != n || f.LastRollID < -1 || f.LastRollID > g.RollID || g.GoldPending != nil {
		return errors.New("探险捕鱼版本、人数或生产记录无效")
	}
	if err := f.Map.validateExplorer(g, x.Board); err != nil {
		return err
	}
	if err := f.Tokens.validate(); err != nil {
		return err
	}
	if (f.Pending != nil) != (len(f.Tokens.Pending) > 0) {
		return errors.New("探险捕鱼回应和领取队列不一致")
	}
	for p, seat := range g.Players {
		if seat.Eliminated && (len(f.Tokens.Hands[p]) != 0 || f.Tokens.BootOwner == p) {
			return errors.New("离场玩家不能保留鱼筹码或旧靴子")
		}
	}
	for _, claim := range f.Tokens.Pending {
		if g.Players[claim.Player].Eliminated {
			return errors.New("鱼筹码不能等待离场玩家回应")
		}
	}
	if q := f.Pending; q != nil {
		if x.Economy == nil || x.Economy.Turn == nil || x.Setup != nil || q.Gold != nil || len(q.Received) != n || f.LastRollID != g.RollID || g.RollID < 1 {
			return errors.New("探险捕鱼回应缺少已完成生产")
		}
		t := x.Economy.Turn
		if t.NoProduction || t.productionNumber() == 7 || q.Resume != "explorer_ready" && q.Resume != "explorer_aqueduct" || q.Resume != "explorer_"+t.Phase || (t.Phase == "aqueduct") != (g.CitiesKnights != nil) {
			return errors.New("探险捕鱼回应的恢复阶段无效")
		}
		if g.CardEvent != nil || g.Trade != nil || g.CitiesKnights != nil && (g.CitiesKnights.Pending != nil || g.CitiesKnights.Event != nil) || x.Pirate != nil && x.Pirate.Pending != nil {
			return errors.New("探险捕鱼不能与其他回应重叠")
		}
		for _, count := range q.Received {
			if count < 0 || count > len(g.Bank)*catanExplorerStock(n).resources {
				return errors.New("探险捕鱼普通资源生产数量无效")
			}
		}
	}
	return nil
}

func (s *State) validateExplorerFishingState() error {
	g := s.Catan
	if g == nil || g.Explorer == nil {
		return nil
	}
	x := g.Explorer
	if err := x.validateFishing(g); err != nil {
		return err
	}
	f := g.Fishing
	if f == nil {
		return nil
	}
	for _, started := range f.Started {
		if started != (x.Setup == nil) {
			return errors.New("探险起始鱼筹码领取记录与开局不一致")
		}
	}
	if x.Setup != nil {
		if f.LastRollID != -1 || f.Pending != nil || f.Tokens.BootOwner != -1 || len(f.Tokens.Discard) != 0 {
			return errors.New("开局放置完成前不能领取或支付鱼筹码")
		}
		for _, hand := range f.Tokens.Hands {
			if len(hand) != 0 {
				return errors.New("开局未完成却已领取鱼筹码")
			}
		}
		return nil
	}
	if x.Economy == nil || x.Economy.Turn == nil || f.LastRollID < 0 || f.LastRollID != g.RollID && !(x.Economy.Turn.Phase == "city" && f.LastRollID == g.RollID-1) {
		return errors.New("探险鱼筹码生产序号与当前回合不一致")
	}
	if !s.Finished && (f.Pending != nil) != (s.Phase == "catan_fish_replace") {
		return errors.New("探险捕鱼主阶段与回应队列不一致")
	}
	return nil
}

func (s *State) catanExplorerStartingFish(vertices []int) error {
	g, f := s.Catan, s.Catan.Fishing
	if f == nil {
		return nil
	}
	if len(vertices) < len(g.Players) || f.LastRollID != -1 || slices.Contains(f.Started, true) {
		return errors.New("探险起始捕鱼重复或建筑记录缺失")
	}
	before := f.Tokens.copy()
	if len(g.Players) == 2 {
		// T&B 2025 p10: 2×1, 2×2 and 1×3, no token for setup placement.
		for p := range g.Players {
			for _, value := range []int{1, 1, 2, 2, 3} {
				index := slices.IndexFunc(f.Tokens.DrawPile, func(id int) bool { return catanFishValue(id) == value })
				if index < 0 {
					return errors.New("双人起始鱼筹码库存不足")
				}
				f.Tokens.Hands[p] = append(f.Tokens.Hands[p], f.Tokens.DrawPile[index])
				f.Tokens.DrawPile = slices.Delete(f.Tokens.DrawPile, index, index+1)
			}
		}
		shuffle(f.Tokens.DrawPile)
	} else {
		due := make([]int, len(g.Players))
		for p := range g.Players {
			for _, ground := range f.Map.Grounds {
				if slices.Contains(ground.Vertices[:], vertices[p]) {
					due[p] = 1
				}
			}
			for _, lake := range f.Map.Lakes {
				if slices.Contains(g.Tiles[lake.Tile].Vertices, vertices[p]) {
					due[p] = 1
				}
			}
		}
		if err := f.Tokens.beginDraw(s.Turn, due); err != nil {
			return err
		}
	}
	for p := range f.Started {
		f.Started[p] = true
	}
	f.LastRollID = 0
	s.catanLogFishDraws(before)
	return nil
}

// Called after actual resource/gold payment, before Aqueduct and action.
// Both dice and production cards enter here once; paired secondary turns do not.
func (s *State) catanExplorerFishingProduction(resources [][]int) error {
	g, x := s.Catan, s.Catan.Explorer
	received := make([]int, len(g.Players))
	for p, hand := range resources {
		received[p] = sum(hand)
	}
	if f := g.Fishing; f != nil {
		if f.LastRollID >= g.RollID {
			return errors.New("探险鱼筹码不能重复生产")
		}
		due, err := f.Map.production(g, x.Economy.Turn.productionNumber())
		if err != nil {
			return err
		}
		before := f.Tokens.copy()
		if err = f.Tokens.beginDraw(s.Turn, due); err != nil {
			return err
		}
		f.LastRollID = g.RollID
		s.catanLogFishDraws(before)
		if len(f.Tokens.Pending) > 0 {
			f.Pending = &CatanFishingPending{Resume: "explorer_" + x.Economy.Turn.Phase, Received: received}
			s.catanExplorerSyncPhase()
			return nil
		}
	}
	return s.catanExplorerFinishFishing(received)
}

func (s *State) catanExplorerFinishFishing(received []int) error {
	if s.catanExplorerProductionHelper(received) {
		return nil
	}
	return s.catanExplorerFinishProductionBonuses(received)
}
func (s *State) catanExplorerFinishProductionBonuses(received []int) error {
	g, x := s.Catan, s.Catan.Explorer
	if x.Economy.Turn.Phase == "aqueduct" {
		s.catanStartAqueduct(received)
		if g.CitiesKnights.Pending != nil {
			return nil
		}
		return s.catanExplorerCityFinishProduction()
	}
	return s.catanExplorerAfterProduction()
}

func (s *State) catanExplorerReplaceFish(player int, a Action) error {
	g, f := s.Catan, s.Catan.Fishing
	if f == nil || f.Pending == nil || s.Finished || s.Phase != "catan_fish_replace" || s.CatanPendingActor() != player || a.Prompt < 1 || uint64(a.Prompt) != g.TurnSerial {
		return errors.New("请等待当前玩家选择鱼筹码，或操作序号已过期")
	}
	var id *int
	switch a.Type {
	case "catan_fish_replace":
		id = &a.Card
	case "catan_fish_keep":
	default:
		return errors.New("请选择更换一枚鱼筹码或保留现有筹码")
	}
	before := f.Tokens.copy()
	if err := f.Tokens.replace(player, id); err != nil {
		return err
	}
	if id == nil {
		s.catanLog(player, "保留现有鱼筹码，放弃本次其余领取")
	} else {
		s.catanLog(player, "弃置一枚 %d 鱼筹码并盲抽替换，停止本次领取", catanFishValue(*id))
	}
	s.catanLogFishDraws(before)
	if len(f.Tokens.Pending) == 0 {
		received := f.Pending.Received
		f.Pending = nil
		return s.catanExplorerFinishFishing(received)
	}
	return nil
}

func catanExplorerFishVictoryTarget(g *Catan, b *catanExplorerBoard, player int) int {
	target := b.Target
	if g.Fishing != nil && g.Fishing.Tokens.BootOwner == player {
		target++
	}
	return target
}
