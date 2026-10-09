package game

import (
	"errors"
	"fmt"
	"slices"
)

type catanAttackLandingRoll struct {
	Dice     [2]int `json:"dice"`
	Tiles    []int  `json:"tiles"`
	Shortage bool   `json:"shortage,omitempty"`
}
type catanAttackLandingRecord struct {
	ID     int                      `json:"id"`
	Player int                      `json:"player"`
	Rolls  []catanAttackLandingRoll `json:"rolls"`
}

// NewCatanAttack selects the verified printed map for the actual player count.
// Two-player rules and other combinations retain separate entry gates.
func NewCatanAttack(n int) (*State, error) {
	if n < 3 {
		return nil, errors.New("双人蛮族进攻请使用双人规则入口")
	}
	return newCatanAttackState(n, CatanOptions{FiveSix: n > 4})
}

// Internal variants may reuse the constructor with already validated options.
func newCatanAttackState(n int, options CatanOptions) (*State, error) {
	if options.Helpers || options.AllHelpers || n == 2 && options != (CatanOptions{}) {
		return nil, errors.New("蛮族进攻助手组合尚未核对")
	}
	var s *State
	var err error
	if n == 2 {
		s, err = newCatanTwoCore()
	} else {
		s, err = NewCatan(n, options)
	}
	if err != nil {
		return nil, err
	}
	board, m, err := newCatanAttackBoard(n)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	g.Tiles, g.Vertices, g.Edges, g.Ports = board.Tiles, board.Vertices, board.Edges, board.Ports
	g.HexSize, g.Robber = board.HexSize, -1
	g.DevDeck, g.DevDiscard = []int{}, []int{}
	g.Attack, err = newCatanAttackPieces(g, m)
	if err != nil {
		return nil, err
	}
	if n == 2 {
		g.Attack.TwoRules = CatanTwoAttackRules
		if err = g.prepareTwoNeutrals(); err != nil {
			return nil, err
		}
	}
	s.Log = []string{"蛮族进攻：起始先建村庄，再逆序建城市；城市只领每相邻地块1张起始资源", "蛮族进攻：不使用强盗或最大骑士军队；自己回合达到12分获胜"}
	if n == 2 {
		s.Log = append(s.Log, "双人蛮族：两次生产，共享中立骑士；真人与中立村庄分别触发登陆；筹码可移动蛮族")
	}
	s.catanScores()
	return s, s.validateCatanAttack()
}

