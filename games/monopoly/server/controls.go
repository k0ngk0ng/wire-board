package monopoly

import "slices"

type PropertyControls struct {
	Mortgage        bool   `json:"mortgage"`
	Redeem          bool   `json:"redeem"`
	Build           bool   `json:"build"`
	Sell            bool   `json:"sell"`
	SellGroup       bool   `json:"sellGroup"`
	MortgageValue   int    `json:"mortgageValue"`
	RedeemCost      int    `json:"redeemCost"`
	BuildCost       int    `json:"buildCost"`
	SellValue       int    `json:"sellValue"`
	GroupSellValue  int    `json:"groupSellValue"`
	BuildKind       string `json:"buildKind"`
	BuildingAuction bool   `json:"buildingAuction"`
}
type Controls struct {
	Active        bool                     `json:"active"`
	Roll          bool                     `json:"roll"`
	End           bool                     `json:"end"`
	Buy           bool                     `json:"buy"`
	Auction       bool                     `json:"auction"`
	Bid           bool                     `json:"bid"`
	AuctionPass   bool                     `json:"auctionPass"`
	Trade         bool                     `json:"trade"`
	JailPay       bool                     `json:"jailPay"`
	Bankrupt      bool                     `json:"bankrupt"`
	MinBid        int                      `json:"minBid"`
	CashAvailable int                      `json:"cashAvailable"`
	JailCards     []string                 `json:"jailCards"`
	BidTargets    []int                    `json:"bidTargets"`
	Properties    map[int]PropertyControls `json:"properties"`
}

func (s *State) preservesBuild(levels map[int]int, mortgage int) bool {
	if !s.buildingAuction() || s.Auction.Bidder < 0 {
		return true
	}
	next := *s
	next.Properties = append([]Property(nil), s.Properties...)
	for id, level := range levels {
		next.Properties[id].Buildings = level
	}
	if mortgage >= 0 {
		next.Properties[mortgage].Mortgaged = true
	}
	return next.buildKind(s.Auction.Bidder, s.Auction.Property) == s.Auction.Kind
}

// Read-only presentation hints. Apply remains authoritative; these predicates
// reuse the same group, funding and auction rules and never advance randomness.
func (s *State) controls(p int) Controls {
	c := Controls{Properties: map[int]PropertyControls{}, JailCards: []string{}, BidTargets: []int{}}
	if p < 0 || p >= len(s.Players) || s.Players[p].Bankrupt || s.Phase == "finished" {
		return c
	}
	c.Active = slices.Contains(s.actors(), p)
	c.CashAvailable = s.available(p)
	c.Roll = c.Active && s.Phase == "roll" && p == s.Turn
	c.End = c.Active && s.Phase == "end" && p == s.Turn
	c.Auction = c.Active && s.Phase == "purchase" && p == s.Turn
	c.Buy = c.Auction && c.CashAvailable >= board[s.Players[p].Position].Price
	c.Trade = c.Active && s.tradeAllowed()
	c.JailPay = c.Roll && s.Players[p].Jailed && c.CashAvailable >= 50
	if c.Roll && s.Players[p].Jailed {
		c.JailCards = append(c.JailCards, s.Players[p].JailCards...)
	}
	c.Bankrupt = c.Active && s.Debt != nil && s.Debt.Debtor == p && s.liquidation(p) < s.Debt.Amount
	if s.Auction != nil {
		c.MinBid = s.auctionMinimum()
		eligible := slices.Contains(s.auctionActors(), p)
		c.Bid = c.Active && eligible && s.Players[p].Cash >= c.MinBid
		c.AuctionPass = c.Active && eligible
		if s.buildingAuction() {
			c.BidTargets = s.buildTargets(p, s.Auction.Kind)
		}
	}
	for id, prop := range s.Properties {
		if prop.Owner != p {
			continue
		}
		x := PropertyControls{MortgageValue: board[id].Price / 2, RedeemCost: board[id].Price/2 + mortgageFee(id), BuildCost: board[id].BuildingCost, SellValue: board[id].BuildingCost / 2, BuildKind: s.buildKind(p, id)}
		allowed := c.Active && s.Phase != "mortgage_transfer"
		x.Mortgage = allowed && !prop.Mortgaged && s.groupBuildings(id) == 0 && s.preservesBuild(nil, id)
		x.Redeem = allowed && s.Phase != "debt" && prop.Mortgaged && c.CashAvailable >= x.RedeemCost
		if x.BuildKind != "" && (s.Phase == "roll" || s.Phase == "end" || s.Phase == "purchase") {
			stock := s.Houses
			if x.BuildKind == "hotel" {
				stock = s.Hotels
			}
			if stock == 1 {
				competitors := 0
				for i, player := range s.Players {
					if !player.Bankrupt && s.canFundBuild(i, x.BuildKind, 10) {
						competitors++
					}
				}
				x.BuildingAuction = competitors > 1
			}
			x.Build = c.Active && stock > 0 && ((x.BuildingAuction && s.canFundBuild(p, x.BuildKind, 10)) || (!x.BuildingAuction && c.CashAvailable >= x.BuildCost))
		}
		if prop.Buildings > 0 {
			even := true
			for _, i := range s.colorGroup(id) {
				if s.Properties[i].Buildings > prop.Buildings {
					even = false
				}
			}
			x.Sell = allowed && even && (prop.Buildings < 5 || s.Houses >= 4) && s.preservesBuild(map[int]int{id: prop.Buildings - 1}, -1)
		}
		clear := map[int]int{}
		complete := true
		for _, i := range s.colorGroup(id) {
			clear[i] = 0
			x.GroupSellValue += s.Properties[i].Buildings * board[i].BuildingCost / 2
			if s.Properties[i].Owner != p {
				complete = false
			}
		}
		x.SellGroup = allowed && complete && x.GroupSellValue > 0 && s.preservesBuild(clear, -1)
		c.Properties[id] = x
	}
	return c
}
