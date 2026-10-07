package game

import "errors"

// E&P 5–6 (2025), p3: one production followed by two independent action /
// movement portions. TurnSerial identifies a portion, not a production roll;
// cargo, fish, farm and pirate allowances therefore reset for each actor.
func (e *catanExplorerEconomy) beginSecondary(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, player int, sequence uint64) error {
	if err := e.validate(g, f, c); err != nil {
		return err
	}
	if g.Paired == nil || !g.Paired.Second || g.Paired.Secondary != player || player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || e.Turn == nil || e.Turn.NoProduction || e.Turn.Phase != "ready" && e.Turn.Phase != "abandoned" || c.Turn == nil || c.Turn.Phase != "ended" || sequence == 0 || sequence != e.Turn.Sequence+1 {
		return errors.New("不能跳过第一位玩家或重复开始配对行动")
	}
	if err := c.beginAction(g, f, player, sequence); err != nil {
		return err
	}
	e.Turn = &catanExplorerEconomyTurn{Player: player, Sequence: sequence, Phase: "ready", NoProduction: true, Discard: make([]int, len(g.Players))}
	return nil
}

func (s *State) catanExplorerAdvanceTurn() error {
	g, x := s.Catan, s.Catan.Explorer
	resetCityAction := func() {
		if k := g.CitiesKnights; k != nil {
			k.ActionSerial = g.TurnSerial
			k.TradePowers = nil
		}
	}
	if pair := g.Paired; pair != nil {
		if !pair.Second && pair.Secondary != pair.Primary && !g.Players[pair.Secondary].Eliminated {
			pair.Second = true
			s.Turn = pair.Secondary
			g.TurnSerial++
			if err := x.Economy.beginSecondary(g, x.Fleet, x.Cargo, s.Turn, g.TurnSerial); err != nil {
				return err
			}
			g.Dice = []int{0, 0}
			s.catanLog(s.Turn, "开始配对行动：不掷生产骰，可交易银行、建设和航行，不能与其他玩家交易")
			resetCityAction()
			return nil
		}
		// Rotate from the first marker, never from the secondary actor's seat.
		s.Turn = pair.Primary
	}
	for {
		s.Turn = (s.Turn + 1) % len(g.Players)
		if s.Turn == g.StartPlayer {
			s.Round++
		}
		if !g.Players[s.Turn].Eliminated {
			break
		}
	}
	// Validate the old economy portion while it still has its original markers.
	g.TurnSerial++
	if err := x.Economy.beginProduction(g, x.Fleet, x.Cargo, s.Turn, g.TurnSerial); err != nil {
		return err
	}
	if pair := g.Paired; pair != nil {
		pair.Primary, pair.Secondary, pair.Second = s.Turn, g.pairedPartner(s.Turn), false
	}
	resetCityAction()
	return nil
}

func (s *State) validateCatanExplorerPaired() error {
	g, x := s.Catan, s.Catan.Explorer
	pair, n := g.Paired, len(g.Players)
	if n <= 4 {
		if pair != nil {
			return errors.New("二至四人探险不使用配对回合")
		}
		return nil
	}
	if pair == nil || pair.Primary < 0 || pair.Primary >= n || pair.Secondary < 0 || pair.Secondary >= n {
		return errors.New("五六人探险缺少配对标记")
	}
	if x.Setup != nil {
		if pair.Primary != x.Setup.Start || pair.Secondary != (pair.Primary+3)%n || pair.Second {
			return errors.New("探险开局配对标记无效")
		}
		return nil
	}
	t := x.Economy.Turn
	actor := pair.Primary
	if pair.Second {
		actor = pair.Secondary
	}
	if actor != s.Turn || t == nil || t.NoProduction != pair.Second || pair.Second != (g.TurnSerial%2 == 0) {
		return errors.New("探险配对行动玩家或序号不一致")
	}
	allPresent := true
	for _, p := range g.Players {
		allPresent = allPresent && !p.Eliminated
	}
	if allPresent {
		round := (g.TurnSerial - 1) / 2
		if pair.Primary != (g.StartPlayer+int(round%uint64(n)))%n || pair.Secondary != (pair.Primary+3)%n || s.Round != 1+int(round/uint64(n)) {
			return errors.New("探险配对标记轮换不一致")
		}
	} else if !pair.Second && pair.Secondary != g.pairedPartner(pair.Primary) {
		return errors.New("离场后的探险配对标记无效")
	}
	return nil
}
