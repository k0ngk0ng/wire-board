package game

import "errors"

func catanExplorerMissionScenario(scenario string) bool {
	return scenario == "pirate-lairs" || scenario == "fish-for-catan"
}

// Private: the six lair numbers are still explicit acceptance components,
// never an invented production default. Public options remain closed.
func newCatanExplorerFishState(players int, numbers []int) (*State, error) {
	return newCatanExplorerMissionState(players, "fish-for-catan", "variable", numbers)
}
func (s *State) applyCatanExplorerFish(player int, a Action) (bool, error) {
	kind := map[string]string{"catan_explorer_fish_roll": "roll", "catan_explorer_fish_load": "load", "catan_explorer_fish_deliver": "deliver"}[a.Type]
	if kind == "" {
		return false, nil
	}
	g, x := s.Catan, s.Catan.Explorer
	if x.Fish == nil {
		return true, errors.New("本场景没有鱼群任务")
	}
	if err := x.Fish.apply(g, x.Board, x.Fleet, x.Cargo, player, g.TurnSerial, kind, a.Slot, a.Card, x.Pirate.Tile, catanRandom); err != nil {
		return true, err
	}
	switch kind {
	case "roll":
		result := "没有鱼群出现"
		if x.Fish.LastRoll.Spawned >= 0 {
			result = "渔场出现一枚鱼群"
		}
		s.catanLog(player, "捕捞骰掷出 %d，%s", x.Fish.LastRoll.Die, result)
	case "load":
		s.catanLog(player, "船只 #%d 从渔场装载一枚鱼群", a.Slot+1)
	case "deliver":
		s.catanLog(player, "向议会岛交付一枚鱼群，任务前进一步")
	}
	g.Trade = nil
	s.catanExplorerMissionScore()
	s.catanExplorerVictory()
	return true, nil
}
func (c catanExplorerCargo) fishContents(at catanExplorerCargoLocation) []int {
	ids := []int{}
	for id, loc := range c.Fish {
		if loc == at {
			ids = append(ids, id)
		}
	}
	return ids
}
func (s *State) catanExplorerFishChoices(player int) []Action {
	g, x := s.Catan, s.Catan.Explorer
	out := []Action{}
	if x.Fish == nil || s.Finished || player != s.Turn || s.Phase != "catan_explorer_move" {
		return out
	}
	if x.Fish.LastRoll == nil || x.Fish.LastRoll.Sequence != g.TurnSerial {
		out = append(out, Action{Type: "catan_explorer_fish_roll"})
	}
	for ship := player * 3; ship < (player+1)*3; ship++ {
		if x.Fleet.Positions[ship] < 0 {
			continue
		}
		loc := catanExplorerCargoLocation{"ship", ship}
		for id, at := range x.Cargo.Fish {
			kind := ""
			if at.Kind == "shoal" && x.Cargo.used(loc) == 0 && catanExplorerTouches(g, x.Fleet.Positions[ship], at.Index) {
				kind = "load"
			}
			if at == loc {
				edge := g.Edges[x.Fleet.Positions[ship]]
				for _, v := range x.Board.Council.Anchors {
					if edge.A == v || edge.B == v {
						kind = "deliver"
					}
				}
			}
			if kind != "" {
				out = append(out, Action{Type: "catan_explorer_fish_" + kind, Slot: ship, Card: id})
			}
		}
	}
	return out
}
