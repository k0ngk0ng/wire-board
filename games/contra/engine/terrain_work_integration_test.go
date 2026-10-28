package contra

// Before actual producer wiring: use real combat/scroll operations, not the
// recorded operation list to drive TerrainWork. Check their shared value at
// native weapon completion and enemy entry; failed phases must not publish it.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalTerrainWorkActualCombatAndScroll(t *testing.T) {
	meta := loadFrameOracle(t)
	resources := levelResources(t, "levels.json")
	checked := 0
	for _, name := range []string{"stage8-full-v1", "stage6-v2", "guardian-cases-v5", "vertical-cases-v1", "indoor-cases-v2"} {
		f, e := os.Open("../../../docs/research/nes-contra/evidence/terrain-work-" + name + ".json.gz")
		if e != nil {
			t.Fatal(e)
		}
		z, e := gzip.NewReader(f)
		if e != nil {
			t.Fatal(e)
		}
		var data struct {
			Frames []struct {
				Frame      int
				Operations []struct {
					Name, AfterRAM string
					After          uint16
					Input          struct {
						RAM  string
						Work uint16
					}
				}
				Checkpoints []struct {
					Name string
					Work uint16
				}
			}
		}
		e = json.NewDecoder(z).Decode(&data)
		z.Close()
		f.Close()
		if e != nil {
			t.Fatal(e)
		}
		for _, row := range data.Frames {
			before := ""
			var initial, wantWeapon, wantEnemy uint16
			hasScroll := false
			for _, op := range row.Operations {
				switch op.Name {
				case "players":
					before = op.Input.RAM
					initial = op.Input.Work
				case "weapons":
					wantWeapon = op.After
				case "scroll":
					hasScroll = true
				}
			}
			for _, cp := range row.Checkpoints {
				if cp.Name == "enemy-entry" {
					wantEnemy = cp.Work
				}
			}
			if before == "" || !hasScroll {
				t.Fatal("incomplete native frame", row.Frame)
			}
			b, _ := hex.DecodeString(before)
			_, s := campaignInitializeFromRAM(t, meta, before)
			// This gate invokes DB81's rectangle producer only. Some staged
			// cases leave $23 in text mode; they do not validate presentation.
			s.Presentation.Queue.Mode = GraphicsRectangular
			if uint16(b[0x42])|uint16(b[0x43])<<8 == 0x90e3 && uint16(b[0x44])|uint16(b[0x45])<<8 == 0xb68a {
				s.Terrain.Variant = BaseBossMap
			}
			work := TerrainWork{Value: initial}
			var inputs [2]PlayerInput
			for i := range inputs {
				inputs[i] = PlayerInput{Held: Buttons(b[0xf1+i]), Pressed: Buttons(b[0xf5+i])}
			}
			if _, e = s.Combat.AdvancePlayersAndProjectilesWithEffects(inputs, &work); e != nil {
				t.Fatal(row.Frame, e)
			}
			if work.Value != wantWeapon {
				t.Fatalf("%s frame%d combat work%04x/%04x", name, row.Frame, work.Value, wantWeapon)
			}
			_, e = s.Terrain.AdvanceQueued(TerrainScrollContext{World: &s.Combat.World, Terrain: &s.Combat.Terrain, Header: &s.Header, ScreenCleared: &s.Combat.ScreenCleared, Queue: &s.Presentation.Queue, Resources: resources, Work: &work})
			if e != nil {
				t.Fatal(row.Frame, e)
			}
			if work.Value != wantEnemy {
				t.Fatalf("%s frame%d enemy-entry work%04x/%04x", name, row.Frame, work.Value, wantEnemy)
			}
			checked++
		}
	}
	if checked != 2677 {
		t.Fatal("incomplete actual producer coverage", checked)
	}
}
