package monopoly

import (
	"errors"
	"slices"
	"time"
)

func (s *State) available(p int) int {
	cash := s.Players[p].Cash
	if s.Auction != nil && s.Auction.Bidder == p {
		cash -= s.Auction.Bid
	}
	return cash
}
func (s *State) pay(from, to, amount int) {
	if from < 0 {
		s.BankNet -= amount
	} else {
		s.Players[from].Cash -= amount
	}
	if to < 0 {
		s.BankNet += amount
	} else {
		s.Players[to].Cash += amount
	}
}
func (s *State) groupBuildings(id int) int {
	n := 0
	for i, p := range s.Properties {
		if i == id || (board[id].Kind == "street" && board[id].Group == board[i].Group) {
			n += p.Buildings
		}
	}
	return n
}
func (s *State) liquidation(p int) int {
	cash := s.Players[p].Cash
	for i, prop := range s.Properties {
		if prop.Owner == p {
			if !prop.Mortgaged {
				cash += board[i].Price / 2
			}
			cash += prop.Buildings * board[i].BuildingCost / 2
		}
	}
	return cash
}
func (s *State) owns(p, id int) bool {
	return id >= 0 && id < len(s.Properties) && board[id].Price > 0 && s.Properties[id].Owner == p
}

// Money is integral. Round each 10% mortgage fee up rather than silently
// truncating (e.g. a 175 mortgage costs 18 interest and 193 to redeem).
func mortgageFee(id int) int { return (board[id].Price/2 + 9) / 10 }

func (s *State) mortgage(p, id int, now time.Time) error {
	if s.Phase == "mortgage_transfer" || !s.owns(p, id) || s.Properties[id].Mortgaged || s.groupBuildings(id) > 0 {
		return errors.New("抵押前须拥有该地产，并卖掉同色组全部建筑")
	}
	s.Properties[id].Mortgaged = true
	s.pay(-1, p, board[id].Price/2)
	s.note("玩家 %d 抵押%s，获得 %d", p+1, board[id].Name, board[id].Price/2)
	return s.settleDebt(now)
}
func (s *State) unmortgage(p, id int) error {
	if s.Phase == "debt" || s.Phase == "mortgage_transfer" || !s.owns(p, id) || !s.Properties[id].Mortgaged {
		return errors.New("当前不能赎回该地产")
	}
	cost := board[id].Price/2 + mortgageFee(id)
	if s.available(p) < cost {
		return errors.New("赎回资金不足，竞价金额已保留")
	}
	s.pay(p, -1, cost)
	s.Properties[id].Mortgaged = false
	s.note("玩家 %d 以 %d 赎回%s", p+1, cost, board[id].Name)
	return nil
}
func (s *State) settleDebt(now time.Time) error {
	if s.Debt == nil || s.Players[s.Debt.Debtor].Cash < s.Debt.Amount {
		return nil
	}
	d := s.Debt
	s.pay(d.Debtor, d.Creditor, d.Amount)
	s.note("玩家 %d 支付 %d（%s）", d.Debtor+1, d.Amount, d.Reason)
	s.Debt = nil
	return s.drain(now)
}
func (s *State) beginAuction(id int, now time.Time) error {
	if id < 0 || id >= 40 || board[id].Price == 0 || s.Properties[id].Owner != -1 {
		return errors.New("该地产不能拍卖")
	}
	s.Auction = &Auction{Kind: "property", Property: id, Bidder: -1, Expires: now.Add(time.Duration(s.Options.AuctionSeconds) * time.Second).UnixMilli(), Withdrawn: make([]bool, len(s.Players))}
	s.enter("auction", now)
	s.note("开始拍卖%s，起价 10", board[id].Name)
	if len(s.auctionActors()) == 0 {
		return s.finishAuction(now)
	}
	return nil
}
func (s *State) bid(p, amount, id int, now time.Time) error {
	if s.Phase != "auction" || !slices.Contains(s.auctionActors(), p) || amount < s.auctionMinimum() || amount > s.Players[p].Cash {
		return errors.New("报价须高于当前价格，且不能超过现金")
	}
	if s.buildingAuction() {
		if s.buildKind(p, id) != s.Auction.Kind {
			return errors.New("请指定自己可合法建设的地产")
		}
		s.Auction.Property = id
	}
	s.Auction.Bid, s.Auction.Bidder = amount, p
	grace := time.Duration(min(5, s.Options.AuctionSeconds)) * time.Second
	s.Auction.Expires = max(s.Auction.Expires, now.Add(grace).UnixMilli())
	s.note("玩家 %d 报价 %d", p+1, amount)
	if len(s.auctionActors()) == 0 {
		return s.finishAuction(now)
	}
	return nil
}
func (s *State) finishAuction(now time.Time) error {
	a := s.Auction
	if a == nil {
		return errors.New("没有待结束拍卖")
	}
	if a.Bidder >= 0 {
		if s.Players[a.Bidder].Bankrupt || s.Players[a.Bidder].Cash < a.Bid {
			return errors.New("拍卖预留资金不足")
		}
		if s.buildingAuction() {
			if err := s.placeBuilding(a.Bidder, a.Property, a.Bid); err != nil {
				return err
			}
		} else {
			s.pay(a.Bidder, -1, a.Bid)
			s.Properties[a.Property].Owner = a.Bidder
			s.note("玩家 %d 以 %d 拍得%s", a.Bidder+1, a.Bid, board[a.Property].Name)
		}
	} else {
		if s.buildingAuction() {
			s.note("建筑无人竞价，留在银行")
		} else {
			s.note("%s无人出价，留在银行", board[a.Property].Name)
		}
	}
	s.Auction = nil
	if a.Resume != nil {
		s.enter(a.Resume.Phase, now)
		s.Deadline = now.UnixMilli() + a.Resume.TimeLeft
		return nil
	}
	return s.drain(now)
}

