package bubbleparty

// Failure inventory before implementation:
// missing source maps; invalid or mutable shared grids; 13x11 map displaced;
// multi-cell obstacles expanded down/left or truncated to one cell; original
// spawn blocked by real footprints; insufficient/duplicate player positions;
// directional doors applied in wrong direction or accidentally ignored for
// water; cleared objects retaining collision; teleport loops/blocked endpoints;
// rail speed applied to all terrain; unknown factory flags interpreted as death.
import "testing"

func TestClassicAllMapsAndPlayerCounts(t *testing.T) {
	ids := classicMapIDs()
	if len(ids) != 112 {
		t.Fatalf("maps=%d", len(ids))
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatal("duplicate", id)
		}
		seen[id] = true
		cells, ok := classicCells(id)
		if !ok || len(cells) != Cols*Rows {
			t.Fatal("bad grid", id)
		}
		objects := classicObjects(id)
		if len(objects) != Cols*Rows {
			t.Fatal("bad objects", id)
		}
		for _, n := range []int{1, 2, 3, 4} {
			occupied := map[[2]int]bool{}
			for team := 0; team < 2; team++ {
				spawns, ok := classicSpawns(id, n, team)
				if !ok || len(spawns) != n {
					t.Fatalf("%s %d team%d spawns %v", id, n, team, spawns)
				}
				for _, p := range spawns {
					if p[0] < 0 || p[0] >= Cols || p[1] < 0 || p[1] >= Rows || cells[p[1]*Cols+p[0]] != CellEmpty || occupied[p] {
						t.Fatalf("%s invalid spawn %v", id, p)
					}
					occupied[p] = true
				}
			}
		}
		cells[0] = 99
		objects[0] = 99
		again, _ := classicCells(id)
		if again[0] == 99 || classicObjects(id)[0] == 99 {
			t.Fatal("shared data mutated")
		}
	}
}
func TestClassicVillageCenteredAndFootprints(t *testing.T) {
	cells, _ := classicCells("classic2002-village00")
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			if (x == 0 || y == 0 || x == Cols-1 || y == Rows-1) && cells[y*Cols+x] != CellWall {
				t.Fatal("missing padded wall", x, y)
			}
		}
	}
	if cells[1*Cols+1] != CellEmpty {
		t.Fatal("original origin not shifted 1,1")
	}
	// Fountain is 3x1. Cabin is 4x2 and extends upward from its source anchor.
	found := false
	for _, m := range classicData.Maps {
		for y, row := range m.Objects {
			for x, id := range row {
				if id == 161 {
					grid, _ := classicCells(m.ID)
					ox, oy := classicOffset(m)
					for dy := -1; dy <= 0; dy++ {
						for dx := 0; dx < 4; dx++ {
							if grid[(y+oy+dy)*Cols+x+ox+dx] != CellWall {
								t.Fatal("cabin footprint missing", m.ID, x, y, dx, dy)
							}
						}
					}
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("missing source cabin fixture")
	}
}
func TestNativeDirectionalPassageAndClearing(t *testing.T) {
	cells, _ := classicCells("classic2002-desert00")
	s := State{MapID: "classic2002-desert00", Cells: cells, NativeObjects: make([]int, Cols*Rows)}
	for i := range s.NativeObjects {
		s.NativeObjects[i] = -1
	}
	s.NativeObjects[6*Cols+7] = 119 // TentBlue only permits up/down.
	if s.nativePassage(6, 6, 1, 0, false) || s.nativePassage(7, 6, 1, 0, true) {
		t.Fatal("side of tent is open")
	}
	if !s.nativePassage(7, 5, 0, 1, false) || !s.nativePassage(7, 6, 0, -1, true) {
		t.Fatal("tent vertical passage blocked")
	}
	s.clearNativeObject(7, 6)
	if !s.nativePassage(6, 6, 1, 0, false) || s.NativeObjects[6*Cols+7] != -1 {
		t.Fatal("cleared object retained gate")
	}
	s.NativeObjects[6*Cols+7] = 85 // Herb is destructible by water, not a gate.
	if !s.nativePassage(6, 6, 1, 0, true) {
		t.Fatal("herb blocked water")
	}
	if s.nativePassage(0, 0, -1, 0, false) || s.nativePassage(7, 6, 1, 1, false) {
		t.Fatal("invalid move accepted")
	}
}
func TestNativeTransportOccupancyAndCooldown(t *testing.T) {
	cells, _ := classicCells("classic2002-village00")
	s := State{MapID: "classic2002-village00", Cells: cells, NativeObjects: classicObjects("classic2002-village00"), Players: []Player{{Seat: 0, Alive: true, X: 2, Y: 10}, {Seat: 1, Alive: true, X: 2, Y: 2}}}
	p := &s.Players[0]
	s.nativeTransport(p, 10000)
	if p.Y != 10 {
		t.Fatal("teleported into player")
	}
	s.Players[1].X = 3
	// The retained source pairs actually sit under hard trees, not accessible
	// portal artwork. Keep them blocked; only clear an isolated mechanic fixture.
	s.nativeTransport(p, 10000)
	if p.Y != 10 || cells[2*Cols+2] != CellWall || cells[10*Cols+2] != CellWall {
		t.Fatal("dormant source link bypassed original trees")
	}
	cells[2*Cols+2], cells[10*Cols+2] = CellEmpty, CellEmpty
	s.clearNativeObject(2, 2)
	s.clearNativeObject(2, 10)
	s.nativeTransport(p, 10000)
	if p.X != 2 || p.Y != 2 || p.TeleportUntil <= 10000 {
		t.Fatal("paired portal failed")
	}
	s.nativeTransport(p, 10001)
	if p.Y != 2 {
		t.Fatal("portal bounced during cooldown")
	}
	s.nativeTransport(p, p.TeleportUntil+1)
	if p.Y != 2 {
		t.Fatal("standing on destination looped")
	}
}
func TestNativeRailSpeedAndConveyor(t *testing.T) {
	foundRail, foundBelt := false, false
	for _, m := range classicData.Maps {
		ox, oy := classicOffset(m)
		s := State{MapID: m.ID}
		for y, row := range m.Ground {
			for x := range row {
				f := s.nativeGroundFields(x+ox, y+oy)
				if len(f) < 10 {
					continue
				}
				if f[8] != 0 {
					p := Player{Mount: "railcar", X: x + ox, Y: y + oy, SpeedMS: 150}
					if s.moveMS(&p) != 120 {
						t.Fatal("rail did not accelerate")
					}
					p.Mount = "turtle"
					if s.moveMS(&p) != 600 {
						t.Fatal("rail accelerated turtle")
					}
					foundRail = true
				}
				if f[3] == 2 {
					if d := s.nativeConveyorDirection(x+ox, y+oy); d == "none" {
						t.Fatal("belt lost direction")
					}
					foundBelt = true
				}
			}
		}
	}
	if !foundRail || !foundBelt {
		t.Fatal("missing source terrain fixtures")
	}
}

func TestNativeConveyorMotionSource(t *testing.T) {
	for _, m := range classicData.Maps {
		ox, oy := classicOffset(m)
		s := State{MapID: m.ID}
		for y, row := range m.Ground {
			for x := range row {
				f := s.nativeGroundFields(x+ox, y+oy)
				if len(f) < 6 || f[3] != 2 || f[4] != 8 || f[5] != 3 {
					continue
				}
				p := Player{Alive: true, X: x + ox, Y: y + oy, OffsetY: -0.1}
				dx, dy, distance := s.nativeConveyorMotion(&p)
				if dx != 0 || dy != 1 || distance != float64(StepMS)/30*2/40 {
					t.Fatalf("source belt pull wrong %d,%d %g", dx, dy, distance)
				}
				p.OffsetY = 0
				dx, dy, distance = s.nativeConveyorMotion(&p)
				if dx != 0 || dy != 1 || distance <= 0 {
					t.Fatal("centered belt did not use exit")
				}
				p.Mount = "airship"
				_, _, distance = s.nativeConveyorMotion(&p)
				if distance != 0 {
					t.Fatal("flying mount moved by conveyor")
				}
				return
			}
		}
	}
	t.Fatal("missing source down belt")
}
