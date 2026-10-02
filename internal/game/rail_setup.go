package game

import (
	"errors"
	"fmt"
)

// UpgradeRailSetup preserves selections in games saved by the sequential setup version.
func (s *State) UpgradeRailSetup() bool {
	g := s.Rail
	if g == nil || !g.Setup || len(g.SetupPending) != 0 {
		return false
	}
	g.SetupPending = make([][]Ticket, len(g.Players))
	for i := range g.Players {
		if len(g.Players[i].Tickets) != 0 {
			continue
		}
		if i == s.Turn {
			g.SetupPending[i] = g.Pending
		} else {
			n := min(3, len(g.TicketDeck))
			g.SetupPending[i] = append([]Ticket{}, g.TicketDeck[:n]...)
			g.TicketDeck = g.TicketDeck[n:]
		}
	}
	g.Pending = nil
	s.Turn = 0
	return true
}

func (s *State) applyRailSetup(player int, a Action) error {
	g := s.Rail
	if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated {
		return errors.New("无效玩家")
	}
	pending := g.SetupPending[player]
	if a.Type != "keep" || len(pending) == 0 {
		return errors.New("请等待所有玩家选好初始目的地")
	}
	if len(a.Keep) < min(2, len(pending)) {
		return errors.New("初始目的地至少保留两张")
	}
	selected := map[int]bool{}
	for _, id := range a.Keep {
		if selected[id] {
			return errors.New("不能重复选择任务")
		}
		found := false
		for _, ticket := range pending {
			found = found || ticket.ID == id
		}
		if !found {
			return errors.New("无效的目的地任务")
		}
		selected[id] = true
	}
	for _, ticket := range pending {
		if selected[ticket.ID] {
			g.Players[player].Tickets = append(g.Players[player].Tickets, ticket)
		} else {
			g.TicketDeck = append(g.TicketDeck, ticket)
		}
	}
	g.SetupPending[player] = nil
	s.Log = append(s.Log, fmt.Sprintf("玩家 %d 保留了 %d 张初始目的地任务，放回 %d 张", player+1, len(a.Keep), len(pending)-len(a.Keep)))
	for _, tickets := range g.SetupPending {
		if len(tickets) != 0 {
			return nil
		}
	}
	g.Setup = false
	s.Turn, s.Phase = 0, "turn"
	return nil
}

func (s *State) AutoChooseRailSetup() {
	if s.Rail == nil || !s.Rail.Setup || s.Finished {
		return
	}
	for i, pending := range s.Rail.SetupPending {
		if len(pending) == 0 {
			continue
		}
		keep := []int{}
		for _, ticket := range pending[:min(2, len(pending))] {
			keep = append(keep, ticket.ID)
		}
		_ = s.applyRailSetup(i, Action{Type: "keep", Keep: keep})
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 选择超时，系统自动保留前两张任务", i+1))
	}
}

func (g *Rail) activePlayers() int {
	n := 0
	for _, p := range g.Players {
		if !p.Eliminated {
			n++
		}
	}
	return n
}

func (s *State) EliminateRail(player int) error {
	g := s.Rail
	if g == nil || g.Setup || s.Finished || player != s.Turn || player < 0 || player >= len(g.Players) || g.Players[player].Eliminated {
		return errors.New("无法移除此玩家")
	}
	p := &g.Players[player]
	p.Eliminated = true
	for c, n := range p.Hand {
		for range n {
			g.Discard = append(g.Discard, c)
		}
		p.Hand[c] = 0
	}
	// Claimed routes remain occupied; secret held tickets remain out of the draw pile.
	g.TicketDeck = append(g.TicketDeck, g.Pending...)
	g.Pending = nil
	g.Passes = 0
	g.refill()
	s.Log = append(s.Log, fmt.Sprintf("玩家 %d 超时被移出，列车牌归还弃牌堆，已铺铁路保留", player+1))
	if g.activePlayers() == 1 {
		s.railFinish()
		for i, p := range g.Players {
			if !p.Eliminated {
				s.Turn = i
			}
		}
		return nil
	}
	s.railNext()
	return nil
}
