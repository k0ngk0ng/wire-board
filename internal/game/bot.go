package game

import (
	"errors"
	"sort"
)

// BotAction uses only public information and this seat's private cards. It never
// evaluates deck order or another player's hidden cards. Actions use the same
// rules as human players; choosing an action does not mutate the live game.
func (s *State) BotAction(player int) (Action, error) {
	if s.Finished {
		return Action{}, errors.New("game finished")
	}
	if s.Catan != nil {
		return s.catanBot(player)
	}
	if s.Splendor != nil {
		if player < 0 || player >= len(s.Splendor.Players) || player != s.Turn || s.Splendor.Players[player].Eliminated {
			return Action{}, errors.New("inactive bot seat")
		}
		return s.gemBot(player)
	}
	if s.Rail == nil || player < 0 || player >= len(s.Rail.Players) || s.Rail.Players[player].Eliminated || (!s.Rail.Setup && player != s.Turn) {
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

func gemMissing(p GemPlayer, c Card, tokens []int) int {
	need := 0
	for i, cost := range c.Cost {
		need += max(0, cost-p.Bonus[i]-tokens[i])
	}
	return max(0, need-tokens[5])
}
func (s *State) gemBot(player int) (Action, error) {
	g := s.Splendor
	p := g.Players[player]
	cards := append([]Card{}, p.Reserved...)
	for _, row := range g.Market {
		cards = append(cards, row...)
	}
	// A nearby affordable card gives the bot a consistent collection/discard goal.
	target := Card{Cost: make([]int, 5)}
	best := int(^uint(0) >> 1)
	for _, c := range cards {
		cost := 0
		for i, n := range c.Cost {
			cost += max(0, n-p.Bonus[i])
		}
		rank := gemMissing(p, c, p.Tokens)*12 + cost - c.Points*2
		if cost > 10 {
			rank += 100
		}
		if rank < best {
			best = rank
			target = c
		}
	}
	if s.Phase == "discard" {
		held := append([]int{}, p.Tokens...)
		give := make([]int, 6)
		for sum(held) > 10 {
			color, priority := -1, -999
			for i, n := range held {
				if n == 0 {
					continue
				}
				score := n * 2
				if i == 5 {
					score -= 100
				} else {
					score -= max(0, target.Cost[i]-p.Bonus[i]) * 5
				}
				if score > priority {
					color, priority = i, score
				}
			}
			held[color]--
			give[color]++
		}
		return s.botLegal(player, []botChoice{{Action{Type: "discard", Tokens: give}, 0}})
	}
	if s.Phase == "noble" {
		choices := []botChoice{}
		for _, n := range g.Nobles {
			if eligible(&p, n) {
				choices = append(choices, botChoice{Action{Type: "noble", Noble: n.ID}, 0})
			}
		}
		return s.botLegal(player, choices)
	}
	choices := []botChoice{}
	for _, c := range cards {
		if gemMissing(p, c, p.Tokens) == 0 {
			score := 10000 + c.Points*100 + sum(c.Cost) - p.Bonus[c.Color]*3
			choices = append(choices, botChoice{Action{Type: "buy", Card: c.ID}, score})
		}
	}
	// Enumerate the small set of legal take patterns, preferring useful colors.
	available := 0
	for i := 0; i < 5; i++ {
		if g.Bank[i] > 0 {
			available++
		}
	}
	for mask := 1; mask < 32; mask++ {
		take := make([]int, 6)
		for i := 0; i < 5; i++ {
			if mask&(1<<i) != 0 {
				take[i] = 1
			}
		}
		if sum(take) == min(3, available) {
			valid := true
			for i, n := range take {
				if n > g.Bank[i] {
					valid = false
				}
			}
			if valid {
				choices = append(choices, botChoice{Action{Type: "take", Tokens: take}, gemTakeScore(p, target, take)})
			}
		}
	}
	for i := 0; i < 5; i++ {
		if g.Bank[i] >= 4 {
			take := make([]int, 6)
			take[i] = 2
			choices = append(choices, botChoice{Action{Type: "take", Tokens: take}, gemTakeScore(p, target, take)})
		}
	}
	if len(p.Reserved) < 3 {
		for _, row := range g.Market {
			for _, c := range row {
				score := -50 - gemMissing(p, c, p.Tokens)*5
				if g.Bank[5] > 0 {
					score += 80
				}
				if c.ID == target.ID {
					score += 30
				}
				choices = append(choices, botChoice{Action{Type: "reserve", Card: c.ID}, score})
			}
		}
		for i, deck := range g.Decks {
			if len(deck) > 0 {
				choices = append(choices, botChoice{Action{Type: "reserve", Tier: i + 1}, -500})
			}
		}
	}
	choices = append(choices, botChoice{Action{Type: "pass"}, -1000})
	return s.botLegal(player, choices)
}
func gemTakeScore(p GemPlayer, target Card, take []int) int {
	held := append([]int{}, p.Tokens...)
	useful := 0
	for i, n := range take {
		held[i] += n
		if i < 5 {
			useful += min(n, max(0, target.Cost[i]-p.Bonus[i]-p.Tokens[i]))
		}
	}
	return 100 + useful*40 - gemMissing(p, target, held)*15 - max(0, sum(held)-10)*20
}

func (s *State) botRouteOpen(r Route, player int) bool {
	g := s.Rail
	if _, ok := g.Owners[r.ID]; ok {
		return false
	}
	for _, other := range baseRailData.Routes {
		if (other.A == r.A && other.B == r.B) || (other.A == r.B && other.B == r.A) {
			if owner, ok := g.Owners[other.ID]; ok && (len(g.Players) < 4 || owner == player) {
				return false
			}
		}
	}
	return true
}

// Shortest remaining distance uses owned tracks for free and excludes blocked tracks.
func (s *State) botPath(player int, ticket Ticket) (int, []int) {
	data := baseRailData
	n := len(data.Cities)
	dist := make([]int, n)
	visited := make([]bool, n)
	prev := make([]Route, n)
	for i := range dist {
		dist[i] = 10000
	}
	dist[ticket.A] = 0
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
		if at == ticket.B {
			break
		}
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
			cost := r.Length
			if owner, ok := s.Rail.Owners[r.ID]; ok {
				if owner != player {
					continue
				}
				cost = 0
			} else if !s.botRouteOpen(r, player) {
				continue
			}
			if dist[at]+cost < dist[next] {
				dist[next] = dist[at] + cost
				prev[next] = r
			}
		}
	}
	path := []int{}
	if dist[ticket.B] < 10000 {
		for at := ticket.B; at != ticket.A; {
			r := prev[at]
			if r.ID == 0 {
				break
			}
			path = append(path, r.ID)
			if r.A == at {
				at = r.B
			} else {
				at = r.A
			}
		}
	}
	return dist[ticket.B], path
}
func (s *State) railBot(player int) (Action, error) {
	g := s.Rail
	p := g.Players[player]
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
	for _, r := range baseRailData.Routes {
		if r.Length > p.Trains || !s.botRouteOpen(r, player) {
			continue
		}
		for c := 0; c < 8; c++ {
			if r.Color >= 0 && r.Color != c {
				continue
			}
			missing := max(0, r.Length-p.Hand[c]-p.Hand[8])
			if missing == 0 && g.Drawn == 0 {
				choices = append(choices, botChoice{Action{Type: "claim", Route: r.ID, Color: c, Wild: max(0, r.Length-p.Hand[c])}, 1000 + priorities[r.ID] + r.Length*5})
			}
			if missing > 0 {
				wanted[c] = max(wanted[c], priorities[r.ID]+r.Length*3-missing*2)
			}
		}
	}
	for i, c := range g.Face {
		if c < 0 || (c == 8 && g.Drawn > 0) {
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
