package game

import "errors"

// NewCatanExplorerSpices starts the 2025 fish-and-spice scenario.
// Five/six players use its own enlarged map and paired turns.
func NewCatanExplorerSpices(players int) (*State, error) {
	return newCatanExplorerSpiceState(players)
}

func newCatanExplorerSpiceState(players int) (*State, error) {
	return newCatanExplorerMissionState(players, "spices-for-catan", "variable", nil)
}

func (s *State) applyCatanExplorerSpice(player int, a Action) (bool, error) {
	kind := map[string]string{"catan_explorer_spice_land": "land", "catan_explorer_spice_deliver": "deliver", "catan_explorer_spice_gold": "gold"}[a.Type]
	if kind == "" {
		return false, nil
	}
	g, x := s.Catan, s.Catan.Explorer
	if x.Spice == nil {
		return true, errors.New("本场景没有香料任务")
	}
	if err := x.Spice.apply(g, x.Board, x.Fleet, x.Cargo, x.Economy, player, g.TurnSerial, kind, a.Target, a.Slot, a.Card); err != nil {
		return true, err
	}
	switch kind {
	case "land":
		s.catanLog(player, "派驻船员至农场 #%d，领取一袋香料与农场能力", a.Target+1)
	case "deliver":
		s.catanLog(player, "向议会岛交付一袋香料，任务前进一步")
	case "gold":
		s.catanLog(player, "使用金币农场，将%s×1换为1金币", catanCardName(a.Card))
	}
	g.Trade = nil
	s.catanExplorerMissionScore()
	s.catanExplorerVictory()
	return true, nil
}

// Inspect only revealed farms, owned freight and resources. Hidden inventory
// must never influence action previews. Rules remain authoritative in Apply.
func (s *State) catanExplorerSpiceChoices(player int) []Action {
	g, x := s.Catan, s.Catan.Explorer
	out := []Action{}
	if x.Spice == nil || s.Finished || player != s.Turn {
		return out
	}
	if s.Phase == "catan_turn" {
		used := 0
		if u := x.Spice.GoldUse; u != nil && u.Sequence == g.TurnSerial {
			used = u.Count
		}
		if used < x.Cargo.farmCount(x.Board, player, "gold") {
			for r, n := range g.Players[player].Resources {
				if n > 0 {
					out = append(out, Action{Type: "catan_explorer_spice_gold", Card: r})
				}
			}
		}
		return out
	}
	if s.Phase != "catan_explorer_move" {
		return out
	}
	for ship := player * 3; ship < (player+1)*3; ship++ {
		pos := x.Fleet.Positions[ship]
		if pos < 0 {
			continue
		}
		loc := catanExplorerCargoLocation{"ship", ship}
		for _, farm := range x.Board.publicView().Farms {
			if x.Cargo.farmFriend(player, farm.Tile) || !catanExplorerTouches(g, pos, farm.Tile) {
				continue
			}
			for _, crew := range x.Cargo.contents(loc) {
				if crew/11 == player && crew%11 >= 2 {
					out = append(out, Action{Type: "catan_explorer_spice_land", Target: farm.Tile, Slot: ship, Card: crew})
				}
			}
		}
		edge := g.Edges[pos]
		for _, anchor := range x.Board.Council.Anchors {
			if edge.A == anchor || edge.B == anchor {
				for _, sack := range x.Cargo.spiceContents(loc) {
					out = append(out, Action{Type: "catan_explorer_spice_deliver", Slot: ship, Card: sack})
				}
				break
			}
		}
	}
	return out
}
