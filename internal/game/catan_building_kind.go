package game

// Cities and Explorer harbor settlements both score two points, but only a
// city produces commodities, counts toward barbarian strength, and can hold a
// wall/metropolis. Older standalone Explorer saves stored only Level=2.
func (g *Catan) harborAt(vertex int) bool {
	if vertex < 0 || vertex >= len(g.Vertices) || g.Vertices[vertex].Level != 2 {
		return false
	}
	return g.Vertices[vertex].Harbor || g.Explorer != nil && g.CitiesKnights == nil
}
func (g *Catan) cityAt(vertex int) bool {
	return vertex >= 0 && vertex < len(g.Vertices) && g.Vertices[vertex].Level == 2 && !g.harborAt(vertex)
}

// Cargo kernels are also constructed before the Explorer aggregate is attached.
// In their standalone mode every level-two building is a harbor, including old
// snapshots. A combined Cities & Knights board needs explicit harbor markers.
func catanExplorerHarborAt(g *Catan, vertex int) bool {
	return vertex >= 0 && vertex < len(g.Vertices) && g.Vertices[vertex].Level == 2 && (g.CitiesKnights == nil || g.Vertices[vertex].Harbor)
}
