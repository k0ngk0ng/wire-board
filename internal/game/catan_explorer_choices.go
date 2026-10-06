package game

import "slices"

// These are actor-only previews. Construction and movement probes use public
// tiles and one's own resources, never Board.Hidden/Numbers or discovery awards.
// In particular, a depleted reward bank must not make a fog destination vanish
// depending on the secret terrain beneath it. Apply remains authoritative.
func (s *State) catanExplorerChoices(viewer int) []Action {
	result := []Action{}
	g := s.Catan
	if g == nil || g.Explorer == nil || s.Finished || viewer < 0 || viewer >= len(g.Players) || g.Players[viewer].Eliminated || viewer != s.Turn {
		return result
	}
	if actions, handled := s.catanExplorerSpecialChoices(viewer); handled {
		return actions
	}
	sequence := g.TurnSerial
	add := func(a Action) { a.Prompt = int(sequence); result = append(result, a) }
	if s.Phase == "catan_roll" {
		add(Action{Type: "catan_roll"})
		return result
	}
	if s.Phase != "catan_turn" && s.Phase != "catan_explorer_move" {
		return result
	}
	x := g.Explorer
	// Every probe owns its mutable pieces. Immutable board topology is shared;
	// neither public tiles nor any private hidden board are ever modified here.
	probe := func(a Action) bool {
		base := *g
		base.Explorer = nil
		base.Players = slices.Clone(g.Players)
		for i := range base.Players {
			base.Players[i].Resources = slices.Clone(g.Players[i].Resources)
		}
		base.Bank = slices.Clone(g.Bank)
		base.Vertices = slices.Clone(g.Vertices)
		base.Edges = slices.Clone(g.Edges)
		cargo, fleet, economy := clone(*x.Cargo), clone(*x.Fleet), clone(*x.Economy)
		switch a.Type {
		case "catan_road":
			return cargo.buildRoad(&base, &fleet, viewer, sequence, a.Edge) == nil
		case "catan_settlement":
			return cargo.buildSettlement(&base, &fleet, viewer, sequence, a.Vertex) == nil
		case "catan_explorer_harbor":
			return cargo.buildHarbor(&base, &fleet, viewer, sequence, a.Vertex) == nil
		case "catan_explorer_ship":
			return cargo.buildShip(&base, &fleet, viewer, sequence, a.Slot, a.Edge) == nil
		case "catan_explorer_unit":
			return cargo.buildUnit(&base, &fleet, viewer, sequence, a.Card, catanExplorerCargoLocation{a.Choice, a.Target}, a.Cards) == nil
		case "catan_explorer_bank":
			return economy.bankTrade(&base, &fleet, &cargo, viewer, sequence, a.Color, a.Target) == nil
		case "catan_explorer_wool":
			return fleet.wool(&base, viewer, sequence, a.Slot) == nil
		case "catan_explorer_settle":
			return cargo.settle(&base, &fleet, viewer, sequence, a.Slot, a.Vertex) == nil
		case "catan_explorer_transfer":
			return cargo.transfer(&base, &fleet, viewer, sequence, a.Slot, a.Vertex, a.Give, a.Take) == nil
		}
		return false
	}
	offer := func(a Action) {
		if probe(a) {
			add(a)
		}
	}
	if s.Phase == "catan_turn" {
		for _, edge := range g.Edges {
			if edge.Owner == -1 && catanExplorerLandEdge(g, edge.ID) && catanExplorerCanPay(g, viewer, []int{1, 1, 0, 0, 0}) {
				offer(Action{Type: "catan_road", Edge: edge.ID})
			}
			if catanExplorerSeaEdge(g, edge.ID) && (g.Vertices[edge.A].Owner == viewer && g.Vertices[edge.A].Level == 2 || g.Vertices[edge.B].Owner == viewer && g.Vertices[edge.B].Level == 2) {
				for ship := viewer * 3; ship < (viewer+1)*3; ship++ {
					offer(Action{Type: "catan_explorer_ship", Slot: ship, Edge: edge.ID})
				}
			}
		}
		locations := []catanExplorerCargoLocation{}
		for _, v := range g.Vertices {
			if catanExplorerBotSite(g, viewer, v.ID, true) && catanExplorerCanPay(g, viewer, []int{1, 1, 1, 1, 0}) {
				offer(Action{Type: "catan_settlement", Vertex: v.ID})
			}
			if v.Owner == viewer && v.Level == 1 {
				offer(Action{Type: "catan_explorer_harbor", Vertex: v.ID})
			}
			if v.Owner == viewer && v.Level == 2 {
				locations = append(locations, catanExplorerCargoLocation{"harbor", v.ID})
			}
		}
		for ship := viewer * 3; ship < (viewer+1)*3; ship++ {
			loc := catanExplorerCargoLocation{"ship", ship}
			if x.Cargo.buildLocation(g, x.Fleet, viewer, loc) {
				locations = append(locations, loc)
			}
		}
		if catanExplorerCanPay(g, viewer, []int{1, 1, 1, 1, 0}) || x.Lairs != nil && catanExplorerCanPay(g, viewer, []int{0, 0, 1, 0, 1}) {
			for _, loc := range locations {
				discards := [][]int{nil}
				for _, unit := range x.Cargo.contents(loc) {
					discards = append(discards, []int{unit})
				}
				limit := 2
				if x.Lairs != nil {
					limit = 11
				}
				for unit := viewer * 11; unit < viewer*11+limit; unit++ {
					for _, discard := range discards {
						offer(Action{Type: "catan_explorer_unit", Card: unit, Choice: loc.Kind, Target: loc.Index, Cards: discard})
					}
				}
			}
		}
		for give := -1; give < 5; give++ {
			for take := -1; take < 5; take++ {
				if give != take {
					offer(Action{Type: "catan_explorer_bank", Color: give, Target: take})
				}
			}
		}
		add(Action{Type: "catan_explorer_begin_move"})
		return result
	}
	for ship := viewer * 3; ship < (viewer+1)*3; ship++ {
		if x.Fleet.Positions[ship] < 0 {
			continue
		}
		offer(Action{Type: "catan_explorer_wool", Slot: ship})
		for _, path := range x.Fleet.destinations(g, viewer, sequence, ship) {
			add(Action{Type: "catan_explorer_sail", Slot: ship, Targets: path})
		}
		edge := g.Edges[x.Fleet.Positions[ship]]
		for _, v := range []int{edge.A, edge.B} {
			offer(Action{Type: "catan_explorer_settle", Slot: ship, Vertex: v})
			if !x.Cargo.docked(g, x.Fleet, viewer, ship, v) {
				continue
			}
			load := explorerCargoSubsets(x.Cargo.contents(catanExplorerCargoLocation{"harbor", v}))
			unload := explorerCargoSubsets(x.Cargo.contents(catanExplorerCargoLocation{"ship", ship}))
			for _, give := range load {
				for _, take := range unload {
					if len(give)+len(take) > 0 {
						offer(Action{Type: "catan_explorer_transfer", Slot: ship, Vertex: v, Give: give, Take: take})
					}
				}
			}
		}
	}
	for _, a := range s.catanExplorerLandingChoices(viewer) {
		add(a)
	}
	add(Action{Type: "catan_end"})
	return result
}

