package engine_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/hotlinemiami/engine"
)

func TestRecoveredMapRulesMatchSource(t *testing.T) {
	raw, err := os.ReadFile("../data/maps.json")
	if err != nil {
		t.Fatal(err)
	}
	var source []engine.Map
	if err = json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	if len(source) != 6 || len(engine.Maps()) != 6 {
		t.Fatal("six recovered maps must be registered")
	}
	for _, original := range source {
		world := engine.MapByID(original.ID)
		if world.Width != original.Width*engine.Unit || world.Height != original.Height*engine.Unit || len(world.Walls) != len(original.Walls) || len(world.Doors) != len(original.Doors) || len(world.Spawns) != len(original.Spawns) {
			t.Fatalf("native rules differ for %s", original.ID)
		}
		for i, wall := range original.Walls {
			native := world.Walls[i]
			if native.X != wall.X*engine.Unit || native.Y != wall.Y*engine.Unit || native.W != wall.W*engine.Unit || native.H != wall.H*engine.Unit || native.Kind != wall.Kind {
				t.Fatalf("wall source mismatch %s/%d", original.ID, i)
			}
		}
		for i, point := range original.Spawns {
			if world.Spawns[i].X != point.X*engine.Unit || world.Spawns[i].Y != point.Y*engine.Unit {
				t.Fatalf("spawn source mismatch %s/%d", original.ID, i)
			}
		}
		for i, door := range original.Doors {
			native := world.Doors[i]
			if native.ID != door.ID || native.X != door.X*engine.Unit || native.Y != door.Y*engine.Unit || native.W != door.W*engine.Unit || native.H != door.H*engine.Unit {
				t.Fatalf("door source mismatch %s/%d", original.ID, i)
			}
		}
		if len(world.WeaponSpawns) != len(original.WeaponSpawns) {
			t.Fatalf("weapon spawn count mismatch %s", original.ID)
		}
		for i, point := range original.WeaponSpawns {
			native := world.WeaponSpawns[i]
			if native.X != point.X*engine.Unit || native.Y != point.Y*engine.Unit || native.Weapon != point.Weapon {
				t.Fatalf("weapon spawn source mismatch %s/%d", original.ID, i)
			}
		}
		for _, count := range []int{2, 3, 4} {
			s, err := engine.New(engine.Options{Map: world.ID, RoundSeconds: 180}, count, 91)
			if err != nil {
				t.Fatalf("map %s players %d: %v", world.ID, count, err)
			}
			engine.Advance(s, 1)
		}
	}
}

func TestRecoveredMapComputerMatches(t *testing.T) {
	for mapIndex, world := range engine.Maps() {
		for _, count := range []int{2, 3, 4} {
			t.Run(world.ID+string(rune('0'+count)), func(t *testing.T) {
				weapons := make([]string, count)
				characters := make([]string, count)
				for seat := range weapons {
					weapons[seat] = engine.Weapons()[(mapIndex*3+count+seat)%len(engine.Weapons())].ID
					characters[seat] = engine.Characters()[(mapIndex+count+seat)%len(engine.Characters())]
				}
				s, err := engine.New(engine.Options{Map: world.ID, RoundSeconds: 180, Weapons: weapons, Characters: characters}, count, 91)
				if err != nil {
					t.Fatal(err)
				}
				for !s.Finished {
					if s.Frame%6 == 0 {
						for seat := range s.Players {
							input(t, s, seat, engine.BotInput(s, seat))
						}
					}
					engine.Step(s)
				}
				kills := 0
				for _, p := range s.Players {
					kills += p.Kills
				}
				if kills == 0 {
					t.Fatal("computers never reached combat on recovered map")
				}
				t.Logf("map=%s players=%d frames=%d kills=%d weapons=%v characters=%v", world.ID, count, s.Frame, kills, weapons, characters)
			})
		}
	}
}
