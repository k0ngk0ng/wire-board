package game

// The printed caravans scenario only ends on the victory target, and the
// official text stops placing caravans once the supply is exhausted. A shared
// board can freeze: nobody has a legal settlement, city upgrade, road or ship
// left while the caravan supply is gone and neither a victory card nor enough
// knight cards remain to change any score. Such a table would roll forever, so
// this site rule ends it on points. The marker is written to the log; the room
// history keeps the base caravans rule version.
const CatanCaravansDeadEndRules = "catan-caravans-dead-end-site-v1"

// catanCaravansDeadEnd ends a frozen caravans table and reports whether the
// game was finished by the rule above.
func (s *State) catanCaravansDeadEnd() bool {
	g := s.Catan
	c := g.Caravans
	if c == nil || g.setup() || s.Finished || c.Map == nil || len(c.Wagons) < c.Map.Supply {
		return false
	}
	knights := 0
	for _, kind := range g.DevDeck {
		switch kind {
		case 4: // A hidden victory card could still decide the game.
			return false
		case 0: // A knight card can still reach the largest army bonus.
			knights++
		}
	}
	for p, seat := range g.Players {
		if seat.Eliminated {
			continue
		}
		if seat.Knights+knights >= 3 && g.ArmyOwner != p {
			return false
		}
		if g.settlementPiecesLeft(p) > 0 {
			for _, v := range g.Vertices {
				if g.canSettlement(p, v.ID, false) {
					return false
				}
			}
		}
		if g.cityPiecesLeft(p) > 0 {
			for _, v := range g.Vertices {
				if g.canCityUpgrade(p, v.ID) {
					return false
				}
			}
		}
		roads, _, _ := g.pieces(p)
		if roads < 15 {
			for _, e := range g.Edges {
				if g.canRoad(p, e.ID) {
					return false
				}
			}
		}
		if g.shipCount(p) < 15 {
			for _, e := range g.Edges {
				if g.canShip(p, e.ID) {
					return false
				}
			}
		}
	}
	s.catanScores()
	best, vps, cards := -1, -1, -1
	winners := []int{}
	for i, seat := range g.Players {
		if seat.Eliminated {
			continue
		}
		hand := sum(seat.Resources)
		if seat.Score > best || seat.Score == best && (seat.Dev[4] > vps || seat.Dev[4] == vps && hand > cards) {
			best, vps, cards, winners = seat.Score, seat.Dev[4], hand, []int{i}
		} else if seat.Score == best && seat.Dev[4] == vps && hand == cards {
			winners = append(winners, i)
		}
	}
	s.Finished, s.Phase, s.Winners = true, "finished", winners
	g.Trade = nil
	s.Log = append(s.Log, "商队供应已尽且棋盘上再无可建造位置：按"+CatanCaravansDeadEndRules+"比较总分（同分先比胜利点牌，再比手牌）结束对局")
	return true
}
