package game

// A separate vertex for every border crossing prevents a Swiss country from
// acting as an unbuilt shortcut between two different points on its border.
func (g *Rail) terminals(city int) []int {
	data := g.data()
	if city < 0 || city >= len(data.Cities) {
		return nil
	}
	c := data.Cities[city]
	if c.Kind != "country" {
		return []int{city}
	}
	out := []int{}
	for _, v := range data.Cities {
		if v.Kind == "border" && v.Country == c.Country {
			out = append(out, v.ID)
		}
	}
	return out
}
func (g *Rail) components(player int, borrowed []int) []int {
	groups := make([]int, len(g.data().Cities))
	for i := range groups {
		groups[i] = i
	}
	root := func(v int) int {
		for groups[v] != v {
			v = groups[v]
		}
		return v
	}
	borrow := map[int]bool{}
	for _, id := range borrowed {
		borrow[id] = true
	}
	for _, r := range g.data().Routes {
		owner, ok := g.Owners[r.ID]
		if (ok && owner == player) || borrow[r.ID] {
			groups[root(r.A)] = root(r.B)
		}
	}
	for i := range groups {
		groups[i] = root(i)
	}
	return groups
}
func (g *Rail) connectedIn(groups []int, a, b int) bool {
	for _, from := range g.terminals(a) {
		for _, to := range g.terminals(b) {
			if groups[from] == groups[to] {
				return true
			}
		}
	}
	return false
}
func (g *Rail) connected(player, a, b int) bool {
	return g.connectedIn(g.components(player, nil), a, b)
}
func (g *Rail) ticketIn(groups []int, t Ticket) Ticket {
	t.Complete = false
	t.Value = -t.Points
	if len(t.Options) == 0 {
		t.Complete = g.connectedIn(groups, t.A, t.B)
		if t.Complete {
			t.Value = t.Points
		}
		return t
	}
	// Country tickets score the best completed alternative, otherwise lose the
	// smallest value. The alternatives on one card never score independently.
	for _, o := range t.Options {
		if g.connectedIn(groups, t.A, o.To) && (!t.Complete || o.Points > t.Value) {
			t.Complete = true
			t.Value = o.Points
		}
	}
	return t
}
func (g *Rail) evaluateTickets(player int) ([]Ticket, int, int, []int) {
	p := g.Players[player]
	choices := make([][]int, len(p.Stations))
	for i, city := range p.Stations {
		choices[i] = []int{0}
		for _, r := range g.data().Routes {
			if r.A != city && r.B != city {
				continue
			}
			if owner, ok := g.Owners[r.ID]; ok && owner != player {
				choices[i] = append(choices[i], r.ID)
			}
		}
	}
	bestScore, bestComplete := -100000, -1
	var bestTickets []Ticket
	var bestRoutes []int
	var visit func(int, []int)
	visit = func(at int, borrowed []int) {
		if at < len(choices) {
			for _, id := range choices[at] {
				visit(at+1, append(borrowed, id))
			}
			return
		}
		groups := g.components(player, borrowed)
		score, complete := 0, 0
		tickets := make([]Ticket, len(p.Tickets))
		for i, t := range p.Tickets {
			tickets[i] = g.ticketIn(groups, t)
			score += tickets[i].Value
			if tickets[i].Complete {
				complete++
			}
		}
		if score > bestScore || (score == bestScore && complete > bestComplete) {
			bestScore, bestComplete = score, complete
			bestTickets = tickets
			bestRoutes = append([]int{}, borrowed...)
		}
	}
	visit(0, nil)
	return bestTickets, bestScore, bestComplete, bestRoutes
}

