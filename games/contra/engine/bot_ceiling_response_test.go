package contra

import "testing"

// Original rising head contact clears vertical speed and continues falling;
// it does not invalidate a route that reaches a visible lower platform.
func TestBotJumpCanLandAfterVisibleCeilingContact(t *testing.T) {
	o := BotObservation{Actor: 0, Seats: 1, Phase: "battle", GridKnown: true}
	o.Players[0] = BotPlayer{X: 92, Y: 100, Sprite: 13, Alive: true, Visible: true}
	for y := range o.TerrainKnown {
		for x := range o.TerrainKnown[y] {
			o.TerrainKnown[y][x] = true
		}
	}
	o.Terrain[7][5] = TerrainPlatform
	o.Terrain[9][7], o.Terrain[9][8] = TerrainPlatform, TerrainPlatform
	for x := 5; x <= 9; x++ {
		o.Terrain[4][x] = TerrainSolid
	}
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
	p := Player{State: 1, X: 92, Sprite: 13}
	p.setY(100)
	world := OutdoorWorld{Level: 0, StopScroll: 0xff}
	work := BaseThrowWork{}
	landed := false
	for tick := 0; tick < 96; tick++ {
		held, pressed := ButtonRight, Buttons(0)
		if tick == 0 {
			held |= ButtonA
			pressed = ButtonA
		}
		_, e := p.StepOutdoor(&PlayerStepContext{World: &world, Terrain: &terrain, ThrowWork: &work, Input: PlayerInput{Held: held, Pressed: pressed}, Fire: func(FireRequest) FireResult { return FireResult{} }})
		if e != nil {
			t.Fatal(e)
		}
		world.Frame++
		if p.X >= 112 && p.y() >= 128 && p.y() < 144 && p.Motion.Jump == 0 {
			landed = true
			t.Log("native lower landing", tick+1, p.X, p.y())
			break
		}
		if p.y() >= 224 {
			break
		}
	}
	if !landed {
		t.Fatal("native fixture does not reach lower platform")
	}
	var b Bot
	if x, y, _, ok := b.planJump(o, false); !ok || x < 112 || y < 120 {
		t.Fatal("visible ceiling discards reachable falling route", x, y, ok)
	}
}
