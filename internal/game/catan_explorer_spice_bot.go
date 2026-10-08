package game

import "slices"

// Count only entitlements already visible in public cargo. The six-farm
// inventory is known; no planning depends on the faces of unexplored tiles.
func catanExplorerBotSpiceRemaining(g *Catan, player int) int {
	n := 6
	for _, sack := range g.Explorer.Cargo.Spice {
		if sack.Owner == player {
			n--
		}
	}
	return n
}

func catanExplorerSpiceGoal(g *Catan, player, ship, edge int) int {
	x := g.Explorer
	loc := catanExplorerCargoLocation{"ship", ship}
	e := g.Edges[edge]
	if len(x.Cargo.spiceContents(loc)) > 0 {
		for _, anchor := range x.Board.Council.Anchors {
			if e.A == anchor || e.B == anchor {
				return 2200
			}
		}
		return -100000
	}
	fishGoal := catanExplorerBotFishGoal(g, player, ship, edge)
	units := x.Cargo.contents(loc)
	if len(x.Cargo.fishContents(loc)) > 0 || len(units) == 0 && catanExplorerBotFishingShip(g, player, ship) {
		return fishGoal
	}
	settler := len(units) == 1 && units[0]%11 < 2
	crew := len(units)
	if settler {
		crew = 0
	}
	value := fishGoal
	if settler || crew > 0 {
		for _, tile := range g.Tiles {
			if tile.Resource == CatanFog && catanExplorerTouches(g, edge, tile.ID) {
				value = max(value, 600)
			}
		}
	}
	if settler {
		for _, v := range []int{e.A, e.B} {
			if catanExplorerBotSite(g, player, v, false) {
				value = max(value, 1100+g.vertexValue(player, v))
			}
		}
		return value
	}
	needed := catanExplorerBotNeedsCrew(g, player)
	if crew > 0 {
		for _, farm := range x.Board.publicView().Farms {
			if !x.Cargo.farmFriend(player, farm.Tile) && catanExplorerTouches(g, edge, farm.Tile) {
				priority := 1500
				if farm.Ability == "swift" {
					priority += 100
				}
				value = max(value, priority)
			}
		}
	}
	for _, v := range []int{e.A, e.B} {
		if g.Vertices[v].Owner != player || !catanExplorerHarborAt(g, v) {
			continue
		}
		bay := catanExplorerCargoLocation{"harbor", v}
		if len(x.Cargo.spiceContents(bay)) > 0 {
			value = max(value, 1600)
		}
		if crew == 0 {
			value = max(value, 700+50*x.Cargo.used(bay))
		} else if !needed {
			if _, _, ok := catanExplorerBotRetireCargo(g, ship, v); ok {
				value = max(value, 850)
			}
		}
	}
	return value
}

// Trade only surplus over the intended build. One-for-one farm exchanges can
// fund a missing resource or a later toll without selling the build itself.
func (s *State) catanExplorerSpiceBotGold(player int, reserve []int) (Action, bool) {
	g, x := s.Catan, s.Catan.Explorer
	if x.Spice == nil {
		return Action{}, false
	}
	for _, a := range s.catanExplorerSpiceChoices(player) {
		if a.Type == "catan_explorer_spice_gold" && a.Card < len(reserve) && g.Players[player].Resources[a.Card] > reserve[a.Card] {
			a.Prompt = int(g.TurnSerial)
			return a, true
		}
	}
	return Action{}, false
}

func (s *State) catanExplorerSpiceBotCargo(player int) (Action, bool) {
	g, x := s.Catan, s.Catan.Explorer
	prompt := int(g.TurnSerial)
	choices := s.catanExplorerSpiceChoices(player)
	for _, kind := range []string{"catan_explorer_spice_deliver", "catan_explorer_spice_land"} {
		for _, a := range choices {
			if a.Type == kind {
				a.Prompt = prompt
				return a, true
			}
		}
	}
	if a, ok := s.catanExplorerFishBotCargo(player); ok {
		return a, true
	}
	if a, ok := s.catanExplorerBotLanding(player); ok {
		return a, true
	}
	needed := catanExplorerBotNeedsCrew(g, player)
	for ship := player * 3; ship < (player+1)*3; ship++ {
		pos := x.Fleet.Positions[ship]
		if pos < 0 {
			continue
		}
		loc := catanExplorerCargoLocation{"ship", ship}
		// Existing mission freight must reach the council before a harbor detour.
		if len(x.Cargo.spiceContents(loc)) > 0 || len(x.Cargo.fishContents(loc)) > 0 {
			continue
		}
		e := g.Edges[pos]
		for _, v := range []int{e.A, e.B} {
			if !x.Cargo.docked(g, x.Fleet, player, ship, v) {
				continue
			}
			bay := catanExplorerCargoLocation{"harbor", v}
			// Recover human-stored sacks, including a simultaneous exchange for crew
			// or a settler. Probe only public cargo legality, never hidden exploration.
			sacks := x.Cargo.spiceContents(bay)
			if len(sacks) > 0 {
				for _, unload := range [][]int{nil, x.Cargo.contents(loc)} {
					for _, load := range [][]int{sacks, sacks[:1]} {
						c, f := clone(*x.Cargo), clone(*x.Fleet)
						base := *g
						// Retain read-only combination metadata for cargo validation.
						if c.transferAllFreight(&base, &f, player, g.TurnSerial, ship, v, nil, unload, nil, nil, load, nil) == nil {
							return Action{Type: "catan_explorer_transfer", Prompt: prompt, Slot: ship, Vertex: v, Take: unload, SpiceLoad: slices.Clone(load)}, true
						}
					}
				}
			}
			if !needed {
				if load, unload, ok := catanExplorerBotRetireCargo(g, ship, v); ok {
					return Action{Type: "catan_explorer_transfer", Prompt: prompt, Slot: ship, Vertex: v, Give: load, Take: unload}, true
				}
			}
			if catanExplorerBotFishingShip(g, player, ship) {
				continue
			}
			space := 2 - x.Cargo.used(loc)
			load := []int{}
			for _, id := range x.Cargo.contents(bay) {
				if id%11 >= 2 && !needed {
					continue
				}
				size := catanExplorerUnitSize(id)
				if size <= space {
					load = append(load, id)
					space -= size
				}
			}
			if len(load) > 0 {
				return Action{Type: "catan_explorer_transfer", Prompt: prompt, Slot: ship, Vertex: v, Give: load}, true
			}
		}
	}
	return Action{}, false
}
