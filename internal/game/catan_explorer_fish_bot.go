package game

// This strategy uses only the public mission track, revealed shoals, cargo and
// visible topology. The shared voyage search accounts for actual MPs/tribute.
func catanExplorerBotWantsFish(g *Catan, player int) bool {
	x := g.Explorer
	return x.Fish != nil && x.Fish.publicView(len(g.Players)).Progress[player] < 7
}
func catanExplorerBotFishingShip(g *Catan, player, ship int) bool {
	// Keep the third vessel's hold free of newly recruited units. The first two
	// can still settle, deliver crews, or opportunistically collect nearby fish.
	return ship == player*3+2 && catanExplorerBotWantsFish(g, player)
}
func catanExplorerBotFishGoal(g *Catan, player, ship, edge int) int {
	x := g.Explorer
	if x.Fish == nil {
		return -100000
	}
	loc := catanExplorerCargoLocation{"ship", ship}
	e := g.Edges[edge]
	if len(x.Cargo.fishContents(loc)) > 0 {
		for _, v := range x.Board.Council.Anchors {
			if e.A == v || e.B == v {
				return 2200
			}
		}
		return -100000
	}
	if x.Cargo.used(loc) != 0 || !catanExplorerBotWantsFish(g, player) {
		return -100000
	}
	value := -100000
	for _, at := range x.Cargo.Fish {
		if at.Kind == "shoal" && catanExplorerTouches(g, edge, at.Index) {
			value = max(value, 1650)
		}
	}
	for _, v := range []int{e.A, e.B} {
		if g.Vertices[v].Owner == player && catanExplorerHarborAt(g, v) && len(x.Cargo.fishContents(catanExplorerCargoLocation{"harbor", v})) > 0 {
			value = max(value, 1650)
		}
	}
	// Without an available haul, wait at a known unblocked shoal or explore.
	// A wait goal scores at the current berth, preventing pointless oscillation.
	for _, shoal := range x.Board.publicView().Shoals {
		if shoal.Tile != x.Pirate.Tile && catanExplorerTouches(g, edge, shoal.Tile) {
			value = max(value, 850)
		}
	}
	for _, tile := range g.Tiles {
		if tile.Resource == CatanFog && catanExplorerTouches(g, edge, tile.ID) {
			value = max(value, 600)
		}
	}
	return value
}
func (s *State) catanExplorerFishBotCargo(player int) (Action, bool) {
	g, x := s.Catan, s.Catan.Explorer
	if x.Fish == nil {
		return Action{}, false
	}
	choices := s.catanExplorerFishChoices(player)
	// Deliver existing cargo before deciding where to spawn or collect new fish.
	for _, kind := range []string{"catan_explorer_fish_deliver", "catan_explorer_fish_load"} {
		if kind == "catan_explorer_fish_load" && !catanExplorerBotWantsFish(g, player) {
			continue
		}
		for _, a := range choices {
			if a.Type == kind {
				a.Prompt = int(g.TurnSerial)
				return a, true
			}
		}
	}
	if !catanExplorerBotWantsFish(g, player) {
		return Action{}, false
	}
	for ship := player * 3; ship < (player+1)*3; ship++ {
		at := x.Fleet.Positions[ship]
		if at < 0 || x.Cargo.used(catanExplorerCargoLocation{"ship", ship}) > 0 {
			continue
		}
		e := g.Edges[at]
		for _, v := range []int{e.A, e.B} {
			if !x.Cargo.docked(g, x.Fleet, player, ship, v) {
				continue
			}
			fish := x.Cargo.fishContents(catanExplorerCargoLocation{"harbor", v})
			if len(fish) > 0 {
				return Action{Type: "catan_explorer_transfer", Prompt: int(g.TurnSerial), Slot: ship, Vertex: v, Cards: fish}, true
			}
		}
	}
	if len(x.Board.publicView().Shoals) > 0 {
		for _, a := range choices {
			if a.Type == "catan_explorer_fish_roll" {
				a.Prompt = int(g.TurnSerial)
				return a, true
			}
		}
	}
	return Action{}, false
}