func (s *State) validateCatanAttack() error {
	g := s.Catan
	if g == nil || g.Attack == nil {
		return nil
	}
	a := g.Attack
	if err := s.validateWonderLanding(); err != nil {
		return err
	}
	if err := s.validateAttackTribeRoute(); err != nil {
		return err
	}
	if (g.AttackTransport != nil || a.Map != nil && a.Map.Transport != "") && !g.attackTransport() {
		return errors.New("蛮族运输缺少匹配组件")
	}
	if a.Map != nil && a.Map.Caravans != "" && !g.caravansAttack() {
		return errors.New("商队蛮族缺少商队组件")
	}
	if !g.caravansAttack() && slices.Contains([]string{"catan_caravan_bid", "catan_caravan_vote", "catan_caravan_place"}, s.Phase) {
		return errors.New("普通蛮族不能进入商队回应")
	}
	if g.caravansAttack() {
		if err := s.validateCaravans(); err != nil {
			return err
		}
	}
	if a.Map != nil && a.Map.Rivers != "" && !g.riversAttack() {
		return errors.New("河流蛮族缺少配套河流组件")
	}
	if g.attackKnights() {
		return s.validateAttackCityState()
	}
	if !g.twoAttack() && (a.NeutralPrisoners != 0 || a.TwoLanding || a.Pending != nil && a.Pending.Neutral || s.Phase == "catan_two_build" || s.Phase == "catan_two_trade") {
		return errors.New("多人蛮族混入双人组件")
	}
	n := len(g.Players)
	if n < 2 || n > 6 || (n == 2 || g.Two != nil || a.TwoRules != "") && !g.twoAttack() || g.Caravans != nil && !g.caravansAttack() || g.Rivers != nil && !g.riversAttack() || g.Fishing != nil && !g.fishingAttack() || g.Seafarers != nil && !g.attackSea() || g.CitiesKnights != nil || g.BaseSetup != nil || (g.Harbors != nil || g.FriendlyRobber != nil) && !g.tradersVariants() || (g.Options.Helpers || g.Options.AllHelpers) && !g.tradersHelpers() || (n > 4) != g.Options.FiveSix || (n > 4) != (g.Paired != nil) {
		return errors.New("蛮族进攻人数或尚未接入的组合无效")
	}
	if g.fishingAttack() {
		if err := g.validateFishing(); err != nil {
			return err
		}
		if (g.Fishing.Pending != nil) != (s.Phase == "catan_fish_replace") {
			return errors.New("蛮族捕鱼回应阶段冲突")
		}
	}
	if g.twoAttack() {
		if err := s.validateTwoAttack(); err != nil {
			return err
		}
	}
	if tr := g.tribe(); g.attackSea() && tr != nil {
		if (tr.Pending != nil) != (s.Phase == "catan_port") || tr.Pending != nil && (tr.Pending.Player != s.Turn || s.Finished) {
			return errors.New("蛮族部落港口回应阶段无效")
		}
	}
	if err := a.validate(g); err != nil {
		return err
	}
	if g.Robber != -1 || g.ArmyOwner != -1 || len(g.DevDeck) != 0 || len(g.DevDiscard) != 0 || a.Bought < 0 || a.Bought > 2 || a.Sequence < 0 || (a.Landing == nil) != (a.Sequence == 0) {
		return errors.New("蛮族进攻基础牌堆、强盗或登陆状态无效")
	}
	if g.setup() && (a.Bought != 0 || a.Sequence != 0 || len(a.Knights) > 0) {
		return errors.New("起始建设不能触发登陆或骑士行动")
	}
	if !(g.attackSea() && (s.Phase == "catan_attack_landing" || s.Phase == "catan_gold" || g.tribe() != nil && s.Phase == "catan_port" || g.attackPirates() && s.Phase == "catan_fleet_reward")) && !(g.attackTransport() && s.Phase == "catan_transport_move") && !slices.Contains([]string{"catan_setup_settlement", "catan_setup_city", "catan_setup_road", "catan_roll", "catan_turn", "catan_discard", "catan_steal", "catan_card_event", "catan_fish_replace", "catan_helper", "catan_attack_card", "catan_attack_end", "catan_two_build", "catan_two_trade", "catan_caravan_bid", "catan_caravan_vote", "catan_caravan_place", "finished"}, s.Phase) {
		return errors.New("蛮族进攻阶段无效")
	}
	if a.CardSequence < 0 || (a.Pending != nil) != (s.Phase == "catan_attack_card") || g.setup() && a.CardSequence != 0 {
		return errors.New("蛮族进攻发展卡响应状态无效")
	}
	if q := a.Pending; q != nil {
		if q.WarshipUsed && (!g.attackPirates() || q.Card != "knighthood") {
			return errors.New("战舰升级只能属于海盗群岛授勋牌")
		}
		if q.Resume != "" && (q.Resume != "catan_roll" || !g.fishingAttack()) {
			return errors.New("蛮族发展卡返回阶段无效")
		}
		if q.ID != a.CardSequence || q.ID < 1 || q.Player != s.Turn || q.Player < 0 || q.Player >= n || g.Players[q.Player].Eliminated || g.Trade != nil || s.Finished {
			return errors.New("蛮族进攻发展卡回应者或序号无效")
		}
		if q.Card == "capture" && len(g.attackCaptureTargets()) == 0 || (q.Card == "knighthood" || q.Card == "swift_knight") && len(a.recruitEdges(g, g.attackRecruitOwner(q.Player), q.Card)) == 0 && g.attackCardWarship() < 0 || q.Card == "treason" && !g.attackGoldReady(2) {
			return errors.New("蛮族进攻发展卡没有可完成的效果")
		}
	}
	supply := 19
	if n > 4 {
		supply = 24
	}
	if !catanBundle(g.Bank) {
		return errors.New("资源银行无效")
	}
	total := slices.Clone(g.Bank)
	for _, p := range g.Players {
		if !catanBundle(p.Resources) || !catanBundle(p.Dev) || !catanBundle(p.NewDev) || sum(p.Dev) != 0 || sum(p.NewDev) != 0 || p.Knights != 0 {
			return errors.New("蛮族进攻不能持有基础发展卡或军队计数")
		}
		for c, count := range p.Resources {
			total[c] += count
		}
	}
	g.addCaravanEscrow(total)
	for _, count := range total {
		if count != supply {
			return errors.New("资源库存不守恒")
		}
	}
	if err := s.validateCatanAttackEvent(); err != nil {
		return err
	}
	if q := a.Landing; q != nil {
		if q.ID != a.Sequence || q.Player < 0 || q.Player >= n || len(q.Rolls) > 3 {
			return errors.New("登陆记录无效")
		}
		seen := map[int]bool{}
		for index, roll := range q.Rolls {
			total := roll.Dice[0] + roll.Dice[1]
			if roll.Dice[0] < 1 || roll.Dice[0] > 6 || roll.Dice[1] < 1 || roll.Dice[1] > 6 || total == 7 || seen[total] || (len(roll.Tiles) > 2 && !g.attackTransport() || len(roll.Tiles) > len(a.Map.Coast)) {
				return errors.New("登陆点数记录无效")
			}
			seen[total] = true
			if roll.Shortage && !g.attackTransport() && (!g.attackLandingCanShortage(total) || len(roll.Tiles) != 1 || index != len(q.Rolls)-1) {
				return errors.New("最后一枚蛮族的随机登陆记录无效")
			}
			for i, id := range roll.Tiles {
				if !slices.Contains(a.Map.Coast, id) || !a.Map.landingNumber(g, id, total) || slices.Contains(roll.Tiles[:i], id) {
					return errors.New("登陆地块记录无效")
				}
			}
		}
	}
	if err := s.validateCatanAttackPlan(); err != nil {
		return err
	}
	return s.validateCatanAttackEnd()
}

