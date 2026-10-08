package game

import "slices"

type catanExplorerBotPlan struct {
	action Action
	cost   []int
	value  int
}

func catanExplorerMissionUnfinished(g *Catan) bool {
	l := g.Explorer.Lairs
	if l == nil {
		return false
	}
	total := 6
	if g.Explorer.Board.Scenario == "fish-for-catan" {
		total = 5
	}
	if len(l.Sites) < total {
		return true
	}
	for _, site := range l.Sites {
		if site.Resolved == 0 {
			return true
		}
	}
	return false
}

func catanExplorerBotNeedsCrew(g *Catan, player int) bool {
	return catanExplorerMissionUnfinished(g) || g.Explorer.Spice != nil && catanExplorerBotSpiceRemaining(g, player) > 0
}

// After the last mission, reuse crew ships for settlers. A harbor exchange is
// simultaneous, so a stored settler can swap with one or two retiring crew.
func catanExplorerBotRetireCargo(g *Catan, ship, harbor int) (load, unload []int, ok bool) {
	x := g.Explorer
	aboard := x.Cargo.contents(catanExplorerCargoLocation{"ship", ship})
	if len(aboard) == 0 || aboard[0]%11 < 2 {
		return nil, nil, false
	}
	bay := catanExplorerCargoLocation{"harbor", harbor}
	for _, id := range x.Cargo.contents(bay) {
		if id%11 < 2 {
			return []int{id}, aboard, true
		}
	}
	if x.Cargo.used(bay)+len(aboard) <= 2 {
		return nil, aboard, true
	}
	return nil, nil, false
}

// Mission planning reads visible topology, public cargo, and the actor's hand.
// Never probe Apply/discover to rank a fog destination: that would read its face.
func (s *State) catanExplorerMissionPlans(player int) []catanExplorerBotPlan {
	g := s.Catan
	x := g.Explorer
	if x.Lairs == nil && x.Spice == nil {
		return nil
	}
	prompt := int(g.TurnSerial)
	plans := []catanExplorerBotPlan{}
	unresolved := catanExplorerBotNeedsCrew(g, player)
	ships, freeShip := 0, -1
	for id := player * 3; id < (player+1)*3; id++ {
		if x.Fleet.Positions[id] >= 0 {
			ships++
		} else if freeShip < 0 {
			freeShip = id
		}
	}
	targetShips := 2
	if catanExplorerBotWantsFish(g, player) {
		targetShips = 3
	}
	if ships < targetShips && freeShip >= 0 {
		for _, edge := range g.Edges {
			if !catanExplorerSeaEdge(g, edge.ID) || slices.Contains(x.Fleet.Positions, edge.ID) {
				continue
			}
			harbor := false
			fog := false
			for _, v := range []int{edge.A, edge.B} {
				harbor = harbor || g.Vertices[v].Owner == player && catanExplorerHarborAt(g, v)
			}
			for _, tile := range edge.Tiles {
				fog = fog || g.Tiles[tile].Resource == CatanFog
			}
			if harbor && !fog {
				value := 105
				if ships == 0 {
					value = 180
				}
				plans = append(plans, catanExplorerBotPlan{Action{Type: "catan_explorer_ship", Prompt: prompt, Slot: freeShip, Edge: edge.ID}, []int{1, 0, 1, 0, 0}, value})
			}
		}
	}
	crew, settlers, freeCrew, freeSettler := 0, 0, -1, -1
	for id := player * 11; id < (player+1)*11; id++ {
		if id%11 < 2 {
			if x.Cargo.Units[id].Kind != "supply" {
				settlers++
			} else if freeSettler < 0 {
				freeSettler = id
			}
		} else {
			if x.Cargo.Units[id].Kind != "supply" {
				if x.Spice == nil || x.Cargo.Units[id].Kind != "farm" {
					crew++
				}
			} else if freeCrew < 0 {
				freeCrew = id
			}
		}
	}
	locations := []catanExplorerCargoLocation{}
	for ship := player * 3; ship < (player+1)*3; ship++ {
		loc := catanExplorerCargoLocation{"ship", ship}
		if x.Cargo.buildLocation(g, x.Fleet, player, loc) {
			locations = append(locations, loc)
		}
	}
	for _, v := range g.Vertices {
		loc := catanExplorerCargoLocation{"harbor", v.ID}
		if x.Cargo.buildLocation(g, x.Fleet, player, loc) {
			locations = append(locations, loc)
		}
	}
	settlements := 0
	for _, v := range g.Vertices {
		if v.Owner == player && v.Level == 1 {
			settlements++
		}
	}
	crewTarget := 3
	if x.Spice != nil && !catanExplorerMissionUnfinished(g) {
		crewTarget = min(2, catanExplorerBotSpiceRemaining(g, player))
	}
	for _, loc := range locations {
		if loc.Kind == "ship" && catanExplorerBotFishingShip(g, player, loc.Index) {
			continue
		}
		used := x.Cargo.used(loc)
		if unresolved && crew < crewTarget && freeCrew >= 0 && used < 2 {
			value := 125
			if loc.Kind == "ship" {
				value += 25
			}
			plans = append(plans, catanExplorerBotPlan{Action{Type: "catan_explorer_unit", Prompt: prompt, Card: freeCrew, Choice: loc.Kind, Target: loc.Index}, []int{0, 0, 1, 0, 1}, value})
		}
		retiredCrew := x.Cargo.contents(loc)
		replaceCrew := !unresolved && len(retiredCrew) == 2 && retiredCrew[0]%11 >= 2 && retiredCrew[1]%11 >= 2 && x.Cargo.buildBerthsFull(g, x.Fleet, player)
		if settlers == 0 && settlements < 5 && freeSettler >= 0 && (used == 0 || replaceCrew) {
			value := 100
			if !unresolved {
				value = 145
			}
			if loc.Kind == "ship" {
				value += 15
			}
			action := Action{Type: "catan_explorer_unit", Prompt: prompt, Card: freeSettler, Choice: loc.Kind, Target: loc.Index}
			if replaceCrew {
				action.Cards = retiredCrew
			}
			plans = append(plans, catanExplorerBotPlan{action, []int{1, 1, 1, 1, 0}, value})
		}
	}
	return plans
}

