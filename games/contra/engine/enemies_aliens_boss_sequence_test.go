package contra

// Continuous phase acceptance keeps the independent pool, terrain and shared
// Boss fields across calls. The single declared destruction input is applied
// locally; no native after record becomes the next independent initial state.
import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestContinuousOriginalAlienBosses(t *testing.T) {
	for _, spec := range []struct {
		name         string
		ticks, calls int
	}{{"guardian", 1200, 3063}, {"heart", 1000, 1179}} {
		t.Run(spec.name, func(t *testing.T) {
			type state struct {
				Enemies, Players                [][]uint8
				World                           []uint8
				Graphics, Collision, Supertiles string
			}
			var data struct {
				Original      bool     `json:"source_is_original_execution"`
				Fields        [][2]any `json:"world_fields"`
				DestroyedTick int
				Kind          uint8
				Records       []struct {
					ID            int
					Before, After state
					Sounds        []uint8
					AudioResets   int
					Slots         []struct {
						Slot          int
						Kind, Routine uint8
					}
					Requests []struct{ X, Y, Code, Used uint8 }
				}
			}
			f, err := os.Open("testdata/original-enemies-aliens-boss-" + spec.name + "-sequence.json.gz")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			z, err := gzip.NewReader(f)
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			if err = json.NewDecoder(z).Decode(&data); err != nil {
				t.Fatal(err)
			}
			if !data.Original || len(data.Records) != spec.ticks {
				t.Fatal("native sequence coverage")
			}
			world := func(v []uint8) map[string]uint8 {
				m := map[string]uint8{}
				for i, p := range data.Fields {
					m[p[0].(string)] = v[i]
				}
				return m
			}
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
			initial := world(data.Records[0].Before.World)
			s := EnemySystem{Slots: enemyPoolFromOracle(data.Records[0].Before.Enemies), AlienAimIndex: initial["alienAimIndex"]}
			terrainBytes, _ := hex.DecodeString(data.Records[0].Before.Collision)
			terrain := CollisionWindow(terrainBytes)
			live := map[string]uint8{}
			for _, key := range []string{"autoScrollB", "presentationPace", "cycle1", "cycle3", "bossDefeated", "delayLow", "delayHigh", "generationBias", "addressLow", "addressHigh"} {
				live[key] = initial[key]
			}
			count, rejected, resetsTotal := 0, 0, 0
			for _, r := range data.Records {
				if r.ID == data.DestroyedTick {
					s.Slots[15].Routine = 4
				}
				w, after := world(r.Before.World), world(r.After.World)
				expectedBefore, _ := hex.DecodeString(r.Before.Collision)
				if s.Slots != enemyPoolFromOracle(r.Before.Enemies) || s.AlienAimIndex != w["alienAimIndex"] || !bytes.Equal(terrain[:], expectedBefore) {
					t.Fatalf("tick %d independent state drifted", r.ID)
				}
				for key, v := range live {
					if v != w[key] {
						t.Fatalf("tick %d live %s differs before original phase", r.ID, key)
					}
				}
				address := uint16(live["addressHigh"])<<8 | uint16(live["addressLow"])
				auto, pace := live["autoScrollB"], live["presentationPace"]
				header := PaletteState{}
				header.CycleIndices[1] = live["cycle1"]
				header.CycleIndices[3] = live["cycle3"]
				b := AlienBossContext{AutoScroll: &auto, PresentationPace: &pace, CollisionAddress: &address}
				mappedBytes, _ := hex.DecodeString(r.Before.Supertiles)
				mapped := [128]uint8(mappedBytes)
				q := GraphicsQueue{Used: w["queueUsed"]}
				requests, called, resets := 0, 0, 0
				c := EnemyStepContext{Level: 7, GameLoop: w["loop"], Frame: w["frame"], Random: w["random"], FrameScroll: w["frameScroll"], WeaponStrength: w["weaponStrength"], AttackEnabled: w["soldierAllowed"] != 0, Terrain: &terrain, Scroll: CollisionScroll{X: w["scrollX"], Y: w["scrollY"], NameTable: w["ppuControl"]}, Scene: EnemySceneContext{Header: &header}}
				for i, p := range r.Before.Players {
					c.Players[i] = EnemyTarget{State: p[0], X: p[1], Y: p[3], GameOver: p[41]}
				}
				c.OnBossControl = func(control BossControl) error {
					switch control.Kind {
					case BossAudioReset:
						resets++
					case BossBeginDefeat:
						live["bossDefeated"] = 1
						live["delayLow"] = 255
					case BossExitReady:
						live["delayLow"] = control.Delay
						live["delayHigh"] = 0
					default:
						t.Fatal("invalid Boss control")
					}
					return nil
				}
				c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
					if requests >= len(r.Requests) {
						t.Fatalf("tick %d extra draw request", r.ID)
					}
					want := r.Requests[requests]
					requests++
					if req.X != want.X || req.Y != want.Y || req.Code != want.Code || q.Used != want.Used {
						t.Fatalf("tick %d draw request %+v/%d want %+v", r.ID, req, q.Used, want)
					}
					v, e := q.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tileset})
					if !v.Accepted {
						rejected++
					}
					return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address, AttributeLookupWritten: v.AttributeLookupWritten, AttributeLookup: v.AttributeLookup}, e
				}
				s.PendingSounds = nil
				s.GenerationBiasChanged = false
				for slot := 15; slot >= 0; slot-- {
					e := s.Slots[slot]
					if e.Routine == 0 {
						continue
					}
					if called >= len(r.Slots) || r.Slots[called].Slot != slot || r.Slots[called].Kind != e.Kind || r.Slots[called].Routine != e.Routine {
						t.Fatalf("tick %d dispatch slot%d differs", r.ID, slot)
					}
					called++
					count++
					switch e.Kind {
					case 2:
						err = s.StepWeaponBox(slot, c)
					case 16:
						err = s.StepAlienGuardian(slot, c, b)
					case 17:
						err = s.StepAlienFetus(slot, c)
					case 22:
						err = s.StepAlienHeart(slot, c, b)
					default:
						t.Fatalf("unexpected kind %d", e.Kind)
					}
					if err != nil {
						t.Fatalf("tick %d slot %d: %v", r.ID, slot, err)
					}
					if s.GenerationBiasChanged {
						live["generationBias"] = s.GenerationBiasValue
						s.GenerationBiasChanged = false
					}
				}
				want := enemyPoolFromOracle(r.After.Enemies)
				if s.Slots != want {
					for i, e := range s.Slots {
						if e != want[i] {
							t.Fatalf("tick %d slot %d\ngot %+v\nwant %+v", r.ID, i, e, want[i])
						}
					}
				}
				live["autoScrollB"], live["presentationPace"] = auto, pace
				live["cycle1"], live["cycle3"] = header.CycleIndices[1], header.CycleIndices[3]
				live["addressLow"], live["addressHigh"] = uint8(address), uint8(address>>8)
				for key, v := range live {
					if v != after[key] {
						t.Fatalf("tick %d live %s got%02x want%02x", r.ID, key, v, after[key])
					}
				}
				if s.AlienAimIndex != after["alienAimIndex"] || resets != r.AudioResets {
					t.Fatalf("tick %d aim/audio-reset differs", r.ID)
				}
				resetsTotal += resets
				if !reflect.DeepEqual(append([]uint8{}, s.PendingSounds...), append([]uint8{}, r.Sounds...)) {
					t.Fatalf("tick %d sounds %v want %v", r.ID, s.PendingSounds, r.Sounds)
				}
				encoded, _ := hex.DecodeString(r.Before.Graphics)
				at := int(w["queueUsed"])
				for _, packet := range q.Packets {
					raw, e := packet.Encode()
					if e != nil {
						t.Fatal(e)
					}
					copy(encoded[at:], raw)
					at += len(raw)
				}
				expected, _ := hex.DecodeString(r.After.Graphics)
				expectedMap, _ := hex.DecodeString(r.After.Collision)
				if q.Used != after["queueUsed"] || requests != len(r.Requests) || called != len(r.Slots) || !bytes.Equal(encoded, expected) || !bytes.Equal(terrain[:], expectedMap) {
					t.Fatalf("tick %d graphics/collision/call count differs", r.ID)
				}
			}
			if count != spec.calls || rejected == 0 || live["delayLow"] != 0xa0 || live["delayHigh"] != 0 || s.Slots[15].Routine != 0 {
				t.Fatalf("incomplete Boss sequence: calls%d reject%d delay%d/%d", count, rejected, live["delayLow"], live["delayHigh"])
			}
			if data.Kind == 22 && (resetsTotal != 1 || live["bossDefeated"] != 1 || live["cycle1"] != 0x4b || live["cycle3"] != 0x4b || live["presentationPace"] != 3) {
				t.Fatal("incomplete Heart ending controls")
			}
			t.Logf("%d continuous original %s phases, %d entities, %d graphics refusals matched", spec.ticks, spec.name, count, rejected)
		})
	}
}
