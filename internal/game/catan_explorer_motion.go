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
	Cargo    []catanExplorerCargoMotion `json:"cargo,omitempty"`
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
	case "catan_explorer_sail", "catan_explorer_transfer", "catan_explorer_settle", "catan_explorer_unit", "catan_explorer_ship", "catan_explorer_harbor":
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
	x.Motion = m
}
