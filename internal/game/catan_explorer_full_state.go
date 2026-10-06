package game

// Private full-scenario acceptance constructor. Six lair numbers remain explicit
// pending physical component verification; public recipes are still closed.
func newCatanExplorerFullState(players int, numbers []int) (*State, error) {
	return newCatanExplorerMissionState(players, "explorers-and-pirates", "variable", numbers)
}
