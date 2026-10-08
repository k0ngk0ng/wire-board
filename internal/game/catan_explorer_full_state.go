package game

// Explicit-inventory full-scenario constructor for acceptance fixtures.
// Public creation uses the separately versioned site number recipe.
func newCatanExplorerFullState(players int, numbers []int) (*State, error) {
	return newCatanExplorerMissionState(players, "explorers-and-pirates", "variable", numbers)
}
