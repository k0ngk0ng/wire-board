package engine

import "testing"

func TestDoorUseMovementAndRecloseE2E(t *testing.T) {
	door := Brush{ID: 1, Kind: "func_door", Bounds: Bounds{Min: Vec3{50 * Unit, -32 * Unit, 0}, Max: Vec3{60 * Unit, 32 * Unit, 72 * Unit}}, Move: Vec3{Y: 96 * Unit}, Speed: 100 * Unit, Wait: 4 * FPS}
	state := &State{Options: Options{RoundSeconds: 120, MaxRounds: 2}, Phase: "live", PhaseEndsAt: 120 * FPS, World: Map{Root: ContentsEmpty, Brushes: []Brush{door}}, Brushes: []BrushState{{ID: 1}}, Players: []Player{{Seat: 0, Team: "ct", Alive: true, Health: 100, Life: 1, Z: 36 * Unit, Grounded: true}, {Seat: 1, Team: "t", Alive: true, Health: 100, Life: 1, X: 2000 * Unit}}}
	start, end := Vec3{Z: 36 * Unit}, Vec3{X: 100 * Unit, Z: 36 * Unit}
	if TraceWorld(state, start, end, -1).Fraction == NormalUnit {
		t.Fatal("closed door did not block")
	}
	if err := QueueInput(state, 0, Input{Seq: 1, Life: 1, Use: true}); err != nil {
		t.Fatal(err)
	}
	Advance(state, 120)
	if !state.Brushes[0].Open || state.Brushes[0].Progress != 1000 || TraceWorld(state, start, end, -1).Fraction != NormalUnit {
		t.Fatal("door did not move clear")
	}
	Advance(state, 700)
	if state.Brushes[0].Open || state.Brushes[0].Progress != 0 {
		t.Fatal("door did not close after wait")
	}
}

func TestOriginalRotatingDoorAwayFromActivatorE2E(t *testing.T) {
	world := MapByID("cs_assault")
	doorIndex := -1
	for i, b := range world.Brushes {
		if b.ID == 11 {
			doorIndex = i
			break
		}
	}
	if doorIndex < 0 {
		t.Fatal("original rotating door absent")
	}
	door := world.Brushes[doorIndex]
	for _, entry := range []struct {
		name   string
		x      int
		want   int
		oneway bool
	}{{"east", door.Bounds.Max.X + 20*Unit, -1, false}, {"west", door.Bounds.Min.X - 20*Unit, 1, false}, {"oneway", door.Bounds.Max.X + 20*Unit, 1, true}} {
		t.Run(entry.name, func(t *testing.T) {
			state := &State{World: world, Phase: "live", Players: []Player{{Seat: 0, Alive: true, Health: 100, Life: 1, X: entry.x, Y: (door.Bounds.Min.Y + door.Bounds.Max.Y) / 2, Z: 36 * Unit, Input: Input{Use: true}}}}
			state.World.Brushes = append([]Brush(nil), world.Brushes...)
			if entry.oneway {
				state.World.Brushes[doorIndex].Flags |= 16
			}
			for _, b := range world.Brushes {
				state.Brushes = append(state.Brushes, BrushState{ID: b.ID, Open: b.Open, Health: b.Health})
			}
			for frame := int64(1); frame <= 100; frame++ {
				state.Frame = frame
				stepBrushes(state)
			}
			dynamic := state.Brushes[doorIndex]
			if dynamic.Direction != entry.want || !dynamic.Open || dynamic.Progress != 1000 {
				t.Fatalf("wrong opening %+v", dynamic)
			}
			if entry.name == "east" {
				start := Vec3{200014, 564387, 9224}
				end := Vec3{164562, 565414, 9224}
				if trace := TraceWorld(state, start, end, 0); trace.StartSolid || trace.Fraction != NormalUnit {
					t.Fatalf("original entrance remained blocked after away opening %+v", trace)
				}
				leafStart := Vec3{door.Origin.X - 32*Unit, door.Origin.Y - 24*Unit, 36 * Unit}
				leafEnd := Add(leafStart, Vec3{Y: 48 * Unit})
				if trace := TraceWorld(state, leafStart, leafEnd, -1); trace.Brush != doorIndex {
					t.Fatalf("opened door leaf lost collision %+v", trace)
				}
			}
			old := dynamic.Direction
			state.Frame++
			stepBrushes(state)
			if state.Brushes[doorIndex].Direction != old || !state.Brushes[doorIndex].Open {
				t.Fatal("repeated use reversed an auto-return door")
			}
		})
	}
}
