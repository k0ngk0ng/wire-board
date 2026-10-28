package contra

import "testing"

// Original Down+A admission scans the current column. A reachable adjacent
// platform alone cannot authorize a drop. Immediate solid backing also makes
// the supporting platform non-droppable.
func TestBotDropRequiresVisibleSupportInCurrentColumn(t *testing.T) {
	o := BotObservation{Actor: 0, Seats: 1, Phase: "battle", GridKnown: true}
	o.Players[0] = BotPlayer{X: 120, Y: 68, Sprite: 13, Alive: true, Visible: true}
	for y := range o.TerrainKnown {
		for x := range o.TerrainKnown[y] {
			o.TerrainKnown[y][x] = true
		}
	}
	o.Terrain[5][7] = TerrainPlatform
	o.Terrain[9][8], o.Terrain[9][9] = TerrainPlatform, TerrainPlatform
	var b Bot
	if x, y, ok := b.planDropThrough(o); ok {
		t.Fatal("adjacent landing bypasses same-column admission", x, y)
	}
	o.Terrain[13][7] = TerrainPlatform
	if _, _, ok := b.planDropThrough(o); !ok {
		t.Fatal("visible same-column support fails to admit reachable drop")
	}
	o.Terrain[6][7] = TerrainSolid
	if _, _, ok := b.planDropThrough(o); ok {
		t.Fatal("solid-backed platform is treated as droppable")
	}
}