// Resolve every required landing immediately. Dice only select coastal
// numbers; they do not advance production RollID or overwrite its dice.
// Compute off to the side so a supply/rule error cannot partly place pieces.
func (s *State) catanAttackLanding(roll func() [2]int, choose func(int) int) error {
	g := s.Catan
	a := g.Attack
	if g.attackBalancedLanding() {
		return s.startWonderLandings(1, roll)
	}
	if g.attackTransport() && !g.attackKnights() {
		return s.attackTransportLanding(roll, choose)
	}
	if g.attackKnights() {
		return s.catanAttackCityBuildLanding(roll)
	}
	if a == nil || g.setup() || s.Phase != "catan_turn" || s.Finished {
		return errors.New("当前不能进行蛮族登陆")
	}
	counts := slices.Clone(a.Barbarians)
	remaining := a.supply()
	event := &catanAttackLandingRecord{ID: a.Sequence + 1, Player: s.Turn, Rolls: []catanAttackLandingRoll{}}
	used := map[int]bool{}
	for len(event.Rolls) < 3 && remaining > 0 {
		dice := roll()
		total := dice[0] + dice[1]
		if dice[0] < 1 || dice[0] > 6 || dice[1] < 1 || dice[1] > 6 {
			return errors.New("登陆骰子无效")
		}
		if total == 7 || used[total] {
			continue
		}
		used[total] = true
		targets := []int{}
		for _, id := range a.Map.Coast {
			if a.Map.landingNumber(g, id, total) && counts[id] < 3 {
				targets = append(targets, id)
			}
		}
		shortage := len(targets) > remaining
		if shortage {
			// Site supplemental rule: uniformly choose one eligible coastal hex
			// when the final piece cannot cover both matching 5–6 player hexes.
			if remaining != 1 || len(targets) != 2 || choose == nil {
				return errors.New("最后一枚蛮族的随机分配无效")
			}
			pick := choose(len(targets))
			if pick < 0 || pick >= len(targets) {
				return errors.New("蛮族登陆随机目标无效")
			}
			targets = []int{targets[pick]}
		}
		for _, id := range targets {
			counts[id]++
			remaining--
		}
		event.Rolls = append(event.Rolls, catanAttackLandingRoll{Dice: dice, Tiles: targets, Shortage: shortage})
	}
	a.Barbarians = counts
	a.Sequence = event.ID
	a.Landing = event
	for _, r := range event.Rolls {
		if r.Shortage {
			s.catanLog(s.Turn, "本站补充规则：仅剩1个蛮族，在同点数的两个可登陆地块中随机分配至地块 #%d", r.Tiles[0]+1)
		}
		if len(r.Tiles) == 0 {
			s.catanLog(s.Turn, "蛮族登陆掷出 %d+%d：对应沿海地块已被征服", r.Dice[0], r.Dice[1])
			continue
		}
		for _, id := range r.Tiles {
			s.catanLog(s.Turn, "蛮族登陆掷出 %d+%d：地块 #%d 增加1个蛮族（%d/3）", r.Dice[0], r.Dice[1], id+1, counts[id])
			if counts[id] == 3 {
				s.Log = append(s.Log, fmt.Sprintf("地块 #%d 被征服：停止生产并禁止在相邻交点或边新建", id+1))
			}
		}
	}
	if len(event.Rolls) == 0 {
		s.Log = append(s.Log, "蛮族供应已空，本次建设没有新增登陆")
	}
	s.catanScores()
	s.catanVictory()
	return nil
}