// Remove bridges from the player's graph. Vertices in the same remaining
// component admit two edge-disjoint paths, including paths sharing cities.
func (g *Rail) mandalaComponents(player int) []int {
	type edge struct{ to, id int }
	adj := make([][]edge, len(g.data().Cities))
	for _, r := range g.data().Routes {
		if owner, ok := g.Owners[r.ID]; ok && owner == player {
			adj[r.A] = append(adj[r.A], edge{r.B, r.ID})
			adj[r.B] = append(adj[r.B], edge{r.A, r.ID})
		}
	}
	entered, low := make([]int, len(adj)), make([]int, len(adj))
	clock := 0
	bridges := map[int]bool{}
	var dfs func(int, int)
	dfs = func(v, parent int) {
		clock++
		entered[v] = clock
		low[v] = clock
		for _, e := range adj[v] {
			if e.id == parent {
				continue
			}
			if entered[e.to] == 0 {
				dfs(e.to, e.id)
				low[v] = min(low[v], low[e.to])
				if low[e.to] > entered[v] {
					bridges[e.id] = true
				}
			} else {
				low[v] = min(low[v], entered[e.to])
			}
		}
	}
	for v := range adj {
		if entered[v] == 0 {
			dfs(v, -1)
		}
	}
	groups := make([]int, len(adj))
	for i := range groups {
		groups[i] = -1
	}
	for start := range adj {
		if groups[start] >= 0 {
			continue
		}
		groups[start] = start
		queue := []int{start}
		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			for _, e := range adj[v] {
				if !bridges[e.id] && groups[e.to] < 0 {
					groups[e.to] = start
					queue = append(queue, e.to)
				}
			}
		}
	}
	return groups
}
func (g *Rail) largestNetwork(player int) int {
	groups := g.components(player, nil)
	used := map[int]bool{}
	for _, r := range g.data().Routes {
		if owner, ok := g.Owners[r.ID]; ok && owner == player {
			used[r.A] = true
			used[r.B] = true
		}
	}
	sizes := map[int]int{}
	best := 0
	for v := range used {
		sizes[groups[v]]++
		best = max(best, sizes[groups[v]])
	}
	return best
}

// Tie breakers are shared by the game result and the rating ledger.
func (g *Rail) TieBreak(player int) (int, int, int) {
	p := g.Players[player]
	switch g.info().ID {
	case "europe":
		return p.Completed, g.info().Stations - len(p.Stations), p.Bonus
	case "legendaryasia":
		return p.Completed, p.MountainRoutes, 0
	case "nordiccountries":
		return p.Completed, 0, 0
	default:
		return p.Completed, p.Bonus, 0
	}
}
func (s *State) railFinish() {
	g := s.Rail
	bestMetric := 0
	for i := range g.Players {
		p := &g.Players[i]
		if p.Eliminated {
			continue
		}
		p.Tickets, p.TicketScore, p.Completed, p.StationRoutes = g.evaluateTickets(i)
		p.Longest = longestRailRoutes(g.data().Routes, g.Owners, i)
		p.Network = g.largestNetwork(i)
		p.StationScore = 4 * (g.info().Stations - len(p.Stations))
		p.MandalaCount, p.MandalaScore, p.Bonus = 0, 0, 0
		if g.Map == "india" {
			components := g.mandalaComponents(i)
			for j, t := range p.Tickets {
				if t.Complete && t.A != t.B && components[t.A] == components[t.B] {
					p.Tickets[j].Mandala = true
					p.MandalaCount++
				}
			}
			p.MandalaScore = [...]int{0, 5, 10, 20, 30, 40}[min(5, p.MandalaCount)]
		}
		metric := p.Longest
		if g.info().Bonus == "tickets" {
			metric = p.Completed
		} else if g.info().Bonus == "network" {
			metric = p.Network
		}
		bestMetric = max(bestMetric, metric)
	}
	best := -1
	s.Winners = nil
	for i := range g.Players {
		p := &g.Players[i]
		if p.Eliminated {
			continue
		}
		metric := p.Longest
		if g.info().Bonus == "tickets" {
			metric = p.Completed
		} else if g.info().Bonus == "network" {
			metric = p.Network
		}
		if metric == bestMetric && (metric > 0 || g.info().Bonus == "tickets") {
			p.Bonus = 10
		}
		p.Score = p.RouteScore + p.TicketScore + p.StationScore + p.MandalaScore + p.Bonus
		comparison := 1
		if best >= 0 {
			a, b, c := g.TieBreak(i)
			x, y, z := g.TieBreak(best)
			comparison = 0
			for _, pair := range [][2]int{{p.Score, g.Players[best].Score}, {a, x}, {b, y}, {c, z}} {
				if pair[0] > pair[1] {
					comparison = 1
					break
				}
				if pair[0] < pair[1] {
					comparison = -1
					break
				}
			}
		}
		if comparison > 0 {
			best = i
			s.Winners = []int{i}
		} else if comparison == 0 {
			s.Winners = append(s.Winners, i)
		}
	}
	s.Finished = true
	s.Phase = "finished"
}
