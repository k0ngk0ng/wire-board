package contra

// Before the shared producer fix: do not retain a stale graphic locator after a
// terrain query, omit writes for Y>=224, overwrite the low byte on EB99 overflow,
// reorder slots, or suppress Guardian clearing after graphics refusal. Native
// write traces are evidence only; they must never become independent inputs.
import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type alienLocatorTrace struct {
	Cases []struct {
		Name  string
		Setup struct {
			Category        string
			Draw            bool
			GuardianRoutine uint8
		}
	}
	Records []struct {
		ID    int
		Slots []struct {
			Slot, Kind, Routine, WorkWriteCount int
			Locator                             uint16
		}
		TerrainWorkWrites []struct {
			PC                         int
			Bank, Slot, Address, Value uint8
		}
		LocatorReads []struct {
			PC, WorkWriteCount int
			Bank, Slot         uint8
			Value              uint16
		}
	}
}

func alienLocatorFixture(t *testing.T) (enemyTickOracle, alienLocatorTrace) {
	t.Helper()
	d := loadEnemyTicks(t, "aliens-locator")
	f, err := os.Open("testdata/original-enemy-collision-tick-aliens-locator.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var trace alienLocatorTrace
	if err = json.NewDecoder(z).Decode(&trace); err != nil {
		t.Fatal(err)
	}
	if len(d.Records) != 74 || len(trace.Records) != 74 || len(trace.Cases) != 74 {
		t.Fatal("mixed locator source coverage")
	}
	return d, trace
}

func TestOriginalAlienLocatorProducerEvidence(t *testing.T) {
	d, trace := alienLocatorFixture(t)
	counts := map[string]int{}
	writes := 0
	calls := 0
	for index, r := range trace.Records {
		c := trace.Cases[index]
		counts[c.Setup.Category]++
		writes += len(r.TerrainWorkWrites)
		calls += len(r.Slots)
		w := enemyTickWorld(d, d.Records[index].Before.World)
		last := uint16(w["locatorHigh"])<<8 | uint16(w["locatorLow"])
		guard := r.Slots[len(r.Slots)-1]
		producer := r.Slots[len(r.Slots)-2]
		if guard.Slot != 13 || guard.Kind != 16 || producer.Slot != 14 || guard.Routine != int(c.Setup.GuardianRoutine) {
			t.Fatalf("%s source slot order", c.Name)
		}
		producerWrites := 0
		low, high := false, false
		for _, write := range r.TerrainWorkWrites[:guard.WorkWriteCount] {
			switch write.Address {
			case 0x12:
				last = last&0xff00 | uint16(write.Value)
			case 0x13:
				last = last&255 | uint16(write.Value)<<8
			default:
				t.Fatal("unexpected traced byte")
			}
			if write.Slot == 14 {
				producerWrites++
				if write.Bank != 0x10 {
					t.Fatal("wrong source bank marker")
				}
				low = low || write.PC == 0xe036 && write.Address == 0x12
				high = high || write.PC == 0xe038 && write.Address == 0x13
			}
		}
		if last != guard.Locator || len(r.LocatorReads) != 1 {
			t.Fatalf("%s no observed producer-to-consumer path", c.Name)
		}
		read := r.LocatorReads[0]
		consumed := last
		extraWrites := 0
		if c.Setup.GuardianRoutine == 11 {
			consumed -= 0x20
			extraWrites = 2
		}
		if read.PC != 0xb686 || read.Bank != 0x10 || read.Slot != 13 || read.Value != consumed || read.WorkWriteCount != guard.WorkWriteCount+extraWrites {
			t.Fatalf("%s native clear consumed another value", c.Name)
		}
		if strings.Contains(c.Setup.Category, "overflow") {
			if producerWrites != 1 || low || high {
				t.Fatalf("%s overflow incorrectly performed a full lookup", c.Name)
			}
			write := r.TerrainWorkWrites[guard.WorkWriteCount-1]
			if write.PC != 0xeba1 || write.Address != 0x13 || guard.Locator&255 != producer.Locator&255 {
				t.Fatalf("%s overflow did not retain low byte", c.Name)
			}
		} else if !low || !high || producerWrites != 5 {
			t.Fatalf("%s full lookup outputs were not observed", c.Name)
		}
		requests := d.Records[index].Requests
		for _, request := range requests[len(requests)-2:] {
			if request.Used != 64 {
				t.Fatalf("%s Guardian graphic budget was not full", c.Name)
			}
		}
		if c.Setup.Draw {
			if len(r.Slots) != 3 || len(requests) != 4 || requests[0].Used != 10 || requests[1].Used != 37 || producer.WorkWriteCount != 12 || producer.Locator == guard.Locator {
				t.Fatalf("%s accepted graphics were not overwritten before Guardian", c.Name)
			}
			write := r.TerrainWorkWrites[producer.WorkWriteCount-1]
			if write.Slot != 15 || write.PC != 0xc6ce || write.Bank != 0x16 {
				t.Fatalf("%s missing accepted graphics locator producer", c.Name)
			}
		}
		if d.Records[index].Before.Collision == d.Records[index].After.Collision {
			t.Fatalf("%s original refusal did not modify collision", c.Name)
		}
	}
	for category, count := range map[string]int{"query-normal": 36, "query-high-y": 16, "probe-overflow": 16, "graphics-overwritten": 4, "graphics-overflow": 2} {
		if counts[category] != count {
			t.Fatal("missing source category", category, counts)
		}
	}
	if writes != 1332 || calls != 154 {
		t.Fatal("source trace count changed", writes, calls)
	}
	t.Log("74 original mixed phases, 154 entity calls, 1332 address writes prove the ordered handoff")
}

func TestOriginalMixedAlienLocatorHandoff(t *testing.T) {
	d, trace := alienLocatorFixture(t)
	raw, err := os.ReadFile("leveldata/stage-8.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	tileset, err := resources.Graphics(7, RegularMap)
	if err != nil {
		t.Fatal(err)
	}
	fields := playerOracleData{PlayerFields: d.PlayerFields}
	for index, r := range d.Records {
		t.Run(trace.Cases[index].Name, func(t *testing.T) {
			w, after := enemyTickWorld(d, r.Before.World), enemyTickWorld(d, r.After.World)
			decode := func(value string, n int) []uint8 {
				b, e := hex.DecodeString(value)
				if e != nil || len(b) != n {
					t.Fatal("invalid source buffer", n, e)
				}
				return b
			}
			players := [2]Player{playerFromOracle(fields, r.Before.Players[0]), playerFromOracle(fields, r.Before.Players[1])}
			inputs := [2]PlayerInput{}
			for i, p := range r.Before.Players {
				v := oraclePlayerValues(fields, p)
				inputs[i] = PlayerInput{Held: Buttons(v["held"]), Pressed: Buttons(v["pressed"])}
			}
			weapons := originalWeaponPool(t, r.Before.Weapons)
			scores := arcadeScoresFromOracle(t, r.Before.Score)
			terrain := CollisionWindow(decode(r.Before.Collision, 128))
			mapped := [128]uint8(decode(r.Before.Supertiles, 128))
			encoded := decode(r.Before.Graphics, 128)
			h := decode(r.Before.Header, 15)
			header := PaletteState{CollisionLimits: [3]uint8(h[:3]), CycleIndices: [4]uint8(h[3:7]), PaletteIndices: [8]uint8(h[7:])}
			palette := PaletteOutput{FadeTimer: w["paletteTimer"], PauseCycle: w["palettePause"], FlashTimer: w["flash"], Cycle: w["paletteCycle"], RequestedColors: w["paletteRequest"]}
			copy(palette.Desired[:], decode(r.Before.Palette, 32))
			q := GraphicsQueue{Used: w["queueUsed"]}
			requests := 0
			address := uint16(w["locatorHigh"])<<8 | uint16(w["locatorLow"])
			cleared, tank, bossScroll, auto, pace := w["screenCleared"], w["tankScroll"], w["bossScrollComplete"], w["autoScrollB"], w["presentationPace"]
			c := EnemyStepContext{Level: 7, SceneType: w["sceneType"], Frame: w["frame"], FrameScroll: w["frameScroll"], GameLoop: w["gameLoop"], WeaponStrength: w["weaponStrength"], AttackEnabled: w["attackEnabled"] != 0, Screen: w["screen"], ScreenScroll: w["screenScroll"], SpawnVariantSeed: w["generationBias"], ScreenCleared: &cleared, TankScroll: &tank, Terrain: &terrain, SolidBulletBackground: w["solidFlags"]&0x80 != 0, SolidPickupBackground: w["solidFlags"]&1 != 0,
				Scroll: CollisionScroll{X: w["scrollX"], Y: w["scrollY"], NameTable: w["control"]}, Scene: EnemySceneContext{BossScrollComplete: &bossScroll, Palette: &palette, Header: &header}, AlienBoss: AlienBossContext{AutoScroll: &auto, PresentationPace: &pace, CollisionAddress: &address}}
			random := RandomState{Value: w["random"]}
			c.SharedRandom = &random
			c.OnBossControl = func(BossControl) error { t.Fatal("unexpected terminal Boss control"); return nil }
			c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
				if requests >= len(r.Requests) {
					t.Fatalf("extra independent graphic request %+v", req)
				}
				want := r.Requests[requests]
				requests++
				if req.X != want.X || req.Y != want.Y || req.Code != want.Code || q.Used != want.Used || want.Tile {
					t.Fatalf("graphic request %+v/%d want %+v", req, q.Used, want)
				}
				v, e := q.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tileset})
				// Only the real graphics operation may update its locator here.
				// Native write traces are deliberately never replayed into the engine.
				if v.Accepted {
					address = v.Address
				}
				return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address, AttributeLookupWritten: v.AttributeLookupWritten, AttributeLookup: v.AttributeLookup}, e
			}
			s := EnemySystem{Slots: enemyPoolFromOracle(r.Before.Enemies), ScreenReadOffset: w["screenReadOffset"], Base: BaseEnemyState{w["cores"], w["plating"], w["attackRounds"], w["redSoldier"], w["grenadeLauncher"]}, BossPartsDestroyed: w["bossParts"], AlienAimIndex: w["alienAimIndex"], JointScroll: w["jointScroll"]}
			collisions := EnemyCollisionContext{Players: &players, Inputs: inputs, Weapons: &weapons, Scores: &scores, Level: 7, SceneType: c.SceneType, Demo: w["demo"] != 0, Flash: w["flash"]}
			result, err := s.Tick(&c, &collisions)
			if err != nil {
				t.Fatal(err)
			}
			want := enemyPoolFromOracle(r.After.Enemies)
			if s.Slots != want {
				for i, e := range s.Slots {
					if e != want[i] {
						t.Fatalf("pool slot%d got%+v want%+v", i, e, want[i])
					}
				}
			}
			for i, p := range players {
				expected := oraclePlayerValues(fields, r.After.Players[i])
				for name, value := range playerAsOracle(p, inputs[i]) {
					if value != expected[name] {
						t.Fatalf("player%d %s %d/%d", i, name, value, expected[name])
					}
				}
			}
			if weapons != originalWeaponPool(t, r.After.Weapons) || scores != arcadeScoresFromOracle(t, r.After.Score) {
				t.Fatal("projectile/score state differs")
			}
			if address != uint16(after["locatorHigh"])<<8|uint16(after["locatorLow"]) {
				t.Errorf("final locator got %04x want %02x%02x; prior slot's native query must survive rejected Guardian draws", address, after["locatorHigh"], after["locatorLow"])
			}
			if !bytes.Equal(terrain[:], decode(r.After.Collision, 128)) {
				t.Error("Guardian cleared different collision cells after the preceding terrain producer")
			}
			at := int(w["queueUsed"])
			for _, packet := range q.Packets {
				b, e := packet.Encode()
				if e != nil {
					t.Fatal(e)
				}
				copy(encoded[at:], b)
				at += len(b)
			}
			if requests != len(r.Requests) || q.Used != after["queueUsed"] || !bytes.Equal(encoded, decode(r.After.Graphics, 128)) || !bytes.Equal(mapped[:], decode(r.After.Supertiles, 128)) {
				t.Fatal("graphics output differs")
			}
			if result.AudioReset != r.AudioReset || !bytes.Equal(result.Sounds, r.Sounds) || len(s.PendingSounds) != 0 || result.Flash != after["flash"] || c.SpawnVariantSeed != after["generationBias"] || s.GenerationBiasChanged {
				t.Fatal("audio/flash/generation handoff differs")
			}
			if cleared != after["screenCleared"] || tank != after["tankScroll"] || bossScroll != after["bossScrollComplete"] || auto != after["autoScrollB"] || pace != after["presentationPace"] || random.Value != after["random"] || s.ScreenReadOffset != after["screenReadOffset"] || s.AlienAimIndex != after["alienAimIndex"] || s.BossPartsDestroyed != after["bossParts"] {
				t.Fatal("unrelated shared World state differs")
			}
			headerBytes := append(append(append([]uint8{}, header.CollisionLimits[:]...), header.CycleIndices[:]...), header.PaletteIndices[:]...)
			if !bytes.Equal(headerBytes, decode(r.After.Header, 15)) || !bytes.Equal(palette.Desired[:], decode(r.After.Palette, 32)) {
				t.Fatal("palette/header differs")
			}
		})
	}
}
