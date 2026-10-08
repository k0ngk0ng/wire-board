package game

import (
	"errors"
	"slices"
)

// Internal constructor takes an explicit token inventory; the public wrapper
// supplies and labels the site recipe. Untagged historical fixtures stay valid.
func newCatanExplorerCityState(players int, scenario string, numbers []int) (*State, error) {
	if players < 3 || players > 6 {
		return nil, errors.New("探险城市骑士组合需要三至六人")
	}
	g, b, f, c, e, setup, err := newCatanExplorerMissionSetupVariant(players, scenario, "variable", catanRandom(players), true)
	if err != nil {
		return nil, err
	}
	x := &catanExplorer{Board: b, Fleet: f, Cargo: c, Economy: e, Setup: setup, Pirate: newCatanExplorerPirate()}
	g.Explorer = x
	if catanExplorerMissionScenario(scenario) {
		x.Lairs, err = newCatanExplorerLairs(players, numbers)
		if err != nil {
			return nil, err
		}
		shuffle(x.Lairs.Deck)
	} else if len(numbers) != 0 {
		return nil, errors.New("无巢穴任务的组合不能配置巢穴数字")
	}
	if catanExplorerFishScenario(scenario) {
		x.Fish = &catanExplorerFish{Deliveries: []catanExplorerFishDelivery{}}
	}
	if catanExplorerSpiceScenario(scenario) {
		x.Spice = &catanExplorerSpice{Deliveries: []catanExplorerSpiceDelivery{}}
	}
	s := &State{Kind: "catan", Catan: g, Turn: setup.Start, Round: 1, Phase: "catan_explorer_setup", Log: []string{"探险家与城市骑士：随机先手，顺序城市、逆序港口，再放道路与移民船"}}
	return s, s.validateCatanExplorerCities()
}

// This is the aggregate boundary for State.Apply/restore, rather than the
// deliberately narrower controller validators used by rule-unit fixtures.
func (s *State) validateCatanExplorerCities() error {
	g := s.Catan
	if g == nil || g.Explorer == nil || g.CitiesKnights == nil || g.Explorer.Board == nil || !g.Explorer.Board.CitiesKnights {
		return errors.New("组合地图与城市骑士组件不匹配")
	}
	x, k, n := g.Explorer, g.CitiesKnights, len(g.Players)
	if n < 3 || n > 6 || len(k.Players) != n || s.Kind != "catan" || s.Turn < 0 || s.Turn >= n || g.StartPlayer < 0 || g.StartPlayer >= n || s.Round < 1 || g.RollID < 0 || len(g.Dice) != 2 || len(g.DiscardDue) != n || g.TurnSerial > uint64(^uint(0)>>1) {
		return errors.New("组合人数、主回合或骰子记录无效")
	}
	if err := s.validateExplorerCityInventory(); err != nil {
		return err
	}
	if err := x.validateComponents(g); err != nil {
		return err
	}
	if err := s.validateCatanExplorerPaired(); err != nil {
		return err
	}
	if x.Setup != nil {
		step := x.Setup.current(n)
		if step == nil || step.Player != s.Turn || s.Phase != "catan_explorer_setup" || s.Round != 1 || s.Finished || len(s.Winners) > 0 || g.RollID != 0 || x.SkippedRolls != 0 || g.FreeRoads != 0 || g.Trade != nil || g.Dice[0] != 0 || g.Dice[1] != 0 || sum(g.DiscardDue) != 0 {
			return errors.New("组合开局主状态不一致")
		}
		return nil
	}
	if g.TurnSerial == 0 || g.SetupStep != 2*n || x.Economy.Turn == nil {
		return errors.New("组合尚未完成开局或缺少生产回合")
	}
	if err := s.validateExplorerCityFlow(); err != nil {
		return err
	}
	alive := 0
	for player, p := range g.Players {
		if !p.Eliminated {
			alive++
		} else if sum(p.Resources) != 0 || x.Economy.Gold[player] != 0 || len(k.Players[player].Progress) != 0 {
			return errors.New("离场玩家的资源、商品、金币与私有进步牌必须归还")
		}
	}
	if alive == 0 || g.Players[s.Turn].Eliminated || x.SkippedRolls < 0 || x.SkippedRolls > n-alive {
		return errors.New("组合活跃玩家或离场跳过的生产次数无效")
	}
	productionTurns := g.TurnSerial
	if g.Paired != nil {
		productionTurns = (g.TurnSerial + 1) / 2
	}
	rolled := int(productionTurns) - x.SkippedRolls
	if x.Economy.Turn.Phase == "roll" {
		rolled--
	}
	if rolled < 0 || g.RollID != rolled {
		return errors.New("组合生产次数与普通/配对回合不一致")
	}
	if alive == n && g.Paired == nil && (s.Turn != (g.StartPlayer+int((g.TurnSerial-1)%uint64(n)))%n || s.Round != 1+int((g.TurnSerial-1)/uint64(n))) {
		return errors.New("组合顺时针轮序不一致")
	}
	if s.Finished {
		if s.Phase != "finished" || !slices.Equal(s.Winners, []int{s.Turn}) || g.Players[s.Turn].Score < x.Board.Target && alive != 1 || g.Trade != nil {
			return errors.New("组合胜负或结束阶段无效")
		}
	} else if len(s.Winners) > 0 || s.Phase == "finished" || x.Lairs != nil && x.Lairs.RewardVictory != nil {
		return errors.New("组合胜利记录与对局状态不一致")
	}
	return s.validateExplorerCityEventQueue()
}

// Called only inside the common whole-state transaction in applyCatan.
func (s *State) applyCatanExplorerCity(player int, a Action) error {
	if a.Skill != "" {
		return errors.New("组合操作不能夹带其他技能")
	}
	if s.Catan.Explorer.Setup != nil {
		return s.applyCatanExplorerSetup(player, a)
	}
	if s.Catan.CitiesKnights.Pending != nil {
		return s.catanExplorerCityRespond(player, a)
	}
	return s.catanExplorerCityAction(player, a)
}
