package monopoly

import (
	"errors"
	"time"
)

func (s *State) buildingAuction() bool {
	return s.Auction != nil && (s.Auction.Kind == "house" || s.Auction.Kind == "hotel")
}
func (s *State) colorGroup(id int) []int {
	if id < 0 || id >= 40 || board[id].Kind != "street" {
		return nil
	}
	ids := []int{}
	for i, space := range board {
		if space.Kind == "street" && space.Group == board[id].Group {
			ids = append(ids, i)
		}
	}
	return ids
}
func (s *State) buildKind(p, id int) string {
	if !s.owns(p, id) || board[id].Kind != "street" || s.Properties[id].Buildings == 5 {
		return ""
	}
	level := s.Properties[id].Buildings
	for _, i := range s.colorGroup(id) {
		prop := s.Properties[i]
		if prop.Owner != p || prop.Mortgaged || prop.Buildings < level {
			return ""
		}
	}
	if level == 4 {
		return "hotel"
	}
	return "house"
}
func (s *State) buildTargets(p int, kind string) []int {
	ids := []int{}
	for i := range s.Properties {
		if s.buildKind(p, i) == kind {
			ids = append(ids, i)
		}
	}
	return ids
}

// A building bidder must retain a complete, unmortgaged color set. Its mortgage
// value therefore cannot fund that same bid. Houses may be sold evenly (or as a
// whole group); the four houses required for a hotel must remain in place.
func (s *State) canFundBuild(p int, kind string, minimum int) bool {
	for _, target := range s.buildTargets(p, kind) {
		cash := s.Players[p].Cash
		for id, prop := range s.Properties {
			if prop.Owner != p {
				continue
			}
			sameGroup := board[id].Group == board[target].Group
			if !sameGroup && !prop.Mortgaged {
				cash += board[id].Price / 2
			}
			if !sameGroup || kind == "house" {
				cash += prop.Buildings * board[id].BuildingCost / 2
			}
		}
		if cash >= minimum {
			return true
		}
	}
	return false
}
func (s *State) placeBuilding(p, id, cost int) error {
	kind := s.buildKind(p, id)
	if kind == "" || s.Players[p].Cash < cost {
		return errors.New("不符合均衡建设或资金要求")
	}
	if kind == "house" {
		if s.Houses == 0 {
			return errors.New("银行没有剩余房屋")
		}
		s.Houses--
	} else {
		if s.Hotels == 0 {
			return errors.New("银行没有剩余旅馆")
		}
		s.Hotels--
		s.Houses += 4
	}
	s.pay(p, -1, cost)
	s.Properties[id].Buildings++
	s.note("玩家 %d 在%s建设，支付 %d", p+1, board[id].Name, cost)
	return nil
}
func (s *State) build(p, id int, now time.Time) error {
	if s.Phase != "roll" && s.Phase != "end" && s.Phase != "purchase" {
		return errors.New("请先完成当前结算或竞拍")
	}
	kind := s.buildKind(p, id)
	if kind == "" {
		return errors.New("须集齐未抵押色组，并均衡建设")
	}
	stock := s.Houses
	if kind == "hotel" {
		stock = s.Hotels
	}
	if stock == 0 {
		return errors.New("银行没有剩余建筑")
	}
	if stock == 1 {
		competitors := 0
		for i, player := range s.Players {
			if !player.Bankrupt && s.canFundBuild(i, kind, 10) {
				competitors++
			}
		}
		if competitors > 1 {
			if !s.canFundBuild(p, kind, 10) {
				return errors.New("无可用竞购资金")
			}
			pause := &Pause{Phase: s.Phase, TimeLeft: max(0, s.Deadline-now.UnixMilli())}
			s.Auction = &Auction{Kind: kind, Property: id, Bidder: -1, Expires: now.Add(time.Duration(s.Options.AuctionSeconds) * time.Second).UnixMilli(), Withdrawn: make([]bool, len(s.Players)), Resume: pause}
			s.enter("auction", now)
			label := "房屋"
			if kind == "hotel" {
				label = "旅馆"
			}
			s.note("银行最后一座%s进入拍卖，起价 10", label)
			return nil
		}
	}
	if s.available(p) < board[id].BuildingCost {
		return errors.New("建造资金不足")
	}
	return s.placeBuilding(p, id, board[id].BuildingCost)
}
func (s *State) sellBuilding(p, id int, now time.Time) error {
	if s.Phase == "mortgage_transfer" || !s.owns(p, id) || s.Properties[id].Buildings == 0 {
		return errors.New("当前不能出售该建筑")
	}
	level := s.Properties[id].Buildings
	for _, i := range s.colorGroup(id) {
		if s.Properties[i].Buildings > level {
			return errors.New("须均衡出售建筑")
		}
	}
	if level == 5 {
		if s.Houses < 4 {
			return errors.New("银行不足 4 座房屋，不能单独降级旅馆；可整组出售建筑")
		}
		s.Houses -= 4
		s.Hotels++
	} else {
		s.Houses++
	}
	s.Properties[id].Buildings--
	s.pay(-1, p, board[id].BuildingCost/2)
	s.note("玩家 %d 出售%s的一座建筑，收回 %d", p+1, board[id].Name, board[id].BuildingCost/2)
	return s.settleDebt(now)
}

// Whole-group liquidation is an explicit atomic operation. It does not borrow
// replacement houses when the physical bank cannot downgrade a hotel.
func (s *State) sellGroup(p, id int, now time.Time) error {
	if s.Phase == "mortgage_transfer" || !s.owns(p, id) || s.groupBuildings(id) == 0 {
		return errors.New("当前不能出售该组建筑")
	}
	ids := s.colorGroup(id)
	for _, i := range ids {
		if s.Properties[i].Owner != p {
			return errors.New("未拥有完整色组")
		}
	}
	refund := 0
	for _, i := range ids {
		level := s.Properties[i].Buildings
		refund += level * board[i].BuildingCost / 2
		if level == 5 {
			s.Hotels++
		} else {
			s.Houses += level
		}
		s.Properties[i].Buildings = 0
	}
	s.pay(-1, p, refund)
	s.note("玩家 %d 出售整组建筑，收回 %d", p+1, refund)
	return s.settleDebt(now)
}