// Rank a visible stopping edge by the ship's actual cargo. Empty ships return
// to load/recruit or recover crew; crew ships seek unfinished lairs; settlers
// seek legal settlement sites. A productive berth scores at distance zero too,
// so ships wait there for next-turn recruitment rather than oscillating.
func catanExplorerMissionGoal(g *Catan, player, ship, edge int) int {
	x := g.Explorer
	if x.Spice != nil {
		value := catanExplorerSpiceGoal(g, player, ship, edge)
		loc := catanExplorerCargoLocation{"ship", ship}
		// Deliver existing mission freight first. An empty fishing vessel also
		// keeps its assignment; other ships can serve either crew mission.
		if x.Lairs == nil || len(x.Cargo.spiceContents(loc)) > 0 || len(x.Cargo.fishContents(loc)) > 0 || len(x.Cargo.contents(loc)) == 0 && catanExplorerBotFishingShip(g, player, ship) {
			return value
		}
		return max(value, catanExplorerLairGoal(g, player, ship, edge))
	}
	return catanExplorerLairGoal(g, player, ship, edge)
}

func catanExplorerLairGoal(g *Catan, player, ship, edge int) int {
	x := g.Explorer
	shipLoc := catanExplorerCargoLocation{"ship", ship}
	units := x.Cargo.contents(shipLoc)
	fishGoal := catanExplorerBotFishGoal(g, player, ship, edge)
	if len(x.Cargo.fishContents(shipLoc)) > 0 || len(units) == 0 && catanExplorerBotFishingShip(g, player, ship) {
		return fishGoal
	}
	settler := len(units) == 1 && units[0]%11 < 2
	crew := len(units)
	if settler {
		crew = 0
	}
	value := fishGoal
	unfinished := catanExplorerBotNeedsCrew(g, player)
	for _, tile := range g.Tiles {
		if tile.Resource == CatanFog && catanExplorerTouches(g, edge, tile.ID) {
			if settler || crew > 0 {
				value = max(value, 600)
			}
		}
	}
	if settler {
		e := g.Edges[edge]
		for _, v := range []int{e.A, e.B} {
			if catanExplorerBotSite(g, player, v, false) {
				value = max(value, 1100+g.vertexValue(player, v))
			}
		}
		return value
	}
	for _, site := range x.Lairs.Sites {
		if !catanExplorerTouches(g, edge, site.Tile) {
			continue
		}
		ids := x.Cargo.contents(catanExplorerCargoLocation{"lair", site.Tile})
		own := 0
		for _, id := range ids {
			if id/11 == player {
				own++
			}
		}
		if site.Resolved == 0 && site.Ready == 0 && crew > 0 && len(ids) < 3 {
			v := 900 + 100*min(crew, 3-len(ids))
			if own == 0 {
				v += 100
			}
			if crew+len(ids) >= 3 {
				v += 250
			}
			value = max(value, v)
		}
		if unfinished && site.Resolved > 0 && site.Resolved < g.TurnSerial && crew < 2 && own > 0 {
			value = max(value, 850+50*min(own, 2-crew))
		}
	}
	if crew == 0 || !unfinished {
		e := g.Edges[edge]
		for _, v := range []int{e.A, e.B} {
			if g.Vertices[v].Owner == player && catanExplorerHarborAt(g, v) {
				if crew == 0 {
					value = max(value, 700+50*x.Cargo.used(catanExplorerCargoLocation{"harbor", v}))
				} else if _, _, ok := catanExplorerBotRetireCargo(g, ship, v); ok {
					value = max(value, 850)
				}
			}
		}
	}
	return value
}

