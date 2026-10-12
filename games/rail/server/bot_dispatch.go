package rail

import (
	"errors"
	"sort"
)

func (s *State) BotAction(player int) (Action, error) {
	if s.Finished {
		return Action{}, errors.New("game finished")
	}
	if player < 0 || player >= len(s.Rail.Players) || s.Rail.Players[player].Eliminated || (!s.Rail.Setup && player != s.Turn) {
		return Action{}, errors.New("inactive bot seat")
	}
	return s.railBot(player)
}

type botChoice struct {
	action Action
	score  int
}

func (s *State) botLegal(player int, choices []botChoice) (Action, error) {
	sort.SliceStable(choices, func(i, j int) bool { return choices[i].score > choices[j].score })
	for _, c := range choices {
		trial := clone(*s)
		if trial.Apply(player, c.action) == nil {
			return c.action, nil
		}
	}
	return Action{}, errors.New("no legal bot action")
}

func (s *State) botRouteOpen(r Route, player int) bool { return s.Rail.routeOpen(r, player) }

// Multi-source shortest paths keep distinct Swiss border crossings separate.
func (s *State) botPath(player int, ticket Ticket) (int, []int) {
	g := s.Rail
	data := g.data()
	n := len(data.Cities)
	dist, previous, via := make([]int, n), make([]int, n), make([]int, n)
	visited := make([]bool, n)
	for i := range dist {
		dist[i] = 10000
		previous[i] = -1
	}
	for _, start := range g.terminals(ticket.A) {
		dist[start] = 0
	}
	targets := g.terminals(ticket.B)
	if len(ticket.Options) > 0 {
		targets = nil
		for _, o := range ticket.Options {
			targets = append(targets, g.terminals(o.To)...)
		}
	}
	for range n {
		at := -1
		for i := range dist {
			if !visited[i] && (at < 0 || dist[i] < dist[at]) {
				at = i
			}
		}
		if at < 0 || dist[at] >= 10000 {
			break
		}
		visited[at] = true
		for _, r := range data.Routes {
			next := -1
			if r.A == at {
				next = r.B
			} else if r.B == at {
				next = r.A
			}
			if next < 0 || visited[next] {
				continue
			}
			cost := r.Length + r.Mountain
			if owner, ok := g.Owners[r.ID]; ok {
				if owner != player {
					continue
				}
				cost = 0
			} else if !g.routeOpen(r, player) {
				continue
			}
			if dist[at]+cost < dist[next] {
				dist[next] = dist[at] + cost
				previous[next] = at
				via[next] = r.ID
			}
		}
	}
	best := -1
	for _, end := range targets {
		if best < 0 || dist[end] < dist[best] {
			best = end
		}
	}
	if best < 0 || dist[best] >= 10000 {
		return 10000, nil
	}
	path := []int{}
	for at := best; previous[at] >= 0; at = previous[at] {
		path = append(path, via[at])
	}
	return dist[best], path
}
func (s *State) railBot(player int) (Action, error) {
	g := s.Rail
	p := g.Players[player]
	if t := g.Tunnel; t != nil {
		pay := make([]int, 9)
		if !t.WildOnly {
			pay[t.Color] = min(t.Extra, p.Hand[t.Color])
		}
		pay[8] = t.Extra - pay[t.Color]
		return s.botLegal(player, []botChoice{
			{Action{Type: "tunnel_pay", Tokens: pay}, 10},
			{Action{Type: "tunnel_cancel"}, 0},
		})
	}
	if g.Setup || s.Phase == "tickets" {
		pending := g.Pending
		minimum := 1
		if g.Setup {
			pending = g.SetupPending[player]
			minimum = 2
		}
		tickets := append([]Ticket{}, pending...)
		if len(tickets) == 0 {
			return Action{}, errors.New("bot already selected destinations")
		}
		sort.SliceStable(tickets, func(i, j int) bool {
			a, _ := s.botPath(player, tickets[i])
			b, _ := s.botPath(player, tickets[j])
			return a*3-tickets[i].Points < b*3-tickets[j].Points
		})
		keep := []int{}
		for i, t := range tickets {
			distance, _ := s.botPath(player, t)
			if i < minimum || distance == 0 {
				keep = append(keep, t.ID)
			}
		}
		return s.botLegal(player, []botChoice{{Action{Type: "keep", Keep: keep}, 0}})
	}
	priorities := map[int]int{}
	for _, t := range p.Tickets {
		distance, path := s.botPath(player, t)
		if distance > 0 && distance < 10000 {
			for _, id := range path {
				priorities[id] += 30 + t.Points
			}
		}
	}
	choices := []botChoice{}
	wanted := make([]int, 8)
	for _, r := range g.data().Routes {
		if r.Length+r.Mountain > p.Trains || !s.botRouteOpen(r, player) {
			continue
		}
		if g.Drawn == 0 {
			for _, pay := range g.paymentOptions(player, r) {
				score := 1000 + priorities[r.ID] + railRoutePoints(r.Length)*3 + 2*r.Mountain - sum(pay.Tokens)
				if r.Tunnel {
					score -= 15
				}
				if g.info().Bonus == "network" {
					groups := g.components(player, nil)
					if groups[r.A] != groups[r.B] {
						score += 12
					}
				}
				if g.Map == "india" && g.connected(player, r.A, r.B) {
					score += 10
				}
				choices = append(choices, botChoice{Action{Type: "claim", Route: r.ID, Color: pay.Color, Tokens: pay.Tokens}, score})
			}
		}
		for c := 0; c < 8; c++ {
			if r.Color >= 0 && r.Color != c {
				continue
			}
			wild := 0
			if g.wildAllowed(r) {
				wild = p.Hand[8]
			}
			missing := max(0, r.Length-p.Hand[c]-wild)
			if missing > 0 {
				wanted[c] = max(wanted[c], priorities[r.ID]+r.Length*3-missing*2)
			}
		}
	}
	if g.Drawn == 0 && len(p.Stations) < g.info().Stations {
		_, before, _, _ := g.evaluateTickets(player)
		occupied := map[int]bool{}
		for _, other := range g.Players {
			for _, city := range other.Stations {
				occupied[city] = true
			}
		}
		cost := len(p.Stations) + 1
		for _, city := range g.data().Cities {
			if occupied[city.ID] {
				continue
			}
			// Evaluate a temporary station using only our own tickets and public tracks.
			g.Players[player].Stations = append(append([]int{}, p.Stations...), city.ID)
			_, after, _, _ := g.evaluateTickets(player)
			g.Players[player].Stations = p.Stations
			if after-before <= 4 {
				continue
			}
			for c := 0; c < 8; c++ {
				wild := max(0, cost-p.Hand[c])
				if wild <= p.Hand[8] {
					choices = append(choices, botChoice{Action{Type: "station", Vertex: city.ID, Color: c, Wild: wild}, 1050 + (after-before)*3})
				}
			}
		}
	}
	for i, c := range g.Face {
		if c < 0 || (c == 8 && g.Drawn > 0 && !g.info().WildSingle) {
			continue
		}
		score := 10
		if c == 8 {
			score = 15
		} else {
			score += wanted[c]
		}
		choices = append(choices, botChoice{Action{Type: "draw", Slot: i}, score})
	}
	if len(g.Deck)+len(g.Discard) > 0 {
		choices = append(choices, botChoice{Action{Type: "draw", Slot: -1}, 5})
	}
	if g.Drawn == 0 && len(g.TicketDeck) > 0 {
		choices = append(choices, botChoice{Action{Type: "tickets"}, -100})
	}
	choices = append(choices, botChoice{Action{Type: "pass"}, -1000})
	return s.botLegal(player, choices)
}
