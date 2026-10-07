package game

// 2025 base components plus the 5–6 extension, p2. Inventory is selected
// from the original player count, including players who later leave.
// Each scenario uses these counts for both private and public room recipes.
type catanExplorerInventory struct {
	resources, gold, fish, spice, lairs int
}

func catanExplorerStock(players int) catanExplorerInventory {
	if players > 4 {
		return catanExplorerInventory{24, catanExplorerGoldSupply + 6 + 6*3, 8, 36, 8}
	}
	return catanExplorerInventory{19, catanExplorerGoldSupply, 6, 24, 6}
}