func explorerCargoSubsets(units []int) [][]int {
	out := [][]int{{}}
	for _, unit := range units {
		for _, subset := range slices.Clone(out) {
			out = append(out, append(slices.Clone(subset), unit))
		}
	}
	return out
}

// Keep a shortest path for each destination/toll pair. A longer toll-free
// path must survive a shorter paid route. Occupied edges may be crossed; fog stops
// expansion, even when the compulsory stopping edge itself is overcrowded.
func (f catanExplorerSailing) destinations(g *Catan, player int, sequence uint64, ship int) [][]int {
	out := [][]int{}
	if f.allowed(g, player, sequence) != nil || ship < 0 || ship >= len(f.Positions) || ship/3 != player || f.Positions[ship] < 0 || f.Turn.Ships[ship].Closed {
		return out
	}
	from := f.Positions[ship]
	type route struct {
		edge int
		path []int
		toll bool
	}
	queue := []route{{edge: from}}
	pirateOwner, pirateTile, gold := -1, -1, 0
	if g.Explorer != nil && g.Explorer.Pirate != nil {
		pirateOwner, pirateTile, gold = g.Explorer.Pirate.Owner, g.Explorer.Pirate.Tile, g.Explorer.Economy.Gold[player]
	}
	seen := map[[2]int]bool{{from, 0}: true}
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if len(item.path) >= f.Turn.Ships[ship].Remaining {
			continue
		}
		for _, edge := range g.Edges {
			if edge.ID == item.edge || !catanExplorerAdjacentEdges(g.Edges[item.edge], edge) || !catanExplorerSeaEdge(g, edge.ID) {
				continue
			}
			toll := item.toll || pirateOwner >= 0 && pirateOwner != player && !f.Turn.Ships[ship].Tribute && (slices.Contains(g.Edges[item.edge].Tiles, pirateTile) || slices.Contains(edge.Tiles, pirateTile))
			cost := 0
			if toll {
				cost = 1
			}
			key := [2]int{edge.ID, cost}
			if seen[key] || toll && gold == 0 {
				continue
			}
			seen[key] = true
			path := append(slices.Clone(item.path), edge.ID)
			if _, err := f.quote(g, player, sequence, ship, path, pirateOwner, pirateTile); err == nil {
				out = append(out, path)
			}
			fog := false
			for _, tile := range g.Tiles {
				if tile.Resource == CatanFog && catanExplorerTouches(g, edge.ID, tile.ID) {
					fog = true
					break
				}
			}
			if !fog {
				queue = append(queue, route{edge.ID, path, toll})
			}
		}
	}
	return out
}

