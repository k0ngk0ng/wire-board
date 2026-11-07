package contra

import "testing"

// Falling after Down+A tests support before the next gravity/X update and
// has no jumping phase gate. The center may land at a platform's first pixel.
func TestBotDropFindsNativeFirstContactAtPlatformEdge(t *testing.T) {
	o := BotObservation{Actor: 0, Seats: 1, Phase: "battle", GridKnown: true, ViewX: 8}
	o.Players[0] = BotPlayer{X: 80, Y: 84, Sprite: 13, Visible: true, Alive: true}
	for y := range o.TerrainKnown {
		for x := range o.TerrainKnown[y] {
			o.TerrainKnown[y][x] = true
		}
	}
	o.Terrain[6][5] = TerrainPlatform
	o.Terrain[13][5] = TerrainPlatform
	o.Terrain[8][6], o.Terrain[8][7] = TerrainPlatform, TerrainPlatform
	var terrain CollisionWindow
	for y, row := range o.Terrain {
		for x, kind := range row {
			if x >= 16 {
				continue
			}
			v := uint8(kind)
			if kind == TerrainSolid {
				v = 3
			}
			terrain[y*4+x/4] |= v << uint(6-2*(x%4))
		}
	}
	p := Player{State: 1, X: 80, Sprite: 13}
	p.setY(84)
	world := OutdoorWorld{Level: 0, StopScroll: 0xff, Scroll: CollisionScroll{X: 8}}
	work := BaseThrowWork{}
	landed := false
	for tick := 0; tick < 96; tick++ {
		held, pressed := ButtonRight, Buttons(0)
		if tick == 0 {
			held = ButtonDown | ButtonA
			pressed = ButtonA
		}
		_, e := p.StepOutdoor(&PlayerStepContext{World: &world, Terrain: &terrain, ThrowWork: &work, Input: PlayerInput{Held: held, Pressed: pressed}, Fire: func(FireRequest) FireResult { return FireResult{} }})
		if e != nil {
			t.Fatal(e)
		}
		world.Frame++
		if tick > 0 && p.Fall == 0 && p.X >= 88 && p.y() < 144 {
			landed = true
			t.Log("native lower edge", tick+1, p.X, p.y())
			break
		}
		if p.y() >= 224 {
			break
		}
	}
	if !landed {
		t.Fatal("native legal drop did not reach edge")
	}
	var b Bot
	if x, y, ok := b.planDropThrough(o); !ok || x < 88 || y >= 144 {
		t.Fatal("drop predictor skipped reachable first platform", x, y, ok)
	}
}
