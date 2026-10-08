package game

import "slices"

// This bounded, public-geometry-only search advises the bot; it does not change
// human placement legality. A failed search is not proof of an impossible
// opening: callers keep their ordinary legal-choice fallback.
func (s catanExplorerSetup) botOpening(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, budget int) (int, bool) {
	if !s.CitiesKnights || s.current(len(g.Players)) == nil {
		return 0, false
	}
	// Copy only geometry that search mutates, never the private decks or economy.
	board, fleet := *g, *f
	board.Vertices, board.Edges = slices.Clone(g.Vertices), slices.Clone(g.Edges)
	fleet.Positions = slices.Clone(f.Positions)
	s.Settlements, s.Harbors = slices.Clone(s.Settlements), slices.Clone(s.Harbors)
	plan := s.plan(len(g.Players))
	adjacent := make([][]int, len(g.Vertices))
	for _, edge := range g.Edges {
		adjacent[edge.A] = append(adjacent[edge.A], edge.B)
		adjacent[edge.B] = append(adjacent[edge.B], edge.A)
	}
	free := func(v int) bool {
		if board.Vertices[v].Level != 0 {
			return false
		}
		for _, next := range adjacent[v] {
			if board.Vertices[next].Level != 0 {
				return false
			}
		}
		return true
	}
	var search func() (int, bool)
	search = func() (int, bool) {
		if s.Step == len(plan) {
			return 0, true
		}
		if budget <= 0 {
			return 0, false
		}
		budget--
		// A necessary (not sufficient) condition avoids exploring branches where
		// cities already leave fewer green points than remaining harbors.
		remaining := 0
		for _, step := range plan[s.Step:] {
			if step.Kind == "harbor" {
				remaining++
			}
		}
		available := []int{}
		for _, v := range b.HarborStarts {
			if free(v) {
				available = append(available, v)
			}
		}
		if len(available) < remaining {
			return 0, false
		}
		step := plan[s.Step]
		options := s.choices(&board, b, &fleet)
		if step.Kind == "city" {
			// Try cities that preserve the coast first; stable ties use vertex ID.
			blocked := func(v int) int {
				n := 0
				for _, h := range available {
					if h == v || slices.Contains(adjacent[v], h) {
						n++
					}
				}
				return n
			}
			slices.SortStableFunc(options, func(a, b int) int { return blocked(a) - blocked(b) })
		}
		for _, target := range options {
			if budget <= 0 {
				break
			}
			var undo func()
			switch step.Kind {
			case "city", "harbor", "settlement":
				old := board.Vertices[target]
				locations := s.Settlements
				if step.Kind == "harbor" {
					locations = s.Harbors
				}
				index := catanExplorerSetupIndex(len(g.Players), step.Owner)
				oldLocation := locations[index]
				board.Vertices[target].Owner, board.Vertices[target].Level = step.Owner, 2
				if step.Kind == "settlement" {
					board.Vertices[target].Level = 1
				}
				board.Vertices[target].Harbor = step.Kind == "harbor"
				locations[index] = target
				undo = func() { board.Vertices[target] = old; locations[index] = oldLocation }
			case "road":
				old := board.Edges[target].Owner
				board.Edges[target].Owner = step.Owner
				undo = func() { board.Edges[target].Owner = old }
			case "ship":
				id := step.Owner * 3
				old := fleet.Positions[id]
				fleet.Positions[id] = target
				undo = func() { fleet.Positions[id] = old }
			default:
				return 0, false
			}
			s.Step++
			_, ok := search()
			s.Step--
			undo()
			if ok {
				return target, true
			}
		}
		return 0, false
	}
	return search()
}