func (s *State) catanAttackView(v map[string]any, player int) {
	g := s.Catan
	a := g.Attack
	if a == nil {
		return
	}
	public := v["attack"].(map[string]any)
	if g.attackKnights() {
		public["city"] = s.catanAttackCityPlanView(player)
	}
	if end, ok := public["end"].(map[string]any); ok {
		if moves, ok := end["moves"].([]any); ok {
			for _, raw := range moves {
				if m, ok := raw.(map[string]any); ok {
					delete(m, "tokens")
				}
			}
		}
	}
	delete(public, "deck")
	delete(public, "tribeRoute")
	public["devRemaining"] = len(a.Deck)
	public["canBuyCard"] = len(a.Deck)+len(a.Discard) > 0 && g.attackGoldReady(2)
	public["supply"] = a.supply()
	if g.attackKnights() {
		public["supply"] = a.supply() + a.City.Issued
		public["landingSupplyRule"] = "ledger"
	}
	if !g.attackKnights() {
		public["landingSupplyRule"] = "random-last"
	}
	if g.attackWonders() {
		public["landingSupplyRule"] = "desert-reserve"
		reserve := 0
		for _, id := range a.Map.Reserves {
			reserve += a.Barbarians[id]
		}
		public["reserveRemaining"] = reserve
	}
	public["goldRule"] = "ledger"
	if g.attackTransport() {
		public["gold"] = slices.Clone(g.Transport.Gold)
		public["goldBank"] = g.Transport.GoldBank
		public["goldIssued"] = g.Transport.GoldIssued
		public["bought"] = g.Transport.Bought
	}
	public["treasonRule"] = "as-much-as-possible"
	conquered, buildings := []int{}, []int{}
	for _, t := range g.Tiles {
		if a.conquered(t.ID) {
			conquered = append(conquered, t.ID)
		}
	}
	for _, point := range g.Vertices {
		if point.Level > 0 && a.conqueredBuilding(g, point.ID) && !g.attackCityMetropolis(point.ID) {
			buildings = append(buildings, point.ID)
		}
	}
	public["conquered"] = conquered
	public["conqueredBuildings"] = buildings
	left := make([]int, len(g.Players))
	for p := range left {
		left[p] = 6
	}
	for _, k := range a.Knights {
		if k.Player >= 0 {
			left[k.Player]--
		}
	}
	if g.attackKnights() {
		for _, k := range a.City.Knights {
			if k.Owner < 0 {
				continue
			}
			left[k.Owner]--
		}
	}
	public["knightsLeft"] = left
	if q := a.WonderLanding; q != nil {
		sources, targets := g.wonderLandingChoices()
		public["landingSources"], public["landingTargets"] = sources, targets
		public["wonderLanding"] = map[string]any{"id": q.ID, "player": q.Player, "cursor": q.Cursor, "remaining": len(q.Dice) - q.Cursor}
		public["landingNumber"] = q.Dice[q.Cursor][0] + q.Dice[q.Cursor][1]
	}
	public["canAct"] = !s.Finished && (a.WonderLanding != nil && a.WonderLanding.Player == player || a.Pending != nil && a.Pending.Player == player || a.EndPlan != nil && a.EndPlan.Player == player)
	s.catanAttackPlanView(public, player)
	if a.Pending != nil && a.Pending.Player == player && !s.Finished {
		switch a.Pending.Card {
		case "capture":
			public["targets"] = g.attackCaptureTargets()
		case "knighthood", "swift_knight":
			public["edges"] = a.recruitEdges(g, g.attackRecruitOwner(player), a.Pending.Card)
			if id := g.attackCardWarship(); id >= 0 {
				public["warshipEdge"] = id
				public["canSkipWarship"] = len(a.recruitEdges(g, player, a.Pending.Card)) == 0
			}
		case "treason":
			plans := g.attackTreasonPlans()
			sources, destinations := []int{}, []int{}
			for _, plan := range plans {
				for _, id := range plan.Sources {
					if id >= 0 && !slices.Contains(sources, id) {
						sources = append(sources, id)
					}
				}
				for _, id := range plan.Destinations {
					if !slices.Contains(destinations, id) {
						destinations = append(destinations, id)
					}
				}
			}
			public["sources"], public["destinations"] = sources, destinations
			public["treasonPlans"] = plans
			public["treasonCount"] = len(plans[0].Sources)
			public["fromBoard"] = min(len(plans[0].Sources), len(g.attackCaptureTargets()))
		}
	}
}

func (g *Catan) attackLandingCanShortage(total int) bool {
	if g.attackSea() && g.tribe() != nil {
		return len(g.Players) > 4 || total == 2 || total == 12
	}
	return len(g.Players) > 4 && (total == 5 || total == 9 || g.attackSea() && g.Seafarers.Scenario == "desert" && total == 3)
}
