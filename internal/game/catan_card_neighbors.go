package game

import "errors"

// Color is -1 until selected. Hands remain unchanged until all choices are
// ready, so nobody can pass on a newly received card. Persist this only on the
// server; catanView removes the entire private collection.
type CatanEventGift struct {
	From  int `json:"from"`
	To    int `json:"to"`
	Color int `json:"color"`
}

func (g *Catan) beginNeighborGifts(start int) {
	seats := []int{}
	for offset := range len(g.Players) {
		p := (start + offset) % len(g.Players)
		if !g.Players[p].Eliminated {
			seats = append(seats, p)
		}
	}
	if len(seats) < 2 {
		return
	}
	for i, p := range seats {
		if sum(g.Players[p].Resources) > 0 {
			// The next live seat in clockwise turn order is the left neighbor.
			g.CardEvent.Gifts = append(g.CardEvent.Gifts, CatanEventGift{From: p, To: seats[(i+1)%len(seats)], Color: -1})
			g.CardEvent.Players = append(g.CardEvent.Players, p)
		}
	}
}

func (g *Catan) chooseNeighborGift(player int, a Action) error {
	if a.Type != "catan_event_gift" || !g.cardBundle(a.Give) || sum(a.Give) != 1 || !catanHas(g.Players[player].Resources, a.Give) {
		return errors.New("请选择自己的一张资源或商品交给左邻")
	}
	for i := range g.CardEvent.Gifts {
		gift := &g.CardEvent.Gifts[i]
		if gift.From == player && gift.Color == -1 {
			for color, count := range a.Give {
				if count == 1 {
					gift.Color = color
					return nil
				}
			}
		}
	}
	return errors.New("缺少待选的好邻居交牌")
}

func (s *State) catanTransferNeighborGifts() error {
	g := s.Catan
	seen := map[int]bool{}
	// Check every outgoing card before changing any hand. In particular, an
	// incoming gift cannot make an invalid persisted outgoing gift valid.
	for _, gift := range g.CardEvent.Gifts {
		if gift.From < 0 || gift.From >= len(g.Players) || gift.To < 0 || gift.To >= len(g.Players) || gift.From == gift.To || seen[gift.From] || g.Players[gift.From].Eliminated || g.Players[gift.To].Eliminated {
			return errors.New("无效好邻居交牌座位")
		}
		if gift.Color < 0 || gift.Color >= len(g.Players[gift.From].Resources) || gift.Color >= len(g.Players[gift.To].Resources) || g.Players[gift.From].Resources[gift.Color] < 1 {
			return errors.New("缺少好邻居原有手牌")
		}
		seen[gift.From] = true
	}
	for _, gift := range g.CardEvent.Gifts {
		g.Players[gift.From].Resources[gift.Color]--
	}
	for _, gift := range g.CardEvent.Gifts {
		g.Players[gift.To].Resources[gift.Color]++
		s.catanLog(gift.From, "好邻居：向玩家%d交出1张牌", gift.To+1)
	}
	return nil
}
