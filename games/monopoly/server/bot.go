package monopoly

import (
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"slices"
)

func botAction(ctx gameapi.Context, match gameapi.Match) (json.RawMessage, error) {
	s, err := decode(match)
	if err != nil {
		return nil, err
	}
	p := ctx.Actor
	if !slices.Contains(s.actors(), p) {
		return nil, errors.New("电脑当前不能行动")
	}
	a := Action{Window: s.Window}
	if s.tradeAllowed() {
		for _, offer := range s.Trades {
			if offer.To != p || offer.Expires <= ctx.Now.UnixMilli() {
				continue
			}
			a.Type, a.TradeID = "trade_reject", offer.ID
			_, fee, err := s.checkTrade(offer)
			if err == nil && s.bundleValue(offer.Give)-s.bundleValue(offer.Want)-fee >= 0 && s.available(p)+offer.Give.Cash-offer.Want.Cash-fee >= 100 {
				a.Type = "trade_accept"
			}
			return json.Marshal(a)
		}
	}
	if s.Phase == "roll" || s.Phase == "end" {
		if investment := s.investment(p); investment != nil {
			investment.Window = s.Window
			return json.Marshal(investment)
		}
		if s.Phase == "end" && p == s.Turn {
			if terms := s.completionOffer(p); terms != nil {
				return json.Marshal(Action{Type: "trade_offer", Window: s.Window, Offer: terms})
			}
		}
	}
	switch s.Phase {
	case "auction":
		if !slices.Contains(s.auctionActors(), p) {
			a.Type = "wait"
			break
		}
		a.Type = "auction_pass"
		ceiling := board[s.Auction.Property].Price
		if s.buildingAuction() {
			targets := s.buildTargets(p, s.Auction.Kind)
			a.Property = targets[0]
			ceiling = board[a.Property].BuildingCost
		}
		if bid := s.auctionMinimum(); bid <= ceiling && s.available(p)-bid >= 100 {
			a.Type = "bid"
			a.Amount = min(ceiling, s.available(p)-100, max(bid, s.Auction.Bid+max(10, ceiling/10)))
		}
	case "debt":
		if p != s.Debt.Debtor {
			a.Type = "wait"
			break
		}
		a.Type = "bankrupt"
		for id, prop := range s.Properties {
			if prop.Owner != p || prop.Buildings == 0 {
				continue
			}
			greatest := true
			for _, other := range s.colorGroup(id) {
				if s.Properties[other].Buildings > prop.Buildings {
					greatest = false
				}
			}
			if greatest {
				a.Type = "sell_building"
				if prop.Buildings == 5 && s.Houses < 4 {
					a.Type = "sell_group"
				}
				a.Property = id
				return json.Marshal(a)
			}
		}
		for id, prop := range s.Properties {
			if prop.Owner == p && !prop.Mortgaged && s.groupBuildings(id) == 0 {
				a.Type = "mortgage"
				a.Property = id
				break
			}
		}
	case "mortgage_transfer":
		if p != s.Transfer.Player {
			a.Type = "wait"
		} else {
			a.Type = "accept_mortgage"
			a.Choice = "keep"
		}
	default:
		if p != s.Turn {
			a.Type = "wait"
		} else {
			switch s.Phase {
			case "purchase":
				a.Type = "auction"
				if s.available(p) >= board[s.Players[p].Position].Price {
					a.Type = "buy"
				}
			case "end":
				a.Type = "end"
			case "roll":
				a.Type = "roll"
				if s.Players[p].Jailed && len(s.Players[p].JailCards) > 0 {
					a.Type = "jail_card"
					a.Choice = s.Players[p].JailCards[0]
				}
			}
		}
	}
	return json.Marshal(a)
}

// Invest only from public ownership/rent data, keeping a cash buffer. Each
// action changes real assets, so repeated bot ticks cannot create a wait loop.
func (s *State) investment(p int) *Action {
	budget := s.available(p) - 150
	if budget <= 0 {
		return nil
	}
	for id, prop := range s.Properties {
		if prop.Owner != p || !prop.Mortgaged || board[id].Kind != "street" {
			continue
		}
		complete, cost := true, 0
		for _, i := range s.colorGroup(id) {
			if s.Properties[i].Owner != p {
				complete = false
			}
			if s.Properties[i].Mortgaged {
				cost += board[i].Price/2 + mortgageFee(i)
			}
		}
		if complete && cost <= budget {
			return &Action{Type: "unmortgage", Property: id}
		}
	}
	best, score := -1, -1
	for id, prop := range s.Properties {
		kind := s.buildKind(p, id)
		if kind == "" || board[id].BuildingCost > budget || (kind == "house" && s.Houses == 0) || (kind == "hotel" && s.Hotels == 0) {
			continue
		}
		current := board[id].Rents[prop.Buildings]
		if prop.Buildings == 0 {
			current *= 2
		}
		value := (board[id].Rents[prop.Buildings+1] - current) * 1000 / board[id].BuildingCost
		if value > score {
			best, score = id, value
		}
	}
	if best >= 0 {
		return &Action{Type: "build", Property: best}
	}
	return nil
}

func (s *State) completionOffer(p int) *TradeTerms {
	if len(s.TradeSent) > p && s.TradeSent[p] == s.TurnSerial {
		return nil
	}
	for _, offer := range s.Trades {
		if offer.From == p {
			return nil
		}
	}
	for id, prop := range s.Properties {
		if board[id].Kind != "street" || prop.Owner < 0 || prop.Owner == p || s.groupBuildings(id) > 0 {
			continue
		}
		complete := true
		for _, other := range s.colorGroup(id) {
			if other != id && s.Properties[other].Owner != p {
				complete = false
			}
		}
		if !complete {
			continue
		}
		price := board[id].Price * 2
		fee := 0
		want := Bundle{Properties: []int{id}}
		if prop.Mortgaged {
			price /= 2
			fee = board[id].Price/2 + mortgageFee(id)
			want.Redeem = []int{id}
		}
		if s.available(p)-price-fee < 150 {
			continue
		}
		return &TradeTerms{Target: prop.Owner, Give: Bundle{Cash: price}, Want: want}
	}
	return nil
}
