package contra

import "testing"

// Before body-side probes: the planner can pass through the bottom edge of a
// floating solid while its head is above it. Fixed-camera native legal input
// verifies the barrier; removing it must restore the reachable forward jump.
func TestBotJumpCannotPassThroughVisibleFloatingWall(t *testing.T) {
	o := BotObservation{Actor: 0, Seats: 1, Phase: "battle", GridKnown: true}
	o.Players[0] = BotPlayer{X: 88, Y: 164, Sprite: 13, Alive: true, Visible: true}
	for y := range o.TerrainKnown {
		for x := range o.TerrainKnown[y] {
			o.TerrainKnown[y][x] = true
		}
	}
	o.Terrain[11][5] = TerrainPlatform
	o.Terrain[11][9] = TerrainPlatform
	o.Terrain[11][10] = TerrainPlatform
	o.Terrain[7][7] = TerrainSolid
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
	p := Player{State: 1, X: 88, Sprite: 13}
	p.setY(164)
	world := OutdoorWorld{Level: 0, StopScroll: 0xff}
	work := BaseThrowWork{}
	fell := false
	for tick := 0; tick < 96; tick++ {
		held, pressed := ButtonRight, Buttons(0)
		if tick == 0 {
			held |= ButtonA
			pressed = ButtonA
		}
		_, err := p.StepOutdoor(&PlayerStepContext{World: &world, Terrain: &terrain, ThrowWork: &work, Input: PlayerInput{Held: held, Pressed: pressed}, Fire: func(FireRequest) FireResult { return FireResult{} }})
		if err != nil {
			t.Fatal(err)
		}
		world.Frame++
		if p.X >= 144 && p.Motion.Jump == 0 && p.y() < 224 {
			t.Fatal("native fixture unexpectedly reached the far platform")
		}
		if p.y() >= 224 {
			fell = true
			t.Log("native fall", tick+1, p.X, p.y())
			break
		}
	}
	if !fell {
		t.Fatal("native fixture did not establish unreachable landing")
	}
	var b Bot
	if x, y, _, ok := b.planJump(o, false); ok && x >= 144 {
		t.Fatal("visual planner crosses solid body-side wall", x, y)
	}
	o.Terrain[7][7] = TerrainEmpty
	if x, _, _, ok := b.planJump(o, false); !ok || x < 144 {
		t.Fatal("removing wall did not restore forward route", x, ok)
	}
}