func (s *State) acceptMortgage(p int, choice string, now time.Time) error {
	if s.Phase != "mortgage_transfer" || s.Transfer == nil || s.Transfer.Player != p {
		return errors.New("没有待处理的抵押转让")
	}
	id := s.Transfer.Property
	cost := mortgageFee(id)
	if choice == "redeem" {
		cost += board[id].Price / 2
	} else if choice != "keep" {
		return errors.New("请选择保留抵押或立即赎回")
	}
	if choice == "redeem" && s.Players[p].Cash < cost {
		return errors.New("赎回资金不足，可先选择保留抵押并筹资付息")
	}
	if choice == "redeem" {
		s.Properties[id].Mortgaged = false
	}
	s.Transfer = nil
	s.Effects = append([]Effect{{Kind: "payment", Actor: p, Target: -1, Amount: cost, Reason: "接收抵押地产费用"}}, s.Effects...)
	return s.drain(now)
}

func (s *State) bankrupt(p int, now time.Time) error {
	if s.Phase != "debt" || s.Debt == nil || s.Debt.Debtor != p {
		return errors.New("当前没有待处理欠款")
	}
	if s.liquidation(p) >= s.Debt.Amount {
		return errors.New("仍可通过卖房或抵押筹足资金，不能宣告破产")
	}
	creditor := s.Debt.Creditor
	s.Debt = nil
	for id, prop := range s.Properties {
		if prop.Owner != p || prop.Buildings == 0 {
			continue
		}
		s.pay(-1, p, prop.Buildings*board[id].BuildingCost/2)
		if prop.Buildings == 5 {
			s.Hotels++
		} else {
			s.Houses += prop.Buildings
		}
		s.Properties[id].Buildings = 0
	}
	s.pay(p, creditor, s.Players[p].Cash)
	queue := []Effect{}
	for id, prop := range s.Properties {
		if prop.Owner != p {
			continue
		}
		s.Properties[id].Owner = creditor
		if creditor < 0 {
			s.Properties[id].Mortgaged = false
			queue = append(queue, Effect{Kind: "auction", Property: id})
		} else if prop.Mortgaged {
			queue = append(queue, Effect{Kind: "mortgage_transfer", Actor: creditor, Property: id})
		}
	}
	for _, card := range s.Players[p].JailCards {
		if creditor >= 0 {
			s.Players[creditor].JailCards = append(s.Players[creditor].JailCards, card)
		} else {
			deck := 0
			if card == "chest" {
				deck = 1
			}
			s.Decks[deck] = append(s.Decks[deck], 0)
		}
	}
	s.Players[p].JailCards = []string{}
	s.Players[p].Bankrupt = true
	s.Trades = slices.DeleteFunc(s.Trades, func(t Trade) bool { return t.From == p || t.To == p })
	s.Eliminated = append(s.Eliminated, p)
	s.note("玩家 %d 破产出局", p+1)
	s.Effects = append(queue, s.Effects...)
	return s.drain(now)
}

func (s *State) drain(now time.Time) error {
	// The estate has already been transferred by bankrupt(). With a single
	// survivor the game is over; do not start fees or auctions after victory.
	if s.active() == 1 {
		s.Effects = nil
		s.Debt = nil
		s.Transfer = nil
		s.Auction = nil
		s.enter("finished", now)
		s.note("本局结束，最后未破产的玩家获胜")
		return nil
	}
	for len(s.Effects) > 0 {
		e := s.Effects[0]
		s.Effects = s.Effects[1:]
		switch e.Kind {
		case "release_move":
			if !s.Players[e.Actor].Bankrupt {
				s.Players[e.Actor].Jailed = false
				s.Players[e.Actor].JailTurns = 0
				return s.move(e.Actor, e.Amount, now)
			}
		case "payment":
			if s.Players[e.Actor].Bankrupt {
				continue
			}
			if e.Target >= 0 && s.Players[e.Target].Bankrupt {
				continue
			}
			if s.Players[e.Actor].Cash < e.Amount {
				s.Debt = &Debt{Debtor: e.Actor, Creditor: e.Target, Amount: e.Amount, Reason: e.Reason}
				s.enter("debt", now)
				return nil
			}
			s.pay(e.Actor, e.Target, e.Amount)
			s.note("玩家 %d 支付 %d（%s）", e.Actor+1, e.Amount, e.Reason)
		case "auction":
			if s.active() > 1 {
				return s.beginAuction(e.Property, now)
			}
		case "mortgage_transfer":
			if !s.Players[e.Actor].Bankrupt && s.Properties[e.Property].Owner == e.Actor && s.Properties[e.Property].Mortgaged {
				s.Transfer = &Transfer{Player: e.Actor, Property: e.Property}
				s.enter("mortgage_transfer", now)
				return nil
			}
		default:
			return errors.New("未知待续效果")
		}
	}
	if s.active() == 1 {
		s.enter("finished", now)
		s.note("本局结束，最后未破产的玩家获胜")
		return nil
	}
	if s.Players[s.Turn].Bankrupt {
		s.nextTurn()
		s.enter("roll", now)
	} else {
		s.enter(s.NextPhase, now)
	}
	return nil
}
