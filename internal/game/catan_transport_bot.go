package game

import (
	"errors"
	"slices"
)

// A path uses only the public graph. Passing through any other commodity
// center would end movement, so it cannot be used as a shortcut.
func catanTransportPath(g *Catan, m *catanTransportMap, from, to, player, gold int) []int {
	// A shortest useful path never repeats a vertex, even with a large ledger.
	gold = min(gold, len(g.Vertices)-1)
	type key struct{ vertex, paid int }
	type step struct {
		previous key
		edge     int
	}
	start := key{from, 0}
	queue := []key{start}
	parent := map[key]step{start: {start, -1}}
	var finish key
	found := false
	for len(queue) > 0 {
		here := queue[0]
		queue = queue[1:]
		if here.vertex == to {
			finish = here
			found = true
			break
		}
		for _, e := range g.Edges {
			next := -1
			if e.A == here.vertex {
				next = e.B
			} else if e.B == here.vertex {
				next = e.A
			}
			if next < 0 || next != to && m.siteAt(next) >= 0 {
				continue
			}
			paid := here.paid
			if e.Owner != -1 && e.Owner != player {
				paid++
			}
			if paid > gold {
				continue
			}
			there := key{next, paid}
			if _, seen := parent[there]; seen {
				continue
			}
			parent[there] = step{here, e.ID}
			queue = append(queue, there)
		}
	}
	if !found {
		return nil
	}
	reverse := []int{}
	for at := finish; at != start; at = parent[at].previous {
		reverse = append(reverse, parent[at].edge)
	}
	path := []int{}
	for i := len(reverse) - 1; i >= 0; i-- {
		path = append(path, reverse[i])
	}
	return path
}

func (s *State) catanTransportBot(player int) (Action, bool, error) {
	g := s.Catan
	t := g.Transport
	if player != s.Turn {
		return Action{}, false, nil
	}
	if s.Phase == "catan_transport_barbarian" {
		best := -1
		score := -1000
		for _, e := range g.Edges {
			occupied := false
			for _, id := range t.Barbarians {
				occupied = occupied || id == e.ID
			}
			if occupied {
				continue
			}
			value := 0
			if e.Owner == player {
				value = -10
			} else if e.Owner >= 0 && !g.Players[e.Owner].Eliminated {
				value = 10 + sum(g.Players[e.Owner].Resources)
			}
			if value > score {
				best, score = e.ID, value
			}
		}
		if best < 0 {
			return Action{}, true, errors.New("没有可移动蛮族的边")
		}
		return Action{Type: "catan_transport_barbarian", Offer: int(t.BarbarianSequence), Card: 0, Edge: best}, true, nil
	}
	if s.Phase == "catan_transport_move" {
		q := t.Travel
		base := Action{Offer: int(t.Sequence)}
		if q.Pending >= 0 {
			if g.attackTransport() {
				for _, tile := range g.attackBattleTiles() {
					for _, edge := range g.AttackTransport.Pieces.edges(g, tile, q.Pending) {
						p := clone(g.AttackTransport.Pieces)
						if p.relocate(g, g.attackTransportBoard(), q.Pending, tile, edge) == nil {
							base.Type = "catan_transport_relocate"
							base.Tile = tile
							base.Edge = edge
							return base, true, nil
						}
					}
				}
				return base, true, errors.New("无法重新放置共享蛮族")
			}
			for _, e := range g.Edges {
				ok := true
				for _, id := range t.Barbarians {
					ok = ok && id != e.ID
				}
				if ok {
					base.Type = "catan_transport_relocate"
					base.Edge = e.ID
					return base, true, nil
				}
			}
			return base, true, errors.New("无法重新放置蛮族")
		}
		if q.Arrived >= 0 && !t.ArrivalResolved {
			base.Type = "catan_transport_arrival"
			base.Choice = "keep"
			if t.canDeliver() {
				base.Choice = "deliver"
			}
			return base, true, nil
		}
		// The public projection deliberately excludes hidden supply order and
		// delivered-token identities. Only own wheat/gold is used for payments.
		public := t.publicView()
		w := public.Wagons[player]
		var path []int
		for i, site := range t.Map.Sites {
			if site.Center == w.Position {
				continue
			}
			if w.Cargo != nil {
				if !t.Map.accepts(i, w.Cargo.Cargo) {
					continue
				}
			} else if public.Supply[catanTransportOriginIndex(site.Kind)] == 0 {
				continue
			}
			candidate := catanTransportPath(g, t.Map, w.Position, site.Center, player, t.Gold[player])
			if len(candidate) > 0 && (path == nil || len(candidate) < len(path)) {
				path = candidate
			}
		}
		if len(path) > 0 {
			edge := path[0]
			edges := t.Barbarians[:]
			attempted := q.Attempted[:]
			if g.attackTransport() {
				edges = g.AttackTransport.Pieces.blockingEdges()
				attempted = g.AttackTransport.Attempted
			}
			for piece, id := range edges {
				if id == edge && q.Level > 0 && !attempted[piece] {
					base.Type = "catan_transport_drive"
					base.Card = piece
					return base, true, nil
				}
			}
			if _, err := t.quoteTravel(g, *q, edge); err == nil {
				base.Type = "catan_transport_step"
				base.Edge = edge
				return base, true, nil
			}
			if !q.WheatUsed && g.fishingTransport() {
				if ids := g.fishPayment(player, g.fishActionCost(player, "catan_transport_fish")); ids != nil {
					copy := *q
					copy.WheatUsed = true
					copy.FishUsed = true
					copy.Points += 2
					if _, err := t.quoteTravel(g, copy, edge); err == nil {
						base.Type = "catan_transport_fish"
						base.Tokens = ids
						return base, true, nil
					}
				}
			}
			if !q.WheatUsed && g.Players[player].Resources[3] > 0 {
				copy := *q
				copy.WheatUsed = true
				copy.Points += 2
				if _, err := t.quoteTravel(g, copy, edge); err == nil {
					base.Type = "catan_transport_wheat"
					return base, true, nil
				}
			}
		}
		base.Type = "catan_transport_stop"
		return base, true, nil
	}
	if s.Phase == "catan_turn" {
		if g.transportKnights() && g.CitiesKnights.BarbarianPosition < catanBarbarianDistance-2 {
			for _, knight := range g.CitiesKnights.Knights {
				if knight.Owner != player {
					continue
				}
				pieces := g.transportKnightBarbarians(&knight)
				if len(pieces) == 0 {
					continue
				}
				for _, e := range g.Edges {
					if e.Owner >= 0 && e.Owner != player && !g.Players[e.Owner].Eliminated && sum(g.Players[e.Owner].Resources) > 0 && !slices.Contains(t.Barbarians[:], e.ID) {
						return Action{Type: "catan_transport_knight_chase", Vertex: knight.Vertex, Card: pieces[0], Edge: e.ID}, true, nil
					}
				}
			}
		}
		level := t.Wagons[player].Level
		if cost := catanTransportUpgradeCost(level); cost != nil && catanHas(g.Players[player].Resources, cost) {
			return Action{Type: "catan_transport_upgrade"}, true, nil
		}
		// Preserve two coins for tolls instead of spending the wagon's entire travel budget.
		if t.Bought < 2 && t.Gold[player] >= 4 {
			choice := g.catanResourceChoiceBot(player, 1)
			for c, count := range choice {
				if count > 0 && g.Bank[c] > 0 {
					return Action{Type: "catan_coin_buy", Color: c}, true, nil
				}
			}
		}
	}
	return Action{}, false, nil
}