func catanExplorerMissionVoyage(g *Catan, player, ship int) []int {
	x := g.Explorer
	from := x.Fleet.Positions[ship]
	owner, tile := x.Pirate.Owner, x.Pirate.Tile
	type route struct {
		edge int
		path []int
		toll bool
	}
	queue := []route{{edge: from}}
	seen := map[[2]int]bool{{from, 0}: true}
	best := catanExplorerMissionGoal(g, player, ship, from)
	var bestPath []int
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if len(item.path) > 0 {
			value := catanExplorerMissionGoal(g, player, ship, item.edge) - 35*len(item.path)
			if item.toll {
				value -= 35
			}
			// A full remote berth is not a useful goal, though it may be crossed.
			count := 0
			for id, pos := range x.Fleet.Positions {
				if id != ship && pos == item.edge {
					count++
				}
			}
			if value > best && count < 2 {
				for limit := min(len(item.path), x.Fleet.Turn.Ships[ship].Remaining); limit > 0; limit-- {
					if quote, err := x.Fleet.quote(g, player, g.TurnSerial, ship, item.path[:limit], owner, tile); err == nil && quote.Gold <= x.Economy.Gold[player] {
						best, bestPath = value, slices.Clone(item.path[:limit])
						break
					}
				}
			}
		}
		fog := false
		for _, t := range g.Tiles {
			if t.Resource == CatanFog && catanExplorerTouches(g, item.edge, t.ID) {
				fog = true
				break
			}
		}
		if fog {
			continue
		}
		for _, edge := range g.Edges {
			if edge.ID == item.edge || !catanExplorerSeaEdge(g, edge.ID) || !catanExplorerAdjacentEdges(g.Edges[item.edge], edge) {
				continue
			}
			toll := item.toll || owner >= 0 && owner != player && !x.Fleet.Turn.Ships[ship].Tribute && (slices.Contains(g.Edges[item.edge].Tiles, tile) || slices.Contains(edge.Tiles, tile))
			cost := 0
			if toll {
				cost = 1
			}
			key := [2]int{edge.ID, cost}
			if seen[key] || toll && x.Economy.Gold[player] == 0 {
				continue
			}
			seen[key] = true
			queue = append(queue, route{edge.ID, append(slices.Clone(item.path), edge.ID), toll})
		}
	}
	return bestPath
}

func (s *State) catanExplorerMissionCargo(player int) (Action, bool) {
	g := s.Catan
	x := g.Explorer
	if x.Spice != nil {
		return s.catanExplorerSpiceBotCargo(player)
	}
	if x.Lairs == nil {
		return Action{}, false
	}
	unfinished := catanExplorerBotNeedsCrew(g, player)
	if a, ok := s.catanExplorerFishBotCargo(player); ok {
		return a, true
	}
	if a, ok := s.catanExplorerBotLanding(player); ok {
		return a, true
	}
	return s.catanExplorerLairPortCargo(player, unfinished)
}

func (s *State) catanExplorerBotLanding(player int) (Action, bool) {
	g := s.Catan
	unfinished := catanExplorerBotNeedsCrew(g, player)
	// Land before collecting or sailing; do not strand a third crew contribution.
	var best Action
	score := -1
	for _, a := range s.catanExplorerLandingChoices(player) {
		value := -1
		switch a.Type {
		case "catan_explorer_land":
			value = 100 + len(a.Cards)
		case "catan_explorer_pickup":
			if unfinished && !catanExplorerBotFishingShip(g, player, a.Slot) {
				value = 10 + len(a.Cards)
			}
		case "catan_explorer_chase":
			value = 0
		}
		if value > score {
			score, best = value, a
		}
	}
	if score >= 0 {
		best.Prompt = int(g.TurnSerial)
		return best, true
	}
	return Action{}, false
}

func (s *State) catanExplorerLairPortCargo(player int, unfinished bool) (Action, bool) {
	g, x := s.Catan, s.Catan.Explorer
	for ship := player * 3; ship < (player+1)*3; ship++ {
		edge := x.Fleet.Positions[ship]
		if edge < 0 {
			continue
		}
		space := 2 - x.Cargo.used(catanExplorerCargoLocation{"ship", ship})
		e := g.Edges[edge]
		for _, v := range []int{e.A, e.B} {
			if !x.Cargo.docked(g, x.Fleet, player, ship, v) {
				continue
			}
			if !unfinished {
				if load, unload, ok := catanExplorerBotRetireCargo(g, ship, v); ok {
					return Action{Type: "catan_explorer_transfer", Prompt: int(g.TurnSerial), Slot: ship, Vertex: v, Give: load, Take: unload}, true
				}
			}
			if catanExplorerBotFishingShip(g, player, ship) {
				continue
			}
			load := []int{}
			for _, id := range x.Cargo.contents(catanExplorerCargoLocation{"harbor", v}) {
				size := catanExplorerUnitSize(id)
				if !unfinished && id%11 >= 2 {
					continue
				}
				if size <= space {
					load = append(load, id)
					space -= size
				}
			}
			if len(load) > 0 {
				return Action{Type: "catan_explorer_transfer", Prompt: int(g.TurnSerial), Slot: ship, Vertex: v, Give: load}, true
			}
		}
	}
	return Action{}, false
}
