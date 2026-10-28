package contra

// Failures declared before implementation: resetting per-frame scroll after
// actor updates; wrong actor order or active-player mask; timer decrements before
// firing/animation; losing preserved inactive player data; peer camera correction
// uses total horizontal scroll instead of the original one pixel; missing forced
// autoscroll; bullets advance before players; wrong odd/even display selection;
// an unsupported second actor leaves the first actor's partially advanced state.

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type frameOracleSnapshot struct {
	Players                [2][]uint8
	World                  []uint8
	Projectiles, Collision string
	Display                struct{ Codes, X, Y, Attributes string }
}
type frameOracle struct {
	PlayerFields []struct{ Name, Address string } `json:"player_fields"`
	WorldFields  []struct{ Name, Address string } `json:"world_fields"`
	Scenarios    []struct {
		Name    string
		Records []struct {
			ID, Frame     int
			Before, After frameOracleSnapshot
		}
	}
}

func loadFrameOracle(t *testing.T) frameOracle {
	t.Helper()
	f, err := os.Open("../../../docs/research/nes-contra/evidence/world-phase-original.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var d frameOracle
	if err = json.NewDecoder(z).Decode(&d); err != nil {
		t.Fatal(err)
	}
	if len(d.PlayerFields) != 44 || len(d.WorldFields) != 24 || len(d.Scenarios) != 3 {
		t.Fatal("original frame oracle shape changed")
	}
	return d
}
func combatFromOracle(t *testing.T, d frameOracle, s frameOracleSnapshot) (CombatFrame, [2]PlayerInput) {
	t.Helper()
	meta := playerOracleData{PlayerFields: d.PlayerFields, WorldFields: d.WorldFields}
	var f CombatFrame
	var inputs [2]PlayerInput
	for actor, row := range s.Players {
		f.Players[actor] = playerFromOracle(meta, row)
		v := oraclePlayerValues(meta, row)
		inputs[actor] = PlayerInput{Held: Buttons(v["held"]), Pressed: Buttons(v["pressed"])}
	}
	f.World = worldFromOracle(meta, s.World)
	for i, field := range d.WorldFields {
		switch field.Name {
		case "weaponStrength":
			f.World.WeaponStrength = s.World[i]
		case "vulnerablePlayer":
			f.World.AttackEnabled = s.World[i]
		case "indoorScroll":
			f.World.IndoorScroll = s.World[i]
		case "screenCleared":
			f.ScreenCleared = s.World[i]
		case "solidCollision":
			f.SolidCollision = s.World[i]
		}
	}
	f.Weapons = originalWeaponPool(t, s.Projectiles)
	raw, err := hex.DecodeString(s.Collision)
	if err != nil || len(raw) != 128 {
		t.Fatal("invalid original frame collision")
	}
	f.Terrain = CollisionWindow(raw)
	codes, _ := hex.DecodeString(s.Display.Codes)
	xs, _ := hex.DecodeString(s.Display.X)
	ys, _ := hex.DecodeString(s.Display.Y)
	attrs, _ := hex.DecodeString(s.Display.Attributes)
	if len(codes) != 10 || len(xs) != 10 || len(ys) != 10 || len(attrs) != 10 {
		t.Fatal("invalid original logical sprite buffer")
	}
	for i := range f.Display {
		f.Display[i] = LogicalSprite{Code: codes[i], X: xs[i], Y: ys[i], Attributes: attrs[i]}
	}
	return f, inputs
}
func TestOriginalCombatFramePhase(t *testing.T) {
	d := loadFrameOracle(t)
	checked := 0
	for _, scenario := range d.Scenarios {
		for _, record := range scenario.Records {
			frame, inputs := combatFromOracle(t, d, record.Before)
			want, _ := combatFromOracle(t, d, record.After)
			if _, err := frame.AdvancePlayersAndProjectiles(inputs); err != nil {
				t.Fatalf("%s frame %d: %v", scenario.Name, record.Frame, err)
			}
			if !reflect.DeepEqual(frame, want) {
				if frame.Players != want.Players {
					t.Fatalf("%s frame %d players\ngot=%+v\nwant=%+v", scenario.Name, record.Frame, frame.Players, want.Players)
				}
				if frame.World != want.World {
					t.Fatalf("%s frame %d world got=%+v want=%+v", scenario.Name, record.Frame, frame.World, want.World)
				}
				if frame.Weapons != want.Weapons {
					t.Fatalf("%s frame %d projectile pool\ngot=%+v\nwant=%+v", scenario.Name, record.Frame, frame.Weapons, want.Weapons)
				}
				t.Fatalf("%s frame %d display/collision differs: %+v / %+v", scenario.Name, record.Frame, frame.Display, want.Display)
			}
			checked++
		}
	}
	if checked != 1220 {
		t.Fatal("missing original whole frame phases", checked)
	}
	t.Logf("%d complete player/projectile frame phases match", checked)
}
func TestCombatFrameFailureIsAtomic(t *testing.T) {
	d := loadFrameOracle(t)
	f, input := combatFromOracle(t, d, d.Scenarios[1].Records[0].Before)
	f.Players[1].State = 254
	before := f
	if _, err := f.AdvancePlayersAndProjectiles(input); err == nil {
		t.Fatal("unknown actor phase accepted")
	}
	if !reflect.DeepEqual(f, before) {
		t.Fatal("failed second actor left a partially advanced first actor")
	}
}
