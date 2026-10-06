package game

import "slices"

// Only public board changes belong here. Never copy an Action wholesale:
// trade/discard payloads may contain private hand information.
type catanExplorerMotion struct {
	ID       uint64                     `json:"id"`
	Player   int                        `json:"player"`
	Kind     string                     `json:"kind"`
	Ship     int                        `json:"ship"`
	Path     []int                      `json:"path,omitempty"`
	Revealed []int                      `json:"revealed,omitempty"`
	Vertex   int                        `json:"vertex"`
	Fish     []catanExplorerFishMotion  `json:"fish,omitempty"`
	FishRoll *catanExplorerFishRoll     `json:"fishRoll,omitempty"`
	Cargo    []catanExplorerCargoMotion `json:"cargo,omitempty"`
	Pirate   *catanExplorerPirateMotion `json:"pirate,omitempty"`
	Lair     *catanExplorerLairMotion   `json:"lair,omitempty"`
	Chase    *catanExplorerChase        `json:"chase,omitempty"`
}

type catanExplorerPirateMotion struct {
	FromOwner int `json:"fromOwner"`
	FromTile  int `json:"fromTile"`
	ToOwner   int `json:"toOwner"`
	ToTile    int `json:"toTile"`
}
type catanExplorerLairMotion struct {
	Tile     int   `json:"tile"`
	Ready    bool  `json:"ready"`
	Resolved bool  `json:"resolved"`
	Hero     int   `json:"hero"`
	Dice     []int `json:"dice,omitempty"`
}

type catanExplorerFishMotion struct {
	Fish int                        `json:"fish"`
	From catanExplorerCargoLocation `json:"from"`
	To   catanExplorerCargoLocation `json:"to"`
}

type catanExplorerCargoMotion struct {
	Unit int                        `json:"unit"`
	From catanExplorerCargoLocation `json:"from"`
	To   catanExplorerCargoLocation `json:"to"`
}

func (s *State) recordCatanExplorerMotion(before *State, player int, a Action) {
	x, old := s.Catan.Explorer, before.Catan.Explorer
	x.ActionID = old.ActionID + 1
	x.Motion = nil
	switch a.Type {
	case "catan_explorer_fish_roll", "catan_explorer_fish_load", "catan_explorer_fish_deliver", "catan_explorer_land", "catan_explorer_pickup", "catan_explorer_resolve", "catan_explorer_battle", "catan_explorer_pirate_place", "catan_explorer_chase", "catan_explorer_sail", "catan_explorer_transfer", "catan_explorer_settle", "catan_explorer_unit", "catan_explorer_ship", "catan_explorer_harbor":
	default:
		return
	}
	m := &catanExplorerMotion{ID: x.ActionID, Player: player, Kind: a.Type, Ship: -1, Vertex: -1}
	switch a.Type {
	case "catan_explorer_sail":
		m.Ship = a.Slot
		m.Path = append([]int{old.Fleet.Positions[a.Slot]}, slices.Clone(a.Targets)...)
	case "catan_explorer_ship":
		m.Ship = a.Slot
	case "catan_explorer_settle", "catan_explorer_transfer":
		m.Ship, m.Vertex = a.Slot, a.Vertex
	case "catan_explorer_harbor":
		m.Vertex = a.Vertex
	case "catan_explorer_fish_load", "catan_explorer_fish_deliver", "catan_explorer_land", "catan_explorer_pickup":
		m.Ship = a.Slot
	case "catan_explorer_chase":
		m.Ship = a.Target
		m.Chase = clone(x.Pirate.LastChase)
	}
	if a.Type == "catan_explorer_pirate_place" && old.Pirate != nil && x.Pirate != nil {
		m.Pirate = &catanExplorerPirateMotion{old.Pirate.Owner, old.Pirate.Tile, x.Pirate.Owner, x.Pirate.Tile}
	}
	if x.Lairs != nil && (a.Type == "catan_explorer_land" || a.Type == "catan_explorer_resolve" || a.Type == "catan_explorer_battle") {
		at := x.Lairs.site(a.Target)
		if at >= 0 {
			site, prior := x.Lairs.Sites[at], old.Lairs.Sites[old.Lairs.site(a.Target)]
			m.Lair = &catanExplorerLairMotion{Tile: site.Tile, Ready: site.Ready > 0 && prior.Ready == 0, Resolved: site.Resolved > 0 && prior.Resolved == 0, Hero: site.Hero}
			if len(site.Rounds) > len(prior.Rounds) {
				m.Lair.Dice = slices.Clone(site.Rounds[len(site.Rounds)-1])
			}
		}
	}
	for i, tile := range s.Catan.Tiles {
		if before.Catan.Tiles[i].Resource == 8 && tile.Resource != 8 {
			m.Revealed = append(m.Revealed, i)
		}
	}
	for id, loc := range x.Cargo.Units {
		if loc != old.Cargo.Units[id] {
			m.Cargo = append(m.Cargo, catanExplorerCargoMotion{Unit: id, From: old.Cargo.Units[id], To: loc})
		}
	}
	for id, loc := range x.Cargo.Fish {
		if loc != old.Cargo.Fish[id] {
			m.Fish = append(m.Fish, catanExplorerFishMotion{Fish: id, From: old.Cargo.Fish[id], To: loc})
		}
	}
	if a.Type == "catan_explorer_fish_roll" && x.Fish != nil {
		m.FishRoll = clone(x.Fish.LastRoll)
	}
	x.Motion = m
}
