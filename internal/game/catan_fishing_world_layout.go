package game

// New World places ports before fishing grounds. A coastal edge is a slot:
// a port uses one slot (no adjacent ports), a ground uses two consecutive
// concave slots. Both can touch each other but cannot share an edge.
// This planner uses only public geometry and already placed pieces, never
// their shuffled identities, and does not change an approved map.
type fishingWorldCoast struct {
	port   []bool // whether a NEW port can use each slot
	ground []bool // whether a NEW ground can use this slot and the next (cyclic)
}

func (g *Catan) fishingWorldCoastlines() ([]fishingWorldCoast, bool) {
	if g.Seafarers == nil || g.Seafarers.Scenario != "new_world" {
		return nil, false
	}
	coastal := map[int]bool{}
	atVertex := map[int][]int{}
	for i, e := range g.Edges {
		if e.ID != i || e.A < 0 || e.A >= len(g.Vertices) || e.B < 0 || e.B >= len(g.Vertices) || e.A == e.B {
			return nil, false
		}
		if g.edgeTerrain(e.ID, true) && g.edgeTerrain(e.ID, false) {
			coastal[e.ID] = true
			atVertex[e.A] = append(atVertex[e.A], e.ID)
			atVertex[e.B] = append(atVertex[e.B], e.ID)
		}
	}
	// On the hex board, land/water boundaries are disjoint closed loops.
	// A malformed topology is not treated as extra placement capacity.
	for _, edges := range atVertex {
		if len(edges) != 2 {
			return nil, false
		}
	}
	occupied, portBlocked := map[int]bool{}, map[int]bool{}
	for _, p := range g.Ports {
		if !coastal[p.Edge] || occupied[p.Edge] || portBlocked[p.Edge] {
			return nil, false
		}
		occupied[p.Edge] = true
		e := g.Edges[p.Edge]
		for _, vertex := range []int{e.A, e.B} {
			for _, edge := range atVertex[vertex] {
				portBlocked[edge] = true
			}
		}
	}
	possible := map[[2]int]bool{}
	for _, c := range g.fishingCoasts(g.findIslands()) {
		possible[c.Edges] = true
	}
	if g.Fishing != nil {
		for _, ground := range g.Fishing.Map.Grounds {
			if !possible[ground.Edges] || occupied[ground.Edges[0]] || occupied[ground.Edges[1]] {
				return nil, false
			}
			occupied[ground.Edges[0]], occupied[ground.Edges[1]] = true, true
		}
	}
	result, visited := []fishingWorldCoast{}, map[int]bool{}
	for _, first := range g.Edges {
		if !coastal[first.ID] || visited[first.ID] {
			continue
		}
		edges, edge, vertex := []int{}, first.ID, first.A
		for !visited[edge] {
			visited[edge] = true
			edges = append(edges, edge)
			e := g.Edges[edge]
			if e.A == vertex {
				vertex = e.B
			} else {
				vertex = e.A
			}
			next := atVertex[vertex]
			if next[0] == edge {
				edge = next[1]
			} else {
				edge = next[0]
			}
		}
		if edge != first.ID || vertex != first.A || len(edges) < 3 {
			return nil, false
		}
		coast := fishingWorldCoast{port: make([]bool, len(edges)), ground: make([]bool, len(edges))}
		for i, edge := range edges {
			next := edges[(i+1)%len(edges)]
			coast.port[i] = !occupied[edge] && !portBlocked[edge]
			coast.ground[i] = !occupied[edge] && !occupied[next] && possible[fishingEdgeKey([2]int{edge, next})]
		}
		result = append(result, coast)
	}
	return result, true
}

// Exact joint capacity, rather than separately counting enough ports and fish:
// the same coastal slots cannot satisfy both inventories. Counts refer only to
// unplaced pieces. The cap is the physical 5/6-player inventory, not a claim
// that the still-unverified 5/6 combination recipe is enabled.
func (g *Catan) fishingWorldCanFinish(ports, grounds int) bool {
	if ports < 0 || ports > 11 || grounds < 0 || grounds > 8 {
		return false
	}
	coasts, valid := g.fishingWorldCoastlines()
	if !valid {
		return false
	}
	width := grounds + 1
	possible := make([]bool, (ports+1)*width)
	possible[0] = true
	for _, coast := range coasts {
		local := coast.capacities(ports, grounds)
		next := make([]bool, len(possible))
		for a, ok := range possible {
			if !ok {
				continue
			}
			for b, fits := range local {
				if fits && a/width+b/width <= ports && a%width+b%width <= grounds {
					next[a+b] = true
				}
			}
		}
		possible = next
	}
	return possible[ports*width+grounds]
}

func (c fishingWorldCoast) capacities(ports, grounds int) []bool {
	width, n := grounds+1, len(c.port)
	result := make([]bool, (ports+1)*width)
	if n < 3 || len(c.ground) != n {
		return result
	}
	// Cut-crossing ground; port in slot zero; or neither (slot zero may be
	// unused or start a ground). These exhaust the cycle, including adjacency
	// across the cut.
	for mode := 0; mode < 3; mode++ {
		start, end, initialP, initialF, previous := 1, n, 0, 0, 0
		if mode == 1 {
			if !c.port[0] || ports == 0 {
				continue
			}
			initialP, previous = 1, 1
		} else if mode == 2 {
			if !c.ground[n-1] || grounds == 0 {
				continue
			}
			end, initialF = n-1, 1
		}
		// Scan from zero when it may start a ground, but forbid a port there.
		if mode == 0 {
			start = 0
		}
		size := len(result) * 2
		dp := make([][]bool, end+1)
		for i := range dp {
			dp[i] = make([]bool, size)
		}
		dp[start][(initialP*width+initialF)*2+previous] = true
		for at := start; at < end; at++ {
			for state, ok := range dp[at] {
				if !ok {
					continue
				}
				count, prev := state/2, state%2
				p, f := count/width, count%width
				dp[at+1][count*2] = true // leave this slot unused
				if c.port[at] && prev == 0 && p < ports && !(mode == 0 && at == 0) && !(mode == 1 && at == n-1) {
					dp[at+1][((p+1)*width+f)*2+1] = true
				}
				if at+1 < end && c.ground[at] && f < grounds {
					dp[at+2][(p*width+f+1)*2] = true
				}
			}
		}
		for state, ok := range dp[end] {
			if ok {
				result[state/2] = true
			}
		}
	}
	return result
}
