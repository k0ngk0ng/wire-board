package contra

// Original 8584 -> E54F observations distinguish dispatch families, retained
// inputs, flame writes and the later collision override. Inputs are declared;
// every expected attribute is a real original read at 972E, not a guessed byte.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestOriginalWeaponToBaseThrowWork(t *testing.T) {
	checkOriginalBaseThrowWork(t, "testdata/original-base-throw-work.json.gz", 19)
}
func TestOriginalBaseThrowEarlierSlotProducers(t *testing.T) {
	checkOriginalBaseThrowWork(t, "testdata/original-base-throw-work-producers.json.gz", 27)
}
func checkOriginalBaseThrowWork(t *testing.T, path string, count int) {
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var d struct {
		Original bool `json:"source_is_original_execution"`
		Cases    []struct{ Name string }
		Records  []struct {
			Case                                  int
			BeforeWeapon, BeforeEnemy, AfterEnemy string
			Throws                                []struct {
				Creator, Child int
				Attribute      uint8
			}
		}
	}
	if e = json.NewDecoder(z).Decode(&d); e != nil {
		t.Fatal(e)
	}
	if !d.Original || len(d.Records) != count {
		t.Fatal("wrong source phases")
	}
	meta := loadFrameOracle(t)
	resources := levelResources(t, "stage-4.json")
	for _, row := range d.Records {
		t.Run(d.Cases[row.Case-1].Name, func(t *testing.T) {
			before, entry, after := campaignSceneRAM(t, row.BeforeWeapon), campaignSceneRAM(t, row.BeforeEnemy), campaignSceneRAM(t, row.AfterEnemy)
			work := BaseThrowWork{Known: true, Value: before[0x0a]}
			weapons := originalWeaponPool(t, hex.EncodeToString(before[0x368:0x4b8]))
			terrain := CollisionWindow(before[0x680:0x700])
			scroll := CollisionScroll{X: before[0xfd], Y: before[0xfc], NameTable: before[0xff]}
			weapons.Step(BulletStep{StageType: before[0x40], ScrollType: before[0x41], Scroll: before[0x68], SolidCollision: before[0x59]&0x80 != 0, ScreenCleared: before[0x6a] != 0, ThrowWork: &work, Terrain: func(x, y uint8) TerrainKind { return terrain.At(x, y, scroll).Kind }})
			if !work.Known || work.Value != entry[0x0a] {
				t.Fatal("actual player projectile producer differs", work, entry[0x0a])
			}
			if weapons != originalWeaponPool(t, hex.EncodeToString(entry[0x368:0x4b8])) {
				t.Fatal("metadata integration changed projectile rules")
			}
			world := guardianWorldFromOriginal(t, meta, row.BeforeEnemy)
			a := &world.Active
			w := a.Combat.World
			tiles, e := resources.Graphics(3, RegularMap)
			if e != nil {
				t.Fatal(e)
			}
			animations, e := OriginalTileAnimations(3)
			if e != nil {
				t.Fatal(e)
			}
			q := GraphicsQueue{Used: entry[0x21]}
			c := EnemyStepContext{ThrowWork: &work, Level: 3, SceneType: 1, Frame: w.Frame, FrameScroll: w.FrameScroll, Screen: w.Screen, ScreenScroll: w.ScreenScroll, WeaponStrength: w.WeaponStrength, GameLoop: world.Campaign.Loops, AttackEnabled: w.AttackEnabled != 0, SharedRandom: &a.Random, SpawnVariantSeed: a.GenerationBias, ScreenCleared: &a.Combat.ScreenCleared, Terrain: &a.Combat.Terrain, Scroll: w.Scroll}
			change := func(req EnemyTerrainRequest, tile bool) (EnemyTerrainResponse, error) {
				gc := GraphicsContext{Scroll: c.Scroll, Supertiles: &a.Terrain.Supertiles, Tileset: tiles}
				var r GraphicsRequestResult
				var e error
				if tile {
					r, e = q.RequestTileAnimation(req.X, req.Y, req.Code, gc, animations)
				} else {
					r, e = q.RequestSupertile(req.X, req.Y, req.Code, gc)
				}
				return EnemyTerrainResponse{Accepted: r.Accepted, AddressValid: r.Accepted, Address: r.Address, AttributeLookupWritten: r.AttributeLookupWritten, AttributeLookup: r.AttributeLookup}, e
			}
			c.ChangeTerrain = func(r EnemyTerrainRequest) (EnemyTerrainResponse, error) { return change(r, false) }
			c.ChangeTiles = func(r EnemyTerrainRequest) (EnemyTerrainResponse, error) { return change(r, true) }
			collisions := EnemyCollisionContext{Players: &a.Combat.Players, Weapons: &a.Combat.Weapons, Scores: &a.Scores, Level: 3, SceneType: 1}
			if _, e = a.Enemies.Tick(&c, &collisions); e != nil {
				t.Fatal(e)
			}
			want := guardianWorldFromOriginal(t, meta, row.AfterEnemy)
			if a.Enemies.Slots != want.Active.Enemies.Slots {
				for i, e := range a.Enemies.Slots {
					if e != want.Active.Enemies.Slots[i] {
						t.Fatalf("slot%d got%+v want%+v", i, e, want.Active.Enemies.Slots[i])
					}
				}
			}
			if !work.Known || work.Value != after[0x0a] {
				t.Fatal("enemy phase inherited work differs", work, after[0x0a])
			}
			found := false
			for _, throw := range row.Throws {
				if throw.Creator == 14 {
					found = true
				}
				if a.Enemies.Slots[throw.Child].Attributes != throw.Attribute {
					t.Fatal("native consumer byte not retained", throw)
				}
			}
			if !found {
				t.Fatal("missing actual source consumer")
			}

		})
	}
}
func TestOriginalContinuousBaseThrowWork(t *testing.T) {
	meta := loadFrameOracle(t)
	for _, stage := range []int{2, 4} {
		t.Run(fmt.Sprint(stage), func(t *testing.T) {
			f, e := os.Open(fmt.Sprintf("../../../docs/research/nes-contra/evidence/world-eight-stage-%d-original.json.gz", stage))
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			z, e := gzip.NewReader(f)
			if e != nil {
				t.Fatal(e)
			}
			defer z.Close()
			var d struct {
				Frames []struct{ Before, After string }
			}
			if e = json.NewDecoder(z).Decode(&d); e != nil {
				t.Fatal(e)
			}
			if len(d.Frames) != 800 {
				t.Fatal("incomplete original route")
			}
			r := levelResources(t, fmt.Sprintf("stage-%d.json", stage))
			w := guardianWorldFromOriginal(t, meta, d.Frames[0].Before)
			first := campaignSceneRAM(t, d.Frames[0].Before)
			w.Active.ThrowWork = BaseThrowWork{Known: true, Value: first[0x0a]}
			for i, row := range d.Frames {
				if i > 0 {
					if _, e = w.Active.Presentation.Commit(); e != nil {
						t.Fatal(e)
					}
				}
				b, a := campaignSceneRAM(t, row.Before), campaignSceneRAM(t, row.After)
				w.Active.Random.Value = b[0x34]
				var inputs [2]PlayerInput
				for j := range inputs {
					inputs[j] = PlayerInput{Held: Buttons(b[0xf1+j]), Pressed: Buttons(b[0xf5+j])}
				}
				if _, e = w.Advance(uint64(i), inputs, r); e != nil {
					t.Fatal(i, e)
				}
				if !w.Active.ThrowWork.Known || w.Active.ThrowWork.Value != a[0x0a] {
					t.Fatal("whole World source carry differs", i, w.Active.ThrowWork, a[0x0a])
				}
				want := guardianWorldFromOriginal(t, meta, row.After)
				if !reflect.DeepEqual(w.Active.Enemies.Slots, want.Active.Enemies.Slots) {
					t.Fatal("world enemy output changed", i)
				}
			}
		})
	}
}
