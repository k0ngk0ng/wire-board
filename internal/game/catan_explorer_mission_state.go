package game

import "errors"

// Explicit-inventory constructor for legacy saves and isolated rule fixtures.
// Public missions select the labelled site recipe in NewCatanExplorerMission.
func newCatanExplorerLairsState(players int, layout string, numbers []int) (*State, error) {
	return newCatanExplorerMissionState(players, "pirate-lairs", layout, numbers)
}
func newCatanExplorerMissionState(players int, scenario, layout string, numbers []int) (*State, error) {
	if players < 2 || players > 6 {
		return nil, errors.New("探险任务需要二至六人")
	}
	g, b, f, c, e, setup, err := newCatanExplorerMissionSetup(players, scenario, layout, catanRandom(players))
	if err != nil {
		return nil, err
	}
	g.Explorer = &catanExplorer{Board: b, Fleet: f, Cargo: c, Economy: e, Setup: setup, Pirate: newCatanExplorerPirate()}
	label := "海盗巢穴"
	if catanExplorerMissionScenario(scenario) {
		lairs, err := newCatanExplorerLairs(players, numbers)
		if err != nil {
			return nil, err
		}
		shuffle(lairs.Deck)
		g.Explorer.Lairs = lairs
	}
	if catanExplorerFishScenario(scenario) {
		label = "鱼群任务"
		g.Explorer.Fish = &catanExplorerFish{Deliveries: []catanExplorerFishDelivery{}}
	}
	if catanExplorerSpiceScenario(scenario) {
		label = "香料与鱼群任务"
		g.Explorer.Spice = &catanExplorerSpice{Deliveries: []catanExplorerSpiceDelivery{}}
	}
	if scenario == "explorers-and-pirates" {
		label = "探险家与海盗三任务"
	}
	s := &State{Kind: "catan", Catan: g, Turn: setup.Start, Round: 1, Phase: "catan_explorer_setup", Log: []string{label + "：随机先手，顺序港口、逆序村庄，再放道路与移民船"}}
	return s, s.validateCatanExplorer()
}
func (s *State) applyCatanExplorerSetup(player int, a Action) error {
	g := s.Catan
	x := g.Explorer
	if a.Type != "catan_explorer_setup" {
		return errors.New("请先完成探险开局")
	}
	if err := x.Setup.place(g, x.Board, x.Fleet, x.Cargo, x.Economy, player, a.Prompt, a.Choice, a.Target); err != nil {
		return err
	}
	s.catanLog(player, "放置起始%s #%d", map[string]string{"city": "城市", "harbor": "港口", "settlement": "村庄", "road": "道路", "ship": "移民船"}[a.Choice], a.Target+1)
	if step := x.Setup.current(len(g.Players)); step != nil {
		s.Turn = step.Player
	} else {
		s.Turn = x.Setup.Start
		if err := s.catanExplorerStartingFish(x.Setup.Settlements); err != nil {
			return err
		}
		x.Setup = nil
	}
	s.catanExplorerSyncPhase()
	return nil
}
func (s *State) catanExplorerPhase() string {
	x := s.Catan.Explorer
	if s.Catan.Fishing != nil && s.Catan.Fishing.Pending != nil {
		return "catan_fish_replace"
	}
	if x.Setup != nil {
		return "catan_explorer_setup"
	}
	if x.Pirate != nil && x.Pirate.Pending != nil {
		return "catan_explorer_pirate_" + x.Pirate.Pending.Stage
	}
	if x.Lairs != nil && x.Lairs.Battle != nil {
		return "catan_explorer_battle"
	}
	if x.Economy.Turn == nil {
		return ""
	}
	switch x.Economy.Turn.Phase {
	case "roll":
		return "catan_roll"
	case "discard":
		return "catan_discard"
	case "pirate":
		return "catan_explorer_pirate_place"
	case "ready":
		if x.Cargo.Turn.Phase == "action" {
			return "catan_turn"
		}
		if x.Cargo.Turn.Phase == "movement" {
			return "catan_explorer_move"
		}
		if x.Cargo.Turn.Phase == "ended" && s.catanExplorerHasReadyLair() {
			return "catan_explorer_resolve"
		}
	}
	return ""
}
func (s *State) catanExplorerAfterProduction() error {
	x := s.Catan.Explorer
	if x.Economy.Turn.Phase == "pirate" && x.Pirate != nil && x.Pirate.Pending == nil {
		if _, err := x.Pirate.apply(s.Catan, x.Board, x.Fleet, x.Cargo, x.Economy, s.Turn, s.Catan.TurnSerial, "seven", 0, false, nil); err != nil {
			return err
		}
	}
	s.catanExplorerSyncPhase()
	return nil
}
func (s *State) catanExplorerMissionScore() {
	g := s.Catan
	x := g.Explorer
	scores := make([]int, len(g.Players))
	if x.Lairs != nil {
		scores = x.Lairs.scores()
	}
	if x.Spice != nil {
		for p, score := range x.Spice.publicView(g).Scores {
			scores[p] += score
		}
	}
	if x.Fish != nil {
		for p, score := range x.Fish.publicView(len(g.Players)).Scores {
			scores[p] += score
		}
	}
	for p := range g.Players {
		score := scores[p]
		if k := g.CitiesKnights; k != nil {
			score += k.Players[p].DefenderPoints + k.Players[p].ProgressPoints
			if k.Merchant != nil && k.Merchant.Owner == p {
				score++
			}
			for track := range 3 {
				if g.cityMetropolisOwner(track) == p {
					score += 2
				}
			}
		}
		for _, v := range g.Vertices {
			if v.Owner == p {
				score += v.Level
			}
		}
		g.Players[p].Score = score
	}
}
func (s *State) catanExplorerHasReadyLair() bool {
	x := s.Catan.Explorer
	if x.Lairs == nil {
		return false
	}
	for _, site := range x.Lairs.Sites {
		if site.Ready == s.Catan.TurnSerial && site.Resolved == 0 {
			return true
		}
	}
	return false
}
func (s *State) catanExplorerNextTurn() error {
	g := s.Catan
	s.catanExplorerMissionScore()
	s.catanExplorerVictory()
	if s.Finished {
		return nil
	}
	g.Trade = nil
	if err := s.catanExplorerAdvanceTurn(); err != nil {
		return err
	}
	s.catanExplorerSyncPhase()
	s.catanExplorerVictory()
	return nil
}
func (s *State) applyCatanExplorerMission(player int, a Action) (bool, error) {
	g := s.Catan
	x := g.Explorer
	if x.Pirate == nil {
		return false, nil
	}
	mandatory := x.Pirate.Pending != nil || (x.Lairs != nil && x.Lairs.Battle != nil) || s.Phase == "catan_explorer_resolve"
	if player != s.Turn {
		if mandatory {
			return true, errors.New("请等待当前玩家完成探险回应")
		}
		return false, nil
	}
	pKinds := map[string]string{"catan_explorer_pirate_place": "place", "catan_explorer_pirate_steal": "steal", "catan_explorer_chase": "chase"}
	if kind := pKinds[a.Type]; kind != "" {
		if x.Lairs != nil && x.Lairs.Battle != nil || s.Phase == "catan_explorer_resolve" {
			return true, errors.New("先完成巢穴战斗")
		}
		result, err := x.Pirate.apply(g, x.Board, x.Fleet, x.Cargo, x.Economy, player, g.TurnSerial, kind, a.Target, a.Choice == "skip", catanRandom)
		if err != nil {
			return true, err
		}
		if kind == "place" {
			s.catanLog(player, "将海盗移至海格 #%d", a.Target+1)
		}
		if kind == "chase" {
			s.catanLog(player, "船只驱赶海盗掷出 %d", x.Pirate.LastChase.Die)
		}
		if kind == "steal" && result.Target >= 0 {
			what := "1张资源"
			if result.Gold {
				what = "1金币"
			}
			s.catanLog(player, "从玩家%d偷取%s", result.Target+1, what)
		}
		s.catanExplorerSyncPhase()
		return true, nil
	}
	kinds := map[string]string{"catan_explorer_land": "land", "catan_explorer_pickup": "pickup", "catan_explorer_resolve": "begin", "catan_explorer_battle": "roll"}
	if kind := kinds[a.Type]; kind != "" {
		if x.Lairs == nil {
			return true, errors.New("本场景没有巢穴任务")
		}
		if x.Pirate.Pending != nil {
			return true, errors.New("先完成海盗回应")
		}
		if err := x.Lairs.apply(g, x.Board, x.Fleet, x.Cargo, x.Economy, player, g.TurnSerial, kind, a.Target, a.Slot, a.Cards, catanRandom); err != nil {
			return true, err
		}
		s.catanExplorerMissionScore()
		s.catanExplorerSyncPhase()
		s.catanLog(player, "%s巢穴 #%d", map[string]string{"land": "派船员登陆", "pickup": "接回船员自", "begin": "结算", "roll": "掷英雄骰于"}[kind], a.Target+1)
		s.catanExplorerVictory()
		if s.Finished {
			return true, nil
		}
		if x.Cargo.Turn.Phase == "ended" && x.Lairs.Battle == nil && !s.catanExplorerHasReadyLair() {
			return true, s.catanExplorerNextTurn()
		}
		return true, nil
	}
	if mandatory {
		return true, errors.New("请先完成海盗或巢穴回应")
	}
	if handled, err := s.applyCatanExplorerSpice(player, a); handled {
		return true, err
	}
	return s.applyCatanExplorerFish(player, a)
}

