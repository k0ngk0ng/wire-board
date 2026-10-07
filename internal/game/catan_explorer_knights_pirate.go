package game

import "errors"

// The progress dispatcher already checked ownership, phase and the first
// invasion restriction. The outer action copy includes card consumption.
func (s *State) catanExplorerCityTaxation(player int) error {
	g, x := s.Catan, s.Catan.Explorer
	if _, err := x.Pirate.apply(g, x.Board, x.Fleet, x.Cargo, x.Economy, player, g.TurnSerial, "taxation", 0, false, nil); err != nil {
		return err
	}
	s.catanExplorerSyncPhase()
	return nil
}

func (s *State) catanExplorerCityPirateAction(player int, a Action, randN func(int) int) error {
	if err := s.validateExplorerCityProduction(); err != nil {
		return err
	}
	g, x := s.Catan, s.Catan.Explorer
	q := x.Pirate.Pending
	if s.Finished || q == nil || player != s.Turn || a.Prompt < 1 || uint64(a.Prompt) != g.TurnSerial || a.Skill != "" || s.Phase != "catan_explorer_pirate_"+q.Stage || a.Type != s.Phase || a.Choice != "" && (q.Stage != "steal" || a.Choice != "skip") {
		return errors.New("不是当前组合海盗回应玩家、阶段或序号")
	}
	next := clone(*s)
	ng, nx := next.Catan, next.Catan.Explorer
	result, err := nx.Pirate.apply(ng, nx.Board, nx.Fleet, nx.Cargo, nx.Economy, player, ng.TurnSerial, q.Stage, a.Target, a.Choice == "skip", randN)
	if err != nil {
		return err
	}
	if q.Stage == "place" {
		next.catanLog(player, "将海盗移至海格 #%d", a.Target+1)
	} else if result.Target >= 0 {
		what := "1张资源或商品"
		if result.Gold {
			what = "1金币"
		}
		next.catanLog(player, "从玩家%d偷取%s", result.Target+1, what)
	} else {
		next.catanLog(player, "放弃向空手对手收取金币")
	}
	next.catanExplorerSyncPhase()
	if err := next.validateExplorerCityProduction(); err != nil {
		return err
	}
	*s = next
	return nil
}

// All affected players can discard independently. The last successful discard
// begins the normal seven-triggered pirate placement in the same transaction.
func (s *State) catanExplorerCityDiscard(player int, a Action) error {
	if err := s.validateExplorerCityProduction(); err != nil {
		return err
	}
	if s.Finished || s.Phase != "catan_discard" || a.Type != "catan_discard" || a.Prompt < 1 || uint64(a.Prompt) != s.Catan.TurnSerial || a.Skill != "" || a.Choice != "" {
		return errors.New("不是当前组合七点弃牌回应")
	}
	next := clone(*s)
	g, x := next.Catan, next.Catan.Explorer
	if err := x.Economy.discard(g, x.Fleet, x.Cargo, player, g.TurnSerial, a.Tokens); err != nil {
		return err
	}
	next.catanLog(player, "七点弃置%d张资源或商品", sum(a.Tokens))
	if err := next.catanExplorerAfterProduction(); err != nil {
		return err
	}
	if err := next.validateExplorerCityProduction(); err != nil {
		return err
	}
	*s = next
	return nil
}
