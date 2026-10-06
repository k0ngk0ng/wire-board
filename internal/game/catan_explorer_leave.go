package game

import "errors"

// Platform timeout removal, not a printed board-game rule. The caller clones
// the complete State and verifies it again before publishing the result.
func (s *State) eliminateCatanExplorer(player int) error {
	g := s.Catan
	x := g.Explorer
	if s.Finished || player < 0 || player >= len(g.Players) || player != s.Turn || g.Players[player].Eliminated || s.Phase != "catan_roll" && s.Phase != "catan_turn" && s.Phase != "catan_explorer_move" {
		return errors.New("当前不能移除此探险玩家；开局、弃牌、海盗和战斗回应由系统先完成")
	}
	if s.Phase == "catan_roll" {
		x.SkippedRolls++
	}
	p := &g.Players[player]
	for r, n := range p.Resources {
		g.Bank[r] += n
		p.Resources[r] = 0
	}
	x.Economy.GoldBank += x.Economy.Gold[player]
	x.Economy.Gold[player] = 0
	for unit := player * 11; unit < (player+1)*11; unit++ {
		x.Cargo.Units[unit] = catanExplorerCargoLocation{"supply", -1}
	}
	for ship := player * 3; ship < (player+1)*3; ship++ {
		x.Fleet.Positions[ship] = -1
	}
	for id, loc := range x.Cargo.Fish {
		if loc.Kind == "ship" && loc.Index/3 == player || loc.Kind == "harbor" && g.Vertices[loc.Index].Owner == player {
			x.Cargo.Fish[id] = catanExplorerCargoLocation{"supply", -1}
		}
	}
	if x.Fish != nil {
		if len(x.Fish.Retired) == 0 {
			x.Fish.Retired = make([]bool, len(g.Players))
		}
		x.Fish.Retired[player] = true
	}
	g.Trade = nil
	p.Eliminated = true
	if x.Pirate != nil && x.Pirate.Owner == player {
		x.Pirate.Owner, x.Pirate.Tile = -1, -1
	}
	if x.Lairs != nil {
		if len(x.Lairs.Retired) == 0 {
			x.Lairs.Retired = make([]bool, len(g.Players))
		}
		x.Lairs.Retired[player] = true
		for i := range x.Lairs.Sites {
			site := &x.Lairs.Sites[i]
			if site.Resolved == 0 && site.Ready > 0 {
				// No rewards have been awarded in the movement phase. Returning
				// the captor's crew drops this site below three; other crew stay.
				site.Ready, site.Captor = 0, -1
				s.catanLog(player, "离场撤回船员，巢穴 #%d 恢复待攻陷", site.Tile+1)
			}
		}
		s.catanExplorerMissionScore()
	}
	// Even a pre-production departure must close its serial without inventing
	// a production roll or restoring previous-vessel movement points.
	x.Cargo.Turn = &catanExplorerCargoTurn{Player: player, Sequence: g.TurnSerial, Phase: "ended"}
	x.Fleet.Turn = &catanExplorerMovementTurn{Player: player, Sequence: g.TurnSerial, Current: -1, Ships: make([]catanExplorerShipMove, len(x.Fleet.Positions)), Exploring: []int{}}
	for ship := range x.Fleet.Turn.Ships {
		x.Fleet.Turn.Ships[ship].Closed = true
	}
	x.Economy.Turn.Phase = "abandoned"
	x.Economy.Turn.Discard = make([]int, len(g.Players))
	s.catanLog(player, "超时离场：资源、金币、船和货物归还；建筑道路保留但不再生产")
	alive := 0
	for _, p := range g.Players {
		if !p.Eliminated {
			alive++
		}
	}
	if alive == 0 {
		return errors.New("不能移除最后一名活跃玩家")
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
	g.TurnSerial++
	if err := x.Economy.beginProduction(g, x.Fleet, x.Cargo, s.Turn, g.TurnSerial); err != nil {
		return err
	}
	s.catanExplorerSyncPhase()
	if alive == 1 {
		s.Finished, s.Phase, s.Winners = true, "finished", []int{s.Turn}
		s.catanLog(s.Turn, "成为唯一未离场玩家，赢得本局")
	} else {
		s.catanExplorerVictory()
	}
	return nil
}