// Do not send every unrelated field of the shared multi-game Action struct.
func catanExplorerChoiceView(actions []Action) []map[string]any {
	out := make([]map[string]any, 0, len(actions))
	for _, a := range actions {
		v := map[string]any{"type": a.Type, "prompt": a.Prompt}
		switch a.Type {
		case "catan_explorer_setup":
			v["choice"], v["target"] = a.Choice, a.Target
		case "catan_explorer_pirate_place", "catan_explorer_pirate_steal", "catan_explorer_chase", "catan_explorer_resolve", "catan_explorer_battle":
			v["target"] = a.Target
			if a.Choice != "" {
				v["choice"] = a.Choice
			}
		case "catan_explorer_land", "catan_explorer_pickup":
			v["target"], v["slot"], v["cards"] = a.Target, a.Slot, a.Cards
		case "catan_road":
			v["edge"] = a.Edge
		case "catan_settlement", "catan_explorer_harbor":
			v["vertex"] = a.Vertex
		case "catan_explorer_ship":
			v["slot"], v["edge"] = a.Slot, a.Edge
		case "catan_explorer_unit":
			v["card"], v["choice"], v["target"], v["cards"] = a.Card, a.Choice, a.Target, a.Cards
		case "catan_explorer_bank":
			v["color"], v["target"] = a.Color, a.Target
		case "catan_explorer_wool":
			v["slot"] = a.Slot
		case "catan_explorer_sail":
			v["slot"], v["targets"] = a.Slot, a.Targets
		case "catan_explorer_settle":
			v["slot"], v["vertex"] = a.Slot, a.Vertex
		case "catan_explorer_transfer":
			v["slot"], v["vertex"], v["give"], v["take"] = a.Slot, a.Vertex, a.Give, a.Take
		}
		out = append(out, v)
	}
	return out
}
