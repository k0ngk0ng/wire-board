package game

import (
	"errors"
	"slices"
)

// Complete-session guards shared by the public 3–6 player recipe and the
// separately marked public two-player recipe.
func (s *State) validateAttackCityState() error {
	g := s.Catan
	if g == nil || !g.attackKnights() {
		return errors.New("缺少蛮族城市骑士组合")
	}
	a, c, k := g.Attack, g.Attack.City, g.CitiesKnights
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
	n := len(g.Players)
	options, _ := NormalizeCatanOptions(CatanOptions{FiveSix: n > 4, Helpers: g.tradersHelpers(), AllHelpers: g.Options.AllHelpers})
	if n < 2 || n > 6 || s.Turn < 0 || s.Turn >= n || (n == 2 || g.Two != nil || a.TwoRules != "" || a.TwoLanding) && !g.twoAttackKnights() || a.NeutralPrisoners != 0 || g.Caravans != nil && !g.caravansAttack() || g.Rivers != nil && !g.riversAttack() || g.Fishing != nil && !g.fishingAttack() || g.Seafarers != nil || g.Transport != nil && !g.attackTransportKnights() || g.BaseSetup != nil || (g.Harbors != nil || g.FriendlyRobber != nil) && !g.tradersVariants() || g.Options != options || (g.Paired != nil) != (n > 4) || g.Robber != -1 || g.ArmyOwner != -1 || a.Rules != catanAttackRules || a.EndPlan != nil || a.End != nil || a.EndSequence != 0 || a.Pending != nil || a.CardSequence != 0 || a.Landing != nil || a.Sequence != 0 || a.Bought < 0 || a.Bought > 2 || len(g.DevDeck) != 0 || len(g.DevDiscard) != 0 {
		return errors.New("蛮族城市骑士人数、组件或组合配置无效")
	}
	if g.riversAttack() {
		if err := g.validateRivers(); err != nil {
			return err
		}
	}
	if g.fishingAttack() {
		if err := g.validateFishing(); err != nil {
			return err
		}
		if (g.Fishing.Pending != nil) != (s.Phase == "catan_fish_replace") {
			return errors.New("道路骑士捕鱼回应阶段冲突")
		}
	}
	if err := s.validateCatanTwo(); err != nil {
		return err
	}
	if err := c.validate(g); err != nil {
		return err
	}
	// Validate original geometry with the recorded swaps reversed. Ordinary
	// Attack restoration retains its exact printed-number check.
	board := clone(*g)
	numbers := g.attackPrintedNumbers()
	for id := range board.Tiles {
		board.Tiles[id].Number = numbers[id]
	}
	if g.attackTransport() {
		board.Attack.City.NumberSwaps = nil
	}
	if err := a.Map.validate(&board); err != nil {
		return err
	}
	if k.Rules != catanCitiesKnightsRules(n) || k.Layout != "variable" || k.RobberStart != -1 || k.Chase != "" || k.ActionSerial == 0 || k.ActionSerial < g.TurnSerial || k.EventDie < -1 || k.EventDie > 5 || len(k.FallenCities) != 0 {
		return errors.New("蛮族城市骑士城市规则或事件记录无效")
	}
	if g.attackTransport() {
		if err := g.validateAttackTransportPieces(); err != nil {
			return err
		}
	} else {
		if a.GoldIssued < 0 || a.GoldIssued > catanGoldLedgerLimit || len(a.Gold) != n || a.GoldBank < 0 || a.GoldBank > a.Map.Gold+a.GoldIssued {
			return errors.New("组合金币银行无效")
		}
		gold := a.GoldBank
		for _, amount := range a.Gold {
			if amount < 0 || amount > a.Map.Gold+a.GoldIssued {
				return errors.New("组合金币持有量无效")
			}
			gold += amount
		}
		if gold != a.Map.Gold+a.GoldIssued {
			return errors.New("组合金币库存不守恒")
		}
	}
	if len(g.Bank) != 8 || !g.cardBundle(g.Bank) {
		return errors.New("组合资源银行无效")
	}
	totals := slices.Clone(g.Bank)
	for _, p := range g.Players {
		if len(p.Resources) != 8 || !g.cardBundle(p.Resources) || !catanBundle(p.Dev) || !catanBundle(p.NewDev) || sum(p.Dev) != 0 || sum(p.NewDev) != 0 || p.Knights != 0 {
			return errors.New("组合资源或普通发展牌库存无效")
		}
		for color, amount := range p.Resources {
			totals[color] += amount
		}
	}
	g.addCaravanEscrow(totals)
	for color, amount := range totals {
		want := 19
		if n > 4 {
			want = 24
		}
		if color >= 5 {
			want = 12
			if n > 4 {
				want = 18
			}
		}
		if amount != want {
			return errors.New("组合资源商品库存不守恒")
		}
	}
	phases := []string{"catan_setup_settlement", "catan_setup_city", "catan_setup_road", "catan_roll", "catan_turn", "catan_discard", "catan_steal", "catan_roads", "catan_card_event", "catan_fish_replace", "catan_helper", catanAttackCityMovePhase, catanAttackCityRetreatPhase, "catan_attack_city_treason_remove", "catan_attack_city_treason_place", "catan_caravan_bid", "catan_caravan_vote", "catan_caravan_place", "finished"}
	phases = append(phases, catanTransportCityPhases...)
	if g.attackTransport() {
		phases = append(phases, "catan_transport_move")
	}
	if g.twoAttackKnights() {
		phases = append(phases, "catan_two_build", "catan_two_trade")
	}
	if !slices.Contains(phases, s.Phase) || s.Finished != (s.Phase == "finished") {
		return errors.New("组合行动阶段无效")
	}
	if s.Phase == "catan_pillage" || s.Phase == "catan_defender_reward" || s.Phase == "catan_knight_retreat" || s.Phase == "catan_treason_remove" || s.Phase == "catan_treason_place" {
		return errors.New("组合不能进入普通骑士响应")
	}
	if err := s.validateAttackCityEnd(); err != nil {
		return err
	}
	if err := s.validateAttackCityPlan(); err != nil {
		return err
	}
	if err := s.validateAttackCityTreason(); err != nil {
		return err
	}
	if err := s.validateCityProgressInventory(); err != nil {
		return err
	}
	if err := s.validateExplorerCityDevelopment(); err != nil {
		return err
	}
	q := k.Pending
	if slices.Contains(catanTransportCityPhases, s.Phase) != (q != nil) && !s.Finished {
		return errors.New("组合城市回应缺失")
	}
	if q != nil {
		ending := s.Phase == "catan_progress_end" && q.Kind == "progress_discard"
		if len(q.Players) == 0 || !slices.Contains(catanTransportCityPhases, "catan_"+q.Kind) || !s.Finished && !ending && s.Phase != "catan_"+q.Kind || g.Trade != nil {
			return errors.New("组合城市回应阶段无效")
		}
		for i, p := range q.Players {
			if p < 0 || p >= n || g.Players[p].Eliminated || slices.Contains(q.Players[:i], p) {
				return errors.New("组合城市回应玩家无效")
			}
		}
		if ending && (len(q.Players) != 1 || q.Players[0] != s.Turn || len(k.Players[s.Turn].Progress) <= 4 || k.Event != nil) {
			return errors.New("组合回合末弃牌无效")
		}
	}
	if e := k.Event; e != nil {
		if e.Attack || e.Face < 0 || e.Face > 5 || e.Face != k.EventDie || e.Red < 1 || e.Red > 6 || e.Yellow < 0 || e.Yellow > 6 || e.Production == 0 && e.Yellow == 0 || e.Production != 0 && (g.EventDeck == nil || e.Yellow != 0 || e.Production < 2 || e.Production > 12) {
			return errors.New("组合事件骰或队列无效")
		}
		for _, task := range e.Tasks {
			if task.Kind != "draw" || task.Player < 0 || task.Player >= n || task.Track < 0 || task.Track > 2 {
				return errors.New("组合进步牌抽取队列无效")
			}
		}
	}
	if err := s.validateExplorerCityEventQueue(); err != nil {
		return err
	}
	if pair := g.Paired; pair != nil {
		if pair.Primary < 0 || pair.Primary >= n || pair.Secondary < 0 || pair.Secondary >= n {
			return errors.New("组合配对座位无效")
		}
		actor := pair.Primary
		if pair.Second {
			actor = pair.Secondary
		}
		if !g.setup() && !s.Finished && (s.Turn != actor || pair.Second && s.Phase == "catan_roll") {
			return errors.New("组合配对行动阶段无效")
		}
	}
	return nil
}
func (s *State) applyAttackCity(player int, a Action) error {
	if err := s.validateAttackCityState(); err != nil {
		return err
	}
	if s.Catan.attackTransport() {
		if err := s.validateCatanTransport(); err != nil {
			return err
		}
	}
	next := clone(*s)
	g := next.Catan
	c := g.Attack.City
	var err error
	switch {
	case c.Plan != nil:
		err = next.catanAttackCityPlanAction(player, a)
	case c.Treason != nil:
		err = next.catanAttackCityTreasonAction(player, a)
	case a.Type == "catan_attack_knight_recruit" || a.Type == "catan_attack_knight_activate" || a.Type == "catan_attack_knight_promote":
		err = next.catanAttackCityKnightAction(player, a)
	case a.Type == "catan_buy_dev" || a.Type == "catan_dev" || a.Type == "catan_knight_recruit" || a.Type == "catan_knight_activate" || a.Type == "catan_knight_promote" || a.Type == "catan_knight_move" || a.Type == "catan_knight_chase":
		err = errors.New("本组合使用道路骑士和进步牌")
	default:
		if g.attackTransport() {
			err = next.applyCatanTransport(player, a)
		} else {
			err = next.applyCatanStep(player, a)
		}
	}
	if err != nil {
		return err
	}
	if err = next.catanTwoAfterAction(s, a); err != nil {
		return err
	}
	next.catanScores()
	if !next.Finished && next.Phase == "catan_turn" {
		next.catanVictory()
	}
	if err = next.validateAttackCityState(); err != nil {
		return err
	}
	if g.attackTransport() {
		if err = next.catanTransportSyncTurn(); err != nil {
			return err
		}
		if err = next.validateCatanTransport(); err != nil {
			return err
		}
	}
	if err = next.validateCatanEventSession(); err != nil {
		return err
	}
	*s = next
	return nil
}