func (s *State) catanExplorerSpecialChoices(player int) ([]Action, bool) {
	g := s.Catan
	x := g.Explorer
	out := []Action{}
	add := func(a Action) { a.Prompt = int(g.TurnSerial); out = append(out, a) }
	if x.Setup != nil {
		step := x.Setup.current(len(g.Players))
		if step != nil && step.Player == player {
			for _, target := range x.Setup.choices(g, x.Board, x.Fleet) {
				out = append(out, Action{Type: "catan_explorer_setup", Prompt: x.Setup.prompt(), Choice: step.Kind, Target: target})
			}
		}
		return out, true
	}
	if x.Pirate == nil {
		return nil, false
	}
	if pending := x.Pirate.Pending; pending != nil {
		if pending.Player == player {
			if pending.Stage == "place" {
				for _, tile := range x.Pirate.destinations(g, x.Board) {
					add(Action{Type: "catan_explorer_pirate_place", Target: tile})
				}
			} else {
				for _, victim := range x.Pirate.victims(g, x.Fleet, x.Economy) {
					add(Action{Type: "catan_explorer_pirate_steal", Target: victim})
					if sum(g.Players[victim].Resources) == 0 {
						add(Action{Type: "catan_explorer_pirate_steal", Target: victim, Choice: "skip"})
					}
				}
			}
		}
		return out, true
	}
	if x.Lairs != nil && x.Lairs.Battle != nil {
		if player == s.Turn {
			add(Action{Type: "catan_explorer_battle", Target: x.Lairs.Battle.Tile})
		}
		return out, true
	}
	if s.Phase == "catan_explorer_resolve" {
		if player == s.Turn {
			for _, site := range x.Lairs.Sites {
				if site.Ready == g.TurnSerial && site.Resolved == 0 {
					add(Action{Type: "catan_explorer_resolve", Target: site.Tile})
				}
			}
		}
		return out, true
	}
	return nil, false
}
func (s *State) catanExplorerLandingChoices(player int) []Action {
	g := s.Catan
	x := g.Explorer
	out := []Action{}
	if x.Pirate == nil || s.Phase != "catan_explorer_move" {
		return out
	}
	for ship := player * 3; ship < (player+1)*3; ship++ {
		if x.Fleet.Positions[ship] < 0 {
			continue
		}
		if x.Pirate.battleReady(g, x.Fleet, player, g.TurnSerial, ship) {
			out = append(out, Action{Type: "catan_explorer_chase", Target: ship})
		}
		if x.Lairs == nil {
			continue
		}
		for _, site := range x.Lairs.Sites {
			if !catanExplorerTouches(g, x.Fleet.Positions[ship], site.Tile) {
				continue
			}
			kind := "catan_explorer_land"
			from := catanExplorerCargoLocation{"ship", ship}
			capacity := 3 - len(x.Cargo.contents(catanExplorerCargoLocation{"lair", site.Tile}))
			if site.Resolved > 0 {
				if site.Resolved >= g.TurnSerial {
					continue
				}
				kind = "catan_explorer_pickup"
				from = catanExplorerCargoLocation{"lair", site.Tile}
				capacity = 2 - x.Cargo.used(catanExplorerCargoLocation{"ship", ship})
			} else if site.Ready > 0 {
				continue
			}
			crew := []int{}
			for _, id := range x.Cargo.contents(from) {
				if id/11 == player && id%11 >= 2 {
					crew = append(crew, id)
				}
			}
			for _, units := range explorerCargoSubsets(crew) {
				if len(units) > 0 && len(units) <= capacity && len(units) <= 2 {
					out = append(out, Action{Type: kind, Target: site.Tile, Slot: ship, Cards: units})
				}
			}
		}
	}
	return out
}
