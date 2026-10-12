package catan

// The printed four-player sea layouts hold two human seats and two neutral
// colours. Room can run out before either human reaches the scenario target:
// every city is built, every road and ship is placed, and the two-edge spacing
// rule leaves no settlement spot anywhere, so no score can change and the table
// would roll forever. Official text has no rule for that table, so this site
// rule ends it on points. The marker goes to the log; room history keeps the
// base combination rule version.
const CatanTwoRiversSeaDeadEndRules = "catan-two-rivers-seafarers-dead-end-site-v1"

// catanTwoRiversSeaDeadEnd ends a frozen two-player rivers and seafarers table
// and reports whether the rule above finished it. Tribe tables stay out: their
// points can still move without any build.
func (s *State) catanTwoRiversSeaDeadEnd() bool {
	g := s.Catan
	if g == nil || g.Two == nil || !g.twoRiversSea() || g.setup() || s.Finished || g.Seafarers == nil || g.Seafarers.Scenario == "tribe" {
		return false
	}
	// A hidden victory-point card can still decide the game, and a knight card
	// can still reach the largest army bonus.
	knights := 0
	for _, kind := range g.DevDeck {
		switch kind {
		case 4:
			return false
		case 0:
			knights++
		}
	}
	armyTarget := 3
	if g.ArmyOwner >= 0 && g.ArmyOwner < len(g.Players) {
		// An existing holder keeps the bonus on a tie.
		armyTarget = max(armyTarget, g.Players[g.ArmyOwner].Knights+1)
	}
	for p, seat := range g.Players {
		if seat.Eliminated {
			continue
		}
		// Dev includes newly purchased cards. They are playable on a later
		// turn, so they also prevent declaring the whole match frozen.
		if seat.Knights+seat.Dev[0]+knights >= armyTarget && g.ArmyOwner != p {
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
	s.Log = append(s.Log, "棋盘上再无可建造位置、发展牌与骑士都不足以改变分数：按"+CatanTwoRiversSeaDeadEndRules+"比较总分（同分先比胜利点牌，再比手牌）结束对局")
	return true
}
