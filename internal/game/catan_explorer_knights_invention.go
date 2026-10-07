package game

import "errors"

func catanInventionNumber(number int) bool {
	return number == 3 || number == 4 || number == 5 || number == 9 || number == 10 || number == 11
}

func (m catanExplorerBoard) numberAt(tile, original int) int {
	if number, ok := m.NumberSwaps[tile]; ok {
		return number
	}
	return original
}

// Drawn numbers record which regional stack supplied a disc. Do not rewrite
// them when Invention moves discs between the starting island and regions.
func (m catanExplorerBoard) originalNumber(base *Catan, tile int) int {
	if base.Tiles[tile].Resource != CatanFog {
		return base.Tiles[tile].Number
	}
	for _, h := range m.Hidden {
		if h.Tile == tile && h.Revealed {
			if h.Resource == CatanGold {
				return m.Liberated[tile]
			}
			return h.Number
		}
	}
	return 0
}

func (m catanExplorerBoard) validateNumberSwaps(g, base *Catan) error {
	before, after := []int{}, []int{}
	for tile, number := range m.NumberSwaps {
		if !m.CitiesKnights || tile < 0 || tile >= len(g.Tiles) {
			return errors.New("数字交换缺少城市骑士规则或地块无效")
		}
		original := m.originalNumber(base, tile)
		if !catanInventionNumber(original) || !catanInventionNumber(number) || original == number || g.Tiles[tile].Number != number {
			return errors.New("发明只能交换已公开的3、4、5、9、10、11数字")
		}
		before, after = append(before, original), append(after, number)
	}
	if !catanExplorerSameInventory(before, after) {
		return errors.New("发明交换的数字库存不守恒")
	}
	return nil
}

// The action dispatcher owns the whole-state transaction, including the card.
func (m *catanExplorerBoard) swapNumbers(g *Catan, left, right int) error {
	if err := m.validate(g); err != nil {
		return err
	}
	if !m.CitiesKnights || g.CitiesKnights == nil || left < 0 || right < 0 || left >= len(g.Tiles) || right >= len(g.Tiles) || left == right || !catanInventionNumber(g.Tiles[left].Number) || !catanInventionNumber(g.Tiles[right].Number) {
		return errors.New("请选择两个已公开且可交换数字的地块")
	}
	base, _, err := catanExplorerGeometryVariant(m.Players, m.Scenario, m.Layout, m.CitiesKnights)
	if err != nil {
		return err
	}
	g.Tiles[left].Number, g.Tiles[right].Number = g.Tiles[right].Number, g.Tiles[left].Number
	if m.NumberSwaps == nil {
		m.NumberSwaps = map[int]int{}
	}
	for _, tile := range []int{left, right} {
		if g.Tiles[tile].Number == m.originalNumber(base, tile) {
			delete(m.NumberSwaps, tile)
		} else {
			m.NumberSwaps[tile] = g.Tiles[tile].Number
		}
	}
	if len(m.NumberSwaps) == 0 {
		m.NumberSwaps = nil
	}
	return m.validate(g)
}
